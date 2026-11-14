package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Redis Pub/Sub is instance-wide (not isolated by Redis DB number).
const settingsInvalidationChannel = "sub2api:settings:updated:v1"

type settingsUpdateEvent struct {
	Type    string `json:"type"`
	Node    string `json:"node"`
	Version string `json:"version"`
	Time    int64  `json:"time"`
}

type settingsInvalidation struct {
	client           *redis.Client
	node             string
	invalidate       func()
	reconcileRuntime func()
	mu               sync.Mutex // serialize frame-origin reloads with local updates
	cancel           context.CancelFunc
	done             chan struct{}
	stopOnce         sync.Once
}

// StartFrontendInvalidation reuses the application's Redis client. The callback
// must only reload local state: receiving an event must never publish another.
// Called once during router setup, before accepting requests.
func (s *SettingService) StartFrontendInvalidation(client *redis.Client, invalidate func()) {
	bus := newSettingsInvalidationWithRuntime(client, invalidate, s.reconcileRuntimeSettings)
	s.frontendInvalidation = bus
	s.SetOnUpdateCallback(bus.localUpdate)
}

func (s *SettingService) StopFrontendInvalidation() {
	if s != nil && s.frontendInvalidation != nil {
		s.frontendInvalidation.stop()
	}
}

func newSettingsInvalidation(client *redis.Client, invalidate func()) *settingsInvalidation {
	return newSettingsInvalidationWithRuntime(client, invalidate, nil)
}

func newSettingsInvalidationWithRuntime(client *redis.Client, invalidate func(), runtime func()) *settingsInvalidation {
	ctx, cancel := context.WithCancel(context.Background())
	b := &settingsInvalidation{client: client, node: uuid.NewString(), invalidate: invalidate, reconcileRuntime: runtime, cancel: cancel, done: make(chan struct{})}
	if client == nil {
		close(b.done)
	} else {
		go b.run(ctx)
	}
	return b
}

func (b *settingsInvalidation) refresh() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.invalidate()
	if b.reconcileRuntime != nil {
		b.reconcileRuntime()
	}
}

func (b *settingsInvalidation) localUpdate() {
	// Synchronous and first: even a failed/unconfigured Redis cannot prevent the
	// writing node from invalidating its HTML before the save returns.
	b.mu.Lock()
	b.invalidate()
	b.mu.Unlock()
	b.publish("settings.updated")
}

func (b *settingsInvalidation) publish(eventType string) {
	if b.client == nil {
		return
	}
	payload, _ := json.Marshal(settingsUpdateEvent{Type: eventType, Node: b.node, Version: uuid.NewString(), Time: time.Now().UnixMilli()})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := b.client.Publish(ctx, settingsInvalidationChannel, payload).Err(); err != nil {
		slog.Warn("settings invalidation publish failed; settings remain saved", "error", err)
	}
}

func (b *settingsInvalidation) receive(payload string) {
	var event settingsUpdateEvent
	if len(payload) > 1024 || json.Unmarshal([]byte(payload), &event) != nil || (event.Type != "settings.updated" && event.Type != "websearch.updated") || event.Node == "" || event.Version == "" || event.Node == b.node {
		return
	}
	// Clearing and reloading from PostgreSQL is idempotent, including duplicates
	// and out-of-order events; never apply event-carried settings or republish.
	b.refresh()
}

func (b *settingsInvalidation) run(ctx context.Context) {
	defer close(b.done)
	sub := b.client.Subscribe(ctx, settingsInvalidationChannel)
	defer sub.Close()
	// Closing PubSub unblocks Receive even if the network is unavailable.
	go func() { <-ctx.Done(); _ = sub.Close() }()
	// Pub/Sub has no replay. Periodic reconciliation bounds stale state after a
	// failed publish or a disconnect (and does not publish anything).
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	reconcileDone := make(chan struct{})
	go func() {
		defer close(reconcileDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.refresh()
			}
		}
	}()
	defer func() { b.cancel(); <-reconcileDone }()
	for ctx.Err() == nil {
		message, err := sub.Receive(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("settings invalidation subscription unavailable; retrying", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}
		switch m := message.(type) {
		case *redis.Subscription:
			b.refresh() // initial subscribe and reconnect catch-up
		case *redis.Message:
			b.receive(m.Payload)
		}
	}
}

func (b *settingsInvalidation) stop() {
	b.stopOnce.Do(func() { b.cancel(); <-b.done })
}

// reconcileRuntimeSettings never uses a write callback, avoiding notification
// loops. Reads are independent so one failed domain cannot block the other.
func (s *SettingService) reconcileRuntimeSettings() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := s.reloadForwardedClientIPSettings(ctx); err != nil {
		slog.Warn("failed to reconcile forwarded IP settings; trust disabled", "error", err)
	}
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.rebuildWebSearchManager(ctx)
}

// Runtime reload deliberately excludes startup's legacy migration writes.
func (s *SettingService) reloadForwardedClientIPSettings(ctx context.Context) error {
	if s.cfg == nil || s.settingRepo == nil {
		return nil
	}
	s.forwardedIPRuntimeMu.Lock()
	defer s.forwardedIPRuntimeMu.Unlock()
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyAPIKeyACLTrustForwardedIP, SettingKeyForwardedClientIPHeaders})
	if err != nil {
		s.cfg.SetForwardedClientIPSettings(false, nil)
		return err
	}
	previous := s.cfg.ForwardedClientIPSettings()
	enabled, headers := previous.TrustForwardedIP, previous.Headers
	if value, ok := values[SettingKeyAPIKeyACLTrustForwardedIP]; ok {
		enabled = value == "true"
	}
	if value, ok := values[SettingKeyForwardedClientIPHeaders]; ok {
		headers, err = parseForwardedClientIPHeadersSetting(value)
		if err != nil {
			s.cfg.SetForwardedClientIPSettings(false, []string{})
			return err
		}
	}
	s.cfg.SetForwardedClientIPSettings(enabled, headers)
	return nil
}

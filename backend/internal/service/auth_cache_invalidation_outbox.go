package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
)

const (
	authInvalidationBatchSize      = 100
	authInvalidationPollInterval   = 500 * time.Millisecond
	authInvalidationLease          = 30 * time.Second
	authInvalidationRedisTimeout   = 2 * time.Second
	authInvalidationSafetyDelay    = 30 * time.Second
	authInvalidationConcurrency    = 16
	authInvalidationEventRetention = 24 * time.Hour
)

type AuthCacheInvalidationEvent struct {
	ID        int64
	CacheKey  string
	Attempts  int
	Stage     int
	CreatedAt time.Time
}

type AuthCacheInvalidationOutboxStats struct {
	Pending         int64
	OldestCreatedAt *time.Time
	MaxAttempts     int
	LastError       string
}

type AuthCacheInvalidationOutboxRepository interface {
	Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]AuthCacheInvalidationEvent, error)
	DeleteClaimed(ctx context.Context, id int64, workerID string) error
	ScheduleSecondPass(ctx context.Context, id int64, workerID string, availableAt time.Time) error
	RetryClaimed(ctx context.Context, id int64, workerID string, availableAt time.Time, lastError string) error
	Stats(ctx context.Context) (AuthCacheInvalidationOutboxStats, error)
	GetBroadcastCursor(ctx context.Context, consumerScope string) (int64, error)
	ListBroadcastEvents(ctx context.Context, afterID int64, limit int) ([]AuthCacheInvalidationEvent, error)
	AdvanceBroadcastCursor(ctx context.Context, consumerScope string, eventID int64) error
	DeleteBroadcastEventsBefore(ctx context.Context, cutoff time.Time) error
}

type AuthCacheInvalidationHealth struct {
	Running    bool          `json:"running"`
	Processed  uint64        `json:"processed"`
	Failures   uint64        `json:"failures"`
	Pending    int64         `json:"pending"`
	OldestLag  time.Duration `json:"oldest_lag"`
	LastError  string        `json:"last_error,omitempty"`
	StatsError string        `json:"stats_error,omitempty"`
	// HealthySLA includes the delayed safety pass. RecoverySLA is the maximum
	// convergence time after Redis becomes healthy, including capped backoff.
	HealthySLA  time.Duration `json:"healthy_sla"`
	RecoverySLA time.Duration `json:"recovery_sla"`
	MaxAttempts int           `json:"max_attempts"`
}

type OpsAuthCacheInvalidationHealth struct {
	Outbox       AuthCacheInvalidationHealth           `json:"outbox"`
	Subscriber   AuthCacheInvalidationSubscriberHealth `json:"subscriber"`
	Lookup       APIKeyAuthLookupMetrics               `json:"lookup"`
	InvalidAbuse InvalidAuthAbuseHealth                `json:"invalid_abuse"`
}

func (s *OpsService) GetAuthCacheInvalidationHealth(ctx context.Context) OpsAuthCacheInvalidationHealth {
	if s == nil {
		return OpsAuthCacheInvalidationHealth{}
	}
	health := OpsAuthCacheInvalidationHealth{}
	if s.authCacheInvalidationWorker != nil {
		health.Outbox = s.authCacheInvalidationWorker.Health(ctx)
	}
	if s.apiKeyService != nil {
		health.Subscriber = s.apiKeyService.AuthCacheInvalidationSubscriberHealth()
		health.Lookup = s.apiKeyService.AuthLookupMetrics()
		health.InvalidAbuse = s.apiKeyService.InvalidAuthAbuseHealth()
	}
	return health
}

type AuthCacheInvalidationWorker struct {
	repo                 AuthCacheInvalidationOutboxRepository
	cache                APIKeyCache
	local                *APIKeyService
	workerID             string
	consumerScope        string
	ctx                  context.Context
	cancel               context.CancelFunc
	wg                   sync.WaitGroup
	start                sync.Once
	stop                 sync.Once
	running              atomic.Bool
	processed            atomic.Uint64
	failures             atomic.Uint64
	lastError            atomic.Value
	lastBroadcastCleanup atomic.Int64
}

func NewAuthCacheInvalidationWorker(repo AuthCacheInvalidationOutboxRepository, cache APIKeyCache, local ...*APIKeyService) *AuthCacheInvalidationWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &AuthCacheInvalidationWorker{
		repo: repo, cache: cache, workerID: uuid.NewString(), consumerScope: "default", ctx: ctx, cancel: cancel,
	}
	if len(local) > 0 {
		w.local = local[0]
	}
	w.lastError.Store("")
	return w
}

func (w *AuthCacheInvalidationWorker) SetConsumerScope(scope string) {
	if w == nil {
		return
	}
	if scope = strings.TrimSpace(scope); scope != "" {
		w.consumerScope = scope
	}
}

func (w *AuthCacheInvalidationWorker) Start() {
	if w == nil || w.repo == nil || w.cache == nil {
		return
	}
	w.start.Do(func() {
		w.running.Store(true)
		w.wg.Add(1)
		go w.run()
	})
}

func (w *AuthCacheInvalidationWorker) Stop() {
	if w == nil {
		return
	}
	w.stop.Do(func() {
		w.cancel()
		w.wg.Wait()
		w.running.Store(false)
	})
}

func (w *AuthCacheInvalidationWorker) run() {
	defer w.wg.Done()
	defer w.running.Store(false)
	ticker := time.NewTicker(authInvalidationPollInterval)
	defer ticker.Stop()
	for {
		if err := w.processBroadcastBatch(w.ctx); err != nil && w.ctx.Err() == nil {
			w.recordFailure(err)
		}
		if err := w.processBatch(w.ctx); err != nil && w.ctx.Err() == nil {
			w.recordFailure(err)
		}
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *AuthCacheInvalidationWorker) processBroadcastBatch(ctx context.Context) error {
	cursor, err := w.repo.GetBroadcastCursor(ctx, w.consumerScope)
	if err != nil {
		return fmt.Errorf("load auth invalidation cursor for %s: %w", w.consumerScope, err)
	}
	events, err := w.repo.ListBroadcastEvents(ctx, cursor, authInvalidationBatchSize)
	if err != nil {
		return fmt.Errorf("list auth invalidation events for %s: %w", w.consumerScope, err)
	}
	for _, event := range events {
		if w.local != nil {
			w.local.invalidateLocalAuthCache(event.CacheKey)
		}
		invalidateCtx, cancel := context.WithTimeout(ctx, authInvalidationRedisTimeout)
		err = w.cache.DeleteAuthCache(invalidateCtx, event.CacheKey)
		if err == nil {
			err = w.cache.PublishAuthCacheInvalidation(invalidateCtx, event.CacheKey)
		}
		cancel()
		if err != nil {
			return fmt.Errorf("invalidate auth cache event %d for %s: %w", event.ID, w.consumerScope, err)
		}
		if err := w.repo.AdvanceBroadcastCursor(ctx, w.consumerScope, event.ID); err != nil {
			return fmt.Errorf("advance auth invalidation cursor for %s: %w", w.consumerScope, err)
		}
		w.processed.Add(1)
		cursor = event.ID
	}

	// Every worker may attempt this bounded retention cleanup; the timestamp keeps
	// it to at most once per process per hour and the SQL is idempotent.
	now := time.Now().UTC()
	lastCleanup := time.Unix(w.lastBroadcastCleanup.Load(), 0)
	if now.Sub(lastCleanup) >= time.Hour && w.lastBroadcastCleanup.CompareAndSwap(lastCleanup.Unix(), now.Unix()) {
		if err := w.repo.DeleteBroadcastEventsBefore(ctx, now.Add(-authInvalidationEventRetention)); err != nil {
			return fmt.Errorf("clean old auth invalidation events: %w", err)
		}
	}
	return nil
}

func (w *AuthCacheInvalidationWorker) processBatch(ctx context.Context) error {
	events, err := w.repo.Claim(ctx, w.workerID, authInvalidationBatchSize, authInvalidationLease)
	if err != nil {
		return fmt.Errorf("claim auth cache invalidations: %w", err)
	}
	semaphore := make(chan struct{}, authInvalidationConcurrency)
	var wg sync.WaitGroup
	for i := range events {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case semaphore <- struct{}{}:
		}
		wg.Add(1)
		go func(event AuthCacheInvalidationEvent) {
			defer wg.Done()
			defer func() { <-semaphore }()
			w.processEvent(ctx, event)
		}(events[i])
	}
	wg.Wait()
	return nil
}

func (w *AuthCacheInvalidationWorker) processEvent(parent context.Context, event AuthCacheInvalidationEvent) {
	if w.local != nil {
		w.local.invalidateLocalAuthCache(event.CacheKey)
	}
	ctx, cancel := context.WithTimeout(parent, authInvalidationRedisTimeout)
	err := w.cache.DeleteAuthCache(ctx, event.CacheKey)
	if err == nil {
		err = w.cache.PublishAuthCacheInvalidation(ctx, event.CacheKey)
	}
	cancel()
	if err != nil {
		w.recordFailure(err)
		retryAt := time.Now().UTC().Add(authInvalidationRetryDelay(event.Attempts + 1))
		retryCtx, retryCancel := context.WithTimeout(context.Background(), 2*time.Second)
		retryErr := w.repo.RetryClaimed(retryCtx, event.ID, w.workerID, retryAt, boundedAuthInvalidationError(err))
		retryCancel()
		if retryErr != nil {
			w.recordFailure(fmt.Errorf("release failed auth invalidation %d: %w", event.ID, retryErr))
		}
		return
	}
	if event.Stage == 0 {
		nextCtx, nextCancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = w.repo.ScheduleSecondPass(nextCtx, event.ID, w.workerID, time.Now().UTC().Add(authInvalidationSafetyDelay))
		nextCancel()
		if err != nil {
			w.recordFailure(fmt.Errorf("schedule second auth invalidation pass %d: %w", event.ID, err))
			return
		}
		w.processed.Add(1)
		w.lastError.Store("")
		return
	}

	ackCtx, ackCancel := context.WithTimeout(context.Background(), 2*time.Second)
	err = w.repo.DeleteClaimed(ackCtx, event.ID, w.workerID)
	ackCancel()
	if err != nil {
		w.recordFailure(fmt.Errorf("ack auth invalidation %d: %w", event.ID, err))
		return
	}
	w.processed.Add(1)
	w.lastError.Store("")
}

func authInvalidationRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 9 {
		attempt = 9
	}
	base := time.Second * time.Duration(1<<(attempt-1))
	return time.Duration(float64(base) * (0.8 + rand.Float64()*0.4))
}

func boundedAuthInvalidationError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 1024 {
		return message[:1024]
	}
	return message
}

func (w *AuthCacheInvalidationWorker) recordFailure(err error) {
	if err == nil {
		return
	}
	w.failures.Add(1)
	w.lastError.Store(boundedAuthInvalidationError(err))
	slog.Warn("auth cache invalidation outbox processing failed", "error", err)
}

func (w *AuthCacheInvalidationWorker) Health(ctx context.Context) AuthCacheInvalidationHealth {
	health := AuthCacheInvalidationHealth{
		HealthySLA:  authInvalidationSafetyDelay + 5*time.Second,
		RecoverySLA: 6 * time.Minute,
	}
	if w == nil {
		return health
	}
	health.Running = w.running.Load()
	health.Processed = w.processed.Load()
	health.Failures = w.failures.Load()
	if value := w.lastError.Load(); value != nil {
		health.LastError, _ = value.(string)
	}
	if w.repo == nil {
		return health
	}
	stats, err := w.repo.Stats(ctx)
	if err != nil {
		health.StatsError = boundedAuthInvalidationError(err)
		return health
	}
	health.Pending = stats.Pending
	health.MaxAttempts = stats.MaxAttempts
	if health.LastError == "" {
		health.LastError = stats.LastError
	}
	if stats.OldestCreatedAt != nil {
		health.OldestLag = time.Since(*stats.OldestCreatedAt)
		if health.OldestLag < 0 {
			health.OldestLag = 0
		}
	}
	return health
}

func authCacheInvalidationScope(cfg *config.Config) string {
	if cfg == nil {
		return "default"
	}
	if explicit := strings.TrimSpace(cfg.APIKeyAuth.InvalidationScope); explicit != "" {
		return explicit
	}
	identity := strings.ToLower(strings.TrimSpace(cfg.Redis.Host)) + ":" + strconv.Itoa(cfg.Redis.Port) +
		"/" + strconv.Itoa(cfg.Redis.DB) + "/" + strings.ToLower(strings.TrimSpace(cfg.Redis.Username)) +
		"/tls=" + strconv.FormatBool(cfg.Redis.EnableTLS)
	sum := sha256.Sum256([]byte(identity))
	return "redis-" + hex.EncodeToString(sum[:16])
}

func ProvideAuthCacheInvalidationWorker(repo AuthCacheInvalidationOutboxRepository, cache APIKeyCache, apiKeyService *APIKeyService, cfg *config.Config) *AuthCacheInvalidationWorker {
	worker := NewAuthCacheInvalidationWorker(repo, cache, apiKeyService)
	worker.SetConsumerScope(authCacheInvalidationScope(cfg))
	worker.Start()
	return worker
}

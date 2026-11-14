package service

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const cacheReconcileInterval = 30 * time.Second
const cacheReconcileTimeout = 5 * time.Second

// ManagedCacheSubscriber is implemented by Redis repositories. Legacy adapters
// may still implement SubscribeUpdates; production uses this joinable lifecycle.
type ManagedCacheSubscriber interface {
	SubscribeUpdatesManaged(context.Context, func()) func()
}

type cacheReconciler struct {
	cancel           context.CancelFunc
	done             chan struct{}
	stopSubscription func()
	once             sync.Once
}

// Events are coalesced. Both notifications and the periodic fallback reload
// directly from DB, so even stale Redis L1/L2 data cannot undo reconciliation.
func startCacheReconciler(name string, cache interface{ SubscribeUpdates(context.Context, func()) }, reload func(context.Context) error, interval time.Duration) *cacheReconciler {
	ctx, cancel := context.WithCancel(context.Background())
	w := &cacheReconciler{cancel: cancel, done: make(chan struct{})}
	wake := make(chan struct{}, 1)
	notify := func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	if managed, ok := cache.(ManagedCacheSubscriber); ok {
		w.stopSubscription = managed.SubscribeUpdatesManaged(ctx, notify)
	} else {
		cache.SubscribeUpdates(ctx, notify)
	}
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-wake:
			case <-ticker.C:
			}
			if ctx.Err() != nil {
				return
			}
			loadCtx, loadCancel := context.WithTimeout(ctx, cacheReconcileTimeout)
			if err := reload(loadCtx); err != nil && ctx.Err() == nil {
				slog.Warn("cache reconciliation failed; will retry", "cache", name, "error", err)
			}
			loadCancel()
		}
	}()
	return w
}

func (w *cacheReconciler) stop() {
	if w != nil {
		w.once.Do(func() {
			w.cancel()
			if w.stopSubscription != nil {
				w.stopSubscription()
			}
			<-w.done
		})
	}
}

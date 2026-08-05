package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const globalBackgroundTaskDisabledOwner = "__sub2api_global_background_tasks_disabled__"

// LeaderLockCache provides cross-instance mutual exclusion for periodic background
// jobs. It is implemented in the repository layer (Redis-backed) so the service
// layer never depends on Redis directly. Release is a compare-and-delete keyed by
// owner so a stale holder can never delete a peer's lock.
type LeaderLockCache interface {
	// TryAcquireLeaderLock sets key=owner with the given TTL iff key is absent.
	// It returns true when the caller becomes the owner.
	TryAcquireLeaderLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
	// ReleaseLeaderLock deletes key iff it is still owned by owner.
	ReleaseLeaderLock(ctx context.Context, key, owner string) error
}

// tryAcquireSingletonLeaderLock provides best-effort single-flight execution of a
// periodic background job across multiple instances. PostgreSQL is authoritative
// whenever it is configured because deployments may use one Redis per region while
// sharing a single database. A database error fails closed; falling back to a
// regional Redis would allow every region to become leader.
//
// Semantics:
//   - acquired      -> returns a non-nil release func and true; callers should
//     defer the release once the job finishes.
//   - held by peer  -> returns (nil, false); callers should skip this cycle.
//   - no backend    -> when neither the cache nor a DB is configured (e.g. unit
//     tests, or a single-instance deployment without Redis) it runs without
//     gating, returning a no-op release and true, so the job is never silently
//     starved.
//
// With PostgreSQL, leadership stays pinned to the winning service instance so
// staggered regional tickers cannot execute sequentially. The returned release is
// a no-op and shutdown releases all persistent advisory locks. Redis-only fallback
// retains the per-cycle TTL and compare-and-delete release behavior.
func tryAcquireSingletonLeaderLock(ctx context.Context, cache LeaderLockCache, db *sql.DB, key, owner string, ttl time.Duration) (func(), bool) {
	release, acquired, _ := tryAcquireSingletonLeaderLockWithError(ctx, cache, db, key, owner, ttl)
	return release, acquired
}

func tryAcquireSingletonLeaderLockWithError(ctx context.Context, cache LeaderLockCache, db *sql.DB, key, owner string, ttl time.Duration) (func(), bool, error) {
	if !backgroundTaskLeaderEligible(owner) {
		return nil, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if db != nil {
		return tryAcquirePersistentDBAdvisoryLock(ctx, db, hashAdvisoryLockID(key), owner)
	}

	if cache != nil {
		ok, err := cache.TryAcquireLeaderLock(ctx, key, owner, ttl)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, nil
		}
		release := func() {
			ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = cache.ReleaseLeaderLock(ctx2, key, owner)
		}
		return release, true, nil
	}

	// No coordination backend available: run without gating.
	return func() {}, true, nil
}

func backgroundTaskLeaderEligible(owner string) bool {
	return owner != globalBackgroundTaskDisabledOwner
}

func applyGlobalBackgroundTaskEligibility(instanceID *string, cfg *config.Config) {
	if instanceID != nil && cfg != nil && cfg.GlobalBackgroundTasks.Disabled {
		*instanceID = globalBackgroundTaskDisabledOwner
	}
}

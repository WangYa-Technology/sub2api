package service

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// fakeLeaderLockCache is an in-memory LeaderLockCache for unit tests. It models the
// compare-and-delete release semantics of the real Redis-backed implementation.
type fakeLeaderLockCache struct {
	mu           sync.Mutex
	owners       map[string]string
	acquireErr   error
	acquireCalls int
}

func (f *fakeLeaderLockCache) TryAcquireLeaderLock(_ context.Context, key, owner string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	f.acquireCalls++
	f.mu.Unlock()
	if f.acquireErr != nil {
		return false, f.acquireErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.owners == nil {
		f.owners = map[string]string{}
	}
	if _, held := f.owners[key]; held {
		return false, nil
	}
	f.owners[key] = owner
	return true, nil
}

func (f *fakeLeaderLockCache) ReleaseLeaderLock(_ context.Context, key, owner string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.owners[key] == owner {
		delete(f.owners, key)
	}
	return nil
}

func (f *fakeLeaderLockCache) heldBy(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.owners[key]
}

func TestTryAcquireSingletonLeaderLock_NoBackendRunsUngated(t *testing.T) {
	release, ok := tryAcquireSingletonLeaderLock(context.Background(), nil, nil, "k", "inst", time.Minute)
	require.True(t, ok)
	require.NotNil(t, release)
	require.NotPanics(t, release)
}

func TestTryAcquireSingletonLeaderLock_ContendedThenReleased(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	ctx := context.Background()
	const key = "leader:test:contended"

	releaseA, ok := tryAcquireSingletonLeaderLock(ctx, cache, nil, key, "A", time.Minute)
	require.True(t, ok, "first instance should acquire")
	require.Equal(t, "A", cache.heldBy(key))

	_, okB := tryAcquireSingletonLeaderLock(ctx, cache, nil, key, "B", time.Minute)
	require.False(t, okB, "peer must be locked out while the lock is held")

	releaseA()
	require.Empty(t, cache.heldBy(key), "release must free the lock")

	releaseB, okB := tryAcquireSingletonLeaderLock(ctx, cache, nil, key, "B", time.Minute)
	require.True(t, okB, "peer should acquire after the holder releases")
	releaseB()
}

func TestTryAcquireSingletonLeaderLock_CacheErrorFailsClosed(t *testing.T) {
	cache := &fakeLeaderLockCache{acquireErr: context.DeadlineExceeded}
	release, ok := tryAcquireSingletonLeaderLock(context.Background(), cache, nil, "k", "inst", time.Minute)
	require.False(t, ok)
	require.Nil(t, release)
}

func TestTryAcquireSingletonLeaderLock_DatabaseTakesPriorityOverRegionalRedis(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() {
		ReleasePersistentLeaderLocks(db)
		_ = db.Close()
	})

	const key = "leader:shared-db"
	lockID := hashAdvisoryLockID(key)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT pg_try_advisory_lock($1)")).
		WithArgs(lockID).
		WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))
	cache := &fakeLeaderLockCache{acquireErr: errors.New("regional redis must not be consulted")}
	release, ok := tryAcquireSingletonLeaderLock(context.Background(), cache, db, key, "instance", time.Minute)
	require.True(t, ok)
	require.NotNil(t, release)
	release()
	require.Zero(t, cache.acquireCalls)

	mock.ExpectPing()
	release, ok = tryAcquireSingletonLeaderLock(context.Background(), cache, db, key, "instance", time.Minute)
	require.True(t, ok, "the elected instance must retain leadership across cycles")
	release()
	_, ok = tryAcquireSingletonLeaderLock(context.Background(), cache, db, key, "peer", time.Minute)
	require.False(t, ok, "a peer must remain excluded after one task cycle finishes")

	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_unlock_all()")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	ReleasePersistentLeaderLocks(db)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT pg_try_advisory_lock($1)")).
		WithArgs(lockID).
		WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))
	release, ok = tryAcquireSingletonLeaderLock(context.Background(), cache, db, key, "peer", time.Minute)
	require.True(t, ok, "a peer must take over after the old process releases leadership")
	release()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_unlock_all()")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	ReleasePersistentLeaderLocks(db)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTryAcquireSingletonLeaderLock_DatabaseErrorDoesNotFallBackToRegionalRedis(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	const key = "leader:db-error"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT pg_try_advisory_lock($1)")).
		WithArgs(hashAdvisoryLockID(key)).
		WillReturnError(errors.New("database unavailable"))

	cache := &fakeLeaderLockCache{}
	release, ok, lockErr := tryAcquireSingletonLeaderLockWithError(context.Background(), cache, db, key, "instance", time.Minute)
	require.Error(t, lockErr)
	require.False(t, ok)
	require.Nil(t, release)
	require.Zero(t, cache.acquireCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTryAcquireSingletonLeaderLock_IneligibleInstanceSkipsAllBackends(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	release, acquired, err := tryAcquireSingletonLeaderLockWithError(context.Background(), cache, nil, "job", globalBackgroundTaskDisabledOwner, time.Minute)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Nil(t, release)
	require.Zero(t, cache.acquireCalls)
}

func TestSubscriptionExpiryService_ReminderSkipsScanWhenNotLeader(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	// A peer already holds the reminder leader lock.
	_, _ = cache.TryAcquireLeaderLock(context.Background(), subscriptionExpiryReminderLeaderLockKey, "peer", time.Minute)

	repo := &subscriptionExpiryRepoStub{}
	settingRepo := &subscriptionExpirySettingRepoStub{values: map[string]string{SettingKeySMTPHost: "smtp.example.com"}}
	svc := NewSubscriptionExpiryService(repo, time.Minute)
	svc.SetSettingRepository(settingRepo)
	svc.SetNotificationEmailService(NewNotificationEmailService(settingRepo, NewEmailService(settingRepo, nil)))
	svc.SetLeaderLock(cache, nil)

	svc.sendExpiryReminders(context.Background())

	require.Zero(t, repo.listCalls, "non-leader must not scan active subscriptions")
}

func TestSubscriptionExpiryService_ReminderScansWhenLeader(t *testing.T) {
	repo := &subscriptionExpiryRepoStub{}
	settingRepo := &subscriptionExpirySettingRepoStub{values: map[string]string{SettingKeySMTPHost: "smtp.example.com"}}
	svc := NewSubscriptionExpiryService(repo, time.Minute)
	svc.SetSettingRepository(settingRepo)
	svc.SetNotificationEmailService(NewNotificationEmailService(settingRepo, NewEmailService(settingRepo, nil)))
	svc.SetLeaderLock(&fakeLeaderLockCache{}, nil)

	svc.sendExpiryReminders(context.Background())

	require.Equal(t, 1, repo.listCalls, "leader should scan active subscriptions once")
}

// Single-instance correctness: the lock is released at the end of each cycle, so
// the same instance must re-acquire it and run on every subsequent cycle (no
// self-lockout). Covers both the cache-backed path and the no-backend path.
func TestSubscriptionExpiryService_ReminderRunsEveryCycleSingleInstance(t *testing.T) {
	cases := map[string]LeaderLockCache{
		"with_cache": &fakeLeaderLockCache{},
		"no_backend": nil,
	}
	for name, cache := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &subscriptionExpiryRepoStub{}
			settingRepo := &subscriptionExpirySettingRepoStub{values: map[string]string{SettingKeySMTPHost: "smtp.example.com"}}
			svc := NewSubscriptionExpiryService(repo, time.Minute)
			svc.SetSettingRepository(settingRepo)
			svc.SetNotificationEmailService(NewNotificationEmailService(settingRepo, NewEmailService(settingRepo, nil)))
			svc.SetLeaderLock(cache, nil)

			// Three consecutive cycles, mimicking the ticker loop.
			svc.sendExpiryReminders(context.Background())
			svc.sendExpiryReminders(context.Background())
			svc.sendExpiryReminders(context.Background())

			require.Equal(t, 3, repo.listCalls, "single instance must run every cycle")
		})
	}
}

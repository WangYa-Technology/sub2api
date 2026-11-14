package repository

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type ruleReconcileRepo struct {
	service.ErrorPassthroughRepository
	mu    sync.Mutex
	id    int64
	err   error
	reads atomic.Int64
}

func (r *ruleReconcileRepo) List(context.Context) ([]*model.ErrorPassthroughRule, error) {
	r.reads.Add(1)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	return []*model.ErrorPassthroughRule{{ID: r.id, Name: "rule", Enabled: true, ErrorCodes: []int{503}, MatchMode: model.MatchModeAny, PassthroughBody: true, PassthroughCode: true}}, nil
}
func (r *ruleReconcileRepo) set(id int64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.id = id
	r.err = err
}

type profileReconcileRepo struct {
	service.TLSFingerprintProfileRepository
	mu    sync.Mutex
	name  string
	err   error
	reads atomic.Int64
}

func (r *profileReconcileRepo) List(context.Context) ([]*model.TLSFingerprintProfile, error) {
	r.reads.Add(1)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	return []*model.TLSFingerprintProfile{{ID: 1, Name: r.name}}, nil
}
func (r *profileReconcileRepo) set(name string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.name = name
	r.err = err
}

// Production repositories + both service layers: reconnect must bypass stale
// Redis data, and a failed refresh must heal without a second notification.
func TestCacheReconciliation_RulesAndProfiles(t *testing.T) {
	for _, kind := range []string{"rules", "profiles"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			r := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: r.Addr()})
			defer client.Close()
			var channel string
			var set func(int64, error)
			var read func() int64
			var reads func() int64
			var notify func(context.Context) error
			var stopA, stopB func()
			if kind == "rules" {
				repo := &ruleReconcileRepo{id: 1}
				aCache, bCache := NewErrorPassthroughCache(client), NewErrorPassthroughCache(client)
				a, b := service.NewErrorPassthroughService(repo, aCache), service.NewErrorPassthroughService(repo, bCache)
				stopA, stopB = a.Stop, b.Stop
				set = repo.set
				reads = repo.reads.Load
				notify = aCache.NotifyUpdate
				channel = errorPassthroughPubSubKey
				read = func() int64 {
					m := b.MatchRule("openai", 503, []byte(`{}`))
					if m == nil {
						return 0
					}
					return m.ID
				}
			} else {
				repo := &profileReconcileRepo{name: "old"}
				aCache, bCache := NewTLSFingerprintProfileCache(client), NewTLSFingerprintProfileCache(client)
				a, b := service.NewTLSFingerprintProfileService(repo, aCache), service.NewTLSFingerprintProfileService(repo, bCache)
				stopA, stopB = a.Stop, b.Stop
				set = func(id int64, err error) {
					name := map[int64]string{1: "old", 2: "updated", 3: "reconnected", 4: "recovered"}[id]
					repo.set(name, err)
				}
				reads = repo.reads.Load
				notify = aCache.NotifyUpdate
				channel = tlsFPProfilePubSubKey
				read = func() int64 {
					p := b.GetProfileByID(1)
					if p == nil {
						return 0
					}
					return map[string]int64{"old": 1, "updated": 2, "reconnected": 3, "recovered": 4}[p.Name]
				}
			}
			defer stopA()
			defer stopB()
			require.Eventually(t, func() bool { return r.PubSubNumSub(channel)[channel] == 2 && reads() >= 4 }, 3*time.Second, 10*time.Millisecond)
			require.Equal(t, int64(1), read())
			// Simulate failed DEL with an old Redis L2 snapshot still present. A remote
			// event must read PostgreSQL rather than returning this old snapshot.
			set(2, nil)
			require.NoError(t, notify(ctx))
			require.Eventually(t, func() bool { return read() == 2 }, 3*time.Second, 10*time.Millisecond)
			// Stop Redis, change DB, reconnect without publishing the lost notification.
			r.Close()
			set(3, nil)
			require.NoError(t, r.Restart())
			require.Eventually(t, func() bool { return read() == 3 }, 8*time.Second, 20*time.Millisecond)
			// Publish while DB reads fail. No further events after recovery: periodic
			// reconciliation must repair both local and stale Redis snapshots.
			set(4, errors.New("temporary DB read failure"))
			before := reads()
			require.NoError(t, notify(ctx))
			require.Eventually(t, func() bool { return reads() > before+1 }, 3*time.Second, 10*time.Millisecond)
			set(4, nil)
			require.Eventually(t, func() bool { return read() == 4 }, 35*time.Second, 20*time.Millisecond)
			require.NoError(t, notify(ctx))
			require.NoError(t, notify(ctx))
			stopB()
			stopB()
			stopA()
			require.Eventually(t, func() bool { return r.PubSubNumSub(channel)[channel] == 0 }, time.Second, 10*time.Millisecond)
		})
	}
}

func TestCacheSubscription_CancelUnavailable(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 50 * time.Millisecond, MaxRetries: -1})
	defer client.Close()
	stop := subscribeCacheUpdates(context.Background(), client, "test", func() {})
	done := make(chan struct{})
	go func() { stop(); stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("subscription shutdown blocked")
	}
}

func TestCacheReconciliation_NoRedisClient(t *testing.T) {
	ruleRepo := &ruleReconcileRepo{id: 1}
	rules := service.NewErrorPassthroughService(ruleRepo, NewErrorPassthroughCache(nil))
	defer rules.Stop()
	require.NotNil(t, rules.MatchRule("openai", 503, []byte(`{}`)))
	profileRepo := &profileReconcileRepo{name: "local"}
	profiles := service.NewTLSFingerprintProfileService(profileRepo, NewTLSFingerprintProfileCache(nil))
	defer profiles.Stop()
	require.Equal(t, "local", profiles.GetProfileByID(1).Name)
}

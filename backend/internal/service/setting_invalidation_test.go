//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestSettingsInvalidation_SaveLocalAndPublishFailure(t *testing.T) {
	for _, offline := range []bool{false, true} {
		t.Run(map[bool]string{false: "no Redis", true: "publish fails"}[offline], func(t *testing.T) {
			repo := &settingUpdateRepoStub{}
			s := NewSettingService(repo, &config.Config{})
			var client *redis.Client
			if offline {
				client = redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
				require.NoError(t, client.Close())
			}
			var cleared bool
			s.StartFrontendInvalidation(client, func() {
				require.Equal(t, "true", repo.updates[SettingKeyCompactHomeEnabled], "invalidation must follow database write")
				cleared = true
			})
			defer s.StopFrontendInvalidation()
			require.NoError(t, s.UpdateSettings(context.Background(), &SystemSettings{CompactHomeEnabled: true}))
			require.True(t, cleared, "local cache cleared before save returns")
			require.Equal(t, "true", repo.updates[SettingKeyCompactHomeEnabled])
			s.StopFrontendInvalidation()
		})
	}
}

func TestSettingsInvalidation_FailedSaveDoesNotNotify(t *testing.T) {
	repo := &settingUpdateRepoStub{setMultipleErr: errors.New("database unavailable")}
	s := NewSettingService(repo, &config.Config{})
	var calls int
	s.StartFrontendInvalidation(nil, func() { calls++ })
	defer s.StopFrontendInvalidation()
	require.Error(t, s.UpdateSettings(context.Background(), &SystemSettings{}))
	require.Zero(t, calls)
}

func TestSettingsInvalidation_TwoNodesAndReconnect(t *testing.T) {
	r := miniredis.RunT(t)
	aClient := redis.NewClient(&redis.Options{Addr: r.Addr()})
	bClient := redis.NewClient(&redis.Options{Addr: r.Addr()})
	defer aClient.Close()
	defer bClient.Close()
	var aCalls, bCalls atomic.Int32
	a := newSettingsInvalidation(aClient, func() { aCalls.Add(1) })
	b := newSettingsInvalidation(bClient, func() { bCalls.Add(1) })
	defer a.stop()
	defer b.stop()
	require.Eventually(t, func() bool { return aCalls.Load() > 0 && bCalls.Load() > 0 }, 3*time.Second, 10*time.Millisecond)
	beforeA, beforeB := aCalls.Load(), bCalls.Load()
	a.localUpdate()
	require.Equal(t, beforeA+1, aCalls.Load())
	require.Eventually(t, func() bool { return bCalls.Load() > beforeB }, 3*time.Second, 10*time.Millisecond)
	payload, _ := json.Marshal(settingsUpdateEvent{Type: "settings.updated", Node: a.node, Version: "same-event", Time: 1})
	beforeB = bCalls.Load()
	b.receive(string(payload))
	b.receive(string(payload))
	require.Equal(t, beforeB+2, bCalls.Load())
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, beforeA+1, aCalls.Load(), "no self delivery or notification loop")
	beforeB = bCalls.Load()
	b.receive(`{"type":"other","node":"x","version":"x"}`)
	b.receive(`invalid`)
	b.receive(string(make([]byte, 1025)))
	require.Equal(t, beforeB, bCalls.Load())
	// Restart only the isolated Redis fixture, never an application service.
	addr := r.Addr()
	r.Close()
	require.NoError(t, r.Restart())
	require.Equal(t, addr, r.Addr())
	require.Eventually(t, func() bool { return bCalls.Load() > beforeB }, 8*time.Second, 20*time.Millisecond)
	beforeB = bCalls.Load()
	a.localUpdate()
	require.Eventually(t, func() bool { return bCalls.Load() > beforeB }, 3*time.Second, 10*time.Millisecond)
	b.stop()
	select {
	case <-b.done:
	default:
		t.Fatal("subscriber did not release resources")
	}
}

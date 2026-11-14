//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type runtimeSettingsRepo struct {
	SettingRepository
	mu                sync.Mutex
	values            map[string]string
	readErr, writeErr error
	allErr            error
}

func (r *runtimeSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.readErr != nil {
		return "", r.readErr
	}
	v, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}
func (r *runtimeSettingsRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.readErr != nil {
		return nil, r.readErr
	}
	m := map[string]string{}
	for _, k := range keys {
		if v, ok := r.values[k]; ok {
			m[k] = v
		}
	}
	return m, nil
}
func (r *runtimeSettingsRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writeErr != nil {
		return r.writeErr
	}
	r.values[key] = value
	return nil
}
func (r *runtimeSettingsRepo) SetMultiple(_ context.Context, values map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writeErr != nil {
		return r.writeErr
	}
	for k, v := range values {
		r.values[k] = v
	}
	return nil
}

type runtimeBuilderRecorder struct {
	mu      sync.Mutex
	cfg     *WebSearchEmulationConfig
	calls   int
	proxies map[int64]string
}

func (r *runtimeBuilderRecorder) build(cfg *WebSearchEmulationConfig, proxies map[int64]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	raw, _ := json.Marshal(cfg)
	r.cfg = &WebSearchEmulationConfig{}
	_ = json.Unmarshal(raw, r.cfg)
	r.calls++
	r.proxies = proxies
}
func (r *runtimeBuilderRecorder) key() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cfg == nil || !r.cfg.Enabled || len(r.cfg.Providers) == 0 {
		return ""
	}
	return r.cfg.Providers[0].APIKey
}

func TestSettingsRuntime_TwoNodesIPAndWebSearch(t *testing.T) {
	ctx := context.Background()
	repo := &runtimeSettingsRepo{values: map[string]string{SettingKeyAPIKeyACLTrustForwardedIP: "false", SettingKeyForwardedClientIPHeaders: `["X-Old-IP"]`}}
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer client.Close()
	aCfg, bCfg := &config.Config{}, &config.Config{}
	aCfg.SetForwardedClientIPSettings(false, []string{"X-Old-IP"})
	bCfg.SetForwardedClientIPSettings(false, []string{"X-Old-IP"})
	a, b := NewSettingService(repo, aCfg), NewSettingService(repo, bCfg)
	aBuilder, bBuilder := &runtimeBuilderRecorder{}, &runtimeBuilderRecorder{}
	a.SetWebSearchManagerBuilder(ctx, aBuilder.build)
	b.SetWebSearchManagerBuilder(ctx, bBuilder.build)
	a.StartFrontendInvalidation(client, func() {})
	b.StartFrontendInvalidation(client, func() {})
	defer a.StopFrontendInvalidation()
	defer b.StopFrontendInvalidation()
	require.Eventually(t, func() bool {
		return redisServer.PubSubNumSub(settingsInvalidationChannel)[settingsInvalidationChannel] == 2
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, a.UpdateSettings(ctx, &SystemSettings{APIKeyACLTrustForwardedIP: true, ForwardedClientIPHeaders: []string{"X-New-IP"}}))
	require.Eventually(t, func() bool { return bCfg.ForwardedClientIPSettings().TrustForwardedIP }, 3*time.Second, 10*time.Millisecond)
	require.Equal(t, []string{"X-New-Ip"}, bCfg.ForwardedClientIPSettings().Headers)
	require.NoError(t, a.UpdateSettings(ctx, &SystemSettings{APIKeyACLTrustForwardedIP: false, ForwardedClientIPHeaders: []string{"X-Final-IP"}}))
	require.Eventually(t, func() bool {
		v := bCfg.ForwardedClientIPSettings()
		return !v.TrustForwardedIP && len(v.Headers) == 1 && v.Headers[0] == "X-Final-Ip"
	}, 3*time.Second, 10*time.Millisecond)
	// This separate settings write must publish even though it bypasses onUpdate.
	observer := client.Subscribe(ctx, settingsInvalidationChannel)
	defer observer.Close()
	_, err := observer.Receive(ctx)
	require.NoError(t, err)
	require.NoError(t, a.SaveWebSearchEmulationConfig(ctx, &WebSearchEmulationConfig{Enabled: true, Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "test-key-new"}}}))
	require.Eventually(t, func() bool { return bBuilder.key() == "test-key-new" }, 3*time.Second, 10*time.Millisecond)
	msg, err := observer.ReceiveMessage(ctx)
	require.NoError(t, err)
	require.NotContains(t, msg.Payload, "test-key-new")
	require.NotContains(t, msg.Payload, "providers")
	var ev settingsUpdateEvent
	require.NoError(t, json.Unmarshal([]byte(msg.Payload), &ev))
	require.Equal(t, "websearch.updated", ev.Type)
	require.NoError(t, a.SaveWebSearchEmulationConfig(ctx, &WebSearchEmulationConfig{Enabled: true, Providers: []WebSearchProviderConfig{{Type: "tavily", APIKey: "test-replacement"}}}))
	require.Eventually(t, func() bool { return bBuilder.key() == "test-replacement" }, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, a.SaveWebSearchEmulationConfig(ctx, &WebSearchEmulationConfig{}))
	require.Eventually(t, func() bool { return bBuilder.key() == "" }, 3*time.Second, 10*time.Millisecond)
	// Notification consumers never write settings or echo another event.
	bBuilder.mu.Lock()
	before := bBuilder.calls
	bBuilder.mu.Unlock()
	b.frontendInvalidation.receive(msg.Payload)
	b.frontendInvalidation.receive(msg.Payload)
	bBuilder.mu.Lock()
	require.Equal(t, before, bBuilder.calls, "unchanged config must not rebuild manager")
	bBuilder.mu.Unlock()
}

func TestSettingsRuntime_IPFailureIsFailClosedAndRecovers(t *testing.T) {
	repo := &runtimeSettingsRepo{values: map[string]string{SettingKeyAPIKeyACLTrustForwardedIP: "true", SettingKeyForwardedClientIPHeaders: `["X-Trusted-IP"]`}}
	cfg := &config.Config{}
	cfg.SetForwardedClientIPSettings(false, nil)
	s := NewSettingService(repo, cfg)
	require.NoError(t, s.reloadForwardedClientIPSettings(context.Background()))
	require.True(t, cfg.ForwardedClientIPSettings().TrustForwardedIP)
	repo.readErr = errors.New("temporary database failure")
	require.Error(t, s.reloadForwardedClientIPSettings(context.Background()))
	require.False(t, cfg.ForwardedClientIPSettings().TrustForwardedIP)
	repo.readErr = nil
	require.NoError(t, s.reloadForwardedClientIPSettings(context.Background()))
	require.True(t, cfg.ForwardedClientIPSettings().TrustForwardedIP)
	repo.values[SettingKeyForwardedClientIPHeaders] = "invalid-json"
	require.Error(t, s.reloadForwardedClientIPSettings(context.Background()))
	require.False(t, cfg.ForwardedClientIPSettings().TrustForwardedIP)
}

type runtimeProxyRepo struct {
	ProxyRepository
	proxies []Proxy
	err     error
}

func (r *runtimeProxyRepo) ListByIDs(context.Context, []int64) ([]Proxy, error) {
	return r.proxies, r.err
}
func TestSettingsRuntime_WebSearchReconcilesProxyAndRecovers(t *testing.T) {
	repo := &runtimeSettingsRepo{values: map[string]string{SettingKeyWebSearchEmulationConfig: `{"enabled":true,"providers":[{"type":"brave","api_key":"test-key","proxy_id":1}]}`}}
	s := NewSettingService(repo, &config.Config{})
	proxies := &runtimeProxyRepo{proxies: []Proxy{{ID: 1, Protocol: "http", Host: "127.0.0.1", Port: 8001}}}
	s.SetProxyRepository(proxies)
	record := &runtimeBuilderRecorder{}
	s.SetWebSearchManagerBuilder(context.Background(), record.build)
	require.Equal(t, 1, record.calls)
	s.rebuildWebSearchManager(context.Background())
	require.Equal(t, 1, record.calls)
	proxies.proxies[0].Port = 8002
	s.rebuildWebSearchManager(context.Background())
	require.Equal(t, 2, record.calls)
	require.Equal(t, "http://127.0.0.1:8002", record.proxies[1])
	repo.readErr = errors.New("DB offline")
	s.rebuildWebSearchManager(context.Background())
	require.False(t, s.webSearchRuntimeLoaded)
	repo.readErr = nil
	s.rebuildWebSearchManager(context.Background())
	require.True(t, s.webSearchRuntimeLoaded)
	require.Equal(t, 3, record.calls)
	proxies.err = errors.New("proxy lookup failed")
	s.rebuildWebSearchManager(context.Background())
	require.Empty(t, record.proxies, "builder must not silently route directly")
	proxies.err = nil
	s.rebuildWebSearchManager(context.Background())
	require.Equal(t, "http://127.0.0.1:8002", record.proxies[1])
}

func TestSettingsRuntime_WebSearchSaveWithoutRedisOrPublishFailure(t *testing.T) {
	for _, offline := range []bool{false, true} {
		t.Run(map[bool]string{false: "no Redis", true: "publish failure"}[offline], func(t *testing.T) {
			repo := &runtimeSettingsRepo{values: map[string]string{}}
			s := NewSettingService(repo, &config.Config{})
			record := &runtimeBuilderRecorder{}
			s.SetWebSearchManagerBuilder(context.Background(), record.build)
			var client *redis.Client
			if offline {
				client = redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
				require.NoError(t, client.Close())
			}
			s.StartFrontendInvalidation(client, func() {})
			defer s.StopFrontendInvalidation()
			cfg := &WebSearchEmulationConfig{Enabled: true, Providers: []WebSearchProviderConfig{{Type: "brave", APIKey: "test-only"}}}
			require.NoError(t, s.SaveWebSearchEmulationConfig(context.Background(), cfg))
			raw, err := repo.GetValue(context.Background(), SettingKeyWebSearchEmulationConfig)
			require.NoError(t, err)
			require.Contains(t, raw, "test-only")
			require.Equal(t, "test-only", record.key())
			before := record.calls
			repo.writeErr = errors.New("write failed")
			require.Error(t, s.SaveWebSearchEmulationConfig(context.Background(), &WebSearchEmulationConfig{}))
			require.Equal(t, before, record.calls, "failed write must not replace the runtime")
		})
	}
}

func (r *runtimeSettingsRepo) GetAll(context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.allErr != nil {
		return nil, r.allErr
	}
	result := map[string]string{}
	for k, v := range r.values {
		result[k] = v
	}
	return result, nil
}

func TestSettingsRuntime_PartialCommittedSaveStillNotifiesAfterReadFailure(t *testing.T) {
	repo := &runtimeSettingsRepo{values: map[string]string{}, allErr: errors.New("read after write failed")}
	s := NewSettingService(repo, &config.Config{})
	calls := 0
	s.StartFrontendInvalidation(nil, func() { calls++ })
	defer s.StopFrontendInvalidation()
	require.NoError(t, s.UpdateSettingsOmitting(context.Background(), &SystemSettings{CompactHomeEnabled: true}, OmittedSettingKeys{SettingKeySiteName: {}}))
	require.Equal(t, "true", repo.values[SettingKeyCompactHomeEnabled])
	require.Equal(t, 1, calls, "a committed partial write must still invalidate and notify")
}

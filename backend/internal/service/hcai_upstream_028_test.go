package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestHCAIModerationEngineSwitchPreservesEndpointCredentials(t *testing.T) {
	cfg := defaultContentModerationConfig()
	cfg.APIKeys = []string{"openai-default"}
	cfg.normalize()
	cfg.ModerationEndpoints = append(cfg.ModerationEndpoints, ContentModerationEndpoint{
		ID: "custom", Name: "custom", BaseURL: "https://custom.example", Model: "custom-model",
		APIKeys: []string{"openai-custom"}, Enabled: true,
	})
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	repo := &contentModerationTestSettingRepo{values: map[string]string{SettingKeyContentModerationConfig: string(raw)}}
	s := &ContentModerationService{settingRepo: repo}
	engine, base := "typesafe", "https://typesafe.example"
	keys := []string{"typesafe-only"}
	_, err = s.UpdateConfig(context.Background(), UpdateContentModerationConfigInput{
		Engine: &engine, BaseURL: &base, APIKeys: &keys,
	})
	require.NoError(t, err)
	stored, err := s.loadConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, cfg.ModerationEndpoints, stored.ModerationEndpoints)
	require.Equal(t, []string{"openai-default"}, stored.engineProfile("openai").APIKeys)
	active := stored.effectiveEngine("typesafe")
	active.normalize()
	targets := active.moderationTargets()
	require.Len(t, targets, 1)
	require.Equal(t, "typesafe-only", targets[0].APIKey)
	require.Equal(t, base, targets[0].Endpoint.BaseURL)
	require.Equal(t, "typesafe", targets[0].Engine)
	engine = "openai"
	_, err = s.UpdateConfig(context.Background(), UpdateContentModerationConfigInput{Engine: &engine})
	require.NoError(t, err)
	stored, err = s.loadConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, cfg.ModerationEndpoints, stored.ModerationEndpoints)
	require.Len(t, stored.effectiveEngine("openai").moderationTargets(), 2)
	require.Equal(t, keys, stored.TypeSafe.APIKeys)
}

func TestHCAIOpenCodeProviderHonorsTrafficOnlyNode(t *testing.T) {
	cfg := &config.Config{GlobalBackgroundTasks: config.GlobalBackgroundTasksConfig{Disabled: true}}
	svc := ProvideOpenCodeGoUsageService(nil, nil, nil, nil, nil, cfg)
	t.Cleanup(svc.Stop)
	require.False(t, backgroundTaskLeaderEligible(svc.instanceID))
	release, acquired := tryAcquireSingletonLeaderLock(context.Background(), nil, nil,
		opencodeGoUsageLeaderLockKey, svc.instanceID, opencodeGoUsageLeaderLockTTL)
	require.False(t, acquired)
	require.Nil(t, release)
}

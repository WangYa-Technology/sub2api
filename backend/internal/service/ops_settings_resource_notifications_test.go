package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpsEmailNotificationConfigDefaultsResourceAlertsOn(t *testing.T) {
	t.Parallel()

	cfg := defaultOpsEmailNotificationConfig()
	require.True(t, cfg.Alert.AccountErrorEnabled)
	require.True(t, cfg.Alert.ProxyExpiryEnabled)
}

func TestOpsEmailNotificationConfigBackfillsResourceAlertDefaults(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	require.NoError(t, repo.Set(ctx, SettingKeyOpsEmailNotificationConfig, `{
		"alert": {
			"enabled": true,
			"recipients": ["ops@example.com"],
			"min_severity": "",
			"rate_limit_per_hour": 0,
			"batching_window_seconds": 0,
			"include_resolved_alerts": false
		},
		"report": {"enabled": false, "recipients": []}
	}`))

	svc := &OpsService{settingRepo: repo}
	cfg, err := svc.GetEmailNotificationConfig(ctx)
	require.NoError(t, err)
	require.True(t, cfg.Alert.AccountErrorEnabled)
	require.True(t, cfg.Alert.ProxyExpiryEnabled)
	require.Equal(t, []string{"ops@example.com"}, cfg.Alert.Recipients)
}

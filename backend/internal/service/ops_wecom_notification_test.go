package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type opsWeComNotifierStub struct {
	calls     int
	failCalls int
	content   string
	url       string
}

func (s *opsWeComNotifierStub) SendMarkdown(_ context.Context, webhookURL, content string) error {
	s.calls++
	s.url = webhookURL
	s.content = content
	if s.calls <= s.failCalls {
		return errors.New("temporary send failure")
	}
	return nil
}

func TestValidateOpsWeComWebhookURL(t *testing.T) {
	t.Parallel()

	valid := "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=1234567890abcdef"
	require.NoError(t, validateOpsWeComWebhookURL(valid))
	for _, raw := range []string{
		"http://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x",
		"https://example.com/cgi-bin/webhook/send?key=x",
		"https://qyapi.weixin.qq.com/cgi-bin/webhook/send",
		"https://qyapi.weixin.qq.com.evil.test/cgi-bin/webhook/send?key=x",
		"https://qyapi.weixin.qq.com:443/cgi-bin/webhook/send?key=x",
		"https://user@qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x",
		"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x#fragment",
	} {
		require.Error(t, validateOpsWeComWebhookURL(raw), raw)
	}
}

func TestOpsWeComConfigMasksWebhookAndBackfillsResourceDefaults(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	svc := &OpsService{settingRepo: repo}
	webhook := "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=1234567890abcdef"

	view, err := svc.UpdateWeComNotificationConfig(ctx, &OpsWeComNotificationConfigUpdateRequest{
		Enabled:               true,
		WebhookURL:            &webhook,
		AccountErrorEnabled:   true,
		ProxyExpiryEnabled:    true,
		MinSeverity:           "P1",
		RateLimitPerHour:      20,
		IncludeResolvedAlerts: true,
	})
	require.NoError(t, err)
	require.True(t, view.WebhookConfigured)
	require.True(t, view.AccountErrorEnabled)
	require.True(t, view.ProxyExpiryEnabled)
	require.NotContains(t, view.WebhookURLMasked, "1234567890abcdef")

	loaded, err := svc.GetWeComNotificationConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, webhook, loaded.WebhookURL)

	require.NoError(t, repo.Set(ctx, SettingKeyOpsWeComNotificationConfig, `{"enabled":false,"webhook_url":""}`))
	legacy, err := svc.GetWeComNotificationConfig(ctx)
	require.NoError(t, err)
	require.True(t, legacy.AccountErrorEnabled)
	require.True(t, legacy.ProxyExpiryEnabled)
}

func TestOpsWeComTestMessageUsesStoredWebhook(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	webhook := "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=1234567890abcdef"
	notifier := &opsWeComNotifierStub{}
	svc := &OpsService{settingRepo: repo, weComNotifier: notifier}
	_, err := svc.UpdateWeComNotificationConfig(ctx, &OpsWeComNotificationConfigUpdateRequest{
		WebhookURL:          &webhook,
		AccountErrorEnabled: true,
		ProxyExpiryEnabled:  true,
	})
	require.NoError(t, err)
	require.NoError(t, svc.TestWeComNotification(ctx))
	require.Equal(t, webhook, notifier.url)
	require.Contains(t, notifier.content, "通知测试")
}

func TestOpsWeComAccountNotificationRetriesAndDeduplicates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, time.August, 1, 8, 0, 0, 0, time.UTC)
	repo := newNotificationEmailMemorySettingRepo()
	notifier := &opsWeComNotifierStub{failCalls: 1}
	opsService := &OpsService{
		settingRepo:   repo,
		weComNotifier: notifier,
		getAccountAvailability: func(context.Context, string, *int64) (*OpsAccountAvailability, error) {
			return &OpsAccountAvailability{Accounts: map[int64]*AccountAvailability{
				42: {
					AccountID:    42,
					AccountName:  "primary-account",
					Platform:     "openai",
					AccountType:  "oauth",
					UpdatedAt:    now,
					HasError:     true,
					ErrorMessage: "credential refresh failed",
				},
			}}, nil
		},
	}
	evaluator := &OpsAlertEvaluatorService{
		opsService:   opsService,
		weComLimiter: newSlidingWindowLimiter(0, time.Hour),
	}
	cfg := &OpsWeComNotificationConfig{
		Enabled:             true,
		WebhookURL:          "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test-key",
		AccountErrorEnabled: true,
	}

	require.Zero(t, evaluator.evaluateAccountStatusWeComNotifications(ctx, now, cfg), "failed delivery must not be persisted")
	require.Equal(t, 1, evaluator.evaluateAccountStatusWeComNotifications(ctx, now, cfg), "next evaluation must retry")
	require.Zero(t, evaluator.evaluateAccountStatusWeComNotifications(ctx, now, cfg), "successful delivery must be deduplicated")
	require.Equal(t, 2, notifier.calls)
	require.Contains(t, notifier.content, "primary-account")
	require.Contains(t, notifier.content, "credential refresh failed")
}

func TestOpsWeComRuleAndRecoveryNotificationsRetryIndependently(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	webhook := "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test-key"
	require.NoError(t, repo.Set(ctx, SettingKeyOpsWeComNotificationConfig, `{
		"enabled": true,
		"webhook_url": "`+webhook+`",
		"account_error_enabled": true,
		"proxy_expiry_enabled": true,
		"include_resolved_alerts": true
	}`))
	notifier := &opsWeComNotifierStub{failCalls: 1}
	evaluator := &OpsAlertEvaluatorService{
		opsService:   &OpsService{settingRepo: repo, weComNotifier: notifier},
		opsRepo:      &opsRepoMock{},
		weComLimiter: newSlidingWindowLimiter(0, time.Hour),
	}
	rule := &OpsAlertRule{ID: 7, Name: "error rate", Severity: "P1", NotifyWeCom: true}
	event := &OpsAlertEvent{ID: 99, RuleID: 7, Status: OpsAlertStatusFiring, Severity: "P1", FiredAt: time.Now().UTC()}

	require.False(t, evaluator.maybeSendAlertWeCom(ctx, nil, rule, event))
	require.False(t, event.WeComSent)
	require.True(t, evaluator.maybeSendAlertWeCom(ctx, nil, rule, event))
	require.True(t, event.WeComSent)
	require.False(t, evaluator.maybeSendAlertWeCom(ctx, nil, rule, event))

	resolvedAt := time.Now().UTC()
	event.Status = OpsAlertStatusResolved
	event.ResolvedAt = &resolvedAt
	require.True(t, evaluator.maybeSendAlertWeCom(ctx, nil, rule, event), "recovery uses a distinct persistent delivery key")
	require.False(t, evaluator.maybeSendAlertWeCom(ctx, nil, rule, event), "recovery must be deduplicated")
	require.Equal(t, 3, notifier.calls)
}

func TestShouldSendOpsAlertByMinSeverity(t *testing.T) {
	t.Parallel()
	require.True(t, shouldSendOpsAlertByMinSeverity("P1", "P0"))
	require.False(t, shouldSendOpsAlertByMinSeverity("P1", "P2"))
	require.True(t, shouldSendOpsAlertByMinSeverity("", "P3"))
}

func TestSanitizeOpsWeComFieldBlocksMentionMarkup(t *testing.T) {
	t.Parallel()
	got := sanitizeOpsWeComField("failure\n<@all>")
	require.False(t, strings.Contains(got, "\n"))
	require.NotContains(t, got, "<@all>")
}

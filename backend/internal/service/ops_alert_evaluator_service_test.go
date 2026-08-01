//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var _ OpsRepository = (*stubOpsRepo)(nil)

type stubOpsRepo struct {
	OpsRepository
	overview *OpsDashboardOverview
	err      error
}

func (s *stubOpsRepo) GetDashboardOverview(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.overview != nil {
		return s.overview, nil
	}
	return &OpsDashboardOverview{}, nil
}

func TestComputeGroupAvailableRatio(t *testing.T) {
	t.Parallel()

	t.Run("正常情况: 10个账号, 8个可用 = 80%", func(t *testing.T) {
		t.Parallel()

		got := computeGroupAvailableRatio(&GroupAvailability{
			TotalAccounts:  10,
			AvailableCount: 8,
		})
		require.InDelta(t, 80.0, got, 0.0001)
	})

	t.Run("边界情况: TotalAccounts = 0 应返回 0", func(t *testing.T) {
		t.Parallel()

		got := computeGroupAvailableRatio(&GroupAvailability{
			TotalAccounts:  0,
			AvailableCount: 8,
		})
		require.Equal(t, 0.0, got)
	})

	t.Run("边界情况: AvailableCount = 0 应返回 0%", func(t *testing.T) {
		t.Parallel()

		got := computeGroupAvailableRatio(&GroupAvailability{
			TotalAccounts:  10,
			AvailableCount: 0,
		})
		require.Equal(t, 0.0, got)
	})
}

func TestCountAccountsByCondition(t *testing.T) {
	t.Parallel()

	t.Run("测试限流账号统计: acc.IsRateLimited", func(t *testing.T) {
		t.Parallel()

		accounts := map[int64]*AccountAvailability{
			1: {IsRateLimited: true},
			2: {IsRateLimited: false},
			3: {IsRateLimited: true},
		}

		got := countAccountsByCondition(accounts, func(acc *AccountAvailability) bool {
			return acc.IsRateLimited
		})
		require.Equal(t, int64(2), got)
	})

	t.Run("测试错误账号统计（排除临时不可调度）: acc.HasError && acc.TempUnschedulableUntil == nil", func(t *testing.T) {
		t.Parallel()

		until := time.Now().UTC().Add(5 * time.Minute)
		accounts := map[int64]*AccountAvailability{
			1: {HasError: true},
			2: {HasError: true, TempUnschedulableUntil: &until},
			3: {HasError: false},
		}

		got := countAccountsByCondition(accounts, func(acc *AccountAvailability) bool {
			return acc.HasError && acc.TempUnschedulableUntil == nil
		})
		require.Equal(t, int64(1), got)
	})

	t.Run("边界情况: 空 map 应返回 0", func(t *testing.T) {
		t.Parallel()

		got := countAccountsByCondition(map[int64]*AccountAvailability{}, func(acc *AccountAvailability) bool {
			return acc.IsRateLimited
		})
		require.Equal(t, int64(0), got)
	})
}

func TestBuildOpsAccountNotificationCandidate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 1, 8, 0, 0, 0, time.UTC)
	until := now.Add(30 * time.Minute)

	t.Run("healthy account is ignored", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, buildOpsAccountNotificationCandidate(&AccountAvailability{}, now))
	})

	t.Run("error account includes its error message", func(t *testing.T) {
		t.Parallel()
		candidate := buildOpsAccountNotificationCandidate(&AccountAvailability{
			HasError:     true,
			ErrorMessage: "credential refresh failed",
		}, now)
		require.NotNil(t, candidate)
		require.Equal(t, "error", candidate.status)
		require.Equal(t, "credential refresh failed", candidate.message)
		require.Equal(t, "-", candidate.until)
	})

	t.Run("temporary unschedulable account prefers its runtime reason", func(t *testing.T) {
		t.Parallel()
		candidate := buildOpsAccountNotificationCandidate(&AccountAvailability{
			ErrorMessage:            "stale error",
			TempUnschedulableUntil:  &until,
			TempUnschedulableReason: "upstream transport timeout",
		}, now)
		require.NotNil(t, candidate)
		require.Equal(t, "temporarily_unschedulable", candidate.status)
		require.Equal(t, "upstream transport timeout", candidate.message)
		require.Equal(t, until.Format(time.RFC3339), candidate.until)
	})

	t.Run("a changed reason produces a new incident fingerprint", func(t *testing.T) {
		t.Parallel()
		first := buildOpsAccountNotificationCandidate(&AccountAvailability{
			HasError:     true,
			ErrorMessage: "first error",
		}, now)
		second := buildOpsAccountNotificationCandidate(&AccountAvailability{
			HasError:     true,
			ErrorMessage: "second error",
		}, now)
		require.NotEqual(t, first.fingerprint, second.fingerprint)
	})

	t.Run("extending the same temporary incident does not duplicate it", func(t *testing.T) {
		t.Parallel()
		firstUntil := now.Add(10 * time.Minute)
		secondUntil := now.Add(20 * time.Minute)
		first := buildOpsAccountNotificationCandidate(&AccountAvailability{
			TempUnschedulableUntil:  &firstUntil,
			TempUnschedulableReason: "transport timeout",
		}, now)
		second := buildOpsAccountNotificationCandidate(&AccountAvailability{
			TempUnschedulableUntil:  &secondUntil,
			TempUnschedulableReason: "transport timeout",
		}, now)
		require.Equal(t, first.fingerprint, second.fingerprint)
	})
}

func TestOpsResourceNotificationStateAndProxyWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 1, 8, 0, 0, 0, time.UTC)
	svc := &OpsAlertEvaluatorService{}
	svc.setAccountNotificationState(42, "incident")
	require.Equal(t, "incident", svc.accountNotificationState(42))
	svc.pruneAccountNotificationStates(map[int64]string{})
	require.Empty(t, svc.accountNotificationState(42), "recovery must clear the incident state")
	firstIncidentAt := now.Add(-2 * time.Hour)
	secondIncidentAt := now.Add(-time.Hour)
	require.NotEqual(t,
		opsAccountIncidentReminderKey("same error", firstIncidentAt),
		opsAccountIncidentReminderKey("same error", secondIncidentAt),
		"the same error after recovery must be deliverable as a new incident",
	)

	inside := now.Add(72 * time.Hour)
	outside := now.Add(72*time.Hour + time.Second)
	expired := now.Add(-time.Second)
	require.True(t, isProxyInOpsExpiryReminderWindow(&inside, now))
	require.False(t, isProxyInOpsExpiryReminderWindow(&outside, now))
	require.False(t, isProxyInOpsExpiryReminderWindow(&expired, now))
	require.False(t, isProxyInOpsExpiryReminderWindow(nil, now))
	require.Equal(t, "2d 3h 5m", formatOpsRemainingTime(51*time.Hour+5*time.Minute))
	require.Equal(t, "临时不可调度", localizedOpsResourceVariables(
		NotificationEmailEventOpsAccountStatusAlert,
		"zh",
		map[string]string{"account_status": "temporarily_unschedulable"},
	)["account_status"])
	require.Equal(t, "2 天 3 小时 5 分钟", localizedOpsResourceVariables(
		NotificationEmailEventOpsProxyExpiryReminder,
		"zh-CN",
		map[string]string{"proxy_remaining_time": "2d 3h 5m"},
	)["proxy_remaining_time"])
}

func TestOpsResourceNotificationDisabledClearsIncidentState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	require.NoError(t, repo.Set(ctx, SettingKeyOpsEmailNotificationConfig, `{
		"alert": {
			"enabled": false,
			"recipients": ["ops@example.com"]
		}
	}`))
	emailService := NewEmailService(repo, nil)
	_ = NewNotificationEmailService(repo, emailService)
	svc := &OpsAlertEvaluatorService{
		opsService:   &OpsService{settingRepo: repo},
		emailService: emailService,
	}
	svc.setAccountNotificationState(42, "account-incident")
	svc.setProxyNotificationState(7, "proxy-expiry")

	require.Zero(t, svc.evaluateResourceNotifications(ctx, time.Now().UTC()))
	require.Empty(t, svc.accountNotificationState(42))
	require.Empty(t, svc.proxyNotificationState(7))
}

func TestSendOpsResourceNotificationSkipsDeliveredRecipientsBeforeRateLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	emailService := NewEmailService(repo, nil)
	notificationService := NewNotificationEmailService(repo, emailService)
	svc := &OpsAlertEvaluatorService{
		emailService: emailService,
		emailLimiter: newSlidingWindowLimiter(1, time.Hour),
	}
	input := NotificationEmailSendInput{
		Event:       NotificationEmailEventOpsAccountStatusAlert,
		SourceType:  "ops_account_status",
		SourceID:    "42",
		ReminderKey: "incident-1",
	}
	recipient := "ops@example.com"
	deliveryKey := notificationEmailDeliveryKey(input.Event, input.SourceType, input.SourceID, recipient, input.ReminderKey)
	require.NoError(t, repo.Set(ctx, deliveryKey, time.Now().UTC().Format(time.RFC3339Nano)))

	require.True(t, svc.sendOpsResourceNotification(ctx, []string{recipient}, input))
	require.True(t, svc.emailLimiter.Allow(time.Now().UTC()), "a deduplicated delivery must not consume the limiter")
	require.NotNil(t, notificationService)
}

// TestComputeRuleMetric_AccountTempUnscheduledCount verifies the new
// account_temp_unscheduled_count metric counts accounts currently in the
// temp-unscheduled window and ignores those whose window has expired or
// were never temp-unscheduled.
func TestComputeRuleMetric_AccountTempUnscheduledCount(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	futureUntil := now.Add(5 * time.Minute)
	pastUntil := now.Add(-1 * time.Minute)

	availability := &OpsAccountAvailability{
		Accounts: map[int64]*AccountAvailability{
			// currently temp-unscheduled (window active)
			1: {TempUnschedulableUntil: &futureUntil},
			2: {TempUnschedulableUntil: &futureUntil},
			// temp-unsched window already expired → should NOT count
			3: {TempUnschedulableUntil: &pastUntil},
			// never temp-unscheduled
			4: {HasError: true},
			5: {IsRateLimited: true},
		},
	}

	opsService := &OpsService{
		getAccountAvailability: func(_ context.Context, _ string, _ *int64) (*OpsAccountAvailability, error) {
			return availability, nil
		},
	}
	svc := &OpsAlertEvaluatorService{
		opsService: opsService,
		opsRepo:    &stubOpsRepo{},
	}

	rule := &OpsAlertRule{MetricType: "account_temp_unscheduled_count"}
	val, ok := svc.computeRuleMetric(context.Background(), rule, nil,
		now.Add(-5*time.Minute), now, "", nil)

	require.True(t, ok)
	require.InDelta(t, 2.0, val, 0.0001, "only 2 accounts have an active temp-unsched window")
}

func TestComputeRuleMetricNewIndicators(t *testing.T) {
	t.Parallel()

	groupID := int64(101)
	platform := "openai"

	availability := &OpsAccountAvailability{
		Group: &GroupAvailability{
			GroupID:        groupID,
			TotalAccounts:  10,
			AvailableCount: 8,
		},
		Accounts: map[int64]*AccountAvailability{
			1: {IsRateLimited: true},
			2: {IsRateLimited: true},
			3: {HasError: true},
			4: {HasError: true, TempUnschedulableUntil: timePtr(time.Now().UTC().Add(2 * time.Minute))},
			5: {HasError: false, IsRateLimited: false},
		},
	}

	opsService := &OpsService{
		getAccountAvailability: func(_ context.Context, _ string, _ *int64) (*OpsAccountAvailability, error) {
			return availability, nil
		},
	}

	svc := &OpsAlertEvaluatorService{
		opsService: opsService,
		opsRepo:    &stubOpsRepo{overview: &OpsDashboardOverview{}},
	}

	start := time.Now().UTC().Add(-5 * time.Minute)
	end := time.Now().UTC()
	ctx := context.Background()

	tests := []struct {
		name       string
		metricType string
		groupID    *int64
		wantValue  float64
		wantOK     bool
	}{
		{
			name:       "group_available_accounts",
			metricType: "group_available_accounts",
			groupID:    &groupID,
			wantValue:  8,
			wantOK:     true,
		},
		{
			name:       "group_available_ratio",
			metricType: "group_available_ratio",
			groupID:    &groupID,
			wantValue:  80.0,
			wantOK:     true,
		},
		{
			name:       "account_rate_limited_count",
			metricType: "account_rate_limited_count",
			groupID:    nil,
			wantValue:  2,
			wantOK:     true,
		},
		{
			name:       "account_error_count",
			metricType: "account_error_count",
			groupID:    nil,
			wantValue:  1,
			wantOK:     true,
		},
		{
			name:       "group_available_accounts without group_id returns false",
			metricType: "group_available_accounts",
			groupID:    nil,
			wantValue:  0,
			wantOK:     false,
		},
		{
			name:       "group_available_ratio without group_id returns false",
			metricType: "group_available_ratio",
			groupID:    nil,
			wantValue:  0,
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rule := &OpsAlertRule{
				MetricType: tt.metricType,
			}
			gotValue, gotOK := svc.computeRuleMetric(ctx, rule, nil, start, end, platform, tt.groupID)
			require.Equal(t, tt.wantOK, gotOK)
			if !tt.wantOK {
				return
			}
			require.InDelta(t, tt.wantValue, gotValue, 0.0001)
		})
	}
}

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestClaimWeComDeliveryIsAtomicViaDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	evaluator := &OpsAlertEvaluatorService{db: db}
	key := "ops_wecom_delivery:v1:abc"

	// 首次 INSERT 成功 → 获得发送权
	mock.ExpectExec(`INSERT INTO settings \(key, value, updated_at\) VALUES \(\$1, \$2, NOW\(\)\) ON CONFLICT \(key\) DO NOTHING`).
		WithArgs(key, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	claimed, err := evaluator.claimWeComDelivery(ctx, key)
	require.NoError(t, err)
	require.True(t, claimed)

	// 键已存在（ON CONFLICT DO NOTHING 影响 0 行）→ 未获得发送权
	mock.ExpectExec(`INSERT INTO settings \(key, value, updated_at\) VALUES \(\$1, \$2, NOW\(\)\) ON CONFLICT \(key\) DO NOTHING`).
		WithArgs(key, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	claimed, err = evaluator.claimWeComDelivery(ctx, key)
	require.NoError(t, err)
	require.False(t, claimed)

	// DB 错误 → 返回错误（fail-closed，不发送）
	mock.ExpectExec(`INSERT INTO settings \(key, value, updated_at\) VALUES \(\$1, \$2, NOW\(\)\) ON CONFLICT \(key\) DO NOTHING`).
		WithArgs(key, sqlmock.AnyArg()).
		WillReturnError(errors.New("connection lost"))
	claimed, err = evaluator.claimWeComDelivery(ctx, key)
	require.Error(t, err)
	require.False(t, claimed)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseWeComDeliveryRemovesClaimViaDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	evaluator := &OpsAlertEvaluatorService{db: db}
	key := "ops_wecom_delivery:v1:abc"

	mock.ExpectExec(`DELETE FROM settings WHERE key = \$1`).
		WithArgs(key).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, evaluator.releaseWeComDelivery(ctx, key))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestClaimWeComDeliveryFallbackWithoutDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	evaluator := &OpsAlertEvaluatorService{opsService: &OpsService{settingRepo: repo}}
	key := opsWeComDeliveryKey("account_status", "42", "fingerprint")

	claimed, err := evaluator.claimWeComDelivery(ctx, key)
	require.NoError(t, err)
	require.True(t, claimed)

	claimed, err = evaluator.claimWeComDelivery(ctx, key)
	require.NoError(t, err)
	require.False(t, claimed, "second claim must observe the existing delivery")

	require.NoError(t, evaluator.releaseWeComDelivery(ctx, key))
	claimed, err = evaluator.claimWeComDelivery(ctx, key)
	require.NoError(t, err)
	require.True(t, claimed, "after rollback the delivery can be retried")
}

func TestWeComRateLimitSharedAcrossNodes(t *testing.T) {
	t.Parallel()

	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer client.Close()

	evaluator := &OpsAlertEvaluatorService{
		redisClient: client,
		weComLimiter: newSlidingWindowLimiter(0, time.Hour),
	}
	ctx := context.Background()
	now := time.Date(2026, 8, 7, 13, 0, 0, 0, time.UTC)

	// 同小时桶内共享计数：第 3 次超限
	require.True(t, evaluator.allowWeComSend(ctx, 2, now))
	require.True(t, evaluator.allowWeComSend(ctx, 2, now))
	require.False(t, evaluator.allowWeComSend(ctx, 2, now), "third send within the hour must be rejected")

	// 下一小时桶重置
	require.True(t, evaluator.allowWeComSend(ctx, 2, now.Add(time.Hour)))

	// 上限 0 = 不限制
	require.True(t, evaluator.allowWeComSend(ctx, 0, now.Add(2*time.Hour)))
}

func TestAllowWeComSendFallsBackToLocalLimiter(t *testing.T) {
	t.Parallel()

	// Redis 不可用时降级为进程内限流，不阻断发送通道。
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	redisServer.Close()
	defer client.Close()

	evaluator := &OpsAlertEvaluatorService{
		redisClient: client,
		weComLimiter: newSlidingWindowLimiter(2, time.Hour),
	}
	ctx := context.Background()
	now := time.Now().UTC()

	require.True(t, evaluator.allowWeComSend(ctx, 2, now))
	require.True(t, evaluator.allowWeComSend(ctx, 2, now))
	require.False(t, evaluator.allowWeComSend(ctx, 2, now))
}

func TestSendOpsWeComOnceRateLimitRollsBackClaim(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	notifier := &opsWeComNotifierStub{}
	evaluator := &OpsAlertEvaluatorService{
		opsService:   &OpsService{settingRepo: repo, weComNotifier: notifier},
		weComLimiter: newSlidingWindowLimiter(0, time.Hour),
	}
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	defer client.Close()
	evaluator.redisClient = client

	cfg := &OpsWeComNotificationConfig{
		Enabled:         true,
		WebhookURL:      "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test-key",
		RateLimitPerHour: 1,
	}

	// 预热：消耗唯一配额（发送成功，占位保留）。
	key1 := opsWeComDeliveryKey("proxy_expiry", "1", "t1")
	res := evaluator.sendOpsWeComOnce(ctx, cfg, key1, "c1")
	require.True(t, res.Sent)
	require.True(t, res.Delivered)

	// 超限：claim 成功后被限流拒绝 → 占位必须回滚，保证下个周期可重试。
	key2 := opsWeComDeliveryKey("proxy_expiry", "2", "t2")
	res = evaluator.sendOpsWeComOnce(ctx, cfg, key2, "c2")
	require.False(t, res.Sent)
	require.False(t, res.Delivered)
	_, err := repo.Get(ctx, key2)
	require.ErrorIs(t, err, ErrSettingNotFound, "rate-limited claim must be rolled back")
	require.Equal(t, 1, notifier.calls)

	// 同周期内其他投递键也会被限流拦截（共享计数）。
	key3 := opsWeComDeliveryKey("proxy_expiry", "3", "t3")
	res = evaluator.sendOpsWeComOnce(ctx, cfg, key3, "c3")
	require.False(t, res.Sent)
}

func TestSendOpsWeComOnceFailedSendRollsBackClaim(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	notifier := &opsWeComNotifierStub{failCalls: 1}
	evaluator := &OpsAlertEvaluatorService{
		opsService:   &OpsService{settingRepo: repo, weComNotifier: notifier},
		weComLimiter: newSlidingWindowLimiter(0, time.Hour),
	}
	cfg := &OpsWeComNotificationConfig{
		Enabled:    true,
		WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test-key",
	}

	key := opsWeComDeliveryKey("proxy_expiry", "9", "t9")
	// 第一次发送失败 → 占位回滚，通知可在下个周期重试。
	res := evaluator.sendOpsWeComOnce(ctx, cfg, key, "c")
	require.False(t, res.Sent)
	_, err := repo.Get(ctx, key)
	require.ErrorIs(t, err, ErrSettingNotFound, "failed send must roll back the claim")

	// 第二次成功 → 占位保留，随后被去重。
	require.True(t, evaluator.sendOpsWeComOnce(ctx, cfg, key, "c").Sent)
	res = evaluator.sendOpsWeComOnce(ctx, cfg, key, "c")
	require.False(t, res.Sent)
	require.True(t, res.Delivered, "successful delivery must be deduplicated")
	require.Equal(t, 2, notifier.calls)
}

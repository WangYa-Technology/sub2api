package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	opsAlertEvaluatorJobName = "ops_alert_evaluator"

	opsAlertEvaluatorTimeout         = 45 * time.Second
	opsAlertEvaluatorLeaderLockKey   = "ops:alert:evaluator:leader"
	opsAlertEvaluatorLeaderLockTTL   = 90 * time.Second
	opsAlertEvaluatorSkipLogInterval = 1 * time.Minute
	opsProxyExpiryReminderWindow     = 3 * 24 * time.Hour
)

var opsAlertEvaluatorReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

type OpsAlertEvaluatorService struct {
	opsService   *OpsService
	opsRepo      OpsRepository
	emailService *EmailService
	proxyRepo    ProxyRepository

	redisClient *redis.Client
	cfg         *config.Config
	instanceID  string

	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup

	mu                        sync.Mutex
	ruleStates                map[int64]*opsAlertRuleState
	accountNotificationStates map[int64]string
	proxyNotificationStates   map[int64]string

	emailLimiter *slidingWindowLimiter
	weComLimiter *slidingWindowLimiter

	skipLogMu sync.Mutex
	skipLogAt time.Time

	warnNoRedisOnce sync.Once
}

type opsAlertRuleState struct {
	LastEvaluatedAt     time.Time
	ConsecutiveBreaches int
}

func NewOpsAlertEvaluatorService(
	opsService *OpsService,
	opsRepo OpsRepository,
	emailService *EmailService,
	redisClient *redis.Client,
	cfg *config.Config,
	proxyRepo ProxyRepository,
) *OpsAlertEvaluatorService {
	return &OpsAlertEvaluatorService{
		opsService:                opsService,
		opsRepo:                   opsRepo,
		emailService:              emailService,
		proxyRepo:                 proxyRepo,
		redisClient:               redisClient,
		cfg:                       cfg,
		instanceID:                uuid.NewString(),
		ruleStates:                map[int64]*opsAlertRuleState{},
		accountNotificationStates: map[int64]string{},
		proxyNotificationStates:   map[int64]string{},
		emailLimiter:              newSlidingWindowLimiter(0, time.Hour),
		weComLimiter:              newSlidingWindowLimiter(0, time.Hour),
	}
}

func (s *OpsAlertEvaluatorService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		if s.stopCh == nil {
			s.stopCh = make(chan struct{})
		}
		s.wg.Add(1)
		go s.run()
	})
}

func (s *OpsAlertEvaluatorService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.stopCh != nil {
			close(s.stopCh)
		}
	})
	s.wg.Wait()
}

func (s *OpsAlertEvaluatorService) run() {
	defer s.wg.Done()

	// Start immediately to produce early feedback in ops dashboard.
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			interval := s.getInterval()
			s.evaluateOnce(interval)
			timer.Reset(interval)
		case <-s.stopCh:
			return
		}
	}
}

func (s *OpsAlertEvaluatorService) getInterval() time.Duration {
	// Default.
	interval := 60 * time.Second

	if s == nil || s.opsService == nil {
		return interval
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg, err := s.opsService.GetOpsAlertRuntimeSettings(ctx)
	if err != nil || cfg == nil {
		return interval
	}
	if cfg.EvaluationIntervalSeconds <= 0 {
		return interval
	}
	if cfg.EvaluationIntervalSeconds < 1 {
		return interval
	}
	if cfg.EvaluationIntervalSeconds > int((24 * time.Hour).Seconds()) {
		return interval
	}
	return time.Duration(cfg.EvaluationIntervalSeconds) * time.Second
}

func (s *OpsAlertEvaluatorService) evaluateOnce(interval time.Duration) {
	if s == nil || s.opsRepo == nil {
		return
	}
	if s.cfg != nil && !s.cfg.Ops.Enabled {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), opsAlertEvaluatorTimeout)
	defer cancel()

	if s.opsService != nil && !s.opsService.IsMonitoringEnabled(ctx) {
		return
	}

	runtimeCfg := defaultOpsAlertRuntimeSettings()
	if s.opsService != nil {
		if loaded, err := s.opsService.GetOpsAlertRuntimeSettings(ctx); err == nil && loaded != nil {
			runtimeCfg = loaded
		}
	}

	release, ok := s.tryAcquireLeaderLock(ctx, runtimeCfg.DistributedLock)
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	startedAt := time.Now().UTC()
	runAt := startedAt

	rules, err := s.opsRepo.ListAlertRules(ctx)
	if err != nil {
		s.recordHeartbeatError(runAt, time.Since(startedAt), err)
		logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] list rules failed: %v", err)
		return
	}

	rulesTotal := len(rules)
	rulesEnabled := 0
	rulesEvaluated := 0
	eventsCreated := 0
	eventsResolved := 0
	emailsSent := 0
	weComSent := 0

	now := time.Now().UTC()
	safeEnd := now.Truncate(time.Minute)
	if safeEnd.IsZero() {
		safeEnd = now
	}

	systemMetrics, _ := s.opsRepo.GetLatestSystemMetrics(ctx, 1)

	// Cleanup stale state for removed rules.
	s.pruneRuleStates(rules)

	for _, rule := range rules {
		if rule == nil || !rule.Enabled || rule.ID <= 0 {
			continue
		}
		rulesEnabled++

		scopePlatform, scopeGroupID, scopeRegion := parseOpsAlertRuleScope(rule.Filters)

		windowMinutes := rule.WindowMinutes
		if windowMinutes <= 0 {
			windowMinutes = 1
		}
		windowStart := safeEnd.Add(-time.Duration(windowMinutes) * time.Minute)
		windowEnd := safeEnd

		metricValue, ok := s.computeRuleMetric(ctx, rule, systemMetrics, windowStart, windowEnd, scopePlatform, scopeGroupID)
		if !ok {
			s.resetRuleState(rule.ID, now)
			continue
		}
		rulesEvaluated++

		breachedNow := compareMetric(metricValue, rule.Operator, rule.Threshold)
		required := requiredSustainedBreaches(rule.SustainedMinutes, interval)
		consecutive := s.updateRuleBreaches(rule.ID, now, interval, breachedNow)

		activeEvent, err := s.opsRepo.GetActiveAlertEvent(ctx, rule.ID)
		if err != nil {
			logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] get active event failed (rule=%d): %v", rule.ID, err)
			continue
		}

		if breachedNow && consecutive >= required {
			if activeEvent != nil {
				if s.maybeSendAlertWeCom(ctx, runtimeCfg, rule, activeEvent) {
					weComSent++
				}
				continue
			}

			// Scoped silencing: if a matching silence exists, skip creating a firing event.
			if s.opsService != nil {
				platform := strings.TrimSpace(scopePlatform)
				region := scopeRegion
				if platform != "" {
					if ok, err := s.opsService.IsAlertSilenced(ctx, rule.ID, platform, scopeGroupID, region, now); err == nil && ok {
						continue
					}
				}
			}

			latestEvent, err := s.opsRepo.GetLatestAlertEvent(ctx, rule.ID)
			if err != nil {
				logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] get latest event failed (rule=%d): %v", rule.ID, err)
				continue
			}
			if latestEvent != nil && rule.CooldownMinutes > 0 {
				cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
				if now.Sub(latestEvent.FiredAt) < cooldown {
					continue
				}
			}

			firedEvent := &OpsAlertEvent{
				RuleID:         rule.ID,
				Severity:       strings.TrimSpace(rule.Severity),
				Status:         OpsAlertStatusFiring,
				Title:          fmt.Sprintf("%s: %s", strings.TrimSpace(rule.Severity), strings.TrimSpace(rule.Name)),
				Description:    buildOpsAlertDescription(rule, metricValue, windowMinutes, scopePlatform, scopeGroupID),
				MetricValue:    float64Ptr(metricValue),
				ThresholdValue: float64Ptr(rule.Threshold),
				Dimensions:     buildOpsAlertDimensions(scopePlatform, scopeGroupID),
				FiredAt:        now,
				CreatedAt:      now,
			}

			created, err := s.opsRepo.CreateAlertEvent(ctx, firedEvent)
			if err != nil {
				logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] create event failed (rule=%d): %v", rule.ID, err)
				continue
			}

			eventsCreated++
			if created != nil && created.ID > 0 {
				if s.maybeSendAlertEmail(ctx, runtimeCfg, rule, created) {
					emailsSent++
				}
				if s.maybeSendAlertWeCom(ctx, runtimeCfg, rule, created) {
					weComSent++
				}
			}
			continue
		}

		// Not breached: resolve active event if present.
		if activeEvent != nil {
			resolvedAt := now
			if err := s.opsRepo.UpdateAlertEventStatus(ctx, activeEvent.ID, OpsAlertStatusResolved, &resolvedAt); err != nil {
				logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] resolve event failed (event=%d): %v", activeEvent.ID, err)
			} else {
				eventsResolved++
				activeEvent.Status = OpsAlertStatusResolved
				activeEvent.ResolvedAt = &resolvedAt
				if s.maybeSendAlertWeCom(ctx, runtimeCfg, rule, activeEvent) {
					weComSent++
				}
			}
		} else if latestEvent, err := s.opsRepo.GetLatestAlertEvent(ctx, rule.ID); err == nil && latestEvent != nil &&
			(latestEvent.Status == OpsAlertStatusResolved || latestEvent.Status == OpsAlertStatusManualResolved) {
			// A failed recovery delivery is retried on later evaluator cycles. The
			// persistent delivery key prevents duplicate recovery messages.
			if s.maybeSendAlertWeCom(ctx, runtimeCfg, rule, latestEvent) {
				weComSent++
			}
		}
	}

	resourceEmailsSent := s.evaluateResourceNotifications(ctx, now)
	resourceWeComSent := s.evaluateWeComResourceNotifications(ctx, now)
	result := truncateString(fmt.Sprintf("rules=%d enabled=%d evaluated=%d created=%d resolved=%d emails_sent=%d wecom_sent=%d resource_emails_sent=%d resource_wecom_sent=%d", rulesTotal, rulesEnabled, rulesEvaluated, eventsCreated, eventsResolved, emailsSent, weComSent, resourceEmailsSent, resourceWeComSent), 2048)
	s.recordHeartbeatSuccess(runAt, time.Since(startedAt), result)
}

func (s *OpsAlertEvaluatorService) evaluateResourceNotifications(ctx context.Context, now time.Time) int {
	if s == nil || s.opsService == nil || s.emailService == nil || s.emailService.notificationEmailService == nil {
		return 0
	}
	emailCfg, err := s.opsService.GetEmailNotificationConfig(ctx)
	if err != nil || emailCfg == nil {
		return 0
	}
	if !emailCfg.Alert.Enabled || len(emailCfg.Alert.Recipients) == 0 {
		s.clearAccountNotificationStates()
		s.clearProxyNotificationStates()
		return 0
	}

	s.emailLimiter.SetLimit(emailCfg.Alert.RateLimitPerHour)
	sent := 0
	if emailCfg.Alert.AccountErrorEnabled {
		sent += s.evaluateAccountStatusNotifications(ctx, now, emailCfg.Alert.Recipients, emailCfg.Alert.MinSeverity)
	} else {
		s.clearAccountNotificationStates()
	}
	if emailCfg.Alert.ProxyExpiryEnabled {
		sent += s.evaluateProxyExpiryNotifications(ctx, now, emailCfg.Alert.Recipients, emailCfg.Alert.MinSeverity)
	} else {
		s.clearProxyNotificationStates()
	}
	return sent
}

func (s *OpsAlertEvaluatorService) evaluateWeComResourceNotifications(ctx context.Context, now time.Time) int {
	if s == nil || s.opsService == nil || s.opsService.weComNotifier == nil || s.opsService.settingRepo == nil {
		return 0
	}
	cfg, err := s.opsService.GetWeComNotificationConfig(ctx)
	if err != nil || cfg == nil || !cfg.Enabled || strings.TrimSpace(cfg.WebhookURL) == "" {
		return 0
	}
	s.weComLimiter.SetLimit(cfg.RateLimitPerHour)
	sent := 0
	if cfg.AccountErrorEnabled {
		sent += s.evaluateAccountStatusWeComNotifications(ctx, now, cfg)
	}
	if cfg.ProxyExpiryEnabled {
		sent += s.evaluateProxyExpiryWeComNotifications(ctx, now, cfg)
	}
	return sent
}

type opsWeComDeliveryResult struct {
	Sent      bool
	Delivered bool
}

func (s *OpsAlertEvaluatorService) sendOpsWeComOnce(ctx context.Context, cfg *OpsWeComNotificationConfig, deliveryKey, content string) opsWeComDeliveryResult {
	if s == nil || s.opsService == nil || s.opsService.weComNotifier == nil || cfg == nil || strings.TrimSpace(deliveryKey) == "" {
		return opsWeComDeliveryResult{}
	}
	delivered, err := s.opsService.weComDeliveryExists(ctx, deliveryKey)
	if err != nil {
		logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] check WeCom delivery failed (delivery=%s): %v", notificationEmailHash(deliveryKey), err)
		return opsWeComDeliveryResult{}
	}
	if delivered {
		return opsWeComDeliveryResult{Delivered: true}
	}
	if !s.weComLimiter.Allow(time.Now().UTC()) {
		return opsWeComDeliveryResult{}
	}
	if err := s.opsService.weComNotifier.SendMarkdown(ctx, cfg.WebhookURL, content); err != nil {
		logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] send WeCom notification failed (delivery=%s): %v", notificationEmailHash(deliveryKey), err)
		return opsWeComDeliveryResult{}
	}
	if err := s.opsService.markWeComDelivered(ctx, deliveryKey); err != nil {
		logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] persist WeCom delivery failed (delivery=%s): %v", notificationEmailHash(deliveryKey), err)
		return opsWeComDeliveryResult{Sent: true}
	}
	return opsWeComDeliveryResult{Sent: true, Delivered: true}
}

func (s *OpsAlertEvaluatorService) evaluateAccountStatusWeComNotifications(ctx context.Context, now time.Time, cfg *OpsWeComNotificationConfig) int {
	availability, err := s.opsService.GetAccountAvailability(ctx, "", nil)
	if err != nil || availability == nil {
		return 0
	}
	sent := 0
	for id, account := range availability.Accounts {
		candidate := buildOpsAccountNotificationCandidate(account, now)
		if candidate == nil || id <= 0 {
			continue
		}
		severity := "P1"
		if candidate.status == "error" {
			severity = "P0"
		}
		if !shouldSendOpsAlertByMinSeverity(cfg.MinSeverity, severity) {
			continue
		}
		reminderKey := opsAccountIncidentReminderKey(candidate.fingerprint, account.UpdatedAt)
		deliveryKey := opsWeComDeliveryKey("account_status", strconv.FormatInt(id, 10), reminderKey)
		result := s.sendOpsWeComOnce(ctx, cfg, deliveryKey, buildOpsAccountWeComMarkdown(id, account, candidate, severity, now))
		if result.Sent {
			sent++
		}
	}
	return sent
}

func (s *OpsAlertEvaluatorService) evaluateProxyExpiryWeComNotifications(ctx context.Context, now time.Time, cfg *OpsWeComNotificationConfig) int {
	if s.proxyRepo == nil || !shouldSendOpsAlertByMinSeverity(cfg.MinSeverity, "P1") {
		return 0
	}
	proxies, err := s.proxyRepo.ListActiveWithAccountCount(ctx)
	if err != nil {
		return 0
	}
	sent := 0
	for i := range proxies {
		proxy := &proxies[i]
		if proxy.ID <= 0 || !isProxyInOpsExpiryReminderWindow(proxy.ExpiresAt, now) {
			continue
		}
		reminderKey := proxy.ExpiresAt.UTC().Format(time.RFC3339Nano)
		deliveryKey := opsWeComDeliveryKey("proxy_expiry", strconv.FormatInt(proxy.ID, 10), reminderKey)
		result := s.sendOpsWeComOnce(ctx, cfg, deliveryKey, buildOpsProxyExpiryWeComMarkdown(proxy, now))
		if result.Sent {
			sent++
		}
	}
	return sent
}

func buildOpsAccountWeComMarkdown(id int64, account *AccountAvailability, candidate *opsAccountNotificationCandidate, severity string, now time.Time) string {
	status := "错误"
	if candidate != nil && candidate.status == "temporarily_unschedulable" {
		status = "临时不可调度"
	}
	return fmt.Sprintf(
		"## Sub2API 账号异常通知\n> 级别：**%s**\n> 账号：%s（ID: %d）\n> 平台：%s\n> 账号类型：%s\n> 状态：%s\n> 错误信息：%s\n> 临时不可调度至：%s\n> 检测时间：%s",
		sanitizeOpsWeComField(severity),
		sanitizeOpsWeComField(account.AccountName),
		id,
		sanitizeOpsWeComField(account.Platform),
		sanitizeOpsWeComField(account.AccountType),
		status,
		sanitizeOpsWeComField(candidate.message),
		sanitizeOpsWeComField(candidate.until),
		now.UTC().Format(time.RFC3339),
	)
}

func buildOpsProxyExpiryWeComMarkdown(proxy *ProxyWithAccountCount, now time.Time) string {
	if proxy == nil {
		return "## Sub2API IP 到期提醒\n> IP：-"
	}
	expiresAt := "-"
	remaining := "-"
	if proxy.ExpiresAt != nil {
		expiresAt = proxy.ExpiresAt.UTC().Format(time.RFC3339)
		remaining = formatOpsRemainingTime(proxy.ExpiresAt.Sub(now))
	}
	return fmt.Sprintf(
		"## Sub2API IP 到期提醒\n> 级别：**P1**\n> IP：%s（ID: %d）\n> 剩余时间：%s\n> 到期时间：%s\n> IP 内账号数量：%d\n> 检测时间：%s",
		sanitizeOpsWeComField(proxy.Name),
		proxy.ID,
		sanitizeOpsWeComField(remaining),
		expiresAt,
		proxy.AccountCount,
		now.UTC().Format(time.RFC3339),
	)
}

func sanitizeOpsWeComField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.NewReplacer("\r", " ", "\n", " ", "<", "‹", ">", "›").Replace(value)
	return truncateUTF8Bytes(value, 1000)
}

type opsAccountNotificationCandidate struct {
	status      string
	message     string
	until       string
	fingerprint string
}

func buildOpsAccountNotificationCandidate(account *AccountAvailability, now time.Time) *opsAccountNotificationCandidate {
	if account == nil {
		return nil
	}
	status := ""
	message := strings.TrimSpace(account.ErrorMessage)
	until := "-"
	if account.HasError {
		status = "error"
	} else if account.TempUnschedulableUntil != nil && now.Before(*account.TempUnschedulableUntil) {
		status = "temporarily_unschedulable"
		if reason := strings.TrimSpace(account.TempUnschedulableReason); reason != "" {
			message = reason
		}
		until = account.TempUnschedulableUntil.UTC().Format(time.RFC3339)
	}
	if status == "" {
		return nil
	}
	if message == "" {
		message = "-"
	}
	fingerprint := strings.Join([]string{status, message}, "\x00")
	return &opsAccountNotificationCandidate{status: status, message: message, until: until, fingerprint: fingerprint}
}

func (s *OpsAlertEvaluatorService) evaluateAccountStatusNotifications(ctx context.Context, now time.Time, recipients []string, minSeverity string) int {
	availability, err := s.opsService.GetAccountAvailability(ctx, "", nil)
	if err != nil || availability == nil {
		return 0
	}
	live := make(map[int64]string)
	sent := 0
	for id, account := range availability.Accounts {
		candidate := buildOpsAccountNotificationCandidate(account, now)
		if candidate == nil || id <= 0 {
			continue
		}
		live[id] = candidate.fingerprint
		if s.accountNotificationState(id) == candidate.fingerprint {
			continue
		}
		severity := "P1"
		if candidate.status == "error" {
			severity = "P0"
		}
		if !shouldSendOpsAlertEmailByMinSeverity(minSeverity, severity) {
			continue
		}

		reminderKey := opsAccountIncidentReminderKey(candidate.fingerprint, account.UpdatedAt)
		input := NotificationEmailSendInput{
			Event:       NotificationEmailEventOpsAccountStatusAlert,
			SourceType:  "ops_account_status",
			SourceID:    strconv.FormatInt(id, 10),
			ReminderKey: reminderKey,
			Variables: map[string]string{
				"account_id":               strconv.FormatInt(id, 10),
				"account_name":             strings.TrimSpace(account.AccountName),
				"platform":                 strings.TrimSpace(account.Platform),
				"account_type":             strings.TrimSpace(account.AccountType),
				"account_status":           candidate.status,
				"account_error_message":    candidate.message,
				"temp_unschedulable_until": candidate.until,
				"triggered_at":             now.UTC().Format(time.RFC3339),
			},
		}
		if s.sendOpsResourceNotification(ctx, recipients, input) {
			s.setAccountNotificationState(id, candidate.fingerprint)
			sent++
		}
	}
	s.pruneAccountNotificationStates(live)
	return sent
}

func opsAccountIncidentReminderKey(fingerprint string, updatedAt time.Time) string {
	identity := fingerprint
	if !updatedAt.IsZero() {
		identity += "\x00" + updatedAt.UTC().Format(time.RFC3339Nano)
	}
	return notificationEmailHash(identity)
}

func (s *OpsAlertEvaluatorService) evaluateProxyExpiryNotifications(ctx context.Context, now time.Time, recipients []string, minSeverity string) int {
	if s.proxyRepo == nil {
		return 0
	}
	if !shouldSendOpsAlertEmailByMinSeverity(minSeverity, "P1") {
		return 0
	}
	proxies, err := s.proxyRepo.ListActiveWithAccountCount(ctx)
	if err != nil {
		return 0
	}
	live := make(map[int64]string)
	sent := 0
	for i := range proxies {
		proxy := &proxies[i]
		if proxy.ID <= 0 || !isProxyInOpsExpiryReminderWindow(proxy.ExpiresAt, now) {
			continue
		}
		fingerprint := proxy.ExpiresAt.UTC().Format(time.RFC3339Nano)
		live[proxy.ID] = fingerprint
		if s.proxyNotificationState(proxy.ID) == fingerprint {
			continue
		}

		input := NotificationEmailSendInput{
			Event:       NotificationEmailEventOpsProxyExpiryReminder,
			SourceType:  "ops_proxy_expiry",
			SourceID:    strconv.FormatInt(proxy.ID, 10),
			ReminderKey: fingerprint,
			Variables: map[string]string{
				"proxy_id":             strconv.FormatInt(proxy.ID, 10),
				"proxy_name":           strings.TrimSpace(proxy.Name),
				"proxy_expires_at":     proxy.ExpiresAt.UTC().Format(time.RFC3339),
				"proxy_remaining_time": formatOpsRemainingTime(proxy.ExpiresAt.Sub(now)),
				"proxy_account_count":  strconv.FormatInt(proxy.AccountCount, 10),
				"triggered_at":         now.UTC().Format(time.RFC3339),
			},
		}
		if s.sendOpsResourceNotification(ctx, recipients, input) {
			s.setProxyNotificationState(proxy.ID, fingerprint)
			sent++
		}
	}
	s.pruneProxyNotificationStates(live)
	return sent
}

func isProxyInOpsExpiryReminderWindow(expiresAt *time.Time, now time.Time) bool {
	return expiresAt != nil && expiresAt.After(now) && !expiresAt.After(now.Add(opsProxyExpiryReminderWindow))
}

func formatOpsRemainingTime(remaining time.Duration) string {
	if remaining <= 0 {
		return "0m"
	}
	totalMinutes := int64(math.Ceil(remaining.Minutes()))
	days := totalMinutes / (24 * 60)
	hours := (totalMinutes % (24 * 60)) / 60
	minutes := totalMinutes % 60
	parts := make([]string, 0, 3)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	return strings.Join(parts, " ")
}

func (s *OpsAlertEvaluatorService) sendOpsResourceNotification(ctx context.Context, recipients []string, input NotificationEmailSendInput) bool {
	allDelivered := true
	attempted := false
	notificationService := s.emailService.notificationEmailService
	for _, recipient := range normalizeEmails(recipients) {
		deliveryKey := notificationEmailDeliveryKey(input.Event, input.SourceType, input.SourceID, recipient, input.ReminderKey)
		legacyDeliveryKey := legacyNotificationEmailDeliveryKey(input.Event, input.SourceType, input.SourceID, recipient, input.ReminderKey)
		alreadyDelivered, err := notificationService.deliveryExists(ctx, deliveryKey, legacyDeliveryKey)
		if err != nil {
			allDelivered = false
			continue
		}
		if alreadyDelivered {
			attempted = true
			continue
		}
		if !s.emailLimiter.Allow(time.Now().UTC()) {
			allDelivered = false
			continue
		}
		attempted = true
		recipientInput := input
		recipientInput.RecipientEmail = recipient
		recipientInput.RecipientName = emailRecipientName(recipient)
		recipientInput.Locale = notificationService.ResolveRecipientLocale(ctx, 0, recipient)
		recipientInput.Variables = localizedOpsResourceVariables(input.Event, recipientInput.Locale, input.Variables)
		if err := notificationService.Send(ctx, recipientInput); err != nil {
			allDelivered = false
			logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] resource notification failed (event=%s recipient=%s): %v", input.Event, notificationEmailHash(recipient), err)
		}
	}
	return attempted && allDelivered
}

func localizedOpsResourceVariables(event, locale string, variables map[string]string) map[string]string {
	localized := make(map[string]string, len(variables))
	for key, value := range variables {
		localized[key] = value
	}
	if !strings.HasPrefix(strings.ToLower(locale), "zh") {
		return localized
	}
	switch event {
	case NotificationEmailEventOpsAccountStatusAlert:
		switch localized["account_status"] {
		case "error":
			localized["account_status"] = "错误"
		case "temporarily_unschedulable":
			localized["account_status"] = "临时不可调度"
		}
	case NotificationEmailEventOpsProxyExpiryReminder:
		localized["proxy_remaining_time"] = strings.NewReplacer(
			"d", " 天",
			"h", " 小时",
			"m", " 分钟",
		).Replace(localized["proxy_remaining_time"])
	}
	return localized
}

func (s *OpsAlertEvaluatorService) accountNotificationState(id int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.accountNotificationStates[id]
}

func (s *OpsAlertEvaluatorService) setAccountNotificationState(id int64, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accountNotificationStates == nil {
		s.accountNotificationStates = map[int64]string{}
	}
	s.accountNotificationStates[id] = state
}

func (s *OpsAlertEvaluatorService) pruneAccountNotificationStates(live map[int64]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.accountNotificationStates {
		if _, ok := live[id]; !ok {
			delete(s.accountNotificationStates, id)
		}
	}
}

func (s *OpsAlertEvaluatorService) clearAccountNotificationStates() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accountNotificationStates = map[int64]string{}
}

func (s *OpsAlertEvaluatorService) proxyNotificationState(id int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.proxyNotificationStates[id]
}

func (s *OpsAlertEvaluatorService) setProxyNotificationState(id int64, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.proxyNotificationStates == nil {
		s.proxyNotificationStates = map[int64]string{}
	}
	s.proxyNotificationStates[id] = state
}

func (s *OpsAlertEvaluatorService) pruneProxyNotificationStates(live map[int64]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.proxyNotificationStates {
		if _, ok := live[id]; !ok {
			delete(s.proxyNotificationStates, id)
		}
	}
}

func (s *OpsAlertEvaluatorService) clearProxyNotificationStates() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proxyNotificationStates = map[int64]string{}
}

func (s *OpsAlertEvaluatorService) pruneRuleStates(rules []*OpsAlertRule) {
	s.mu.Lock()
	defer s.mu.Unlock()

	live := map[int64]struct{}{}
	for _, r := range rules {
		if r != nil && r.ID > 0 {
			live[r.ID] = struct{}{}
		}
	}
	for id := range s.ruleStates {
		if _, ok := live[id]; !ok {
			delete(s.ruleStates, id)
		}
	}
}

func (s *OpsAlertEvaluatorService) resetRuleState(ruleID int64, now time.Time) {
	if ruleID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.ruleStates[ruleID]
	if !ok {
		state = &opsAlertRuleState{}
		s.ruleStates[ruleID] = state
	}
	state.LastEvaluatedAt = now
	state.ConsecutiveBreaches = 0
}

func (s *OpsAlertEvaluatorService) updateRuleBreaches(ruleID int64, now time.Time, interval time.Duration, breached bool) int {
	if ruleID <= 0 {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.ruleStates[ruleID]
	if !ok {
		state = &opsAlertRuleState{}
		s.ruleStates[ruleID] = state
	}

	if !state.LastEvaluatedAt.IsZero() && interval > 0 {
		if now.Sub(state.LastEvaluatedAt) > interval*2 {
			state.ConsecutiveBreaches = 0
		}
	}

	state.LastEvaluatedAt = now
	if breached {
		state.ConsecutiveBreaches++
	} else {
		state.ConsecutiveBreaches = 0
	}
	return state.ConsecutiveBreaches
}

func requiredSustainedBreaches(sustainedMinutes int, interval time.Duration) int {
	if sustainedMinutes <= 0 {
		return 1
	}
	if interval <= 0 {
		return sustainedMinutes
	}
	required := int(math.Ceil(float64(sustainedMinutes*60) / interval.Seconds()))
	if required < 1 {
		return 1
	}
	return required
}

func parseOpsAlertRuleScope(filters map[string]any) (platform string, groupID *int64, region *string) {
	if filters == nil {
		return "", nil, nil
	}
	if v, ok := filters["platform"]; ok {
		if s, ok := v.(string); ok {
			platform = strings.TrimSpace(s)
		}
	}
	if v, ok := filters["group_id"]; ok {
		switch t := v.(type) {
		case float64:
			if t > 0 {
				id := int64(t)
				groupID = &id
			}
		case int64:
			if t > 0 {
				id := t
				groupID = &id
			}
		case int:
			if t > 0 {
				id := int64(t)
				groupID = &id
			}
		case string:
			n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
			if err == nil && n > 0 {
				groupID = &n
			}
		}
	}
	if v, ok := filters["region"]; ok {
		if s, ok := v.(string); ok {
			vv := strings.TrimSpace(s)
			if vv != "" {
				region = &vv
			}
		}
	}
	return platform, groupID, region
}

func (s *OpsAlertEvaluatorService) computeRuleMetric(
	ctx context.Context,
	rule *OpsAlertRule,
	systemMetrics *OpsSystemMetricsSnapshot,
	start time.Time,
	end time.Time,
	platform string,
	groupID *int64,
) (float64, bool) {
	if rule == nil {
		return 0, false
	}
	switch strings.TrimSpace(rule.MetricType) {
	case "cpu_usage_percent":
		if systemMetrics != nil && systemMetrics.CPUUsagePercent != nil {
			return *systemMetrics.CPUUsagePercent, true
		}
		return 0, false
	case "memory_usage_percent":
		if systemMetrics != nil && systemMetrics.MemoryUsagePercent != nil {
			return *systemMetrics.MemoryUsagePercent, true
		}
		return 0, false
	case "concurrency_queue_depth":
		if systemMetrics != nil && systemMetrics.ConcurrencyQueueDepth != nil {
			return float64(*systemMetrics.ConcurrencyQueueDepth), true
		}
		return 0, false
	case "group_available_accounts":
		if groupID == nil || *groupID <= 0 {
			return 0, false
		}
		if s == nil || s.opsService == nil {
			return 0, false
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, false
		}
		if availability.Group == nil {
			return 0, true
		}
		return float64(availability.Group.AvailableCount), true
	case "group_available_ratio":
		if groupID == nil || *groupID <= 0 {
			return 0, false
		}
		if s == nil || s.opsService == nil {
			return 0, false
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, false
		}
		return computeGroupAvailableRatio(availability.Group), true
	case "account_rate_limited_count":
		if s == nil || s.opsService == nil {
			return 0, false
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, false
		}
		return float64(countAccountsByCondition(availability.Accounts, func(acc *AccountAvailability) bool {
			return acc.IsRateLimited
		})), true
	case "account_error_count":
		if s == nil || s.opsService == nil {
			return 0, false
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, false
		}
		return float64(countAccountsByCondition(availability.Accounts, func(acc *AccountAvailability) bool {
			return acc.HasError && acc.TempUnschedulableUntil == nil
		})), true
	case "account_temp_unscheduled_count":
		if s == nil || s.opsService == nil {
			return 0, false
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, false
		}
		now := time.Now().UTC()
		return float64(countAccountsByCondition(availability.Accounts, func(acc *AccountAvailability) bool {
			return acc.TempUnschedulableUntil != nil && now.Before(*acc.TempUnschedulableUntil)
		})), true
	case "group_rate_limit_ratio":
		if groupID == nil || *groupID <= 0 {
			return 0, false
		}
		if s == nil || s.opsService == nil {
			return 0, false
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, false
		}
		if availability.Group == nil || availability.Group.TotalAccounts <= 0 {
			return 0, true
		}
		return (float64(availability.Group.RateLimitCount) / float64(availability.Group.TotalAccounts)) * 100, true
	case "account_error_ratio":
		if s == nil || s.opsService == nil {
			return 0, false
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, false
		}
		total := int64(len(availability.Accounts))
		if total <= 0 {
			return 0, true
		}
		errorCount := countAccountsByCondition(availability.Accounts, func(acc *AccountAvailability) bool {
			return acc.HasError && acc.TempUnschedulableUntil == nil
		})
		return (float64(errorCount) / float64(total)) * 100, true
	case "overload_account_count":
		if s == nil || s.opsService == nil {
			return 0, false
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, false
		}
		return float64(countAccountsByCondition(availability.Accounts, func(acc *AccountAvailability) bool {
			return acc.IsOverloaded
		})), true
	case "proxy_expired_count":
		if s == nil || s.proxyRepo == nil {
			return 0, false
		}
		n, err := s.proxyRepo.CountExpired(ctx)
		if err != nil {
			return 0, false
		}
		return float64(n), true
	case "proxy_expiring_soon_count":
		if s == nil || s.proxyRepo == nil {
			return 0, false
		}
		n, err := s.proxyRepo.CountExpiringSoon(ctx, time.Now())
		if err != nil {
			return 0, false
		}
		return float64(n), true
	}

	overview, err := s.opsRepo.GetDashboardOverview(ctx, &OpsDashboardFilter{
		StartTime: start,
		EndTime:   end,
		Platform:  platform,
		GroupID:   groupID,
		QueryMode: OpsQueryModeRaw,
	})
	if err != nil {
		return 0, false
	}
	if overview == nil {
		return 0, false
	}

	switch strings.TrimSpace(rule.MetricType) {
	case "success_rate":
		if overview.RequestCountSLA <= 0 {
			return 0, false
		}
		return overview.SLA * 100, true
	case "error_rate":
		if overview.RequestCountSLA <= 0 {
			return 0, false
		}
		return overview.ErrorRate * 100, true
	case "upstream_error_rate":
		if overview.RequestCountSLA <= 0 {
			return 0, false
		}
		return overview.UpstreamErrorRate * 100, true
	default:
		return 0, false
	}
}

func compareMetric(value float64, operator string, threshold float64) bool {
	switch strings.TrimSpace(operator) {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

func buildOpsAlertDimensions(platform string, groupID *int64) map[string]any {
	dims := map[string]any{}
	if strings.TrimSpace(platform) != "" {
		dims["platform"] = strings.TrimSpace(platform)
	}
	if groupID != nil && *groupID > 0 {
		dims["group_id"] = *groupID
	}
	if len(dims) == 0 {
		return nil
	}
	return dims
}

func buildOpsAlertDescription(rule *OpsAlertRule, value float64, windowMinutes int, platform string, groupID *int64) string {
	if rule == nil {
		return ""
	}
	scope := "overall"
	if strings.TrimSpace(platform) != "" {
		scope = fmt.Sprintf("platform=%s", strings.TrimSpace(platform))
	}
	if groupID != nil && *groupID > 0 {
		scope = fmt.Sprintf("%s group_id=%d", scope, *groupID)
	}
	if windowMinutes <= 0 {
		windowMinutes = 1
	}
	return fmt.Sprintf("%s %s %.2f (current %.2f) over last %dm (%s)",
		strings.TrimSpace(rule.MetricType),
		strings.TrimSpace(rule.Operator),
		rule.Threshold,
		value,
		windowMinutes,
		strings.TrimSpace(scope),
	)
}

func (s *OpsAlertEvaluatorService) maybeSendAlertEmail(ctx context.Context, runtimeCfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent) bool {
	if s == nil || s.emailService == nil || s.opsService == nil || event == nil || rule == nil {
		return false
	}
	if event.EmailSent {
		return false
	}
	if !rule.NotifyEmail {
		return false
	}

	emailCfg, err := s.opsService.GetEmailNotificationConfig(ctx)
	if err != nil || emailCfg == nil || !emailCfg.Alert.Enabled {
		return false
	}

	if len(emailCfg.Alert.Recipients) == 0 {
		return false
	}
	if !shouldSendOpsAlertEmailByMinSeverity(strings.TrimSpace(emailCfg.Alert.MinSeverity), strings.TrimSpace(rule.Severity)) {
		return false
	}

	if runtimeCfg != nil && runtimeCfg.Silencing.Enabled {
		if isOpsAlertSilenced(time.Now().UTC(), rule, event, runtimeCfg.Silencing) {
			return false
		}
	}

	// Apply/update rate limiter.
	s.emailLimiter.SetLimit(emailCfg.Alert.RateLimitPerHour)

	subject := fmt.Sprintf("[Ops Alert][%s] %s", strings.TrimSpace(rule.Severity), strings.TrimSpace(rule.Name))
	body := buildOpsAlertEmailBody(rule, event)

	anySent := false
	for _, to := range emailCfg.Alert.Recipients {
		addr := strings.TrimSpace(to)
		if addr == "" {
			continue
		}
		if !s.emailLimiter.Allow(time.Now().UTC()) {
			continue
		}
		if s.emailService.notificationEmailService != nil {
			if err := s.emailService.notificationEmailService.Send(ctx, NotificationEmailSendInput{
				Event:          NotificationEmailEventOpsAlert,
				RecipientEmail: addr,
				RecipientName:  emailRecipientName(addr),
				SourceType:     "ops_alert",
				SourceID:       fmt.Sprintf("%d", event.ID),
				Variables:      opsAlertEmailVariables(rule, event),
			}); err == nil {
				anySent = true
				continue
			} else if !shouldFallbackNotificationEmail(err) {
				continue
			}
		}
		if err := s.emailService.SendEmail(ctx, addr, subject, body); err != nil {
			// Ignore per-recipient failures; continue best-effort.
			continue
		}
		anySent = true
	}

	if anySent {
		_ = s.opsRepo.UpdateAlertEventEmailSent(context.Background(), event.ID, true)
	}
	return anySent
}

func (s *OpsAlertEvaluatorService) maybeSendAlertWeCom(ctx context.Context, runtimeCfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent) bool {
	if s == nil || s.opsService == nil || s.opsService.weComNotifier == nil || s.opsRepo == nil || event == nil || rule == nil || !rule.NotifyWeCom {
		return false
	}
	isResolved := event.Status == OpsAlertStatusResolved || event.Status == OpsAlertStatusManualResolved
	if !isResolved && event.WeComSent {
		return false
	}
	cfg, err := s.opsService.GetWeComNotificationConfig(ctx)
	if err != nil || cfg == nil || !cfg.Enabled || strings.TrimSpace(cfg.WebhookURL) == "" {
		return false
	}
	if isResolved && !cfg.IncludeResolvedAlerts {
		return false
	}
	if !shouldSendOpsAlertByMinSeverity(cfg.MinSeverity, rule.Severity) {
		return false
	}
	if runtimeCfg != nil && runtimeCfg.Silencing.Enabled && isOpsAlertSilenced(time.Now().UTC(), rule, event, runtimeCfg.Silencing) {
		return false
	}

	phase := "firing"
	if isResolved {
		phase = "resolved"
	}
	s.weComLimiter.SetLimit(cfg.RateLimitPerHour)
	deliveryKey := opsWeComDeliveryKey("alert", phase, strconv.FormatInt(event.ID, 10))
	result := s.sendOpsWeComOnce(ctx, cfg, deliveryKey, buildOpsAlertWeComMarkdown(rule, event))
	if result.Delivered && !event.WeComSent {
		if err := s.opsRepo.UpdateAlertEventWeComSent(context.Background(), event.ID, true); err != nil {
			logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] update WeCom delivery status failed (event=%d): %v", event.ID, err)
		} else {
			event.WeComSent = true
		}
	}
	return result.Sent
}

func shouldSendOpsAlertByMinSeverity(minSeverity, severity string) bool {
	minSeverity = strings.ToUpper(strings.TrimSpace(minSeverity))
	severity = strings.ToUpper(strings.TrimSpace(severity))
	if minSeverity == "" {
		return true
	}
	rank := map[string]int{"P0": 0, "P1": 1, "P2": 2, "P3": 3}
	minRank, minOK := rank[minSeverity]
	severityRank, severityOK := rank[severity]
	return minOK && severityOK && severityRank <= minRank
}

func buildOpsAlertWeComMarkdown(rule *OpsAlertRule, event *OpsAlertEvent) string {
	status := "触发"
	if event != nil && (event.Status == OpsAlertStatusResolved || event.Status == OpsAlertStatusManualResolved) {
		status = "恢复"
	}
	name, severity, description := "-", "-", "-"
	if rule != nil {
		name = sanitizeOpsWeComField(rule.Name)
		severity = sanitizeOpsWeComField(rule.Severity)
	}
	if event != nil {
		description = sanitizeOpsWeComField(event.Description)
	}
	content := fmt.Sprintf("## Sub2API 运维预警%s\n> 级别：**%s**\n> 规则：%s\n> 详情：%s", status, severity, name, description)
	if event != nil && !event.FiredAt.IsZero() {
		content += "\n> 触发时间：" + event.FiredAt.UTC().Format(time.RFC3339)
	}
	if event != nil && event.ResolvedAt != nil {
		content += "\n> 恢复时间：" + event.ResolvedAt.UTC().Format(time.RFC3339)
	}
	return content
}

func opsAlertEmailVariables(rule *OpsAlertRule, event *OpsAlertEvent) map[string]string {
	variables := map[string]string{
		"rule_name":         "-",
		"severity":          "-",
		"alert_status":      "-",
		"metric_type":       "-",
		"operator":          "-",
		"metric_value":      "-",
		"threshold_value":   "-",
		"triggered_at":      time.Now().UTC().Format(time.RFC3339),
		"alert_description": "-",
	}
	if rule != nil {
		variables["rule_name"] = strings.TrimSpace(rule.Name)
		variables["severity"] = strings.TrimSpace(rule.Severity)
		variables["metric_type"] = strings.TrimSpace(rule.MetricType)
		variables["operator"] = strings.TrimSpace(rule.Operator)
		variables["threshold_value"] = fmt.Sprintf("%.2f", rule.Threshold)
		if strings.TrimSpace(rule.Description) != "" {
			variables["alert_description"] = strings.TrimSpace(rule.Description)
		}
	}
	if event != nil {
		variables["alert_status"] = strings.TrimSpace(event.Status)
		if event.MetricValue != nil {
			variables["metric_value"] = fmt.Sprintf("%.2f", *event.MetricValue)
		}
		if event.ThresholdValue != nil {
			variables["threshold_value"] = fmt.Sprintf("%.2f", *event.ThresholdValue)
		}
		if !event.FiredAt.IsZero() {
			variables["triggered_at"] = event.FiredAt.UTC().Format(time.RFC3339)
		}
		if strings.TrimSpace(event.Description) != "" {
			variables["alert_description"] = strings.TrimSpace(event.Description)
		}
	}
	return variables
}

func buildOpsAlertEmailBody(rule *OpsAlertRule, event *OpsAlertEvent) string {
	if rule == nil || event == nil {
		return ""
	}
	metric := strings.TrimSpace(rule.MetricType)
	value := "-"
	threshold := fmt.Sprintf("%.2f", rule.Threshold)
	if event.MetricValue != nil {
		value = fmt.Sprintf("%.2f", *event.MetricValue)
	}
	if event.ThresholdValue != nil {
		threshold = fmt.Sprintf("%.2f", *event.ThresholdValue)
	}
	return fmt.Sprintf(`
<h2>Ops Alert</h2>
<p><b>Rule</b>: %s</p>
<p><b>Severity</b>: %s</p>
<p><b>Status</b>: %s</p>
<p><b>Metric</b>: %s %s %s</p>
<p><b>Fired at</b>: %s</p>
<p><b>Description</b>: %s</p>
`,
		htmlEscape(rule.Name),
		htmlEscape(rule.Severity),
		htmlEscape(event.Status),
		htmlEscape(metric),
		htmlEscape(rule.Operator),
		htmlEscape(fmt.Sprintf("%s (threshold %s)", value, threshold)),
		event.FiredAt.Format(time.RFC3339),
		htmlEscape(event.Description),
	)
}

func shouldSendOpsAlertEmailByMinSeverity(minSeverity string, ruleSeverity string) bool {
	minSeverity = strings.ToLower(strings.TrimSpace(minSeverity))
	if minSeverity == "" {
		return true
	}

	eventLevel := opsEmailSeverityForOps(ruleSeverity)
	minLevel := strings.ToLower(minSeverity)

	rank := func(level string) int {
		switch level {
		case "critical":
			return 3
		case "warning":
			return 2
		case "info":
			return 1
		default:
			return 0
		}
	}
	return rank(eventLevel) >= rank(minLevel)
}

func opsEmailSeverityForOps(severity string) string {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "P0":
		return "critical"
	case "P1":
		return "warning"
	default:
		return "info"
	}
}

func isOpsAlertSilenced(now time.Time, rule *OpsAlertRule, event *OpsAlertEvent, silencing OpsAlertSilencingSettings) bool {
	if !silencing.Enabled {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if strings.TrimSpace(silencing.GlobalUntilRFC3339) != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(silencing.GlobalUntilRFC3339)); err == nil {
			if now.Before(t) {
				return true
			}
		}
	}

	for _, entry := range silencing.Entries {
		untilRaw := strings.TrimSpace(entry.UntilRFC3339)
		if untilRaw == "" {
			continue
		}
		until, err := time.Parse(time.RFC3339, untilRaw)
		if err != nil {
			continue
		}
		if now.After(until) {
			continue
		}
		if entry.RuleID != nil && rule != nil && rule.ID > 0 && *entry.RuleID != rule.ID {
			continue
		}
		if len(entry.Severities) > 0 {
			match := false
			for _, s := range entry.Severities {
				if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(event.Severity)) || strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(rule.Severity)) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		return true
	}

	return false
}

func (s *OpsAlertEvaluatorService) tryAcquireLeaderLock(ctx context.Context, lock OpsDistributedLockSettings) (func(), bool) {
	if !lock.Enabled {
		return nil, true
	}
	if s.redisClient == nil {
		s.warnNoRedisOnce.Do(func() {
			logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] redis not configured; running without distributed lock")
		})
		return nil, true
	}
	key := strings.TrimSpace(lock.Key)
	if key == "" {
		key = opsAlertEvaluatorLeaderLockKey
	}
	ttl := time.Duration(lock.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = opsAlertEvaluatorLeaderLockTTL
	}

	ok, err := s.redisClient.SetNX(ctx, key, s.instanceID, ttl).Result()
	if err != nil {
		// Prefer fail-closed to avoid duplicate evaluators stampeding the DB when Redis is flaky.
		// Single-node deployments can disable the distributed lock via runtime settings.
		s.warnNoRedisOnce.Do(func() {
			logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] leader lock SetNX failed; skipping this cycle: %v", err)
		})
		return nil, false
	}
	if !ok {
		s.maybeLogSkip(key)
		return nil, false
	}
	return func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		_, _ = opsAlertEvaluatorReleaseScript.Run(releaseCtx, s.redisClient, []string{key}, s.instanceID).Result()
	}, true
}

func (s *OpsAlertEvaluatorService) maybeLogSkip(key string) {
	s.skipLogMu.Lock()
	defer s.skipLogMu.Unlock()

	now := time.Now()
	if !s.skipLogAt.IsZero() && now.Sub(s.skipLogAt) < opsAlertEvaluatorSkipLogInterval {
		return
	}
	s.skipLogAt = now
	logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] leader lock held by another instance; skipping (key=%q)", key)
}

func (s *OpsAlertEvaluatorService) recordHeartbeatSuccess(runAt time.Time, duration time.Duration, result string) {
	if s == nil || s.opsRepo == nil {
		return
	}
	now := time.Now().UTC()
	durMs := duration.Milliseconds()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	msg := strings.TrimSpace(result)
	if msg == "" {
		msg = "ok"
	}
	msg = truncateString(msg, 2048)
	_ = s.opsRepo.UpsertJobHeartbeat(ctx, &OpsUpsertJobHeartbeatInput{
		JobName:        opsAlertEvaluatorJobName,
		LastRunAt:      &runAt,
		LastSuccessAt:  &now,
		LastDurationMs: &durMs,
		LastResult:     &msg,
	})
}

func (s *OpsAlertEvaluatorService) recordHeartbeatError(runAt time.Time, duration time.Duration, err error) {
	if s == nil || s.opsRepo == nil || err == nil {
		return
	}
	now := time.Now().UTC()
	durMs := duration.Milliseconds()
	msg := truncateString(err.Error(), 2048)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.opsRepo.UpsertJobHeartbeat(ctx, &OpsUpsertJobHeartbeatInput{
		JobName:        opsAlertEvaluatorJobName,
		LastRunAt:      &runAt,
		LastErrorAt:    &now,
		LastError:      &msg,
		LastDurationMs: &durMs,
	})
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

type slidingWindowLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	sent   []time.Time
}

func newSlidingWindowLimiter(limit int, window time.Duration) *slidingWindowLimiter {
	if window <= 0 {
		window = time.Hour
	}
	return &slidingWindowLimiter{
		limit:  limit,
		window: window,
		sent:   []time.Time{},
	}
}

func (l *slidingWindowLimiter) SetLimit(limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limit = limit
}

func (l *slidingWindowLimiter) Allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.limit <= 0 {
		return true
	}
	cutoff := now.Add(-l.window)
	keep := l.sent[:0]
	for _, t := range l.sent {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	l.sent = keep
	if len(l.sent) >= l.limit {
		return false
	}
	l.sent = append(l.sent, now)
	return true
}

// computeGroupAvailableRatio returns the available percentage for a group.
// Formula: (AvailableCount / TotalAccounts) * 100.
// Returns 0 when TotalAccounts is 0.
func computeGroupAvailableRatio(group *GroupAvailability) float64 {
	if group == nil || group.TotalAccounts <= 0 {
		return 0
	}
	return (float64(group.AvailableCount) / float64(group.TotalAccounts)) * 100
}

// countAccountsByCondition counts accounts that satisfy the given condition.
func countAccountsByCondition(accounts map[int64]*AccountAvailability, condition func(*AccountAvailability) bool) int64 {
	if len(accounts) == 0 || condition == nil {
		return 0
	}
	var count int64
	for _, account := range accounts {
		if account != nil && condition(account) {
			count++
		}
	}
	return count
}

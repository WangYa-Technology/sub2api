package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	opsCleanupDefaultSchedule  = "0 2 * * *"
	opsCleanupBatchSize        = 5000
	opsCleanupCronStopTimeout  = 3 * time.Second
	opsCleanupRunTimeout       = 30 * time.Minute
	opsCleanupHeartbeatTimeout = 2 * time.Second
)

type opsCleanupTarget struct {
	retentionDays int
	table         string
	timeCol       string
	castDate      bool
	counter       *int64
}

type opsCleanupDeletedCounts struct {
	errorLogs          int64
	ingressRejects     int64
	alertEvents        int64
	systemLogs         int64
	logAudits          int64
	systemMetrics      int64
	hourlyPreagg       int64
	dailyPreagg        int64
	weComDeliveryKeys  int64
}

func (c opsCleanupDeletedCounts) String() string {
	return fmt.Sprintf(
		"error_logs=%d ingress_rejects=%d alert_events=%d system_logs=%d log_audits=%d system_metrics=%d hourly_preagg=%d daily_preagg=%d wecom_delivery_keys=%d",
		c.errorLogs,
		c.ingressRejects,
		c.alertEvents,
		c.systemLogs,
		c.logAudits,
		c.systemMetrics,
		c.hourlyPreagg,
		c.dailyPreagg,
		c.weComDeliveryKeys,
	)
}

// opsCleanupPlan 把"保留天数"翻译成具体的清理动作。
//   - days < 0  → 跳过该项清理（ok=false），保留兼容老数据
//   - days == 0 → TRUNCATE TABLE（O(1) 全清），truncate=true
//   - days > 0  → 批量 DELETE 早于 now-N天 的行，cutoff = now - N 天
func opsCleanupPlan(now time.Time, days int) (cutoff time.Time, truncate, ok bool) {
	if days < 0 {
		return time.Time{}, false, false
	}
	if days == 0 {
		return time.Time{}, true, true
	}
	return now.AddDate(0, 0, -days), false, true
}

func opsCleanupRunOne(
	ctx context.Context,
	db *sql.DB,
	truncate bool,
	cutoff time.Time,
	table, timeCol string,
	castDate bool,
	batchSize int,
) (int64, error) {
	if truncate {
		return truncateOpsTable(ctx, db, table)
	}
	return deleteOldRowsByID(ctx, db, table, timeCol, cutoff, batchSize, castDate)
}

func deleteOldRowsByID(
	ctx context.Context,
	db *sql.DB,
	table string,
	timeColumn string,
	cutoff time.Time,
	batchSize int,
	castCutoffToDate bool,
) (int64, error) {
	if db == nil {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = opsCleanupBatchSize
	}

	where := fmt.Sprintf("%s < $1", timeColumn)
	if castCutoffToDate {
		where = fmt.Sprintf("%s < $1::date", timeColumn)
	}

	q := fmt.Sprintf(`
WITH batch AS (
  SELECT id FROM %s
  WHERE %s
  ORDER BY id
  LIMIT $2
)
DELETE FROM %s
WHERE id IN (SELECT id FROM batch)
`, table, where, table)

	var total int64
	for {
		res, err := db.ExecContext(ctx, q, cutoff, batchSize)
		if err != nil {
			if isMissingRelationError(err) {
				return total, nil
			}
			return total, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += affected
		if affected == 0 {
			break
		}
	}
	return total, nil
}

// truncateOpsTable 用 TRUNCATE TABLE 清空指定表，先 SELECT COUNT(*) 取得清空前行数用于 heartbeat。
func truncateOpsTable(ctx context.Context, db *sql.DB, table string) (int64, error) {
	if db == nil {
		return 0, nil
	}
	var count int64
	if err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
		if isMissingRelationError(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("count %s: %w", table, err)
	}
	if count == 0 {
		return 0, nil
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s", table)); err != nil {
		if isMissingRelationError(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("truncate %s: %w", table, err)
	}
	return count, nil
}

// deleteOldWeComDeliveryKeys 清理 settings 表中 WeCom 通知去重键
// （key 以 opsWeComDeliveryKeyPrefix 开头的记录，由 ops_wecom_notification.go
// 的 opsWeComDeliveryKey 写入）。这类记录永不更新、不参与业务读取，长期运行
// 会让 settings 表无限膨胀，因此由日常清理按保留期定向删除。
//
// 去重键与告警事件（ops_alert_events）同生命周期：事件被清理后去重键即孤儿，
// 故复用 ErrorLogRetentionDays 作为保留期。
//
// 语义与 opsCleanupPlan 一致：
//   - retentionDays < 0：跳过（保持与其他表一致）
//   - retentionDays == 0：清空全部去重键（用远古 cutoff 实现，见下）
//   - retentionDays > 0：删除 updated_at 早于 now-N 天的记录
//
// 注意：settings 表同时存放运行配置（webhook、汇率等），绝不能用 TRUNCATE 全清，
// 因此本函数不参与 opsCleanupRunOne 的 truncate 分支，只按前缀定向删除。
func deleteOldWeComDeliveryKeys(
	ctx context.Context,
	db *sql.DB,
	retentionDays int,
	batchSize int,
) (int64, error) {
	if db == nil {
		return 0, nil
	}
	if retentionDays < 0 {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = opsCleanupBatchSize
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	if retentionDays == 0 {
		// 清理全部去重键：settings 表不可能存在早于 Unix 纪元的时间戳，
		// 用远古 cutoff 等效全量删除，避免对整表 TRUNCATE。
		cutoff = time.Unix(0, 0).UTC()
	}

	q := `
WITH batch AS (
  SELECT id FROM settings
  WHERE key LIKE $1
    AND updated_at < $2
  ORDER BY id
  LIMIT $3
)
DELETE FROM settings
WHERE id IN (SELECT id FROM batch)`

	var total int64
	for {
		res, err := db.ExecContext(ctx, q, opsWeComDeliveryKeyPrefix+"%", cutoff, batchSize)
		if err != nil {
			if isMissingRelationError(err) {
				return total, nil
			}
			return total, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += affected
		if affected == 0 {
			break
		}
	}
	return total, nil
}

func isMissingRelationError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "does not exist") && strings.Contains(s, "relation")
}

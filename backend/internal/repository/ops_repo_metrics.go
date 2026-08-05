package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *opsRepository) InsertSystemMetrics(ctx context.Context, input *service.OpsInsertSystemMetricsInput) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if input == nil {
		return fmt.Errorf("nil input")
	}

	window := input.WindowMinutes
	if window <= 0 {
		window = 1
	}
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	q := `
INSERT INTO ops_system_metrics (
  created_at,
  window_minutes,
  platform,
  group_id,
  source_node_id,
  source_region,
  source_hostname,

  success_count,
  error_count_total,
  business_limited_count,
  error_count_sla,

  upstream_error_count_excl_429_529,
  upstream_429_count,
  upstream_529_count,

  token_consumed,
  account_switch_count,
  qps,
  tps,

  duration_p50_ms,
  duration_p90_ms,
  duration_p95_ms,
  duration_p99_ms,
  duration_avg_ms,
  duration_max_ms,

  ttft_p50_ms,
  ttft_p90_ms,
  ttft_p95_ms,
  ttft_p99_ms,
  ttft_avg_ms,
  ttft_max_ms,

  cpu_usage_percent,
  memory_used_mb,
  memory_total_mb,
  memory_usage_percent,

  db_ok,
  redis_ok,

  redis_conn_total,
  redis_conn_idle,

  db_conn_active,
  db_conn_idle,
  db_conn_waiting,

  goroutine_count,
  concurrency_queue_depth
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,
  $8,$9,$10,$11,
  $12,$13,$14,
  $15,$16,$17,$18,
  $19,$20,$21,$22,$23,$24,
  $25,$26,$27,$28,$29,$30,
  $31,$32,$33,$34,
  $35,$36,
  $37,$38,
  $39,$40,$41,
  $42,$43
)`

	_, err := r.db.ExecContext(
		ctx,
		q,
		createdAt,
		window,
		opsNullString(input.Platform),
		opsNullInt64(input.GroupID),
		opsNullString(input.SourceNodeID),
		opsNullString(input.SourceRegion),
		opsNullString(input.SourceHostname),

		input.SuccessCount,
		input.ErrorCountTotal,
		input.BusinessLimitedCount,
		input.ErrorCountSLA,

		input.UpstreamErrorCountExcl429529,
		input.Upstream429Count,
		input.Upstream529Count,

		input.TokenConsumed,
		input.AccountSwitchCount,
		opsNullFloat64(input.QPS),
		opsNullFloat64(input.TPS),

		opsNullableIntPointer(input.DurationP50Ms),
		opsNullableIntPointer(input.DurationP90Ms),
		opsNullableIntPointer(input.DurationP95Ms),
		opsNullableIntPointer(input.DurationP99Ms),
		opsNullFloat64(input.DurationAvgMs),
		opsNullableIntPointer(input.DurationMaxMs),

		opsNullableIntPointer(input.TTFTP50Ms),
		opsNullableIntPointer(input.TTFTP90Ms),
		opsNullableIntPointer(input.TTFTP95Ms),
		opsNullableIntPointer(input.TTFTP99Ms),
		opsNullFloat64(input.TTFTAvgMs),
		opsNullableIntPointer(input.TTFTMaxMs),

		opsNullFloat64(input.CPUUsagePercent),
		opsNullableInt64Pointer(input.MemoryUsedMB),
		opsNullableInt64Pointer(input.MemoryTotalMB),
		opsNullFloat64(input.MemoryUsagePercent),

		opsNullBool(input.DBOK),
		opsNullBool(input.RedisOK),

		opsNullableIntPointer(input.RedisConnTotal),
		opsNullableIntPointer(input.RedisConnIdle),

		opsNullableIntPointer(input.DBConnActive),
		opsNullableIntPointer(input.DBConnIdle),
		opsNullableIntPointer(input.DBConnWaiting),

		opsNullableIntPointer(input.GoroutineCount),
		opsNullableIntPointer(input.ConcurrencyQueueDepth),
	)
	return err
}

func (r *opsRepository) GetLatestSystemMetrics(ctx context.Context, windowMinutes int) (*service.OpsSystemMetricsSnapshot, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if windowMinutes <= 0 {
		windowMinutes = 1
	}

	q := `
SELECT
  id,
  created_at,
  window_minutes,
  source_node_id,
  source_region,
  source_hostname,

  cpu_usage_percent,
  memory_used_mb,
  memory_total_mb,
  memory_usage_percent,

  db_ok,
  redis_ok,

  redis_conn_total,
  redis_conn_idle,

  db_conn_active,
  db_conn_idle,
  db_conn_waiting,

  goroutine_count,
  concurrency_queue_depth,
  account_switch_count
FROM ops_system_metrics
WHERE window_minutes = $1
  AND platform IS NULL
  AND group_id IS NULL
ORDER BY created_at DESC, id DESC
LIMIT 1`

	var out service.OpsSystemMetricsSnapshot
	var sourceNodeID sql.NullString
	var sourceRegion sql.NullString
	var sourceHostname sql.NullString
	var cpu sql.NullFloat64
	var memUsed sql.NullInt64
	var memTotal sql.NullInt64
	var memPct sql.NullFloat64
	var dbOK sql.NullBool
	var redisOK sql.NullBool
	var redisTotal sql.NullInt64
	var redisIdle sql.NullInt64
	var dbActive sql.NullInt64
	var dbIdle sql.NullInt64
	var dbWaiting sql.NullInt64
	var goroutines sql.NullInt64
	var queueDepth sql.NullInt64
	var accountSwitchCount sql.NullInt64

	if err := r.db.QueryRowContext(ctx, q, windowMinutes).Scan(
		&out.ID,
		&out.CreatedAt,
		&out.WindowMinutes,
		&sourceNodeID,
		&sourceRegion,
		&sourceHostname,
		&cpu,
		&memUsed,
		&memTotal,
		&memPct,
		&dbOK,
		&redisOK,
		&redisTotal,
		&redisIdle,
		&dbActive,
		&dbIdle,
		&dbWaiting,
		&goroutines,
		&queueDepth,
		&accountSwitchCount,
	); err != nil {
		return nil, err
	}
	if sourceNodeID.Valid {
		out.SourceNodeID = sourceNodeID.String
	}
	if sourceRegion.Valid {
		out.SourceRegion = sourceRegion.String
	}
	if sourceHostname.Valid {
		out.SourceHostname = sourceHostname.String
	}

	if cpu.Valid {
		v := cpu.Float64
		out.CPUUsagePercent = &v
	}
	if memUsed.Valid {
		v := memUsed.Int64
		out.MemoryUsedMB = &v
	}
	if memTotal.Valid {
		v := memTotal.Int64
		out.MemoryTotalMB = &v
	}
	if memPct.Valid {
		v := memPct.Float64
		out.MemoryUsagePercent = &v
	}
	if dbOK.Valid {
		v := dbOK.Bool
		out.DBOK = &v
	}
	if redisOK.Valid {
		v := redisOK.Bool
		out.RedisOK = &v
	}
	if redisTotal.Valid {
		v := int(redisTotal.Int64)
		out.RedisConnTotal = &v
	}
	if redisIdle.Valid {
		v := int(redisIdle.Int64)
		out.RedisConnIdle = &v
	}
	if dbActive.Valid {
		v := int(dbActive.Int64)
		out.DBConnActive = &v
	}
	if dbIdle.Valid {
		v := int(dbIdle.Int64)
		out.DBConnIdle = &v
	}
	if dbWaiting.Valid {
		v := int(dbWaiting.Int64)
		out.DBConnWaiting = &v
	}
	if goroutines.Valid {
		v := int(goroutines.Int64)
		out.GoroutineCount = &v
	}
	if queueDepth.Valid {
		v := int(queueDepth.Int64)
		out.ConcurrencyQueueDepth = &v
	}
	if accountSwitchCount.Valid {
		v := accountSwitchCount.Int64
		out.AccountSwitchCount = &v
	}

	return &out, nil
}

func (r *opsRepository) UpsertNodeMetrics(ctx context.Context, input *service.OpsNodeMetrics) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if input == nil || input.NodeID == "" {
		return fmt.Errorf("invalid node metrics input")
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO ops_node_metrics (
  node_id, region, hostname, version, started_at, last_seen_at, report_interval_seconds,
  cpu_usage_percent, memory_used_mb, memory_total_mb, memory_usage_percent,
  db_ok, redis_ok,
  db_conn_active, db_conn_idle, db_conn_waiting, db_max_open_conns,
  redis_conn_total, redis_conn_idle, redis_pool_size,
  goroutine_count, concurrency_queue_depth, background_tasks_disabled
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,
  $8,$9,$10,$11,
  $12,$13,
  $14,$15,$16,$17,
  $18,$19,$20,
  $21,$22,$23
)
ON CONFLICT (node_id) DO UPDATE SET
  region = EXCLUDED.region,
  hostname = EXCLUDED.hostname,
  version = EXCLUDED.version,
  started_at = EXCLUDED.started_at,
  last_seen_at = EXCLUDED.last_seen_at,
  report_interval_seconds = EXCLUDED.report_interval_seconds,
  cpu_usage_percent = EXCLUDED.cpu_usage_percent,
  memory_used_mb = EXCLUDED.memory_used_mb,
  memory_total_mb = EXCLUDED.memory_total_mb,
  memory_usage_percent = EXCLUDED.memory_usage_percent,
  db_ok = EXCLUDED.db_ok,
  redis_ok = EXCLUDED.redis_ok,
  db_conn_active = EXCLUDED.db_conn_active,
  db_conn_idle = EXCLUDED.db_conn_idle,
  db_conn_waiting = EXCLUDED.db_conn_waiting,
  db_max_open_conns = EXCLUDED.db_max_open_conns,
  redis_conn_total = EXCLUDED.redis_conn_total,
  redis_conn_idle = EXCLUDED.redis_conn_idle,
  redis_pool_size = EXCLUDED.redis_pool_size,
  goroutine_count = EXCLUDED.goroutine_count,
  concurrency_queue_depth = EXCLUDED.concurrency_queue_depth,
  background_tasks_disabled = EXCLUDED.background_tasks_disabled`,
		input.NodeID,
		input.Region,
		input.Hostname,
		input.Version,
		input.StartedAt,
		input.LastSeenAt,
		input.ReportIntervalSeconds,
		opsNullFloat64(input.CPUUsagePercent),
		opsNullableInt64Pointer(input.MemoryUsedMB),
		opsNullableInt64Pointer(input.MemoryTotalMB),
		opsNullFloat64(input.MemoryUsagePercent),
		opsNullBool(input.DBOK),
		opsNullBool(input.RedisOK),
		opsNullableIntPointer(input.DBConnActive),
		opsNullableIntPointer(input.DBConnIdle),
		opsNullableIntPointer(input.DBConnWaiting),
		opsNullableIntPointer(input.DBMaxOpenConns),
		opsNullableIntPointer(input.RedisConnTotal),
		opsNullableIntPointer(input.RedisConnIdle),
		opsNullableIntPointer(input.RedisPoolSize),
		opsNullableIntPointer(input.GoroutineCount),
		opsNullableIntPointer(input.ConcurrencyQueueDepth),
		input.BackgroundTasksDisabled,
	)
	return err
}

func (r *opsRepository) ListNodeMetrics(ctx context.Context, seenSince time.Time) ([]*service.OpsNodeMetrics, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT
  node_id, region, hostname, version, started_at, last_seen_at, report_interval_seconds,
  cpu_usage_percent, memory_used_mb, memory_total_mb, memory_usage_percent,
  db_ok, redis_ok,
  db_conn_active, db_conn_idle, db_conn_waiting, db_max_open_conns,
  redis_conn_total, redis_conn_idle, redis_pool_size,
  goroutine_count, concurrency_queue_depth, background_tasks_disabled
FROM ops_node_metrics
WHERE last_seen_at >= $1
ORDER BY region ASC, node_id ASC`, seenSince)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*service.OpsNodeMetrics, 0)
	for rows.Next() {
		item := &service.OpsNodeMetrics{}
		var cpu, memPct sql.NullFloat64
		var memUsed, memTotal sql.NullInt64
		var dbOK, redisOK sql.NullBool
		var dbActive, dbIdle, dbWaiting, dbMax sql.NullInt64
		var redisTotal, redisIdle, redisMax sql.NullInt64
		var goroutines, queueDepth sql.NullInt64
		if err := rows.Scan(
			&item.NodeID, &item.Region, &item.Hostname, &item.Version, &item.StartedAt, &item.LastSeenAt, &item.ReportIntervalSeconds,
			&cpu, &memUsed, &memTotal, &memPct,
			&dbOK, &redisOK,
			&dbActive, &dbIdle, &dbWaiting, &dbMax,
			&redisTotal, &redisIdle, &redisMax,
			&goroutines, &queueDepth, &item.BackgroundTasksDisabled,
		); err != nil {
			return nil, err
		}
		item.CPUUsagePercent = opsFloat64Ptr(cpu)
		item.MemoryUsedMB = opsInt64Ptr(memUsed)
		item.MemoryTotalMB = opsInt64Ptr(memTotal)
		item.MemoryUsagePercent = opsFloat64Ptr(memPct)
		item.DBOK = opsBoolPtr(dbOK)
		item.RedisOK = opsBoolPtr(redisOK)
		item.DBConnActive = opsIntPtr(dbActive)
		item.DBConnIdle = opsIntPtr(dbIdle)
		item.DBConnWaiting = opsIntPtr(dbWaiting)
		item.DBMaxOpenConns = opsIntPtr(dbMax)
		item.RedisConnTotal = opsIntPtr(redisTotal)
		item.RedisConnIdle = opsIntPtr(redisIdle)
		item.RedisPoolSize = opsIntPtr(redisMax)
		item.GoroutineCount = opsIntPtr(goroutines)
		item.ConcurrencyQueueDepth = opsIntPtr(queueDepth)
		out = append(out, item)
	}
	return out, rows.Err()
}

func opsFloat64Ptr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	v := value.Float64
	return &v
}

func opsInt64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func opsIntPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	v := int(value.Int64)
	return &v
}

func opsBoolPtr(value sql.NullBool) *bool {
	if !value.Valid {
		return nil
	}
	v := value.Bool
	return &v
}

func (r *opsRepository) UpsertJobHeartbeat(ctx context.Context, input *service.OpsUpsertJobHeartbeatInput) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil ops repository")
	}
	if input == nil {
		return fmt.Errorf("nil input")
	}
	if input.JobName == "" {
		return fmt.Errorf("job_name required")
	}

	q := `
INSERT INTO ops_job_heartbeats (
  job_name,
  last_run_at,
  last_success_at,
  last_error_at,
  last_error,
  last_duration_ms,
  last_result,
  updated_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,NOW()
)
ON CONFLICT (job_name) DO UPDATE SET
  last_run_at = COALESCE(EXCLUDED.last_run_at, ops_job_heartbeats.last_run_at),
  last_success_at = COALESCE(EXCLUDED.last_success_at, ops_job_heartbeats.last_success_at),
  last_error_at = CASE
    WHEN EXCLUDED.last_success_at IS NOT NULL THEN NULL
    ELSE COALESCE(EXCLUDED.last_error_at, ops_job_heartbeats.last_error_at)
  END,
  last_error = CASE
    WHEN EXCLUDED.last_success_at IS NOT NULL THEN NULL
    ELSE COALESCE(EXCLUDED.last_error, ops_job_heartbeats.last_error)
  END,
  last_duration_ms = COALESCE(EXCLUDED.last_duration_ms, ops_job_heartbeats.last_duration_ms),
  last_result = CASE
    WHEN EXCLUDED.last_success_at IS NOT NULL THEN COALESCE(EXCLUDED.last_result, ops_job_heartbeats.last_result)
    ELSE ops_job_heartbeats.last_result
  END,
  updated_at = NOW()`

	_, err := r.db.ExecContext(
		ctx,
		q,
		input.JobName,
		opsNullTime(input.LastRunAt),
		opsNullTime(input.LastSuccessAt),
		opsNullTime(input.LastErrorAt),
		opsNullString(input.LastError),
		opsNullInt(input.LastDurationMs),
		opsNullString(input.LastResult),
	)
	return err
}

func (r *opsRepository) ListJobHeartbeats(ctx context.Context) ([]*service.OpsJobHeartbeat, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}

	q := `
SELECT
  job_name,
  last_run_at,
  last_success_at,
  last_error_at,
  last_error,
  last_duration_ms,
  last_result,
  updated_at
FROM ops_job_heartbeats
ORDER BY job_name ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]*service.OpsJobHeartbeat, 0, 8)
	for rows.Next() {
		var item service.OpsJobHeartbeat
		var lastRun sql.NullTime
		var lastSuccess sql.NullTime
		var lastErrorAt sql.NullTime
		var lastError sql.NullString
		var lastDuration sql.NullInt64

		var lastResult sql.NullString

		if err := rows.Scan(
			&item.JobName,
			&lastRun,
			&lastSuccess,
			&lastErrorAt,
			&lastError,
			&lastDuration,
			&lastResult,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if lastRun.Valid {
			v := lastRun.Time
			item.LastRunAt = &v
		}
		if lastSuccess.Valid {
			v := lastSuccess.Time
			item.LastSuccessAt = &v
		}
		if lastErrorAt.Valid {
			v := lastErrorAt.Time
			item.LastErrorAt = &v
		}
		if lastError.Valid {
			v := lastError.String
			item.LastError = &v
		}
		if lastDuration.Valid {
			v := lastDuration.Int64
			item.LastDurationMs = &v
		}
		if lastResult.Valid {
			v := lastResult.String
			item.LastResult = &v
		}

		out = append(out, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func opsNullBool(v *bool) any {
	if v == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *v, Valid: true}
}

func opsNullFloat64(v *float64) any {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

func opsNullTime(v *time.Time) any {
	if v == nil || v.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *v, Valid: true}
}

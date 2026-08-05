package repository

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryUpsertAndListNodeMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &opsRepository{db: db}
	now := time.Now().UTC().Truncate(time.Second)
	zeroInt := 0
	zeroInt64 := int64(0)

	args := make([]driver.Value, 23)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	for _, i := range []int{8, 9, 13, 14, 15, 16, 17, 18, 19, 20, 21} {
		args[i] = int64(0)
	}
	mock.ExpectExec("INSERT INTO ops_node_metrics").WithArgs(args...).WillReturnResult(sqlmock.NewResult(0, 1))
	err = repo.UpsertNodeMetrics(context.Background(), &service.OpsNodeMetrics{
		NodeID:                  "jp-01",
		Region:                  "japan",
		Hostname:                "host-jp-01",
		Version:                 "0.1.171",
		StartedAt:               now.Add(-time.Hour),
		LastSeenAt:              now,
		ReportIntervalSeconds:   60,
		BackgroundTasksDisabled: false,
		MemoryUsedMB:            &zeroInt64,
		MemoryTotalMB:           &zeroInt64,
		DBConnActive:            &zeroInt,
		DBConnIdle:              &zeroInt,
		DBConnWaiting:           &zeroInt,
		DBMaxOpenConns:          &zeroInt,
		RedisConnTotal:          &zeroInt,
		RedisConnIdle:           &zeroInt,
		RedisPoolSize:           &zeroInt,
		GoroutineCount:          &zeroInt,
		ConcurrencyQueueDepth:   &zeroInt,
	})
	require.NoError(t, err)

	seenSince := now.Add(-24 * time.Hour)
	columns := []string{
		"node_id", "region", "hostname", "version", "started_at", "last_seen_at", "report_interval_seconds",
		"cpu_usage_percent", "memory_used_mb", "memory_total_mb", "memory_usage_percent",
		"db_ok", "redis_ok", "db_conn_active", "db_conn_idle", "db_conn_waiting", "db_max_open_conns",
		"redis_conn_total", "redis_conn_idle", "redis_pool_size", "goroutine_count", "concurrency_queue_depth",
		"background_tasks_disabled",
	}
	mock.ExpectQuery("FROM ops_node_metrics").WithArgs(seenSince).WillReturnRows(
		sqlmock.NewRows(columns).AddRow(
			"jp-01", "japan", "host-jp-01", "0.1.171", now.Add(-time.Hour), now, 60,
			12.5, int64(512), int64(2048), 25.0,
			true, true, 3, 5, nil, 256,
			8, 6, 1024, 120, 2, false,
		),
	)

	nodes, err := repo.ListNodeMetrics(context.Background(), seenSince)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "jp-01", nodes[0].NodeID)
	require.Equal(t, 12.5, *nodes[0].CPUUsagePercent)
	require.Equal(t, 3, *nodes[0].DBConnActive)
	require.Equal(t, 1024, *nodes[0].RedisPoolSize)
	require.NoError(t, mock.ExpectationsWereMet())
}

package repository

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositorySystemMetricsPreservesSourceNode(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &opsRepository{db: db}
	now := time.Now().UTC().Truncate(time.Second)
	zeroInt := 0
	zeroInt64 := int64(0)

	args := make([]driver.Value, 43)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	for _, i := range []int{18, 23, 24, 29, 31, 32, 36, 37, 38, 39, 40, 41, 42} {
		args[i] = int64(0)
	}
	mock.ExpectExec("INSERT INTO ops_system_metrics").WithArgs(args...).WillReturnResult(sqlmock.NewResult(1, 1))
	err = repo.InsertSystemMetrics(context.Background(), &service.OpsInsertSystemMetricsInput{
		CreatedAt:             now,
		WindowMinutes:         1,
		SourceNodeID:          "jp-01",
		SourceRegion:          "japan",
		SourceHostname:        "host-jp-01",
		DurationP50Ms:         &zeroInt,
		DurationMaxMs:         &zeroInt,
		TTFTP50Ms:             &zeroInt,
		TTFTMaxMs:             &zeroInt,
		MemoryUsedMB:          &zeroInt64,
		MemoryTotalMB:         &zeroInt64,
		RedisConnTotal:        &zeroInt,
		RedisConnIdle:         &zeroInt,
		DBConnActive:          &zeroInt,
		DBConnIdle:            &zeroInt,
		DBConnWaiting:         &zeroInt,
		GoroutineCount:        &zeroInt,
		ConcurrencyQueueDepth: &zeroInt,
	})
	require.NoError(t, err)

	columns := []string{
		"id", "created_at", "window_minutes", "source_node_id", "source_region", "source_hostname",
		"cpu_usage_percent", "memory_used_mb", "memory_total_mb", "memory_usage_percent",
		"db_ok", "redis_ok", "redis_conn_total", "redis_conn_idle",
		"db_conn_active", "db_conn_idle", "db_conn_waiting", "goroutine_count",
		"concurrency_queue_depth", "account_switch_count",
	}
	mock.ExpectQuery("(?s)FROM ops_system_metrics.*ORDER BY created_at DESC, id DESC").WithArgs(1).WillReturnRows(
		sqlmock.NewRows(columns).AddRow(
			int64(1), now, 1, "jp-01", "japan", "host-jp-01",
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		),
	)

	snapshot, err := repo.GetLatestSystemMetrics(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "jp-01", snapshot.SourceNodeID)
	require.Equal(t, "japan", snapshot.SourceRegion)
	require.Equal(t, "host-jp-01", snapshot.SourceHostname)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsSystemMetricsSourceNodeMigrationIsRollingDeployCompatible(t *testing.T) {
	content, err := dbmigrations.FS.ReadFile("196_ops_system_metrics_source_node.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Equal(t, 3, strings.Count(sql, "ADD COLUMN IF NOT EXISTS"))
	require.NotContains(t, sql, "NOT NULL")
}

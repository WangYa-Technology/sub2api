package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpsNodeIsOnlineUsesReportingIntervalWithMinimumGrace(t *testing.T) {
	now := time.Now().UTC()

	require.True(t, opsNodeIsOnline(now, &OpsNodeMetrics{
		LastSeenAt:            now.Add(-2 * time.Minute),
		ReportIntervalSeconds: 60,
	}))
	require.False(t, opsNodeIsOnline(now, &OpsNodeMetrics{
		LastSeenAt:            now.Add(-4 * time.Minute),
		ReportIntervalSeconds: 60,
	}))
	require.True(t, opsNodeIsOnline(now, &OpsNodeMetrics{
		LastSeenAt:            now.Add(-90 * time.Minute),
		ReportIntervalSeconds: 3600,
	}))
	require.False(t, opsNodeIsOnline(now, nil))
}

func TestOpsMetricsCollectorTrafficOnlyNodeStillReports(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))

	var reported *OpsNodeMetrics
	globalWrites := 0
	repo := &opsRepoMock{
		UpsertNodeMetricsFn: func(_ context.Context, input *OpsNodeMetrics) error {
			reported = input
			return nil
		},
		InsertSystemMetricsFn: func(_ context.Context, _ *OpsInsertSystemMetricsInput) error {
			globalWrites++
			return nil
		},
	}
	cfg := &config.Config{
		Ops:                   config.OpsConfig{Enabled: true, NodeID: "us-01", Region: "us"},
		GlobalBackgroundTasks: config.GlobalBackgroundTasksConfig{Disabled: true},
	}
	collector := NewOpsMetricsCollector(repo, nil, nil, nil, db, nil, cfg, BuildInfo{Version: "0.1.171"})
	applyGlobalBackgroundTaskEligibility(&collector.instanceID, cfg)
	collector.collectOnce()

	require.NotNil(t, reported)
	require.Equal(t, "us-01", reported.NodeID)
	require.Equal(t, "us", reported.Region)
	require.Equal(t, "0.1.171", reported.Version)
	require.True(t, reported.BackgroundTasksDisabled)
	require.Zero(t, globalWrites)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsMetricsCollectorGlobalSnapshotIncludesSourceNode(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("FROM usage_logs").WillReturnRows(sqlmock.NewRows([]string{"success_count", "token_consumed"}).AddRow(0, 0))
	mock.ExpectQuery("FROM usage_logs").WillReturnRows(sqlmock.NewRows([]string{
		"duration_p50", "duration_p90", "duration_p95", "duration_p99", "duration_avg", "duration_max",
	}).AddRow(nil, nil, nil, nil, nil, nil))
	mock.ExpectQuery("FROM usage_logs").WillReturnRows(sqlmock.NewRows([]string{
		"ttft_p50", "ttft_p90", "ttft_p95", "ttft_p99", "ttft_avg", "ttft_max",
	}).AddRow(nil, nil, nil, nil, nil, nil))
	mock.ExpectQuery("FROM ops_error_logs").WillReturnRows(sqlmock.NewRows([]string{
		"error_total", "business_limited", "error_sla", "upstream_excl", "upstream_429", "upstream_529",
	}).AddRow(0, 0, 0, 0, 0, 0))
	mock.ExpectQuery("FROM ops_error_logs").WillReturnRows(sqlmock.NewRows([]string{"account_switch_count"}).AddRow(0))

	var inserted *OpsInsertSystemMetricsInput
	collector := &OpsMetricsCollector{
		db: db,
		opsRepo: &opsRepoMock{InsertSystemMetricsFn: func(_ context.Context, input *OpsInsertSystemMetricsInput) error {
			inserted = input
			return nil
		}},
	}
	err = collector.collectAndPersist(context.Background(), &OpsNodeMetrics{
		NodeID: "jp-01", Region: "japan", Hostname: "host-jp-01",
	})
	require.NoError(t, err)
	require.NotNil(t, inserted)
	require.Equal(t, "jp-01", inserted.SourceNodeID)
	require.Equal(t, "japan", inserted.SourceRegion)
	require.Equal(t, "host-jp-01", inserted.SourceHostname)
	require.NoError(t, mock.ExpectationsWereMet())
}

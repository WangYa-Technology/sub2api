package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestDashboardOverviewUsesSnapshotSourceNodePoolLimits(t *testing.T) {
	dbMax, redisPool := 20, 100
	repo := &opsRepoMock{
		GetLatestSystemMetricsFn: func(context.Context, int) (*OpsSystemMetricsSnapshot, error) {
			return &OpsSystemMetricsSnapshot{SourceNodeID: "jp-02"}, nil
		},
		ListNodeMetricsFn: func(context.Context, time.Time) ([]*OpsNodeMetrics, error) {
			return []*OpsNodeMetrics{
				{NodeID: "jp-01", DBMaxOpenConns: intPtr(30), RedisPoolSize: intPtr(200)},
				{NodeID: "jp-02", DBMaxOpenConns: &dbMax, RedisPoolSize: &redisPool},
			}, nil
		},
	}
	svc := &OpsService{
		opsRepo: repo,
		cfg: &config.Config{
			Database: config.DatabaseConfig{MaxOpenConns: 999},
			Redis:    config.RedisConfig{PoolSize: 999},
			Ops:      config.OpsConfig{Enabled: true, NodeID: "us-01", Region: "us"},
		},
	}
	now := time.Now().UTC()
	overview, err := svc.GetDashboardOverview(context.Background(), &OpsDashboardFilter{
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
		QueryMode: OpsQueryModeRaw,
	})
	require.NoError(t, err)
	require.Equal(t, "us-01", overview.ServingNode.NodeID)
	require.Equal(t, "us", overview.ServingNode.Region)
	require.Equal(t, 20, *overview.SystemMetrics.DBMaxOpenConns)
	require.Equal(t, 100, *overview.SystemMetrics.RedisPoolSize)
}

func TestDashboardOverviewLegacySnapshotFallsBackToLocalPoolLimits(t *testing.T) {
	repo := &opsRepoMock{
		GetLatestSystemMetricsFn: func(context.Context, int) (*OpsSystemMetricsSnapshot, error) {
			return &OpsSystemMetricsSnapshot{}, nil
		},
	}
	svc := &OpsService{
		opsRepo: repo,
		cfg: &config.Config{
			Database: config.DatabaseConfig{MaxOpenConns: 20},
			Redis:    config.RedisConfig{PoolSize: 100},
			Ops:      config.OpsConfig{Enabled: true},
		},
	}
	now := time.Now().UTC()
	overview, err := svc.GetDashboardOverview(context.Background(), &OpsDashboardFilter{
		StartTime: now.Add(-time.Hour),
		EndTime:   now,
		QueryMode: OpsQueryModeRaw,
	})
	require.NoError(t, err)
	require.Equal(t, 20, *overview.SystemMetrics.DBMaxOpenConns)
	require.Equal(t, 100, *overview.SystemMetrics.RedisPoolSize)
}

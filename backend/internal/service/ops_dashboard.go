package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	opsNodeMetricsLookback     = 24 * time.Hour
	opsNodeMinimumOnlineWindow = 3 * time.Minute
)

func (s *OpsService) GetDashboardOverview(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if s.opsRepo == nil {
		return nil, infraerrors.ServiceUnavailable("OPS_REPO_UNAVAILABLE", "Ops repository not available")
	}
	if filter == nil {
		return nil, infraerrors.BadRequest("OPS_FILTER_REQUIRED", "filter is required")
	}
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, infraerrors.BadRequest("OPS_TIME_RANGE_REQUIRED", "start_time/end_time are required")
	}
	if filter.StartTime.After(filter.EndTime) {
		return nil, infraerrors.BadRequest("OPS_TIME_RANGE_INVALID", "start_time must be <= end_time")
	}

	// Resolve query mode (requested via query param, or DB default).
	filter.QueryMode = s.resolveOpsQueryMode(ctx, filter.QueryMode)

	overview, err := s.opsRepo.GetDashboardOverview(ctx, filter)
	if err != nil && shouldFallbackOpsPreagg(filter, err) {
		rawFilter := cloneOpsFilterWithMode(filter, OpsQueryModeRaw)
		overview, err = s.opsRepo.GetDashboardOverview(ctx, rawFilter)
	}
	if err != nil {
		if errors.Is(err, ErrOpsPreaggregatedNotPopulated) {
			return nil, infraerrors.Conflict("OPS_PREAGG_NOT_READY", "Pre-aggregated ops metrics are not populated yet")
		}
		return nil, err
	}
	overview.ServingNode = resolveOpsNodeIdentity(s.cfg)

	// Best-effort system health + jobs; dashboard metrics should still render if these are missing.
	var metrics *OpsSystemMetricsSnapshot
	metrics, metricsErr := s.opsRepo.GetLatestSystemMetrics(ctx, 1)
	if metricsErr == nil {
		overview.SystemMetrics = metrics
	} else if !errors.Is(metricsErr, sql.ErrNoRows) {
		log.Printf("[Ops] GetLatestSystemMetrics failed: %v", metricsErr)
	}

	now := time.Now().UTC()
	if nodes, err := s.opsRepo.ListNodeMetrics(ctx, now.Add(-opsNodeMetricsLookback)); err == nil {
		for _, node := range nodes {
			if node == nil {
				continue
			}
			node.Online = opsNodeIsOnline(now, node)
		}
		overview.NodeMetrics = nodes
	} else {
		log.Printf("[Ops] ListNodeMetrics failed: %v", err)
	}
	localDBMax, localRedisPool := 0, 0
	if s.cfg != nil {
		localDBMax = s.cfg.Database.MaxOpenConns
		localRedisPool = s.cfg.Redis.PoolSize
	}
	attachSystemMetricPoolLimits(metrics, overview.NodeMetrics, localDBMax, localRedisPool)

	if heartbeats, err := s.opsRepo.ListJobHeartbeats(ctx); err == nil {
		overview.JobHeartbeats = heartbeats
	} else {
		log.Printf("[Ops] ListJobHeartbeats failed: %v", err)
	}

	overview.HealthScore = computeDashboardHealthScore(now, overview)

	return overview, nil
}

func attachSystemMetricPoolLimits(metrics *OpsSystemMetricsSnapshot, nodes []*OpsNodeMetrics, localDBMax, localRedisPool int) {
	if metrics == nil {
		return
	}
	if metrics.SourceNodeID != "" {
		for _, node := range nodes {
			if node != nil && node.NodeID == metrics.SourceNodeID {
				metrics.DBMaxOpenConns = node.DBMaxOpenConns
				metrics.RedisPoolSize = node.RedisPoolSize
				return
			}
		}
		return
	}
	// Compatibility fallback for snapshots created before source_node_id existed.
	if localDBMax > 0 {
		metrics.DBMaxOpenConns = intPtr(localDBMax)
	}
	if localRedisPool > 0 {
		metrics.RedisPoolSize = intPtr(localRedisPool)
	}
}

func opsNodeIsOnline(now time.Time, node *OpsNodeMetrics) bool {
	if node == nil || node.LastSeenAt.IsZero() {
		return false
	}
	onlineWindow := time.Duration(node.ReportIntervalSeconds*2)*time.Second + 30*time.Second
	if onlineWindow < opsNodeMinimumOnlineWindow {
		onlineWindow = opsNodeMinimumOnlineWindow
	}
	return now.Sub(node.LastSeenAt) <= onlineWindow
}

func (s *OpsService) resolveOpsQueryMode(ctx context.Context, requested OpsQueryMode) OpsQueryMode {
	if requested.IsValid() {
		// Allow "auto" to be disabled via config until preagg is proven stable in production.
		// Forced `preagg` via query param still works.
		if requested == OpsQueryModeAuto && s != nil && s.cfg != nil && !s.cfg.Ops.UsePreaggregatedTables {
			return OpsQueryModeRaw
		}
		return requested
	}

	mode := OpsQueryModeAuto
	if s != nil && s.settingRepo != nil {
		if raw, err := s.settingRepo.GetValue(ctx, SettingKeyOpsQueryModeDefault); err == nil {
			mode = ParseOpsQueryMode(raw)
		}
	}

	if mode == OpsQueryModeAuto && s != nil && s.cfg != nil && !s.cfg.Ops.UsePreaggregatedTables {
		return OpsQueryModeRaw
	}
	return mode
}

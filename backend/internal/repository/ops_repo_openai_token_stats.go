package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func queryOpenAITokenStats(ctx context.Context, queryer tokenStatsRowQueryer, filter *service.OpsOpenAITokenStatsFilter) (*service.OpsOpenAITokenStatsResponse, error) {
	if queryer == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if filter == nil {
		return nil, fmt.Errorf("nil filter")
	}
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, fmt.Errorf("start_time/end_time required")
	}
	// 允许 start_time == end_time（结果为空），与 service 层校验口径保持一致。
	if filter.StartTime.After(filter.EndTime) {
		return nil, fmt.Errorf("start_time must be <= end_time")
	}

	dashboardFilter := &service.OpsDashboardFilter{
		StartTime: filter.StartTime.UTC(),
		EndTime:   filter.EndTime.UTC(),
		Platform:  strings.TrimSpace(strings.ToLower(filter.Platform)),
		GroupID:   filter.GroupID,
	}

	join, where, baseArgs, next := buildUsageWhere(dashboardFilter, dashboardFilter.StartTime, dashboardFilter.EndTime, 1)

	baseCTE := `
WITH stats AS MATERIALIZED (
  SELECT
    ul.model AS model,
    COUNT(*)::bigint AS request_count,
    ROUND(
      AVG(
        CASE
          WHEN ul.duration_ms > 0 AND ul.output_tokens > 0
          THEN ul.output_tokens * 1000.0 / ul.duration_ms
        END
      )::numeric,
      2
    )::float8 AS avg_tokens_per_sec,
    ROUND(AVG(ul.first_token_ms)::numeric, 2)::float8 AS avg_first_token_ms,
    COALESCE(SUM(ul.output_tokens), 0)::bigint AS total_output_tokens,
    COALESCE(ROUND(AVG(ul.duration_ms)::numeric, 0), 0)::bigint AS avg_duration_ms,
    COUNT(CASE WHEN ul.first_token_ms IS NOT NULL THEN 1 END)::bigint AS requests_with_first_token
  FROM usage_logs ul
  ` + join + `
  ` + where + `
  GROUP BY ul.model
)
`

	querySQL := baseCTE + `, page AS (
SELECT * FROM stats ORDER BY request_count DESC, model ASC`
	args := make([]any, 0, len(baseArgs)+2)
	args = append(args, baseArgs...)

	if filter.IsTopNMode() {
		querySQL += fmt.Sprintf("\nLIMIT $%d", next)
		args = append(args, filter.TopN)
	} else {
		offset := (filter.Page - 1) * filter.PageSize
		querySQL += fmt.Sprintf("\nLIMIT $%d OFFSET $%d", next, next+1)
		args = append(args, filter.PageSize, offset)
	}

	querySQL += `
)
SELECT (SELECT COUNT(*) FROM stats),
       COALESCE((SELECT json_agg(page ORDER BY request_count DESC, model ASC) FROM page), '[]'::json)`
	// Bound the expensive aggregate and propagate request cancellation to PostgreSQL.
	ctx, cancel := context.WithTimeout(ctx, 55*time.Second)
	defer cancel()
	var total int64
	var raw []byte
	if err := queryer.QueryRowContext(ctx, querySQL, args...).Scan(&total, &raw); err != nil {
		return nil, err
	}
	items := make([]*service.OpsOpenAITokenStatsItem, 0)
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}

	resp := &service.OpsOpenAITokenStatsResponse{
		TimeRange: strings.TrimSpace(filter.TimeRange),
		StartTime: dashboardFilter.StartTime,
		EndTime:   dashboardFilter.EndTime,
		Platform:  dashboardFilter.Platform,
		GroupID:   dashboardFilter.GroupID,
		Items:     items,
		Total:     total,
	}
	if filter.IsTopNMode() {
		topN := filter.TopN
		resp.TopN = &topN
	} else {
		resp.Page = filter.Page
		resp.PageSize = filter.PageSize
	}
	return resp, nil
}

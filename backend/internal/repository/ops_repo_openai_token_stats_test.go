package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryGetOpenAITokenStats_PlatformScope(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	groupID := int64(9)
	tests := []struct {
		name     string
		platform string
		groupID  *int64
		models   []string
	}{
		{name: "all platforms", models: []string{"claude-sonnet-4", "deepseek-chat", "gemini-2.5-pro", "gpt-4o", "o3"}},
		{name: "anthropic group", platform: "anthropic", groupID: &groupID, models: []string{"claude-sonnet-4"}},
		{name: "gemini", platform: "gemini", models: []string{"gemini-2.5-pro"}},
		{name: "openai reasoning model", platform: "openai", models: []string{"o3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock := newSQLMock(t)
			filter := &service.OpsOpenAITokenStatsFilter{
				TimeRange: "1h",
				StartTime: start,
				EndTime:   end,
				Platform:  tt.platform,
				GroupID:   tt.groupID,
				TopN:      20,
			}

			// Match the complete filtering clause in the aggregate so an implicit
			// model restriction cannot silently narrow the selected platform scope.
			where := "WHERE ul.created_at >= $1 AND ul.created_at < $2"
			args := []driver.Value{start, end}
			if tt.groupID != nil {
				where += " AND ul.group_id = $3"
				args = append(args, *tt.groupID)
			}
			if tt.platform != "" {
				placeholder := "$3"
				if tt.groupID != nil {
					placeholder = "$4"
				}
				where += " AND COALESCE(NULLIF(g.platform,''), a.platform) = " + placeholder
				args = append(args, tt.platform)
			}
			queryPrefix := regexp.QuoteMeta(where) + `\s+GROUP BY ul.model\s+\)`
			items := make([]*service.OpsOpenAITokenStatsItem, 0)
			for _, model := range tt.models {
				items = append(items, &service.OpsOpenAITokenStatsItem{Model: model, RequestCount: 2})
			}
			payload, err := json.Marshal(items)
			require.NoError(t, err)
			mock.ExpectQuery(queryPrefix + `\s*, page AS`).WithArgs(append(args, 20)...).
				WillReturnRows(sqlmock.NewRows([]string{"total", "items"}).AddRow(len(tt.models), payload))

			resp, err := queryOpenAITokenStats(context.Background(), db, filter)
			require.NoError(t, err)
			require.Equal(t, tt.platform, resp.Platform)
			require.Equal(t, tt.groupID, resp.GroupID)
			require.Equal(t, int64(len(tt.models)), resp.Total)
			require.Len(t, resp.Items, len(tt.models))
			for i, model := range tt.models {
				require.Equal(t, model, resp.Items[i].Model)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestOpsRepositoryGetOpenAITokenStats_PaginationMode(t *testing.T) {
	db, mock := newSQLMock(t)

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	groupID := int64(9)

	filter := &service.OpsOpenAITokenStatsFilter{
		TimeRange: "1d",
		StartTime: start,
		EndTime:   end,
		Platform:  " OpenAI ",
		GroupID:   &groupID,
		Page:      2,
		PageSize:  10,
	}

	mock.ExpectQuery(`ORDER BY request_count DESC, model ASC\s+LIMIT \$5 OFFSET \$6`).
		WithArgs(start, end, groupID, "openai", 10, 10).
		WillReturnRows(sqlmock.NewRows([]string{"total", "items"}).AddRow(3, `[{"model":"gpt-4o-mini","request_count":20,"avg_tokens_per_sec":21.56,"avg_first_token_ms":120.34,"total_output_tokens":3000,"avg_duration_ms":850,"requests_with_first_token":18},{"model":"gpt-4.1","request_count":20}]`))

	resp, err := queryOpenAITokenStats(context.Background(), db, filter)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int64(3), resp.Total)
	require.Equal(t, 2, resp.Page)
	require.Equal(t, 10, resp.PageSize)
	require.Nil(t, resp.TopN)
	require.Equal(t, "openai", resp.Platform)
	require.NotNil(t, resp.GroupID)
	require.Equal(t, groupID, *resp.GroupID)
	require.Len(t, resp.Items, 2)
	require.Equal(t, "gpt-4o-mini", resp.Items[0].Model)
	require.NotNil(t, resp.Items[0].AvgTokensPerSec)
	require.InDelta(t, 21.56, *resp.Items[0].AvgTokensPerSec, 0.0001)
	require.NotNil(t, resp.Items[0].AvgFirstTokenMs)
	require.InDelta(t, 120.34, *resp.Items[0].AvgFirstTokenMs, 0.0001)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsRepositoryGetOpenAITokenStats_TopNMode(t *testing.T) {
	db, mock := newSQLMock(t)

	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	filter := &service.OpsOpenAITokenStatsFilter{
		TimeRange: "1h",
		StartTime: start,
		EndTime:   end,
		TopN:      5,
	}

	mock.ExpectQuery(`ORDER BY request_count DESC, model ASC\s+LIMIT \$3`).WithArgs(start, end, 5).
		WillReturnRows(sqlmock.NewRows([]string{"total", "items"}).AddRow(1, `[{"model":"gpt-4o","request_count":5,"avg_tokens_per_sec":null,"avg_first_token_ms":null}]`))

	resp, err := queryOpenAITokenStats(context.Background(), db, filter)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.TopN)
	require.Equal(t, 5, *resp.TopN)
	require.Equal(t, 0, resp.Page)
	require.Equal(t, 0, resp.PageSize)
	require.Len(t, resp.Items, 1)
	require.Nil(t, resp.Items[0].AvgTokensPerSec)
	require.Nil(t, resp.Items[0].AvgFirstTokenMs)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsRepositoryGetOpenAITokenStats_EmptyResult(t *testing.T) {
	db, mock := newSQLMock(t)

	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	filter := &service.OpsOpenAITokenStatsFilter{
		TimeRange: "30m",
		StartTime: start,
		EndTime:   end,
		Page:      1,
		PageSize:  20,
	}

	mock.ExpectQuery(`ORDER BY request_count DESC, model ASC\s+LIMIT \$3 OFFSET \$4`).WithArgs(start, end, 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"total", "items"}).AddRow(0, `[]`))

	resp, err := queryOpenAITokenStats(context.Background(), db, filter)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int64(0), resp.Total)
	require.Len(t, resp.Items, 0)
	require.Equal(t, 1, resp.Page)
	require.Equal(t, 20, resp.PageSize)

	require.NoError(t, mock.ExpectationsWereMet())
}

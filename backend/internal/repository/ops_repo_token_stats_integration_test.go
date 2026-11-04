package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Run only against an explicitly selected disposable PostgreSQL database.
// Temporary tables shadow public tables; a single connection keeps them scoped.
func TestTokenStatsPostgresSemantics(t *testing.T) {
	dsn := os.Getenv("OPS_TOKEN_STATS_TEST_DSN")
	if dsn == "" {
		t.Skip("set OPS_TOKEN_STATS_TEST_DSN to disposable PostgreSQL")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE ops_token_stats_cache(cache_key text PRIMARY KEY,payload jsonb NOT NULL,expires_at timestamptz NOT NULL); SET TIME ZONE 'UTC'; CREATE TEMP TABLE usage_logs(model text NOT NULL, created_at timestamptz, output_tokens int, duration_ms int, first_token_ms int,group_id bigint,account_id bigint);
 CREATE TEMP TABLE groups(id bigint,platform text); CREATE TEMP TABLE accounts(id bigint,platform text);
 INSERT INTO groups VALUES(1,'openai'); INSERT INTO accounts VALUES(1,'openai');
 INSERT INTO usage_logs VALUES
 ('a','2026-11-01',100,1000,10,1,1),('a','2026-11-01',300,10000,NULL,1,1),
 ('a','2026-11-01',0,200,30,1,1),('a','2026-11-01',NULL,NULL,NULL,1,1),
 ('b','2026-11-01',NULL,0,NULL,NULL,1),('excluded','2026-11-02',999,1,1,1,1);`)
	require.NoError(t, err)
	f := &service.OpsOpenAITokenStatsFilter{StartTime: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), Platform: "openai", TopN: 20}
	repo := &opsRepository{db: db}
	out, err := repo.GetOpenAITokenStats(ctx, f)
	require.NoError(t, err)
	require.Equal(t, int64(2), out.Total)
	require.Len(t, out.Items, 2)
	a := out.Items[0]
	require.Equal(t, "a", a.Model)
	require.Equal(t, int64(4), a.RequestCount)
	require.InDelta(t, 65, *a.AvgTokensPerSec, 0.001, "average of per-request speeds, not ratio of sums")
	require.InDelta(t, 20, *a.AvgFirstTokenMs, 0.001)
	require.Equal(t, int64(400), a.TotalOutputTokens)
	require.Equal(t, int64(3733), a.AvgDurationMs)
	require.Equal(t, int64(2), a.RequestsWithFirstToken)
	require.Nil(t, out.Items[1].AvgTokensPerSec)
	require.Nil(t, out.Items[1].AvgFirstTokenMs)
	f.TopN = 0
	f.Page = 9
	f.PageSize = 1
	out, err = repo.GetOpenAITokenStats(ctx, f)
	require.NoError(t, err)
	require.Empty(t, out.Items)
	require.Equal(t, int64(2), out.Total)
	f.StartTime = f.EndTime
	out, err = repo.GetOpenAITokenStats(ctx, f)
	require.NoError(t, err)
	require.Empty(t, out.Items)
	require.Zero(t, out.Total)
}

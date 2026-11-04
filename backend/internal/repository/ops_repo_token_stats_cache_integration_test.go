package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Each test creates its own schema in the explicitly supplied disposable DB.
// Independent pools represent applications that do not share Redis or memory.
func tokenStatsIntegrationPools(t *testing.T) (*sql.DB, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("OPS_TOKEN_STATS_TEST_DSN")
	if dsn == "" {
		t.Skip("set OPS_TOKEN_STATS_TEST_DSN to disposable PostgreSQL")
	}
	admin, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("ops_token_test_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE SCHEMA ` + pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec(`DROP SCHEMA ` + pq.QuoteIdentifier(schema) + ` CASCADE`); _ = admin.Close() })
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema
	}
	pools := make([]*sql.DB, 2)
	for i := range pools {
		pools[i], err = sql.Open("postgres", dsn)
		require.NoError(t, err)
		pools[i].SetMaxOpenConns(3)
		db := pools[i]
		t.Cleanup(func() { _ = db.Close() })
	}
	migration, err := migrations.FS.ReadFile("249_ops_token_stats_cache.sql")
	require.NoError(t, err)
	// Migration must also be safe to replay.
	for i := 0; i < 2; i++ {
		_, err = pools[0].Exec(string(migration))
		require.NoError(t, err)
	}
	_, err = pools[0].Exec(`CREATE TABLE usage_fixture(model text,created_at timestamptz,output_tokens int,duration_ms int,first_token_ms int,group_id bigint,account_id bigint);
 INSERT INTO usage_fixture VALUES('test-model','2026-11-01 00:00:00+00',100,1000,10,1,1);
 CREATE TABLE groups(id bigint,platform text);CREATE TABLE accounts(id bigint,platform text);
 INSERT INTO groups VALUES(1,'openai');INSERT INTO accounts VALUES(1,'openai');
 CREATE SEQUENCE scan_count;
 CREATE FUNCTION counted_model(v text) RETURNS text LANGUAGE plpgsql VOLATILE AS $$
 BEGIN PERFORM nextval('scan_count'); PERFORM pg_sleep(0.2); RETURN v; END $$;
 CREATE VIEW usage_logs AS SELECT counted_model(model) AS model,created_at,output_tokens,duration_ms,first_token_ms,group_id,account_id FROM usage_fixture;`)
	require.NoError(t, err)
	return pools[0], pools[1]
}
func cachedTokenStatsFilter() *service.OpsOpenAITokenStatsFilter {
	end := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	return &service.OpsOpenAITokenStatsFilter{TimeRange: "1d", StartTime: end.Add(-24 * time.Hour), EndTime: end, TopN: 20}
}
func TestTokenStatsPostgresCacheCrossInstance(t *testing.T) {
	a, b := tokenStatsIntegrationPools(t)
	r1, r2 := &opsRepository{db: a}, &opsRepository{db: b}
	f := cachedTokenStatsFilter()
	start := make(chan struct{})
	type result struct {
		out *service.OpsOpenAITokenStatsResponse
		err error
	}
	results := make(chan result, 2)
	for _, repo := range []*opsRepository{r1, r2} {
		go func(r *opsRepository) {
			<-start
			out, err := r.GetOpenAITokenStats(context.Background(), f)
			results <- result{out, err}
		}(repo)
	}
	coldStarted := time.Now()
	close(start)
	for i := 0; i < 2; i++ {
		result := <-results
		require.NoError(t, result.err)
		require.Equal(t, int64(1), result.out.Total)
		require.Equal(t, f.EndTime, result.out.EndTime)
	}
	t.Logf("two application pools cold requests: %s", time.Since(coldStarted))
	var scans int
	require.NoError(t, a.QueryRow(`SELECT last_value FROM scan_count`).Scan(&scans))
	require.Equal(t, 1, scans, "one detail scan across independent application pools")
	// Cached reads still work when the expensive source is unavailable.
	_, err := a.Exec(`DROP VIEW usage_logs`)
	require.NoError(t, err)
	hotStarted := time.Now()
	out, err := r2.GetOpenAITokenStats(context.Background(), f)
	require.NoError(t, err)
	require.Equal(t, int64(1), out.Total)
	t.Logf("shared cache hit without detail source: %s", time.Since(hotStarted))
	_, err = a.Exec(`UPDATE ops_token_stats_cache SET expires_at=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	_, err = r1.GetOpenAITokenStats(context.Background(), f)
	require.Error(t, err, "expired result must not be returned or refreshed without a valid source")
	var locked bool
	tx, err := b.Begin()
	require.NoError(t, err)
	require.NoError(t, tx.QueryRow(tokenStatsComputeLockSQL).Scan(&locked))
	require.True(t, locked, "failed computation must release lock")
	require.NoError(t, tx.Rollback())
}
func TestTokenStatsPostgresGlobalGateCancellation(t *testing.T) {
	a, b := tokenStatsIntegrationPools(t)
	owner, err := a.Begin()
	require.NoError(t, err)
	var acquired bool
	require.NoError(t, owner.QueryRow(tokenStatsComputeLockSQL).Scan(&acquired))
	require.True(t, acquired)
	f := cachedTokenStatsFilter()
	f.Platform = "openai"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	_, err = (&opsRepository{db: b}).GetOpenAITokenStats(ctx, f)
	cancel()
	require.Error(t, err)
	var called bool
	require.NoError(t, a.QueryRow(`SELECT is_called FROM scan_count`).Scan(&called))
	require.False(t, called, "no detail scan while global gate held")
	require.NoError(t, owner.Rollback())
	// A canceled owner releases the transaction-level lock without an explicit unlock.
	_, err = a.Exec(`CREATE OR REPLACE FUNCTION counted_model(v text) RETURNS text LANGUAGE plpgsql VOLATILE AS $$ BEGIN PERFORM nextval('scan_count'); PERFORM pg_sleep(2); RETURN v; END $$`)
	require.NoError(t, err)
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	_, err = (&opsRepository{db: b}).GetOpenAITokenStats(ctx, cachedTokenStatsFilter())
	cancel()
	require.Error(t, err)
	require.Eventually(t, func() bool {
		tx, err := a.Begin()
		if err != nil {
			return false
		}
		defer tx.Rollback()
		var ok bool
		return tx.QueryRow(tokenStatsComputeLockSQL).Scan(&ok) == nil && ok
	}, time.Second, 20*time.Millisecond)
	var count int
	require.NoError(t, a.QueryRow(`SELECT count(*) FROM ops_token_stats_cache`).Scan(&count))
	require.Zero(t, count, "canceled computation must not publish")
}
func TestTokenStatsPostgresCacheIsolationCleanup(t *testing.T) {
	a, b := tokenStatsIntegrationPools(t)
	repo := &opsRepository{db: a}
	f := cachedTokenStatsFilter()
	_, err := repo.GetOpenAITokenStats(context.Background(), f)
	require.NoError(t, err)
	variants := []func(*service.OpsOpenAITokenStatsFilter){
		func(f *service.OpsOpenAITokenStatsFilter) { f.Platform = "openai" },
		func(f *service.OpsOpenAITokenStatsFilter) { id := int64(1); f.GroupID = &id },
		func(f *service.OpsOpenAITokenStatsFilter) { f.TopN = 10 },
		func(f *service.OpsOpenAITokenStatsFilter) { f.TopN = 0; f.Page = 2; f.PageSize = 20 },
		func(f *service.OpsOpenAITokenStatsFilter) { f.EndTime = f.EndTime.Add(time.Minute) },
	}
	for _, change := range variants {
		clone := *f
		change(&clone)
		_, err := (&opsRepository{db: b}).GetOpenAITokenStats(context.Background(), &clone)
		require.NoError(t, err)
	}
	var count int
	require.NoError(t, a.QueryRow(`SELECT count(*) FROM ops_token_stats_cache`).Scan(&count))
	require.Equal(t, 6, count)
	_, err = a.Exec(`INSERT INTO ops_token_stats_cache SELECT 'expired-'||i,'{}',clock_timestamp()-interval '1 day' FROM generate_series(1,1200)i`)
	require.NoError(t, err)
	f.TopN = 5
	_, err = repo.GetOpenAITokenStats(context.Background(), f)
	require.NoError(t, err)
	require.NoError(t, a.QueryRow(`SELECT count(*) FROM ops_token_stats_cache WHERE expires_at<=clock_timestamp()`).Scan(&count))
	require.Equal(t, 200, count, "cleanup is bounded to 1000 expired keys")
}

func TestTokenStatsPostgresCacheFailureDoesNotScan(t *testing.T) {
	a, _ := tokenStatsIntegrationPools(t)
	_, err := a.Exec(`DROP TABLE ops_token_stats_cache`)
	require.NoError(t, err)
	_, err = (&opsRepository{db: a}).GetOpenAITokenStats(context.Background(), cachedTokenStatsFilter())
	require.Error(t, err)
	var called bool
	require.NoError(t, a.QueryRow(`SELECT is_called FROM scan_count`).Scan(&called))
	require.False(t, called, "cache failure must not fall back to an uncontrolled detail scan")
}

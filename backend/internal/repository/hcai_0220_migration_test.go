//go:build hcai_migration

package repository

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

// Only run against an empty, disposable database, never a production DSN.
func TestHCAI0220MigrationUpgrade(t *testing.T) {
	dsn := os.Getenv("HCAI_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("requires disposable PostgreSQL")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'").Scan(&count))
	require.Zero(t, count)
	baseline := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if name == "249_mcp_oauth.sql" {
			continue
		}
		data, err := fs.ReadFile(migrations.FS, name)
		require.NoError(t, err)
		baseline[name] = &fstest.MapFile{Data: data}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, baseline))
	_, err = db.ExecContext(ctx, "INSERT INTO ops_token_stats_cache(cache_key,payload,expires_at) VALUES ('hcai-upgrade','{}',now()+interval '1 minute')")
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFS(ctx, db, migrations.FS))
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE filename IN ('249_ops_token_stats_cache.sql','249_mcp_oauth.sql')").Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM ops_token_stats_cache WHERE cache_key='hcai-upgrade'").Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM mcp_oauth_grants").Scan(&count))
	require.Zero(t, count)
	require.NoError(t, applyMigrationsFS(ctx, db, migrations.FS), "restart is idempotent")
}

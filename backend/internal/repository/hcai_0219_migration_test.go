//go:build hcai_migration

package repository

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Run only against a disposable local database; this test applies the full schema.
func TestHCAI0219MigrationUpgrade(t *testing.T) {
	dsn := os.Getenv("HCAI_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("requires a disposable local PostgreSQL database")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var existing int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'").Scan(&existing))
	require.Zero(t, existing, "use an empty disposable database")
	baseline := fstest.MapFS{}
	files, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range files {
		if strings.HasPrefix(name, "247_") || strings.HasPrefix(name, "248_") {
			continue
		}
		data, err := fs.ReadFile(migrations.FS, name)
		require.NoError(t, err)
		baseline[name] = &fstest.MapFile{Data: data}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, baseline))
	var groupID int64
	require.NoError(t, db.QueryRowContext(ctx, "INSERT INTO groups(name) VALUES ('hcai-migration-test') RETURNING id").Scan(&groupID))
	// The same runner is called by normal InitEnt startup.
	require.NoError(t, applyMigrationsFS(ctx, db, migrations.FS))
	var enabled bool
	require.NoError(t, db.QueryRowContext(ctx, "SELECT image_count_billing FROM groups WHERE id=$1", groupID).Scan(&enabled))
	require.False(t, enabled, "existing groups retain old billing by default")
	var added int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE filename IN ('247_scoped_admin_api_keys.sql','248_group_image_count_billing.sql')").Scan(&added))
	require.Equal(t, 2, added)
	var keys int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM scoped_admin_api_keys WHERE deleted_at IS NULL").Scan(&keys))
	require.Zero(t, keys)
	require.NoError(t, applyMigrationsFS(ctx, db, migrations.FS), "restart must be idempotent")
}

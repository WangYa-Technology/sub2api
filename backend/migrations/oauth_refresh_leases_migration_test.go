package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOAuthRefreshLeasesMigration(t *testing.T) {
	content, err := FS.ReadFile("197_oauth_refresh_leases.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS oauth_refresh_leases")
	require.Contains(t, sql, "lock_key_hash CHAR(64) PRIMARY KEY")
	require.Contains(t, sql, "owner_id UUID NOT NULL")
	require.Contains(t, sql, "expires_at TIMESTAMPTZ NOT NULL")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_oauth_refresh_leases_expires_at")
}

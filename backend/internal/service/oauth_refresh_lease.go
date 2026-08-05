package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

const acquireOAuthRefreshLeaseSQL = `
INSERT INTO oauth_refresh_leases (lock_key_hash, owner_id, acquired_at, expires_at)
VALUES ($1, $2, NOW(), NOW() + ($3 * INTERVAL '1 millisecond'))
ON CONFLICT (lock_key_hash) DO UPDATE
SET owner_id = EXCLUDED.owner_id,
    acquired_at = EXCLUDED.acquired_at,
    expires_at = EXCLUDED.expires_at
WHERE oauth_refresh_leases.expires_at <= NOW()
RETURNING TRUE`

const releaseOAuthRefreshLeaseSQL = `
DELETE FROM oauth_refresh_leases
WHERE lock_key_hash = $1 AND owner_id = $2`

func oauthRefreshLeaseKey(cacheKey string) string {
	sum := sha256.Sum256([]byte("oauth:refresh:" + cacheKey))
	return hex.EncodeToString(sum[:])
}

// tryAcquireOAuthRefreshLease uses a single PostgreSQL statement so the pool
// connection is returned before the upstream refresh starts. Expiry provides
// bounded crash recovery without keeping a database session alive.
func tryAcquireOAuthRefreshLease(
	ctx context.Context,
	db *sql.DB,
	cacheKey string,
	ttl time.Duration,
) (func(), bool, error) {
	if db == nil {
		return nil, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ttl <= 0 {
		ttl = defaultRefreshLockTTL
	}

	lockKey := oauthRefreshLeaseKey(cacheKey)
	ownerID := uuid.NewString()
	var acquired bool
	err := db.QueryRowContext(ctx, acquireOAuthRefreshLeaseSQL, lockKey, ownerID, ttl.Milliseconds()).Scan(&acquired)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("acquire OAuth refresh lease: %w", err)
	}
	if !acquired {
		return nil, false, fmt.Errorf("acquire OAuth refresh lease: unexpected false result")
	}

	release := func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), defaultRefreshLockReleaseTimeout)
		defer cancel()
		if _, releaseErr := db.ExecContext(releaseCtx, releaseOAuthRefreshLeaseSQL, lockKey, ownerID); releaseErr != nil {
			slog.Warn("oauth_refresh_database_lease_release_failed",
				"lock_key_hash", lockKey,
				"error", releaseErr,
			)
		}
	}
	return release, true, nil
}

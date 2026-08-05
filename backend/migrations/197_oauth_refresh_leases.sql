-- Short-lived cross-region OAuth refresh leases.
-- Acquisition and release use single statements so application connections are
-- never pinned while an upstream token refresh is in progress.

CREATE TABLE IF NOT EXISTS oauth_refresh_leases (
    lock_key_hash CHAR(64) PRIMARY KEY
        CHECK (lock_key_hash ~ '^[0-9a-f]{64}$'),
    owner_id UUID NOT NULL,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_oauth_refresh_leases_expires_at
    ON oauth_refresh_leases (expires_at);

COMMENT ON TABLE oauth_refresh_leases IS
    'Expiring cross-region OAuth refresh leases; lock keys are SHA-256 hashes';

-- Shared, disposable report results. No usage_logs rewrite, index or backfill.
CREATE TABLE IF NOT EXISTS ops_token_stats_cache (
    cache_key TEXT PRIMARY KEY,
    payload JSONB NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ops_token_stats_cache_expires_at
    ON ops_token_stats_cache (expires_at);

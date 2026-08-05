-- Fan out API-key auth invalidations to every independent Redis data set.
-- The legacy outbox remains intact so old binaries can coexist during a rolling upgrade.

CREATE TABLE IF NOT EXISTS auth_cache_invalidation_events (
    id               BIGSERIAL PRIMARY KEY,
    source_outbox_id BIGINT NOT NULL UNIQUE,
    cache_key        CHAR(64) NOT NULL CHECK (cache_key ~ '^[0-9a-f]{64}$'),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_cache_invalidation_events_created_at
    ON auth_cache_invalidation_events (created_at);

CREATE TABLE IF NOT EXISTS auth_cache_invalidation_consumers (
    consumer_scope TEXT PRIMARY KEY CHECK (length(consumer_scope) BETWEEN 1 AND 200),
    last_event_id  BIGINT NOT NULL DEFAULT 0 CHECK (last_event_id >= 0),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION mirror_auth_cache_invalidation_event()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO auth_cache_invalidation_events (source_outbox_id, cache_key, created_at)
    VALUES (NEW.id, NEW.cache_key, NEW.created_at)
    ON CONFLICT (source_outbox_id) DO NOTHING;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_auth_cache_invalidation_event_mirror
    ON auth_cache_invalidation_outbox;
CREATE TRIGGER trg_auth_cache_invalidation_event_mirror
AFTER INSERT ON auth_cache_invalidation_outbox
FOR EACH ROW EXECUTE FUNCTION mirror_auth_cache_invalidation_event();

-- Preserve invalidations that were pending while this release was installed.
INSERT INTO auth_cache_invalidation_events (source_outbox_id, cache_key, created_at)
SELECT id, cache_key, created_at
FROM auth_cache_invalidation_outbox
ON CONFLICT (source_outbox_id) DO NOTHING;

COMMENT ON TABLE auth_cache_invalidation_events IS
    'Append-only auth invalidation fanout log; cache_key is SHA-256 hex, never plaintext';
COMMENT ON TABLE auth_cache_invalidation_consumers IS
    'Per-Redis-scope cursors for durable auth cache invalidation fanout';

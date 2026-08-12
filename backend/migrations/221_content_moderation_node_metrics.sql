-- Latest content moderation runtime snapshot for each application node.
-- Shared PostgreSQL makes this work across regions with independent Redis.
CREATE TABLE IF NOT EXISTS content_moderation_node_metrics (
    node_id VARCHAR(128) PRIMARY KEY,
    snapshot JSONB NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_moderation_node_metrics_last_seen
    ON content_moderation_node_metrics (last_seen_at DESC);

COMMENT ON TABLE content_moderation_node_metrics IS
    'Latest content moderation runtime snapshot per application node for cluster aggregation.';

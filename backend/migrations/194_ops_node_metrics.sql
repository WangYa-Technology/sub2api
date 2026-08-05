-- Per-application-node runtime metrics for shared-database deployments.
-- One row is retained per stable node_id; updates do not grow minute history.
CREATE TABLE IF NOT EXISTS ops_node_metrics (
    node_id VARCHAR(128) PRIMARY KEY,
    region VARCHAR(64) NOT NULL DEFAULT '',
    hostname VARCHAR(255) NOT NULL DEFAULT '',
    version VARCHAR(64) NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    report_interval_seconds INT NOT NULL DEFAULT 60,

    cpu_usage_percent DOUBLE PRECISION,
    memory_used_mb BIGINT,
    memory_total_mb BIGINT,
    memory_usage_percent DOUBLE PRECISION,

    db_ok BOOLEAN,
    redis_ok BOOLEAN,
    db_conn_active INT,
    db_conn_idle INT,
    db_conn_waiting INT,
    db_max_open_conns INT,
    redis_conn_total INT,
    redis_conn_idle INT,
    redis_pool_size INT,
    goroutine_count INT,
    concurrency_queue_depth INT,
    background_tasks_disabled BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_ops_node_metrics_last_seen
    ON ops_node_metrics (last_seen_at DESC);

COMMENT ON TABLE ops_node_metrics IS 'Latest runtime metrics and heartbeat for each application node connected to the shared database.';

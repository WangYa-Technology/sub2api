-- Record which application node produced each global system metrics snapshot.
-- Nullable columns keep existing rows and mixed-version rolling deployments compatible.
ALTER TABLE ops_system_metrics
    ADD COLUMN IF NOT EXISTS source_node_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS source_region VARCHAR(64),
    ADD COLUMN IF NOT EXISTS source_hostname VARCHAR(255);

COMMENT ON COLUMN ops_system_metrics.source_node_id IS 'Stable application node ID that produced this snapshot.';
COMMENT ON COLUMN ops_system_metrics.source_region IS 'Configured region of the application node that produced this snapshot.';
COMMENT ON COLUMN ops_system_metrics.source_hostname IS 'Hostname of the application node that produced this snapshot.';

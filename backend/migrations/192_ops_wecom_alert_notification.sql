ALTER TABLE ops_alert_rules
    ADD COLUMN IF NOT EXISTS notify_wecom BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE ops_alert_events
    ADD COLUMN IF NOT EXISTS wecom_sent BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_ops_alert_events_wecom_sent
    ON ops_alert_events (wecom_sent, fired_at DESC);

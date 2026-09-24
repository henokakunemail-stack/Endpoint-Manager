-- 0008_alerting.sql
-- Enterprise alerting rules, active incident tracking, and deduplication

CREATE TABLE IF NOT EXISTS alert_rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    rule_type TEXT NOT NULL, -- 'disk_low', 'ram_high', 'device_offline', 'critical_patch'
    threshold_val REAL NOT NULL, -- e.g. 10.0 for disk_free_pct < 10%
    severity TEXT NOT NULL, -- 'info', 'warning', 'critical'
    webhook_url TEXT NOT NULL DEFAULT '',
    is_enabled INTEGER NOT NULL DEFAULT 1,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS alert_incidents (
    id TEXT PRIMARY KEY,
    rule_id TEXT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    severity TEXT NOT NULL,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open', -- 'open', 'acknowledged', 'resolved'
    trigger_count INTEGER NOT NULL DEFAULT 1,
    acknowledged_by TEXT,
    acknowledged_at DATETIME,
    resolved_by TEXT,
    resolved_at DATETIME,
    first_triggered_at DATETIME NOT NULL,
    last_triggered_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(is_enabled);
CREATE INDEX IF NOT EXISTS idx_alert_incidents_status ON alert_incidents(status);
CREATE INDEX IF NOT EXISTS idx_alert_incidents_device ON alert_incidents(device_id);
CREATE INDEX IF NOT EXISTS idx_alert_incidents_rule ON alert_incidents(rule_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_dedup ON alert_incidents(rule_id, device_id) WHERE status != 'resolved';

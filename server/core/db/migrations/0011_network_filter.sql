-- Migration 0011: Network & Web Filter Security Policies
CREATE TABLE IF NOT EXISTS filter_policies (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    target_type TEXT NOT NULL, -- 'all', 'group', 'device'
    target_id TEXT NOT NULL DEFAULT '',
    is_enabled INTEGER NOT NULL DEFAULT 1,
    priority INTEGER NOT NULL DEFAULT 100,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS filter_rules (
    id TEXT PRIMARY KEY,
    policy_id TEXT NOT NULL REFERENCES filter_policies(id) ON DELETE CASCADE,
    rule_type TEXT NOT NULL, -- 'domain', 'ip_port'
    pattern TEXT NOT NULL,   -- e.g. 'tiktok.com', '198.51.100.0/24:443'
    action TEXT NOT NULL DEFAULT 'block', -- 'block', 'allow'
    category TEXT NOT NULL DEFAULT 'custom',
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS device_filter_states (
    device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
    policy_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'synced', -- 'synced', 'pending', 'tampered', 'failed'
    rules_applied INTEGER NOT NULL DEFAULT 0,
    last_applied_at DATETIME NOT NULL,
    error_message TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_filter_policies_target ON filter_policies(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_filter_rules_policy ON filter_rules(policy_id);

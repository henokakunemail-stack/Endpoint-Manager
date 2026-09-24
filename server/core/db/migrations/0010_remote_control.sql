-- 0010_remote_control.sql
-- Remote control session management, transmission statistics, and forensic tracking

CREATE TABLE IF NOT EXISTS remote_control_sessions (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    operator_id TEXT NOT NULL REFERENCES users(id),
    session_mode TEXT NOT NULL DEFAULT 'full_control', -- 'full_control', 'view_only'
    status TEXT NOT NULL DEFAULT 'active', -- 'active', 'ended', 'rejected'
    frames_transmitted INTEGER NOT NULL DEFAULT 0,
    bytes_transmitted INTEGER NOT NULL DEFAULT 0,
    input_events_count INTEGER NOT NULL DEFAULT 0,
    started_at DATETIME NOT NULL,
    ended_at DATETIME,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rc_sessions_device ON remote_control_sessions(device_id);
CREATE INDEX IF NOT EXISTS idx_rc_sessions_operator ON remote_control_sessions(operator_id);
CREATE INDEX IF NOT EXISTS idx_rc_sessions_status ON remote_control_sessions(status);

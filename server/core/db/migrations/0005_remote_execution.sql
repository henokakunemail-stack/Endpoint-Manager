-- 0005_remote_execution.sql
-- Remote command execution and interactive terminal session tracking

CREATE TABLE IF NOT EXISTS remote_executions (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    operator_id TEXT NOT NULL REFERENCES users(id),
    shell_type TEXT NOT NULL, -- 'powershell', 'cmd', 'bash', 'sh'
    command_text TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'running', 'completed', 'failed', 'timeout'
    exit_code INTEGER,
    output TEXT,
    error_message TEXT,
    started_at DATETIME NOT NULL,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS terminal_sessions (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    operator_id TEXT NOT NULL REFERENCES users(id),
    shell_type TEXT NOT NULL, -- 'powershell', 'cmd', 'bash', 'sh'
    status TEXT NOT NULL DEFAULT 'active', -- 'active', 'closed'
    created_at DATETIME NOT NULL,
    closed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_remote_exec_device ON remote_executions(device_id);
CREATE INDEX IF NOT EXISTS idx_remote_exec_status ON remote_executions(status);
CREATE INDEX IF NOT EXISTS idx_remote_exec_started ON remote_executions(started_at);
CREATE INDEX IF NOT EXISTS idx_term_sessions_device ON terminal_sessions(device_id);
CREATE INDEX IF NOT EXISTS idx_term_sessions_status ON terminal_sessions(status);

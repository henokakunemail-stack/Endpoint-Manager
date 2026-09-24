-- 0009_task_scheduler.sql
-- Script repository, automated maintenance schedules, and execution tracking

CREATE TABLE IF NOT EXISTS script_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    script_type TEXT NOT NULL, -- 'powershell', 'cmd', 'bash', 'sh'
    script_content TEXT NOT NULL,
    sha256_hash TEXT NOT NULL,
    default_args TEXT NOT NULL DEFAULT '',
    timeout_seconds INTEGER NOT NULL DEFAULT 300,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS task_schedules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    script_id TEXT NOT NULL REFERENCES script_templates(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL, -- 'device', 'group', 'all'
    target_id TEXT NOT NULL DEFAULT '',
    schedule_type TEXT NOT NULL, -- 'interval', 'cron', 'once'
    schedule_expr TEXT NOT NULL, -- cron expression or interval minutes
    is_enabled INTEGER NOT NULL DEFAULT 1,
    last_run_at DATETIME,
    next_run_at DATETIME,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS scheduled_task_runs (
    id TEXT PRIMARY KEY,
    schedule_id TEXT NOT NULL REFERENCES task_schedules(id) ON DELETE CASCADE,
    script_id TEXT NOT NULL REFERENCES script_templates(id),
    status TEXT NOT NULL DEFAULT 'running', -- 'running', 'completed', 'failed'
    triggered_at DATETIME NOT NULL,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS scheduled_task_device_runs (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES scheduled_task_runs(id) ON DELETE CASCADE,
    device_id TEXT NOT NULL REFERENCES devices(id),
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'dispatched', 'success', 'failed'
    exit_code INTEGER,
    output_log TEXT,
    error_message TEXT,
    started_at DATETIME,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_task_schedules_enabled ON task_schedules(is_enabled);
CREATE INDEX IF NOT EXISTS idx_scheduled_runs_sched ON scheduled_task_runs(schedule_id);
CREATE INDEX IF NOT EXISTS idx_sched_device_runs_dev ON scheduled_task_device_runs(device_id);
CREATE INDEX IF NOT EXISTS idx_sched_device_runs_run ON scheduled_task_device_runs(run_id);

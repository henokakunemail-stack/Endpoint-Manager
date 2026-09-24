-- 0004_software_deployment.sql
-- Software repository, deployment jobs, and task status tracking

CREATE TABLE IF NOT EXISTS software_packages (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    os_target TEXT NOT NULL, -- 'windows', 'linux', 'macos'
    package_type TEXT NOT NULL, -- 'msi', 'exe', 'deb', 'rpm', 'pkg', 'script'
    file_name TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    sha256 TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    install_args TEXT NOT NULL DEFAULT '',
    uninstall_args TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS software_deployments (
    id TEXT PRIMARY KEY,
    package_id TEXT NOT NULL REFERENCES software_packages(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    target_type TEXT NOT NULL, -- 'device', 'group', 'all'
    target_id TEXT NOT NULL,
    created_by TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running', -- 'running', 'completed', 'failed', 'cancelled'
    created_at DATETIME NOT NULL,
    completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS deployment_tasks (
    id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL REFERENCES software_deployments(id) ON DELETE CASCADE,
    package_id TEXT NOT NULL REFERENCES software_packages(id),
    device_id TEXT NOT NULL REFERENCES devices(id),
    status TEXT NOT NULL DEFAULT 'pending', -- 'pending', 'dispatched', 'downloading', 'installing', 'success', 'failed'
    exit_code INTEGER,
    output_log TEXT,
    error_message TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    completed_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_packages_target ON software_packages(os_target);
CREATE INDEX IF NOT EXISTS idx_deploy_tasks_device ON deployment_tasks(device_id);
CREATE INDEX IF NOT EXISTS idx_deploy_tasks_deploy ON deployment_tasks(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deploy_tasks_status ON deployment_tasks(status);

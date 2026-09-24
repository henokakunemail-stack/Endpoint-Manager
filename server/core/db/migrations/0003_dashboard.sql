-- 0003_dashboard.sql
-- Optimizations for real-time dashboard queries on 500+ device fleet

CREATE INDEX IF NOT EXISTS idx_devices_site ON devices(site);
CREATE INDEX IF NOT EXISTS idx_devices_os ON devices(os_name);
CREATE INDEX IF NOT EXISTS idx_inventory_disk_free ON device_inventory(hw_disk_free_pct);

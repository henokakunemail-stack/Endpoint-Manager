export interface UserToken {
  access_token: string
  refresh_token: string
  expires_at: number
}

export interface UserClaims {
  uid: string
  usr: string
  rol: 'admin' | 'technician' | 'viewer'
}

export interface DeviceDTO {
  id: string
  hostname: string
  os_name: string
  os_version: string
  agent_version: string
  status: 'online' | 'offline'
  last_seen_at: string | null
  site: string
  enrolled_at: string
  retired_at?: string | null
  capabilities?: string[]
}

export interface DeviceListResponse {
  devices: DeviceDTO[]
  count: number
  total: number
  limit: number
  offset: number
}

export interface HardwareInfo {
  cpu?: {
    name?: string
    number_of_cores?: number
    logical_processors?: number
  }
  ram_total_bytes?: number
  disks?: Array<{
    name: string
    filesystem?: string
    total_bytes: number
    free_bytes: number
  }>
  nics?: Array<{
    name: string
    mac_address?: string
    ips?: string[]
  }>
  model?: {
    vendor?: string
    product?: string
    serial_number?: string
  }
}

export interface SoftwareInfo {
  name: string
  version?: string
  publisher?: string
  install_date?: string
  product_code?: string
}

export interface OSDetailInfo {
  edition?: string
  build_number?: string
  install_date?: string
  boot_time?: string
  architecture?: string
}

export interface DeviceInventorySnapshot {
  id?: string
  device_id: string
  hw?: HardwareInfo
  hardware?: HardwareInfo
  software: SoftwareInfo[]
  os?: OSDetailInfo
  os_detail?: OSDetailInfo
  ram_bytes?: number
  disk_free_pct?: number
  cpu_model?: string
  hw_ram_bytes?: number
  hw_disk_free_pct?: number
  hw_cpu_model?: string
  collected_at: string
  updated_at?: string
}

export interface DashboardSummary {
  total_devices: number
  online_devices: number
  offline_devices: number
  retired_devices: number
  online_pct: number
  low_disk_alerts: number
  recent_hw_changes_24h: number
  sites_count: number
}

export interface SiteMetric {
  site: string
  total: number
  online: number
  offline: number
  online_pct: number
}

export interface OSMetric {
  os_name: string
  count: number
  pct: number
}

export interface AlertItem {
  id: string
  type: string
  severity: 'info' | 'warning' | 'critical'
  device_id: string
  hostname: string
  site: string
  message: string
  timestamp: string
}

export interface ActivityItem {
  id: string
  actor_type: string
  actor_id: string
  action: string
  target_id: string
  details: string
  created_at: string
}

export interface SoftwarePackageDTO {
  id: string
  name: string
  version: string
  os_target: 'windows' | 'linux' | 'macos'
  package_type: 'msi' | 'exe' | 'deb' | 'rpm' | 'pkg' | 'script'
  file_name: string
  file_size: number
  sha256: string
  install_args: string
  uninstall_args: string
  created_at: string
  updated_at: string
}

export interface SoftwareDeploymentDTO {
  id: string
  package_id: string
  name: string
  target_type: 'device' | 'group' | 'all'
  target_id: string
  created_by: string
  status: 'running' | 'completed' | 'failed' | 'cancelled'
  created_at: string
  completed_at?: string | null
  package_name?: string
  package_version?: string
  total_tasks: number
  success_tasks: number
  failed_tasks: number
}

export interface DeploymentTaskDTO {
  id: string
  deployment_id: string
  package_id: string
  device_id: string
  status: 'pending' | 'dispatched' | 'downloading' | 'installing' | 'success' | 'failed'
  exit_code?: number | null
  output_log?: string | null
  error_message?: string | null
  created_at: string
  updated_at: string
  completed_at?: string | null
  hostname?: string
  site?: string
}

export interface RemoteExecutionDTO {
  id: string
  device_id: string
  operator_id: string
  shell_type: string
  command_text: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'timeout'
  exit_code?: number | null
  output?: string | null
  error_message?: string | null
  started_at: string
  completed_at?: string | null
  operator_name?: string
  hostname?: string
}

export interface TerminalSessionDTO {
  id: string
  device_id: string
  operator_id: string
  shell_type: string
  status: 'active' | 'closed'
  created_at: string
  closed_at?: string | null
  operator_name?: string
  hostname?: string
}

// Fase 6: Patch Management
export interface PatchSummaryDTO {
  total_devices: number
  compliant_devices: number
  compliance_pct: number
  pending_patches: number
  critical_patches: number
  reboot_required: number
}

export interface DevicePatchStatusDTO {
  device_id: string
  hostname: string
  os_name: string
  site: string
  status: string
  pending_count: number
  critical_count: number
  reboot_required: boolean
  last_scan_at?: string | null
}

export interface PatchDetailDTO {
  id: string
  kb_id: string
  title: string
  severity: string
  category: string
  installed: boolean
  size_bytes: number
  published_at?: string | null
}

// Fase 10: Task Scheduler & Script Repository
export interface ScriptDTO {
  id: string
  name: string
  description: string
  shell_type: string
  script_content: string
  sha256_hash: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface ScheduleDTO {
  id: string
  name: string
  script_id: string
  script_name?: string
  target_type: string
  target_id: string
  schedule_type: string
  cron_expr?: string
  interval_seconds?: number
  is_active: boolean
  next_run_at?: string | null
  created_at: string
}

export interface TaskRunDTO {
  id: string
  schedule_id?: string | null
  script_id: string
  script_name?: string
  device_id: string
  hostname?: string
  status: string
  exit_code?: number | null
  output_log?: string | null
  error_message?: string | null
  started_at: string
  completed_at?: string | null
}

// Fase 12: Network & Web Filter
export interface FilterRuleDTO {
  id: string
  target_type: string
  target_id: string
  rule_type: string
  domain_pattern: string
  category: string
  action: 'block' | 'allow'
  is_active: boolean
  priority: number
  created_at: string
}

export interface DeviceFilterComplianceDTO {
  device_id: string
  hostname: string
  site: string
  active_version: string
  status: string
  last_reported_at?: string | null
}

// Fase 9: Alerting & Incidents
export interface AlertIncidentDTO {
  id: string
  rule_id: string
  rule_name: string
  severity: 'info' | 'warning' | 'critical'
  device_id: string
  hostname?: string
  site?: string
  message: string
  status: 'open' | 'acknowledged' | 'resolved'
  triggered_at: string
  acknowledged_at?: string | null
  resolved_at?: string | null
}

export interface AlertRuleDTO {
  id: string
  name: string
  rule_type: string
  severity: 'info' | 'warning' | 'critical'
  threshold_value: number
  is_active: boolean
  created_at: string
}

// Fase 14: Asset & License Management
export interface HardwareAssetDTO {
  id: string
  asset_tag: string
  device_id?: string | null
  model_name: string
  serial_number: string
  vendor: string
  site: string
  department: string
  assigned_user: string
  purchase_date?: string | null
  purchase_cost: number
  warranty_expires_at?: string | null
  status: 'in_use' | 'in_stock' | 'in_repair' | 'disposed' | 'retired'
  notes: string
  created_at: string
  updated_at: string
}

export interface AssetSummaryDTO {
  total_assets: number
  active_assets: number
  total_valuation: number
  warranty_expiring_count: number
}

export interface SoftwareLicenseDTO {
  id: string
  software_name: string
  publisher: string
  license_key: string
  license_type: string
  total_seats: number
  cost: number
  purchased_at?: string | null
  expires_at?: string | null
  notes: string
  created_at: string
  updated_at: string
}

export interface LicenseComplianceSummaryDTO {
  license_id: string
  software_name: string
  publisher: string
  license_type: string
  total_seats: number
  allocated_seats: number
  installed_detected: number
  expires_at?: string | null
  status: 'compliant' | 'over_allocated' | 'expiring_soon' | 'expired'
}

// Fase 13: Agent Self-Update & Rollouts
export interface AgentReleaseDTO {
  id: string
  version: string
  os_name: string
  arch: string
  file_name: string
  file_size: number
  sha256_checksum: string
  changelog: string
  is_active: boolean
  uploaded_by: string
  created_at: string
}

export interface UpdateCampaignDTO {
  id: string
  name: string
  description?: string
  target_version: string
  target_type: string
  target_id: string
  batch_size: number
  stagger_interval_sec: number
  status: string
  created_by: string
  created_at: string
  updated_at: string
  total_devices?: number
  completed_devices?: number
  failed_devices?: number
}

// Fase 7: User Management
export interface UserDTO {
  id: string
  username: string
  display_name: string
  role: 'admin' | 'technician' | 'viewer'
  status: 'active' | 'deactivated'
  created_at: string
  updated_at: string
}


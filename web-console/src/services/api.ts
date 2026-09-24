import type {
  ActivityItem,
  AlertItem,
  DashboardSummary,
  DeviceDTO,
  DeviceInventorySnapshot,
  DeviceListResponse,
  OSMetric,
  SiteMetric,
  UserToken,
  SoftwarePackageDTO,
  SoftwareDeploymentDTO,
  DeploymentTaskDTO,
  RemoteExecutionDTO,
  TerminalSessionDTO,
  PatchSummaryDTO,
  PatchDetailDTO,
  ScriptDTO,
  ScheduleDTO,
  TaskRunDTO,
  FilterRuleDTO,
  DeviceFilterComplianceDTO,
  AlertIncidentDTO,
  AlertRuleDTO,
  HardwareAssetDTO,
  AssetSummaryDTO,
  SoftwareLicenseDTO,
  LicenseComplianceSummaryDTO,
  AgentReleaseDTO,
  UpdateCampaignDTO,
  UserDTO,
} from '../types/api'

const TOKEN_KEY = 'em_access_token'
const REFRESH_KEY = 'em_refresh_token'

export function getStoredToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function getStoredRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY)
}

export function setStoredTokens(tokens: UserToken) {
  localStorage.setItem(TOKEN_KEY, tokens.access_token)
  localStorage.setItem(REFRESH_KEY, tokens.refresh_token)
}

export function clearStoredTokens() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(REFRESH_KEY)
}

export function parseJwtClaims(token: string): { uid: string; usr: string; rol: string } | null {
  try {
    const base64Url = token.split('.')[1]
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/')
    const jsonPayload = decodeURIComponent(
      atob(base64)
        .split('')
        .map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
        .join('')
    )
    return JSON.parse(jsonPayload)
  } catch {
    return null
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getStoredToken()
  const headers = new Headers(options.headers || {})
  if (!(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const res = await fetch(path, { ...options, headers })

  if (res.status === 401 && !path.includes('/api/auth/login')) {
    // Try to refresh token
    const refreshToken = getStoredRefreshToken()
    if (refreshToken) {
      try {
        const refreshRes = await fetch('/api/auth/refresh', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: refreshToken }),
        })
        if (refreshRes.ok) {
          const newTokens: UserToken = await refreshRes.json()
          setStoredTokens(newTokens)
          headers.set('Authorization', `Bearer ${newTokens.access_token}`)
          const retryRes = await fetch(path, { ...options, headers })
          if (!retryRes.ok) {
            const err = await retryRes.json().catch(() => ({}))
            throw new Error(err.error || `HTTP ${retryRes.status}`)
          }
          return retryRes.json()
        }
      } catch {
        // Refresh failed
      }
    }
    clearStoredTokens()
    window.location.href = '/login'
    throw new Error('Session expired, please log in again')
  }

  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error || `Request failed with HTTP ${res.status}`)
  }

  return res.json()
}

export const api = {
  async login(username: string, password: string): Promise<UserToken> {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    if (!res.ok) {
      const err = await res.json().catch(() => ({}))
      throw new Error(err.error || 'Invalid credentials')
    }
    const tokens: UserToken = await res.json()
    setStoredTokens(tokens)
    return tokens
  },

  async logout(): Promise<void> {
    clearStoredTokens()
  },

  // Dashboard APIs
  async getDashboardSummary(): Promise<DashboardSummary> {
    return request<DashboardSummary>('/api/dashboard/summary')
  },

  async getDashboardSites(): Promise<SiteMetric[]> {
    return request<SiteMetric[]>('/api/dashboard/sites')
  },

  async getDashboardOS(): Promise<OSMetric[]> {
    return request<OSMetric[]>('/api/dashboard/os')
  },

  async getDashboardAlerts(): Promise<AlertItem[]> {
    return request<AlertItem[]>('/api/dashboard/alerts')
  },

  async getDashboardActivity(limit = 15): Promise<ActivityItem[]> {
    return request<ActivityItem[]>(`/api/dashboard/activity?limit=${limit}`)
  },

  // Device Management APIs
  async getDevices(limit = 20, offset = 0, status = '', site = ''): Promise<DeviceListResponse> {
    const params = new URLSearchParams({
      limit: limit.toString(),
      offset: offset.toString(),
    })
    if (status) params.append('status', status)
    if (site) params.append('site', site)
    return request<DeviceListResponse>(`/api/devices?${params.toString()}`)
  },

  async getDevice(id: string): Promise<DeviceDTO> {
    return request<DeviceDTO>(`/api/devices/${id}`)
  },

  async getDeviceInventory(id: string): Promise<DeviceInventorySnapshot> {
    return request<DeviceInventorySnapshot>(`/api/devices/${id}/inventory`)
  },

  async collectInventory(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/devices/${id}/inventory/collect`, {
      method: 'POST',
    })
  },

  async pingDevice(id: string): Promise<{ status: string; command_id: string }> {
    return request<{ status: string; command_id: string }>(`/api/devices/${id}/ping`, {
      method: 'POST',
    })
  },

  async retireDevice(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/devices/${id}/retire`, {
      method: 'POST',
    })
  },

  async restoreDevice(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/devices/${id}/restore`, {
      method: 'POST',
    })
  },

  async getAuditLogs(): Promise<{ logs: ActivityItem[]; count: number }> {
    return request<{ logs: ActivityItem[]; count: number }>('/api/audit-logs')
  },

  // Software Deployment APIs
  async getPackages(): Promise<SoftwarePackageDTO[]> {
    return request<SoftwarePackageDTO[]>('/api/software/packages')
  },

  async uploadPackage(formData: FormData): Promise<SoftwarePackageDTO> {
    return request<SoftwarePackageDTO>('/api/software/packages', {
      method: 'POST',
      body: formData,
    })
  },

  async deletePackage(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/software/packages/${id}`, {
      method: 'DELETE',
    })
  },

  async getDeployments(): Promise<SoftwareDeploymentDTO[]> {
    return request<SoftwareDeploymentDTO[]>('/api/software/deployments')
  },

  async getDeployment(id: string): Promise<SoftwareDeploymentDTO> {
    return request<SoftwareDeploymentDTO>(`/api/software/deployments/${id}`)
  },

  async getDeploymentTasks(id: string): Promise<DeploymentTaskDTO[]> {
    return request<DeploymentTaskDTO[]>(`/api/software/deployments/${id}/tasks`)
  },

  async createDeployment(data: {
    name: string
    package_id: string
    target_type: string
    target_id: string
  }): Promise<{ deployment: SoftwareDeploymentDTO; tasks_total: number; dispatched_live: number }> {
    return request<{ deployment: SoftwareDeploymentDTO; tasks_total: number; dispatched_live: number }>(
      '/api/software/deployments',
      {
        method: 'POST',
        body: JSON.stringify(data),
      }
    )
  },

  // Remote Execution APIs (Fase 5)
  async runRemoteCommand(
    deviceId: string,
    shell: string,
    command: string,
    timeoutSec = 60
  ): Promise<{ status: string; execution: RemoteExecutionDTO }> {
    return request<{ status: string; execution: RemoteExecutionDTO }>(
      `/api/devices/${deviceId}/exec`,
      {
        method: 'POST',
        body: JSON.stringify({ shell, command, timeout_sec: timeoutSec }),
      }
    )
  },

  async getExecutions(deviceId: string): Promise<RemoteExecutionDTO[]> {
    return request<RemoteExecutionDTO[]>(`/api/devices/${deviceId}/executions`)
  },

  async getExecution(deviceId: string, execId: string): Promise<RemoteExecutionDTO> {
    return request<RemoteExecutionDTO>(`/api/devices/${deviceId}/executions/${execId}`)
  },

  async getTerminalSessions(deviceId: string): Promise<TerminalSessionDTO[]> {
    return request<TerminalSessionDTO[]>(`/api/devices/${deviceId}/terminal/sessions`)
  },

  // Patch Management APIs (Fase 6)
  async getPatchSummary(): Promise<PatchSummaryDTO> {
    return request<PatchSummaryDTO>('/api/patches/summary')
  },

  async getDevicePatches(deviceId: string): Promise<{ patches: PatchDetailDTO[]; total: number; pending: number }> {
    return request<{ patches: PatchDetailDTO[]; total: number; pending: number }>(`/api/devices/${deviceId}/patches`)
  },

  async scanDevicePatches(deviceId: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/devices/${deviceId}/patches/scan`, {
      method: 'POST',
    })
  },

  async installDevicePatches(deviceId: string, patchIds: string[], rebootPolicy = 'suppress'): Promise<{ status: string; job_id?: string }> {
    return request<{ status: string; job_id?: string }>(`/api/devices/${deviceId}/patches/install`, {
      method: 'POST',
      body: JSON.stringify({ patch_ids: patchIds, reboot_policy: rebootPolicy }),
    })
  },

  // Task Scheduler & Script Repository APIs (Fase 10)
  async getScripts(): Promise<ScriptDTO[]> {
    return request<ScriptDTO[]>('/api/scheduler/scripts')
  },

  async createScript(data: { name: string; description: string; shell_type: string; script_content: string }): Promise<ScriptDTO> {
    return request<ScriptDTO>('/api/scheduler/scripts', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  async deleteScript(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/scheduler/scripts/${id}`, {
      method: 'DELETE',
    })
  },

  async getSchedules(): Promise<ScheduleDTO[]> {
    return request<ScheduleDTO[]>('/api/scheduler/schedules')
  },

  async createSchedule(data: {
    name: string
    script_id: string
    target_type: string
    target_id: string
    schedule_type: string
    cron_expr?: string
    interval_seconds?: number
  }): Promise<ScheduleDTO> {
    return request<ScheduleDTO>('/api/scheduler/schedules', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  async getTaskRuns(): Promise<TaskRunDTO[]> {
    return request<TaskRunDTO[]>('/api/scheduler/runs')
  },

  // Network & Web Filter APIs (Fase 12)
  async getFilterRules(): Promise<FilterRuleDTO[]> {
    return request<FilterRuleDTO[]>('/api/network-filter/rules')
  },

  async createFilterRule(data: {
    target_type: string
    target_id: string
    rule_type: string
    domain_pattern: string
    category: string
    action: string
  }): Promise<FilterRuleDTO> {
    return request<FilterRuleDTO>('/api/network-filter/rules', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  async deleteFilterRule(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/network-filter/rules/${id}`, {
      method: 'DELETE',
    })
  },

  async applyFilterPolicies(): Promise<{ status: string; version: string; rule_count: number }> {
    return request<{ status: string; version: string; rule_count: number }>('/api/network-filter/apply', {
      method: 'POST',
    })
  },

  async getFilterCompliance(): Promise<DeviceFilterComplianceDTO[]> {
    return request<DeviceFilterComplianceDTO[]>('/api/network-filter/compliance')
  },

  // Alerting & Incidents APIs (Fase 9)
  async getAlertIncidents(status = ''): Promise<{ incidents: AlertIncidentDTO[]; count: number }> {
    const q = status ? `?status=${status}` : ''
    return request<{ incidents: AlertIncidentDTO[]; count: number }>(`/api/alerts/incidents${q}`)
  },

  async getAlertRules(): Promise<AlertRuleDTO[]> {
    return request<AlertRuleDTO[]>('/api/alerts/rules')
  },

  async acknowledgeIncident(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/alerts/incidents/${id}/acknowledge`, {
      method: 'POST',
    })
  },

  async resolveIncident(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/alerts/incidents/${id}/resolve`, {
      method: 'POST',
    })
  },

  // Asset & License Management APIs (Fase 14)
  async getAssets(site = '', status = ''): Promise<HardwareAssetDTO[]> {
    const params = new URLSearchParams()
    if (site) params.append('site', site)
    if (status) params.append('status', status)
    const q = params.toString() ? `?${params.toString()}` : ''
    return request<HardwareAssetDTO[]>(`/api/assets${q}`)
  },

  async getAssetSummary(): Promise<AssetSummaryDTO> {
    return request<AssetSummaryDTO>('/api/assets/summary')
  },

  async createAsset(data: Partial<HardwareAssetDTO>): Promise<HardwareAssetDTO> {
    return request<HardwareAssetDTO>('/api/assets', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  async updateAsset(id: string, data: Partial<HardwareAssetDTO>): Promise<HardwareAssetDTO> {
    return request<HardwareAssetDTO>(`/api/assets/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  },

  async deleteAsset(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/assets/${id}`, {
      method: 'DELETE',
    })
  },

  async getLicenses(): Promise<SoftwareLicenseDTO[]> {
    return request<SoftwareLicenseDTO[]>('/api/licenses')
  },

  async createLicense(data: Partial<SoftwareLicenseDTO>): Promise<SoftwareLicenseDTO> {
    return request<SoftwareLicenseDTO>('/api/licenses', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  async getLicenseCompliance(): Promise<{ audited_at: string; compliance: LicenseComplianceSummaryDTO[] }> {
    return request<{ audited_at: string; compliance: LicenseComplianceSummaryDTO[] }>('/api/licenses/compliance')
  },

  async allocateLicense(licenseId: string, deviceId: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/licenses/${licenseId}/allocate`, {
      method: 'POST',
      body: JSON.stringify({ device_id: deviceId }),
    })
  },

  // Agent Self-Update APIs (Fase 13)
  async getAgentReleases(): Promise<AgentReleaseDTO[]> {
    return request<AgentReleaseDTO[]>('/api/agent-updates/releases')
  },

  async uploadAgentRelease(formData: FormData): Promise<AgentReleaseDTO> {
    return request<AgentReleaseDTO>('/api/agent-updates/releases', {
      method: 'POST',
      body: formData,
    })
  },

  async getUpdateCampaigns(): Promise<UpdateCampaignDTO[]> {
    return request<UpdateCampaignDTO[]>('/api/agent-updates/campaigns')
  },

  async createUpdateCampaign(data: {
    name: string
    target_version: string
    target_type: string
    target_id: string
    batch_size: number
    stagger_interval_sec: number
  }): Promise<UpdateCampaignDTO> {
    return request<UpdateCampaignDTO>('/api/agent-updates/campaigns', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  async dispatchDeviceUpdate(deviceId: string, targetVersion: string): Promise<{ status: string; task_id: string }> {
    return request<{ status: string; task_id: string }>(`/api/devices/${deviceId}/update/dispatch`, {
      method: 'POST',
      body: JSON.stringify({ target_version: targetVersion }),
    })
  },

  // User Management APIs (Fase 7)
  async getUsers(): Promise<UserDTO[]> {
    return request<UserDTO[]>('/api/users')
  },

  async createUser(data: { username: string; display_name: string; password?: string; role: string }): Promise<UserDTO> {
    return request<UserDTO>('/api/users', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  async updateUser(id: string, data: { display_name?: string; role?: string }): Promise<UserDTO> {
    return request<UserDTO>(`/api/users/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  },

  async deactivateUser(id: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/users/${id}/deactivate`, {
      method: 'POST',
    })
  },

  async adminResetPassword(id: string, password: string): Promise<{ status: string }> {
    return request<{ status: string }>(`/api/users/${id}/password`, {
      method: 'PUT',
      body: JSON.stringify({ new_password: password }),
    })
  },

  // Reports Export URL Helper
  getReportExportUrl(reportType: 'inventory' | 'patches' | 'deployments' | 'audit', format: 'csv' | 'json'): string {
    const token = getStoredToken()
    return `/api/reports/${reportType}?format=${format}${token ? `&token=${encodeURIComponent(token)}` : ''}`
  },
}

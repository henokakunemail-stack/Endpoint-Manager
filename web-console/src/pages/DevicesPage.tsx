import React, { useCallback, useEffect, useState } from 'react'
import {
  Activity,
  AlertCircle,
  Archive,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Filter,
  Laptop,
  Monitor,
  RefreshCw,
  Search,
  Terminal,
  TerminalSquare,
} from 'lucide-react'
import { DeviceDetailModal } from '../components/DeviceDetailModal'
import { useAuth } from '../context/AuthContext'
import { useToast } from '../context/ToastContext'
import { api } from '../services/api'
import type { DeviceDTO, DeviceListResponse } from '../types/api'

export const DevicesPage: React.FC<{
  onOpenExec?: (device: DeviceDTO) => void
  onOpenTerminal?: (device: DeviceDTO, shell: string) => void
  onOpenRemoteControl?: (device: DeviceDTO) => void
}> = ({ onOpenExec, onOpenTerminal, onOpenRemoteControl }) => {
  const { user } = useAuth()
  const [data, setData] = useState<DeviceListResponse>({
    devices: [],
    count: 0,
    total: 0,
    limit: 10,
    offset: 0,
  })
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [siteFilter, setSiteFilter] = useState('')
  const [currentPage, setCurrentPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [selectedDevice, setSelectedDevice] = useState<DeviceDTO | null>(null)
  const [actionMsg, setActionMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(
    null
  )
  const toast = useToast()

  const fetchDevices = useCallback(async () => {
    setLoading(true)
    try {
      const offset = (currentPage - 1) * pageSize
      const res = await api.getDevices(pageSize, offset, statusFilter, siteFilter)
      setData(res)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to fetch devices'
      setActionMsg({ type: 'error', text: msg })
    } finally {
      setLoading(false)
    }
  }, [currentPage, pageSize, statusFilter, siteFilter])

  useEffect(() => {
    fetchDevices()
  }, [fetchDevices])

  const filteredItems = (data.devices || []).filter((d: DeviceDTO) => {
    if (!search) return true
    const term = search.toLowerCase()
    return (
      d.hostname.toLowerCase().includes(term) ||
      d.id.toLowerCase().includes(term) ||
      d.os_name.toLowerCase().includes(term) ||
      (d.site && d.site.toLowerCase().includes(term))
    )
  })

  const totalPages = Math.max(1, Math.ceil(data.total / pageSize))

  const handleRetire = async (device: DeviceDTO) => {
    if (!window.confirm(`Are you sure you want to retire endpoint "${device.hostname}"?`)) {
      return
    }
    try {
      await api.retireDevice(device.id)
      const successText = `Device ${device.hostname} retired successfully.`
      setActionMsg({
        type: 'success',
        text: successText,
      })
      toast.success(successText, 'Device Retired')
      fetchDevices()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Retire failed'
      setActionMsg({ type: 'error', text: msg })
      toast.error(msg, 'Retire Failed')
    }
  }

  const handlePing = async (device: DeviceDTO) => {
    try {
      const res = await api.pingDevice(device.id)
      const successText = `Ping sent to ${device.hostname} (Command: ${res.command_id})`
      setActionMsg({
        type: 'success',
        text: successText,
      })
      toast.info(successText, 'Ping Dispatched')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Ping failed'
      setActionMsg({ type: 'error', text: msg })
      toast.error(msg, 'Ping Failed')
    }
  }

  const canManage = user?.rol === 'admin' || user?.rol === 'technician'

  return (
    <div className="page-container devices-page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Fleet Endpoint Inventory</h1>
          <p className="page-subtitle">
            Manage enrolled workstations, servers, and telemetry data
          </p>
        </div>
        <div className="header-controls">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={fetchDevices}
            disabled={loading}
          >
            <RefreshCw size={16} className={loading ? 'spinning' : ''} />
            <span>Refresh</span>
          </button>
        </div>
      </div>

      {actionMsg && (
        <div className={`notification-banner ${actionMsg.type}`}>
          {actionMsg.type === 'success' ? <CheckCircle2 size={18} /> : <AlertCircle size={18} />}
          <span>{actionMsg.text}</span>
          <button
            type="button"
            className="banner-dismiss"
            onClick={() => setActionMsg(null)}
          >
            &times;
          </button>
        </div>
      )}

      {/* Filter / Search Bar */}
      <div className="filter-bar">
        <div className="search-wrap">
          <Search size={18} className="search-icon" />
          <input
            type="text"
            className="search-input"
            placeholder="Search by hostname, device ID, OS, or site..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>

        <div className="filter-group">
          <Filter size={16} className="filter-icon" />
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value)
              setCurrentPage(1)
            }}
            className="select-input"
          >
            <option value="">All Statuses</option>
            <option value="online">Online Only</option>
            <option value="offline">Offline Only</option>
            <option value="retired">Retired Only</option>
          </select>

          <select
            value={siteFilter}
            onChange={(e) => {
              setSiteFilter(e.target.value)
              setCurrentPage(1)
            }}
            className="select-input"
          >
            <option value="">All Sites</option>
            <option value="hq">Headquarters (HQ)</option>
            <option value="branch-a">Branch A</option>
            <option value="branch-b">Branch B</option>
          </select>

          <select
            value={pageSize}
            onChange={(e) => {
              setPageSize(Number(e.target.value))
              setCurrentPage(1)
            }}
            className="select-input"
          >
            <option value={10}>10 / page</option>
            <option value={25}>25 / page</option>
            <option value={50}>50 / page</option>
          </select>
        </div>
      </div>

      {/* Table */}
      <div className="table-card">
        <div className="table-wrapper">
          <table className="data-table">
            <thead>
              <tr>
                <th>Status</th>
                <th>Hostname & Identity</th>
                <th>Operating System</th>
                <th>Site</th>
                <th>Agent</th>
                <th>Last Heartbeat</th>
                <th className="text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={7} className="text-center py-8">
                    <div className="table-loader">
                      <RefreshCw size={24} className="spinning" />
                      <span>Loading fleet records...</span>
                    </div>
                  </td>
                </tr>
              ) : filteredItems.length === 0 ? (
                <tr>
                  <td colSpan={7} className="text-center py-8">
                    <div className="empty-state">
                      <Laptop size={32} />
                      <p>No endpoints match the current filters</p>
                    </div>
                  </td>
                </tr>
              ) : (
                filteredItems.map((d: DeviceDTO) => {
                  const isRetired = Boolean(d.retired_at)
                  return (
                  <tr key={d.id} className="device-row">
                    <td>
                      <span className={`status-pill ${isRetired ? 'retired' : d.status}`}>
                        <span className="dot"></span>
                        {isRetired ? 'RETIRED' : d.status.toUpperCase()}
                      </span>
                    </td>
                    <td>
                      <div
                        className="device-identity-cell"
                        onClick={() => setSelectedDevice(d)}
                        title="Click to view hardware specs & software"
                      >
                        <strong className="device-name">{d.hostname}</strong>
                        <span className="device-id font-mono">{d.id.substring(0, 16)}...</span>
                      </div>
                    </td>
                    <td>
                      <div className="os-cell">
                        <span className="os-name">{d.os_name}</span>
                        <span className="os-version">{d.os_version}</span>
                      </div>
                    </td>
                    <td>
                      <span className="site-badge">{d.site || 'HQ'}</span>
                    </td>
                    <td>
                      <span className="font-mono text-sm">{d.agent_version || '0.1.0'}</span>
                    </td>
                    <td>
                      <span className="timestamp-cell">
                        {d.last_seen_at
                          ? new Date(d.last_seen_at).toLocaleTimeString()
                          : 'Never'}
                      </span>
                    </td>
                    <td className="text-right">
                      <div className="row-actions">
                        <button
                          type="button"
                          className="btn btn-sm btn-secondary"
                          onClick={() => setSelectedDevice(d)}
                          title="View Hardware, Disks & Software"
                        >
                          Specs
                        </button>
                        {d.status === 'online' && !isRetired && canManage && (
                          <button
                            type="button"
                            className="btn btn-sm btn-primary"
                            onClick={() => onOpenExec?.(d)}
                            title="Run Remote Command"
                          >
                            <TerminalSquare size={12} />
                          </button>
                        )}
                        {d.status === 'online' && !isRetired && canManage && (
                          <button
                            type="button"
                            className="btn btn-sm btn-primary"
                            onClick={() =>
                              onOpenTerminal?.(
                                d,
                                d.os_name === 'windows' ? 'powershell' : 'bash'
                              )
                            }
                            title="Open Interactive Terminal"
                          >
                            <Terminal size={12} />
                          </button>
                        )}
                        {d.status === 'online' && !isRetired && canManage && (
                          <button
                            type="button"
                            className="btn btn-sm btn-primary"
                            onClick={() => onOpenRemoteControl?.(d)}
                            title="Open Remote Desktop Screen & Control"
                          >
                            <Monitor size={12} />
                          </button>
                        )}
                        {d.status === 'online' && !isRetired && (
                          <button
                            type="button"
                            className="btn btn-sm btn-secondary"
                            onClick={() => handlePing(d)}
                            title="Send Ping"
                          >
                            <Activity size={12} />
                          </button>
                        )}
                        {canManage && !isRetired && (
                          <button
                            type="button"
                            className="btn btn-sm btn-danger-outline"
                            onClick={() => handleRetire(d)}
                            title="Retire Device"
                          >
                            <Archive size={12} />
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                )})
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination Bar */}
        <div className="pagination-bar">
          <div className="pagination-info">
            Showing {data.total > 0 ? data.offset + 1 : 0} to{' '}
            {Math.min(data.offset + (data.devices || []).length, data.total)} of {data.total}{' '}
            endpoints
          </div>
          <div className="pagination-controls">
            <button
              type="button"
              className="btn btn-sm btn-secondary"
              onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
              disabled={currentPage <= 1}
            >
              <ChevronLeft size={16} />
              <span>Previous</span>
            </button>
            <span className="page-indicator">
              Page {currentPage} of {totalPages}
            </span>
            <button
              type="button"
              className="btn btn-sm btn-secondary"
              onClick={() => setCurrentPage((p) => p + 1)}
              disabled={currentPage >= totalPages}
            >
              <span>Next</span>
              <ChevronRight size={16} />
            </button>
          </div>
        </div>
      </div>

      {/* Detail Modal */}
      {selectedDevice && (
        <DeviceDetailModal
          device={selectedDevice}
          onClose={() => setSelectedDevice(null)}
        />
      )}
    </div>
  )
}

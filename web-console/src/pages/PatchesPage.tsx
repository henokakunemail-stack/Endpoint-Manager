import React, { useEffect, useState } from 'react'
import {
  CheckCircle2,
  Clock,
  Play,
  RefreshCw,
  RotateCcw,
  Search,
  ShieldAlert,
  ShieldCheck,
  X,
} from 'lucide-react'
import { api } from '../services/api'
import { useToast } from '../context/ToastContext'
import type { DeviceDTO, PatchDetailDTO, PatchSummaryDTO } from '../types/api'

export const PatchesPage: React.FC = () => {
  const [summary, setSummary] = useState<PatchSummaryDTO | null>(null)
  const [devices, setDevices] = useState<DeviceDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedDevice, setSelectedDevice] = useState<DeviceDTO | null>(null)
  const [devicePatches, setDevicePatches] = useState<PatchDetailDTO[]>([])
  const [patchesLoading, setPatchesLoading] = useState(false)
  const [scanningId, setScanningId] = useState<string | null>(null)
  const [actionMsg, setActionMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const toast = useToast()

  const loadData = async () => {
    try {
      setLoading(true)
      const [sum, devList] = await Promise.all([
        api.getPatchSummary().catch(() => ({
          total_devices: 0,
          compliant_devices: 0,
          compliance_pct: 100,
          pending_patches: 0,
          critical_patches: 0,
          reboot_required: 0,
        })),
        api.getDevices(100, 0),
      ])
      setSummary(sum)
      setDevices(devList.devices || [])
    } catch (err: any) {
      setActionMsg({ type: 'error', text: err.message || 'Failed to load patch data' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const handleOpenPatches = async (device: DeviceDTO) => {
    setSelectedDevice(device)
    setPatchesLoading(true)
    try {
      const res = await api.getDevicePatches(device.id)
      setDevicePatches(res.patches || [])
    } catch {
      setDevicePatches([])
    } finally {
      setPatchesLoading(false)
    }
  }

  const handleScan = async (deviceId: string) => {
    setScanningId(deviceId)
    try {
      await api.scanDevicePatches(deviceId)
      setActionMsg({ type: 'success', text: 'Patch scan dispatched to device' })
      toast.info('Patch scan dispatched to device via live WebSocket', 'Scan Triggered')
      setTimeout(loadData, 2000)
    } catch (err: any) {
      const errMsg = err.message || 'Scan failed'
      setActionMsg({ type: 'error', text: errMsg })
      toast.error(errMsg, 'Scan Failed')
    } finally {
      setScanningId(null)
    }
  }

  const handleInstallAll = async (deviceId: string) => {
    try {
      const patchIds = devicePatches.map((p) => p.id)
      await api.installDevicePatches(deviceId, patchIds, 'suppress')
      const msg = `Install dispatched for ${patchIds.length} patches`
      setActionMsg({ type: 'success', text: msg })
      toast.success(msg, 'Rollout Started')
      setSelectedDevice(null)
      setTimeout(loadData, 2000)
    } catch (err: any) {
      const errMsg = err.message || 'Install failed'
      setActionMsg({ type: 'error', text: errMsg })
      toast.error(errMsg, 'Install Failed')
    }
  }

  const filteredDevices = devices.filter((d) => {
    const matchSearch =
      d.hostname.toLowerCase().includes(searchTerm.toLowerCase()) ||
      d.site.toLowerCase().includes(searchTerm.toLowerCase()) ||
      d.os_name.toLowerCase().includes(searchTerm.toLowerCase())
    return matchSearch
  })

  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <h2 className="page-title">Patch Management & Compliance</h2>
          <p className="page-subtitle">
            Enterprise OS patch monitoring, vulnerability remediation, and scheduled update rollouts
          </p>
        </div>
        <div className="header-controls">
          <button type="button" className="btn btn-secondary" onClick={loadData} disabled={loading}>
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            <span>Refresh</span>
          </button>
        </div>
      </div>

      {actionMsg && (
        <div className={`alert-banner ${actionMsg.type === 'error' ? 'alert-error' : 'alert-success'}`}>
          <span>{actionMsg.text}</span>
          <button type="button" onClick={() => setActionMsg(null)} className="close-btn">
            <X size={14} />
          </button>
        </div>
      )}

      {/* KPI Cards */}
      {summary && (
        <div className="kpi-grid">
          <div className="kpi-card">
            <div className="kpi-header">
              <span className="kpi-label">Fleet Compliance</span>
              <ShieldCheck className="kpi-icon text-success" size={20} />
            </div>
            <div className="kpi-value text-success">{Math.round(summary.compliance_pct)}%</div>
            <span className="kpi-hint">
              {summary.compliant_devices} of {summary.total_devices} devices up to date
            </span>
          </div>

          <div className="kpi-card">
            <div className="kpi-header">
              <span className="kpi-label">Pending Updates</span>
              <Clock className="kpi-icon text-warning" size={20} />
            </div>
            <div className="kpi-value text-warning">{summary.pending_patches}</div>
            <span className="kpi-hint">Patches awaiting deployment</span>
          </div>

          <div className="kpi-card">
            <div className="kpi-header">
              <span className="kpi-label">Critical / Security</span>
              <ShieldAlert className="kpi-icon text-danger" size={20} />
            </div>
            <div className="kpi-value text-danger">{summary.critical_patches}</div>
            <span className="kpi-hint">High priority CVE remediations</span>
          </div>

          <div className="kpi-card">
            <div className="kpi-header">
              <span className="kpi-label">Reboot Required</span>
              <RotateCcw className="kpi-icon text-primary" size={20} />
            </div>
            <div className="kpi-value">{summary.reboot_required}</div>
            <span className="kpi-hint">Pending restart to finalize</span>
          </div>
        </div>
      )}

      {/* Fleet Patch Table */}
      <div className="table-card">
        <div className="table-toolbar">
          <div className="search-wrap">
            <Search size={16} className="search-icon" />
            <input
              type="text"
              className="search-input"
              placeholder="Search by hostname, site, or OS..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>
        </div>

        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>Hostname</th>
                <th>OS & Architecture</th>
                <th>Branch Site</th>
                <th>Status</th>
                <th>Patch Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredDevices.length === 0 ? (
                <tr>
                  <td colSpan={5} className="text-center py-8 text-muted">
                    No devices matching the current filter.
                  </td>
                </tr>
              ) : (
                filteredDevices.map((d) => (
                  <tr key={d.id}>
                    <td>
                      <div className="device-host-cell">
                        <span className="host-name">{d.hostname}</span>
                        <span className="device-id-sub font-mono">{d.id.slice(0, 12)}...</span>
                      </div>
                    </td>
                    <td>
                      <span className="os-badge">{d.os_name} {d.os_version}</span>
                    </td>
                    <td>{d.site || 'Default Site'}</td>
                    <td>
                      <span className={`status-pill ${d.status === 'online' ? 'online' : 'offline'}`}>
                        {d.status}
                      </span>
                    </td>
                    <td>
                      <div className="action-buttons">
                        <button
                          type="button"
                          className="btn-action"
                          onClick={() => handleOpenPatches(d)}
                          title="View available patches"
                        >
                          <ShieldCheck size={15} />
                          <span>View Patches</span>
                        </button>
                        <button
                          type="button"
                          className="btn-action"
                          onClick={() => handleScan(d.id)}
                          disabled={scanningId === d.id || d.status !== 'online'}
                          title="Trigger WUA / Package Manager Scan"
                        >
                          <RefreshCw size={14} className={scanningId === d.id ? 'animate-spin' : ''} />
                          <span>Scan</span>
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Device Patch Detail Modal */}
      {selectedDevice && (
        <div className="modal-overlay" onClick={() => setSelectedDevice(null)}>
          <div className="modal-dialog modal-lg" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <div>
                <h3 className="modal-title">Patch Assessment: {selectedDevice.hostname}</h3>
                <span className="modal-subtitle">
                  {selectedDevice.os_name} {selectedDevice.os_version} — Site: {selectedDevice.site}
                </span>
              </div>
              <button type="button" className="btn-close" onClick={() => setSelectedDevice(null)}>
                <X size={18} />
              </button>
            </div>

            <div className="modal-body">
              {patchesLoading ? (
                <div className="py-8 text-center text-muted">
                  <div className="spinner-inline"></div> Loading patch manifest...
                </div>
              ) : devicePatches.length === 0 ? (
                <div className="py-8 text-center">
                  <CheckCircle2 size={40} className="text-success mx-auto mb-2" />
                  <h4 className="text-success font-semibold">Device is Fully Compliant</h4>
                  <p className="text-muted text-sm mt-1">
                    No missing security updates or pending hotfixes detected.
                  </p>
                </div>
              ) : (
                <div className="patch-list">
                  <div className="patch-list-header text-sm text-muted mb-2">
                    Found {devicePatches.length} available updates
                  </div>
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>KB / Reference</th>
                        <th>Title</th>
                        <th>Severity</th>
                        <th>Category</th>
                      </tr>
                    </thead>
                    <tbody>
                      {devicePatches.map((p) => (
                        <tr key={p.id}>
                          <td className="font-mono text-sm">{p.kb_id || p.id}</td>
                          <td>{p.title}</td>
                          <td>
                            <span className={`badge-severity ${p.severity?.toLowerCase() || 'medium'}`}>
                              {p.severity || 'Important'}
                            </span>
                          </td>
                          <td>{p.category || 'Security Update'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>

            <div className="modal-footer">
              <button type="button" className="btn btn-secondary" onClick={() => setSelectedDevice(null)}>
                Close
              </button>
              {devicePatches.length > 0 && selectedDevice.status === 'online' && (
                <button
                  type="button"
                  className="btn btn-primary"
                  onClick={() => handleInstallAll(selectedDevice.id)}
                >
                  <Play size={16} />
                  <span>Install All Patches</span>
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

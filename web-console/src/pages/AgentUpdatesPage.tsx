import React, { useEffect, useState } from 'react'
import {
  ArrowUpCircle,
  FileCode,
  Layers,
  Play,
  RefreshCw,
  ShieldCheck,
  UploadCloud,
  X,
} from 'lucide-react'
import { api } from '../services/api'
import { useToast } from '../context/ToastContext'
import type { AgentReleaseDTO, UpdateCampaignDTO } from '../types/api'

export const AgentUpdatesPage: React.FC = () => {
  const [subTab, setSubTab] = useState<'releases' | 'campaigns'>('releases')
  const [releases, setReleases] = useState<AgentReleaseDTO[]>([])
  const [campaigns, setCampaigns] = useState<UpdateCampaignDTO[]>([])
  const [loading, setLoading] = useState(true)

  // Modals
  const [isReleaseModalOpen, setIsReleaseModalOpen] = useState(false)
  const [isCampaignModalOpen, setIsCampaignModalOpen] = useState(false)
  const [msg, setMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const toast = useToast()

  // Upload Release Form
  const [relVersion, setRelVersion] = useState('')
  const [relOS, setRelOS] = useState('windows')
  const [relArch, setRelArch] = useState('amd64')
  const [relChangelog, setRelChangelog] = useState('')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [uploading, setUploading] = useState(false)

  // New Campaign Form
  const [newCampaign, setNewCampaign] = useState({
    name: '',
    target_version: '',
    target_type: 'all',
    target_id: '',
    batch_size: 25,
    stagger_interval_sec: 60,
  })

  const loadData = async () => {
    setLoading(true)
    try {
      const [rList, cList] = await Promise.all([
        api.getAgentReleases().catch(() => []),
        api.getUpdateCampaigns().catch(() => []),
      ])
      setReleases(rList)
      setCampaigns(cList)
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to load update catalog' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const handleUploadRelease = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedFile) {
      setMsg({ type: 'error', text: 'Please select an agent binary file to upload' })
      return
    }

    setUploading(true)
    try {
      const fd = new FormData()
      fd.append('file', selectedFile)
      fd.append('version', relVersion)
      fd.append('os_name', relOS)
      fd.append('arch', relArch)
      fd.append('changelog', relChangelog)

      await api.uploadAgentRelease(fd)
      const successText = `Release v${relVersion} (${relOS}/${relArch}) uploaded and registered.`
      setMsg({ type: 'success', text: successText })
      toast.success(successText, 'Release Uploaded')
      setIsReleaseModalOpen(false)
      setSelectedFile(null)
      setRelVersion('')
      setRelChangelog('')
      loadData()
    } catch (err: any) {
      const errorText = err.message || 'Failed to upload release binary'
      setMsg({ type: 'error', text: errorText })
      toast.error(errorText, 'Upload Failed')
    } finally {
      setUploading(false)
    }
  }

  const handleCreateCampaign = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.createUpdateCampaign(newCampaign)
      const successText = `Update campaign '${newCampaign.name}' launched successfully.`
      setMsg({ type: 'success', text: successText })
      toast.success(successText, 'Campaign Launched')
      setIsCampaignModalOpen(false)
      loadData()
    } catch (err: any) {
      const errorText = err.message || 'Failed to launch update campaign'
      setMsg({ type: 'error', text: errorText })
      toast.error(errorText, 'Launch Failed')
    }
  }

  const formatFileSize = (bytes: number) => {
    if (!bytes) return '—'
    const mb = bytes / (1024 * 1024)
    return `${mb.toFixed(2)} MB`
  }

  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <h2 className="page-title">Agent Self-Update & Rollout Campaigns</h2>
          <p className="page-subtitle">
            Autonomous in-place binary upgrades, SHA-256 verification, and phased canary deployments
          </p>
        </div>
        <div className="header-controls">
          <button type="button" className="btn btn-secondary" onClick={loadData} disabled={loading}>
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            <span>Refresh</span>
          </button>
          {subTab === 'releases' && (
            <button type="button" className="btn btn-primary" onClick={() => setIsReleaseModalOpen(true)}>
              <UploadCloud size={16} />
              <span>Publish Release Binary</span>
            </button>
          )}
          {subTab === 'campaigns' && (
            <button type="button" className="btn btn-primary" onClick={() => setIsCampaignModalOpen(true)}>
              <Play size={16} />
              <span>Launch Update Campaign</span>
            </button>
          )}
        </div>
      </div>

      {msg && (
        <div className={`alert-banner ${msg.type === 'error' ? 'alert-error' : 'alert-success'}`}>
          <span>{msg.text}</span>
          <button type="button" onClick={() => setMsg(null)} className="close-btn">
            <X size={14} />
          </button>
        </div>
      )}

      {/* KPI Cards */}
      <div className="kpi-grid">
        <div className="kpi-card">
          <div className="kpi-header">
            <span className="kpi-label">Available Releases</span>
            <ArrowUpCircle className="kpi-icon text-primary" size={20} />
          </div>
          <div className="kpi-value">{releases.length}</div>
          <span className="kpi-hint">Binaries across OS & CPU architectures</span>
        </div>

        <div className="kpi-card">
          <div className="kpi-header">
            <span className="kpi-label">Active Campaigns</span>
            <Layers className="kpi-icon text-warning" size={20} />
          </div>
          <div className="kpi-value text-warning">
            {campaigns.filter((c) => c.status === 'in_progress').length}
          </div>
          <span className="kpi-hint">Fleet rollout waves underway</span>
        </div>

        <div className="kpi-card">
          <div className="kpi-header">
            <span className="kpi-label">Integrity Enforced</span>
            <ShieldCheck className="kpi-icon text-success" size={20} />
          </div>
          <div className="kpi-value text-success">100%</div>
          <span className="kpi-hint">Pre-execution SHA-256 validation</span>
        </div>
      </div>

      {/* Tabs */}
      <div className="tabs-nav">
        <button
          type="button"
          className={`tab-btn ${subTab === 'releases' ? 'active' : ''}`}
          onClick={() => setSubTab('releases')}
        >
          <FileCode size={16} />
          <span>Release Catalog ({releases.length})</span>
        </button>
        <button
          type="button"
          className={`tab-btn ${subTab === 'campaigns' ? 'active' : ''}`}
          onClick={() => setSubTab('campaigns')}
        >
          <Layers size={16} />
          <span>Rollout Campaigns ({campaigns.length})</span>
        </button>
      </div>

      {/* Tab 1: Releases Catalog */}
      {subTab === 'releases' && (
        <div className="table-card">
          <div className="table-responsive">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Version</th>
                  <th>Platform & Arch</th>
                  <th>Filename</th>
                  <th>Size</th>
                  <th>SHA-256 Checksum</th>
                  <th>Uploaded By</th>
                  <th>Published Date</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {releases.length === 0 ? (
                  <tr>
                    <td colSpan={8} className="text-center py-8 text-muted">
                      No agent releases published yet. Click "Publish Release Binary" to upload an agent executable.
                    </td>
                  </tr>
                ) : (
                  releases.map((r) => (
                    <tr key={r.id}>
                      <td>
                        <span className="font-semibold text-primary">v{r.version}</span>
                      </td>
                      <td>
                        <span className="os-badge">{r.os_name} / {r.arch}</span>
                      </td>
                      <td className="font-mono text-sm">{r.file_name}</td>
                      <td>{formatFileSize(r.file_size)}</td>
                      <td>
                        <span className="font-mono text-xs text-dim" title={r.sha256_checksum}>
                          {r.sha256_checksum ? r.sha256_checksum.slice(0, 16) + '...' : '—'}
                        </span>
                      </td>
                      <td>{r.uploaded_by}</td>
                      <td className="text-sm text-muted">{new Date(r.created_at).toLocaleString()}</td>
                      <td>
                        <span className={`status-pill ${r.is_active ? 'online' : 'offline'}`}>
                          {r.is_active ? 'Active' : 'Archived'}
                        </span>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Tab 2: Campaigns */}
      {subTab === 'campaigns' && (
        <div className="table-card">
          <div className="table-responsive">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Campaign Name</th>
                  <th>Target Version</th>
                  <th>Scope</th>
                  <th>Batch / Interval</th>
                  <th>Status</th>
                  <th>Progress</th>
                  <th>Created At</th>
                </tr>
              </thead>
              <tbody>
                {campaigns.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="text-center py-8 text-muted">
                      No rollout campaigns created yet. Click "Launch Update Campaign" to roll out an upgrade.
                    </td>
                  </tr>
                ) : (
                  campaigns.map((c) => (
                    <tr key={c.id}>
                      <td className="font-semibold text-main">{c.name}</td>
                      <td>
                        <span className="font-semibold text-primary">v{c.target_version}</span>
                      </td>
                      <td>
                        <span className="badge-target">{c.target_type}</span>
                      </td>
                      <td>
                        {c.batch_size} devices / {c.stagger_interval_sec}s
                      </td>
                      <td>
                        <span
                          className={`status-pill ${
                            c.status === 'completed'
                              ? 'online'
                              : c.status === 'in_progress'
                              ? 'warning'
                              : 'offline'
                          }`}
                        >
                          {c.status}
                        </span>
                      </td>
                      <td>
                        <div className="text-sm font-semibold">
                          {c.completed_devices || 0} / {c.total_devices || 0} updated
                        </div>
                      </td>
                      <td className="text-sm text-muted">{new Date(c.created_at).toLocaleString()}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Publish Release Modal */}
      {isReleaseModalOpen && (
        <div className="modal-overlay" onClick={() => setIsReleaseModalOpen(false)}>
          <div className="modal-dialog modal-md" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Publish Agent Binary Release</h3>
              <button type="button" className="btn-close" onClick={() => setIsReleaseModalOpen(false)}>
                <X size={18} />
              </button>
            </div>
            <form onSubmit={handleUploadRelease}>
              <div className="modal-body space-y-4">
                <div className="form-group">
                  <label className="form-label">Version Number</label>
                  <input
                    type="text"
                    className="form-input"
                    required
                    placeholder="e.g. 1.2.0"
                    value={relVersion}
                    onChange={(e) => setRelVersion(e.target.value)}
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="form-group">
                    <label className="form-label">Target OS</label>
                    <select className="form-select" value={relOS} onChange={(e) => setRelOS(e.target.value)}>
                      <option value="windows">Windows</option>
                      <option value="linux">Linux</option>
                      <option value="darwin">macOS (Darwin)</option>
                    </select>
                  </div>
                  <div className="form-group">
                    <label className="form-label">Architecture</label>
                    <select className="form-select" value={relArch} onChange={(e) => setRelArch(e.target.value)}>
                      <option value="amd64">x86_64 (amd64)</option>
                      <option value="arm64">ARM64 (Apple Silicon / aarch64)</option>
                    </select>
                  </div>
                </div>
                <div className="form-group">
                  <label className="form-label">Executable Binary File</label>
                  <input
                    type="file"
                    className="form-input"
                    required
                    onChange={(e) => {
                      if (e.target.files && e.target.files.length > 0) {
                        setSelectedFile(e.target.files[0])
                      }
                    }}
                  />
                  <span className="text-xs text-dim mt-1 block">
                    Server will calculate and verify SHA-256 checksum automatically.
                  </span>
                </div>
                <div className="form-group">
                  <label className="form-label">Changelog / Release Notes</label>
                  <textarea
                    className="form-textarea"
                    rows={3}
                    placeholder="Summary of bug fixes, enhancements, or security patches..."
                    value={relChangelog}
                    onChange={(e) => setRelChangelog(e.target.value)}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setIsReleaseModalOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" disabled={uploading}>
                  {uploading ? 'Uploading & Hashing...' : 'Publish Release'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Launch Campaign Modal */}
      {isCampaignModalOpen && (
        <div className="modal-overlay" onClick={() => setIsCampaignModalOpen(false)}>
          <div className="modal-dialog modal-md" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Launch Phased Rollout Campaign</h3>
              <button type="button" className="btn-close" onClick={() => setIsCampaignModalOpen(false)}>
                <X size={18} />
              </button>
            </div>
            <form onSubmit={handleCreateCampaign}>
              <div className="modal-body space-y-4">
                <div className="form-group">
                  <label className="form-label">Campaign Name</label>
                  <input
                    type="text"
                    className="form-input"
                    required
                    placeholder="e.g. Q4 2026 Fleet Upgrade to v1.2.0"
                    value={newCampaign.name}
                    onChange={(e) => setNewCampaign({ ...newCampaign, name: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Target Release Version</label>
                  <select
                    className="form-select"
                    required
                    value={newCampaign.target_version}
                    onChange={(e) => setNewCampaign({ ...newCampaign, target_version: e.target.value })}
                  >
                    <option value="">-- Select Release Version --</option>
                    {Array.from(new Set(releases.map((r) => r.version))).map((v) => (
                      <option key={v} value={v}>
                        v{v}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="form-group">
                  <label className="form-label">Rollout Scope</label>
                  <select
                    className="form-select"
                    value={newCampaign.target_type}
                    onChange={(e) => setNewCampaign({ ...newCampaign, target_type: e.target.value })}
                  >
                    <option value="all">Entire Fleet (All Connected Agents)</option>
                    <option value="group">Branch Site / Device Group</option>
                  </select>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="form-group">
                    <label className="form-label">Batch Size (Devices / Wave)</label>
                    <input
                      type="number"
                      className="form-input"
                      min={1}
                      value={newCampaign.batch_size}
                      onChange={(e) => setNewCampaign({ ...newCampaign, batch_size: Number(e.target.value) })}
                    />
                  </div>
                  <div className="form-group">
                    <label className="form-label">Stagger Interval (Seconds)</label>
                    <input
                      type="number"
                      className="form-input"
                      min={10}
                      value={newCampaign.stagger_interval_sec}
                      onChange={(e) =>
                        setNewCampaign({ ...newCampaign, stagger_interval_sec: Number(e.target.value) })
                      }
                    />
                  </div>
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setIsCampaignModalOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Start Campaign
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

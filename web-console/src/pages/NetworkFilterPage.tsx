import React, { useEffect, useState } from 'react'
import {
  CheckCircle2,
  Globe,
  Plus,
  RefreshCw,
  Send,
  Shield,
  Trash2,
  X,
} from 'lucide-react'
import { api } from '../services/api'
import type { DeviceFilterComplianceDTO, FilterRuleDTO } from '../types/api'

export const NetworkFilterPage: React.FC = () => {
  const [rules, setRules] = useState<FilterRuleDTO[]>([])
  const [compliance, setCompliance] = useState<DeviceFilterComplianceDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [applying, setApplying] = useState(false)
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [msg, setMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const [newRule, setNewRule] = useState({
    target_type: 'all',
    target_id: '',
    rule_type: 'block_domain',
    domain_pattern: '',
    category: 'Security & Phishing',
    action: 'block',
  })

  const loadData = async () => {
    setLoading(true)
    try {
      const [rList, compList] = await Promise.all([
        api.getFilterRules().catch(() => []),
        api.getFilterCompliance().catch(() => []),
      ])
      setRules(rList)
      setCompliance(compList)
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to load filter rules' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const handleApply = async () => {
    setApplying(true)
    try {
      const res = await api.applyFilterPolicies()
      setMsg({
        type: 'success',
        text: `Policy compiled successfully (Version: ${res.version.slice(0, 12)}..., ${res.rule_count} rules pushed live via WebSocket).`,
      })
      loadData()
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to apply filter policies' })
    } finally {
      setApplying(false)
    }
  }

  const handleCreateRule = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.createFilterRule(newRule)
      setMsg({ type: 'success', text: `Rule for '${newRule.domain_pattern}' added to policy draft.` })
      setIsModalOpen(false)
      setNewRule({
        target_type: 'all',
        target_id: '',
        rule_type: 'block_domain',
        domain_pattern: '',
        category: 'Security & Phishing',
        action: 'block',
      })
      loadData()
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to create rule' })
    }
  }

  const handleDeleteRule = async (id: string, domain: string) => {
    if (!confirm(`Delete rule '${domain}'?`)) return
    try {
      await api.deleteFilterRule(id)
      setMsg({ type: 'success', text: 'Rule deleted from draft. Remember to click "Deploy Policy" to push live.' })
      loadData()
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to delete rule' })
    }
  }

  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <h2 className="page-title">Network & Web Security Filter</h2>
          <p className="page-subtitle">
            Zero-CGO DNS sinkholing, corporate domain blocking, and real-time policy enforcement across branch endpoints
          </p>
        </div>
        <div className="header-controls">
          <button type="button" className="btn btn-secondary" onClick={loadData} disabled={loading}>
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            <span>Refresh</span>
          </button>
          <button type="button" className="btn btn-secondary" onClick={() => setIsModalOpen(true)}>
            <Plus size={16} />
            <span>Add Rule</span>
          </button>
          <button type="button" className="btn btn-primary" onClick={handleApply} disabled={applying}>
            <Send size={16} />
            <span>{applying ? 'Deploying...' : 'Deploy Policy to Fleet'}</span>
          </button>
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

      {/* Rules Table */}
      <div className="table-card">
        <div className="card-header-bar">
          <div className="flex items-center gap-2">
            <Shield className="text-primary" size={18} />
            <h3 className="card-title">Corporate Filter Rules ({rules.length})</h3>
          </div>
          <span className="text-sm text-dim">
            Rules apply atomically and trigger automatic local OS DNS cache flush
          </span>
        </div>

        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>Domain / Hostname Pattern</th>
                <th>Category</th>
                <th>Target Fleet</th>
                <th>Action</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {rules.length === 0 ? (
                <tr>
                  <td colSpan={6} className="text-center py-8 text-muted">
                    No filter rules configured. Click "Add Rule" to block or allow domains.
                  </td>
                </tr>
              ) : (
                rules.map((r) => (
                  <tr key={r.id}>
                    <td>
                      <div className="font-mono font-semibold text-main flex items-center gap-2">
                        <Globe size={15} className="text-dim" />
                        <span>{r.domain_pattern}</span>
                      </div>
                    </td>
                    <td>{r.category || 'General'}</td>
                    <td>
                      <span className="badge-target">{r.target_type}</span>
                    </td>
                    <td>
                      <span className={`badge-action ${r.action === 'block' ? 'block' : 'allow'}`}>
                        {r.action.toUpperCase()}
                      </span>
                    </td>
                    <td>
                      <span className="status-pill online">Active</span>
                    </td>
                    <td>
                      <button
                        type="button"
                        className="btn-action text-danger"
                        onClick={() => handleDeleteRule(r.id, r.domain_pattern)}
                      >
                        <Trash2 size={14} />
                        <span>Delete</span>
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Device Compliance Status */}
      <div className="table-card">
        <div className="card-header-bar">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="text-success" size={18} />
            <h3 className="card-title">Fleet Enforcement Compliance</h3>
          </div>
        </div>

        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>Device</th>
                <th>Branch Site</th>
                <th>Active Policy Version</th>
                <th>Sync Status</th>
                <th>Last Applied</th>
              </tr>
            </thead>
            <tbody>
              {compliance.length === 0 ? (
                <tr>
                  <td colSpan={5} className="text-center py-8 text-muted">
                    No compliance reports received yet.
                  </td>
                </tr>
              ) : (
                compliance.map((c) => (
                  <tr key={c.device_id}>
                    <td>
                      <span className="font-semibold text-main">{c.hostname}</span>
                    </td>
                    <td>{c.site || '—'}</td>
                    <td>
                      <span className="font-mono text-sm">{c.active_version ? c.active_version.slice(0, 14) + '...' : '—'}</span>
                    </td>
                    <td>
                      <span className={`status-pill ${c.status === 'applied' ? 'online' : 'warning'}`}>
                        {c.status}
                      </span>
                    </td>
                    <td className="text-sm text-muted">
                      {c.last_reported_at ? new Date(c.last_reported_at).toLocaleString() : '—'}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Add Rule Modal */}
      {isModalOpen && (
        <div className="modal-overlay" onClick={() => setIsModalOpen(false)}>
          <div className="modal-dialog modal-md" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Add Web Filter Rule</h3>
              <button type="button" className="btn-close" onClick={() => setIsModalOpen(false)}>
                <X size={18} />
              </button>
            </div>
            <form onSubmit={handleCreateRule}>
              <div className="modal-body space-y-4">
                <div className="form-group">
                  <label className="form-label">Domain or Hostname</label>
                  <input
                    type="text"
                    className="form-input"
                    required
                    placeholder="e.g. gambling-site.com or malware.tracker.net"
                    value={newRule.domain_pattern}
                    onChange={(e) => setNewRule({ ...newRule, domain_pattern: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Action</label>
                  <select
                    className="form-select"
                    value={newRule.action}
                    onChange={(e) => setNewRule({ ...newRule, action: e.target.value })}
                  >
                    <option value="block">BLOCK (Sinkhole to 0.0.0.0)</option>
                    <option value="allow">ALLOW (Exempt from blocking)</option>
                  </select>
                </div>
                <div className="form-group">
                  <label className="form-label">Category</label>
                  <select
                    className="form-select"
                    value={newRule.category}
                    onChange={(e) => setNewRule({ ...newRule, category: e.target.value })}
                  >
                    <option value="Security & Phishing">Security & Phishing</option>
                    <option value="Adult & Gambling">Adult & Gambling</option>
                    <option value="Bandwidth Heavy / Streaming">Bandwidth Heavy / Streaming</option>
                    <option value="Social Media">Social Media</option>
                    <option value="Custom Policy">Custom Corporate Policy</option>
                  </select>
                </div>
                <div className="form-group">
                  <label className="form-label">Scope</label>
                  <select
                    className="form-select"
                    value={newRule.target_type}
                    onChange={(e) => setNewRule({ ...newRule, target_type: e.target.value })}
                  >
                    <option value="all">Entire Fleet (All Devices)</option>
                    <option value="group">Branch Site Group</option>
                  </select>
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setIsModalOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Add Rule to Policy
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

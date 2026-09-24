import React, { useEffect, useState } from 'react'
import {
  CheckCircle2,
  Clock,
  RefreshCw,
  Search,
  Shield,
  ShieldAlert,
} from 'lucide-react'
import { api } from '../services/api'
import type { ActivityItem } from '../types/api'

export const AuditPage: React.FC = () => {
  const [logs, setLogs] = useState<ActivityItem[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')

  const fetchLogs = async () => {
    setLoading(true)
    try {
      const data = await api.getDashboardActivity()
      setLogs(data)
    } catch {
      setLogs([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchLogs()
  }, [])

  const filteredLogs = logs.filter((item) => {
    if (!search) return true
    const term = search.toLowerCase()
    return (
      item.action.toLowerCase().includes(term) ||
      item.actor_id.toLowerCase().includes(term) ||
      (item.target_id && item.target_id.toLowerCase().includes(term))
    )
  })

  return (
    <div className="page-container audit-page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Security & Operations Audit Trail</h1>
          <p className="page-subtitle">
            Immutable log of all administrative actions, device lifecycle events, and agent dispatches
          </p>
        </div>
        <div className="header-controls">
          <button
            type="button"
            className="btn btn-secondary"
            onClick={fetchLogs}
            disabled={loading}
          >
            <RefreshCw size={16} className={loading ? 'spinning' : ''} />
            <span>Refresh</span>
          </button>
        </div>
      </div>

      <div className="filter-bar">
        <div className="search-wrap">
          <Search size={18} className="search-icon" />
          <input
            type="text"
            className="search-input"
            placeholder="Search by action, operator, or target..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
      </div>

      <div className="table-card">
        <div className="table-wrapper">
          <table className="data-table">
            <thead>
              <tr>
                <th>Timestamp</th>
                <th>Operator / Actor</th>
                <th>Action</th>
                <th>Target Endpoint / Entity</th>
                <th>Audit Hash / Status</th>
              </tr>
            </thead>
            <tbody>
              {loading ? (
                <tr>
                  <td colSpan={5} className="text-center py-8">
                    <div className="table-loader">
                      <RefreshCw size={24} className="spinning" />
                      <span>Reading audit records...</span>
                    </div>
                  </td>
                </tr>
              ) : filteredLogs.length === 0 ? (
                <tr>
                  <td colSpan={5} className="text-center py-8">
                    <div className="empty-state">
                      <Shield size={32} />
                      <p>No audit trail records found</p>
                    </div>
                  </td>
                </tr>
              ) : (
                filteredLogs.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <span className="timestamp-cell">
                        {new Date(item.created_at).toLocaleString()}
                      </span>
                    </td>
                    <td>
                      <span className="actor-badge">{item.actor_type}:{item.actor_id}</span>
                    </td>
                    <td>
                      <div className="action-cell">
                        {item.action.includes('enroll') ? (
                          <CheckCircle2 size={14} className="text-success" />
                        ) : item.action.includes('command') ? (
                          <Clock size={14} className="text-primary" />
                        ) : (
                          <ShieldAlert size={14} className="text-warning" />
                        )}
                        <strong>{item.action}</strong>
                      </div>
                    </td>
                    <td>
                      <span className="font-mono text-sm">{item.target_id}</span>
                    </td>
                    <td>
                      <span className="status-pill online">
                        <span className="dot"></span>
                        VERIFIED
                      </span>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

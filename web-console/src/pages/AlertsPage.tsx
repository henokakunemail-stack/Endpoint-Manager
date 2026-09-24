import React, { useEffect, useState } from 'react'
import {
  AlertCircle,
  AlertTriangle,
  Bell,
  CheckCircle2,
  Filter,
  RefreshCw,
  X,
} from 'lucide-react'
import { api } from '../services/api'
import type { AlertIncidentDTO, AlertRuleDTO } from '../types/api'

export const AlertsPage: React.FC = () => {
  const [incidents, setIncidents] = useState<AlertIncidentDTO[]>([])
  const [rules, setRules] = useState<AlertRuleDTO[]>([])
  const [statusFilter, setStatusFilter] = useState<string>('open')
  const [loading, setLoading] = useState(true)
  const [msg, setMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const loadData = async () => {
    setLoading(true)
    try {
      const [incResp, rList] = await Promise.all([
        api.getAlertIncidents(statusFilter),
        api.getAlertRules().catch(() => []),
      ])
      setIncidents(incResp.incidents || [])
      setRules(rList)
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to load alerts' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [statusFilter])

  const handleAcknowledge = async (id: string) => {
    try {
      await api.acknowledgeIncident(id)
      setMsg({ type: 'success', text: 'Incident acknowledged' })
      loadData()
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to acknowledge incident' })
    }
  }

  const handleResolve = async (id: string) => {
    try {
      await api.resolveIncident(id)
      setMsg({ type: 'success', text: 'Incident resolved' })
      loadData()
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to resolve incident' })
    }
  }

  const openCount = incidents.filter((i) => i.status === 'open').length
  const ackCount = incidents.filter((i) => i.status === 'acknowledged').length

  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <h2 className="page-title">Alerting & Incident Management</h2>
          <p className="page-subtitle">
            Automated fleet anomaly detection, deduplicated incident tracking, and webhook notification triggers
          </p>
        </div>
        <div className="header-controls">
          <button type="button" className="btn btn-secondary" onClick={loadData} disabled={loading}>
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            <span>Refresh</span>
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

      {/* KPI Overview */}
      <div className="kpi-grid">
        <div className="kpi-card">
          <div className="kpi-header">
            <span className="kpi-label">Open Incidents</span>
            <AlertCircle className="kpi-icon text-danger" size={20} />
          </div>
          <div className="kpi-value text-danger">{openCount}</div>
          <span className="kpi-hint">Requiring operator attention</span>
        </div>

        <div className="kpi-card">
          <div className="kpi-header">
            <span className="kpi-label">Acknowledged</span>
            <AlertTriangle className="kpi-icon text-warning" size={20} />
          </div>
          <div className="kpi-value text-warning">{ackCount}</div>
          <span className="kpi-hint">Under investigation by technician</span>
        </div>

        <div className="kpi-card">
          <div className="kpi-header">
            <span className="kpi-label">Active Detection Rules</span>
            <Bell className="kpi-icon text-primary" size={20} />
          </div>
          <div className="kpi-value">{rules.length}</div>
          <span className="kpi-hint">Automated background evaluators</span>
        </div>
      </div>

      {/* Incidents Card */}
      <div className="table-card">
        <div className="table-toolbar">
          <div className="flex items-center gap-2">
            <Filter size={16} className="text-dim" />
            <span className="text-sm font-semibold text-muted">Filter by Status:</span>
            <div className="btn-group">
              <button
                type="button"
                className={`btn btn-sm ${statusFilter === 'open' ? 'btn-primary' : 'btn-secondary'}`}
                onClick={() => setStatusFilter('open')}
              >
                Open
              </button>
              <button
                type="button"
                className={`btn btn-sm ${statusFilter === 'acknowledged' ? 'btn-primary' : 'btn-secondary'}`}
                onClick={() => setStatusFilter('acknowledged')}
              >
                Acknowledged
              </button>
              <button
                type="button"
                className={`btn btn-sm ${statusFilter === 'resolved' ? 'btn-primary' : 'btn-secondary'}`}
                onClick={() => setStatusFilter('resolved')}
              >
                Resolved
              </button>
              <button
                type="button"
                className={`btn btn-sm ${statusFilter === '' ? 'btn-primary' : 'btn-secondary'}`}
                onClick={() => setStatusFilter('')}
              >
                All
              </button>
            </div>
          </div>
        </div>

        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>Severity</th>
                <th>Device</th>
                <th>Detection Rule</th>
                <th>Alert Message</th>
                <th>Status</th>
                <th>Triggered At</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {incidents.length === 0 ? (
                <tr>
                  <td colSpan={7} className="text-center py-8 text-muted">
                    No incidents found matching status filter '{statusFilter || 'all'}'.
                  </td>
                </tr>
              ) : (
                incidents.map((i) => (
                  <tr key={i.id}>
                    <td>
                      <span className={`badge-severity ${i.severity}`}>
                        {i.severity.toUpperCase()}
                      </span>
                    </td>
                    <td>
                      <span className="font-semibold text-main">{i.hostname || i.device_id.slice(0, 10)}</span>
                    </td>
                    <td>{i.rule_name || i.rule_id}</td>
                    <td>{i.message}</td>
                    <td>
                      <span className={`status-pill ${i.status === 'open' ? 'danger' : i.status === 'acknowledged' ? 'warning' : 'online'}`}>
                        {i.status}
                      </span>
                    </td>
                    <td className="text-sm text-muted">
                      {new Date(i.triggered_at).toLocaleString()}
                    </td>
                    <td>
                      <div className="action-buttons">
                        {i.status === 'open' && (
                          <button
                            type="button"
                            className="btn-action text-warning"
                            onClick={() => handleAcknowledge(i.id)}
                            title="Acknowledge incident"
                          >
                            <AlertTriangle size={14} />
                            <span>Ack</span>
                          </button>
                        )}
                        {i.status !== 'resolved' && (
                          <button
                            type="button"
                            className="btn-action text-success"
                            onClick={() => handleResolve(i.id)}
                            title="Resolve incident"
                          >
                            <CheckCircle2 size={14} />
                            <span>Resolve</span>
                          </button>
                        )}
                      </div>
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

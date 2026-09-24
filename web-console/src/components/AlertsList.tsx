import React from 'react'
import { AlertCircle, AlertTriangle, CheckCircle2, HardDrive, WifiOff } from 'lucide-react'
import type { AlertItem } from '../types/api'

interface AlertsListProps {
  alerts: AlertItem[]
}

export const AlertsList: React.FC<AlertsListProps> = ({ alerts }) => {
  const getIcon = (type: string, severity: string) => {
    if (type === 'low_disk') {
      return <HardDrive size={16} className={`alert-icon ${severity}`} />
    }
    if (type === 'offline_long') {
      return <WifiOff size={16} className={`alert-icon ${severity}`} />
    }
    return severity === 'critical' ? (
      <AlertCircle size={16} className="alert-icon critical" />
    ) : (
      <AlertTriangle size={16} className="alert-icon warning" />
    )
  }

  return (
    <div className="dash-card">
      <div className="card-header">
        <div className="card-title-group">
          <AlertTriangle size={18} className="card-icon warning" />
          <h3 className="card-title">Operational Alerts & Health Warnings</h3>
        </div>
        <span className={`card-badge ${alerts.length > 0 ? 'warning' : 'success'}`}>
          {alerts.length} Issues Detected
        </span>
      </div>

      <div className="alerts-body">
        {alerts.length === 0 ? (
          <div className="empty-state success">
            <CheckCircle2 size={24} className="icon-success" />
            <span>All monitored endpoints are operating within normal thresholds.</span>
          </div>
        ) : (
          <div className="alerts-stream">
            {alerts.map((a) => (
              <div key={a.id} className={`alert-item ${a.severity}`}>
                <div className="alert-item-header">
                  <div className="alert-item-title">
                    {getIcon(a.type, a.severity)}
                    <strong>{a.hostname || a.device_id}</strong>
                    {a.site && <span className="site-tag">{a.site}</span>}
                  </div>
                  <span className={`severity-badge ${a.severity}`}>
                    {a.severity.toUpperCase()}
                  </span>
                </div>
                <p className="alert-message">{a.message}</p>
                <span className="alert-time">
                  {new Date(a.timestamp).toLocaleString()}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

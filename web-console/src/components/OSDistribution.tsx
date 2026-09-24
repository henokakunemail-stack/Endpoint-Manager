import React from 'react'
import { Cpu } from 'lucide-react'
import type { OSMetric } from '../types/api'

interface OSDistributionProps {
  metrics: OSMetric[]
}

export const OSDistribution: React.FC<OSDistributionProps> = ({ metrics }) => {
  const getOSColor = (name: string) => {
    switch (name.toLowerCase()) {
      case 'windows':
        return '#0078D4'
      case 'linux':
        return '#E95420'
      case 'macos':
        return '#999999'
      default:
        return '#64748b'
    }
  }

  return (
    <div className="dash-card">
      <div className="card-header">
        <div className="card-title-group">
          <Cpu size={18} className="card-icon" />
          <h3 className="card-title">Operating System Distribution</h3>
        </div>
      </div>

      <div className="os-list">
        {metrics.length === 0 ? (
          <div className="empty-state">No OS telemetry reported</div>
        ) : (
          metrics.map((m) => (
            <div key={m.os_name} className="os-row">
              <div className="os-info">
                <span
                  className="os-badge-dot"
                  style={{ backgroundColor: getOSColor(m.os_name) }}
                ></span>
                <span className="os-label">{m.os_name}</span>
                <span className="os-count">{m.count} devices</span>
              </div>
              <span className="os-pct">{m.pct}%</span>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

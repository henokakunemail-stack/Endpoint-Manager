import React from 'react'
import { Building2 } from 'lucide-react'
import type { SiteMetric } from '../types/api'

interface SiteDistributionProps {
  sites: SiteMetric[]
}

export const SiteDistribution: React.FC<SiteDistributionProps> = ({ sites }) => {
  return (
    <div className="dash-card">
      <div className="card-header">
        <div className="card-title-group">
          <Building2 size={18} className="card-icon" />
          <h3 className="card-title">Multi-Branch Health</h3>
        </div>
        <span className="card-badge">{sites.length} Active Sites</span>
      </div>

      <div className="site-list">
        {sites.length === 0 ? (
          <div className="empty-state">No site telemetry available</div>
        ) : (
          sites.map((s) => (
            <div key={s.site} className="site-row">
              <div className="site-meta">
                <span className="site-name">{s.site.toUpperCase()}</span>
                <span className="site-stats">
                  <strong>{s.online}</strong> online / {s.total} total ({s.online_pct}%)
                </span>
              </div>
              <div className="progress-bar-bg">
                <div
                  className="progress-bar-fill"
                  style={{ width: `${Math.min(100, Math.max(0, s.online_pct))}%` }}
                ></div>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

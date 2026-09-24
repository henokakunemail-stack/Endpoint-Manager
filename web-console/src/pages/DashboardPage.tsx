import React, { useCallback, useEffect, useState } from 'react'
import {
  Activity,
  ArrowRight,
  CheckCircle2,
  Clock,
  Laptop,
  Radio,
  RefreshCw,
  Server,
  ShieldAlert,
  WifiOff,
} from 'lucide-react'
import { AlertsList } from '../components/AlertsList'
import { KPICard } from '../components/KPICard'
import { OSDistribution } from '../components/OSDistribution'
import { SiteDistribution } from '../components/SiteDistribution'
import { api } from '../services/api'
import type {
  ActivityItem,
  AlertItem,
  DashboardSummary,
  OSMetric,
  SiteMetric,
} from '../types/api'

interface DashboardPageProps {
  onNavigateToDevices: () => void
}

export const DashboardPage: React.FC<DashboardPageProps> = ({ onNavigateToDevices }) => {
  const [summary, setSummary] = useState<DashboardSummary | null>(null)
  const [sites, setSites] = useState<SiteMetric[]>([])
  const [osMetrics, setOsMetrics] = useState<OSMetric[]>([])
  const [alerts, setAlerts] = useState<AlertItem[]>([])
  const [activity, setActivity] = useState<ActivityItem[]>([])
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date())

  const loadData = useCallback(async (isSilent = false) => {
    if (!isSilent) setRefreshing(true)
    try {
      const [sum, siteList, osList, alertList, actList] = await Promise.all([
        api.getDashboardSummary().catch(() => null),
        api.getDashboardSites().catch(() => []),
        api.getDashboardOS().catch(() => []),
        api.getDashboardAlerts().catch(() => []),
        api.getDashboardActivity().catch(() => []),
      ])

      if (sum) setSummary(sum)
      setSites(siteList)
      setOsMetrics(osList)
      setAlerts(alertList)
      setActivity(actList)
      setLastUpdated(new Date())
    } catch {
      // Handled per-call
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    loadData(false)
  }, [loadData])

  useEffect(() => {
    if (!autoRefresh) return
    const interval = setInterval(() => {
      loadData(true)
    }, 15000)
    return () => clearInterval(interval)
  }, [autoRefresh, loadData])

  const total = summary?.total_devices ?? 0
  const online = summary?.online_devices ?? 0
  const offline = summary?.offline_devices ?? 0
  const retired = summary?.retired_devices ?? 0
  const onlinePct = summary?.online_pct ?? (total > 0 ? Math.round((online / total) * 100) : 0)

  return (
    <div className="page-container dashboard-page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Fleet Telemetry Dashboard</h1>
          <p className="page-subtitle">
            Real-time health, compliance, and distribution across all corporate sites
          </p>
        </div>
        <div className="header-controls">
          <label className="toggle-label">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
            />
            <span>Auto-refresh (15s)</span>
          </label>
          <span className="last-sync">Updated {lastUpdated.toLocaleTimeString()}</span>
          <button
            type="button"
            className="btn btn-secondary btn-icon"
            onClick={() => loadData(false)}
            disabled={refreshing}
            title="Refresh telemetry"
          >
            <RefreshCw size={16} className={refreshing ? 'spinning' : ''} />
            <span>{refreshing ? 'Refreshing...' : 'Refresh'}</span>
          </button>
        </div>
      </div>

      {loading ? (
        <div className="page-loader">
          <RefreshCw size={32} className="spinning" />
          <span>Aggregating fleet telemetry...</span>
        </div>
      ) : (
        <>
          {/* KPI Summary Cards */}
          <div className="kpi-grid">
            <KPICard
              title="TOTAL MANAGED ENDPOINTS"
              value={total}
              subtitle={`${online} currently connected`}
              icon={<Laptop size={22} />}
              variant="primary"
            />
            <KPICard
              title="FLEET ONLINE RATIO"
              value={`${onlinePct}%`}
              subtitle={`${offline} endpoints offline`}
              icon={<Radio size={22} />}
              variant={onlinePct >= 80 ? 'success' : onlinePct >= 50 ? 'warning' : 'danger'}
            />
            <KPICard
              title="OFFLINE DEVICES"
              value={offline}
              subtitle="Pending reconnection"
              icon={<WifiOff size={22} />}
              variant={offline > 0 ? 'warning' : 'neutral'}
            />
            <KPICard
              title="RETIRED / DECOMMISSIONED"
              value={retired}
              subtitle="Excluded from telemetry"
              icon={<Server size={22} />}
              variant="neutral"
            />
          </div>

          {/* Quick link banner to Device Management */}
          <div className="quick-action-banner" onClick={onNavigateToDevices}>
            <div className="quick-banner-content">
              <Laptop size={20} />
              <div>
                <strong>Explore Detailed Endpoint Inventory</strong>
                <p>
                  Inspect hardware configurations, storage volumes, installed software, and
                  dispatch diagnostics.
                </p>
              </div>
            </div>
            <button type="button" className="btn btn-link">
              <span>View Fleet Inventory</span>
              <ArrowRight size={16} />
            </button>
          </div>

          {/* Mid Section: Alerts & Site Distribution */}
          <div className="dash-row two-col">
            <AlertsList alerts={alerts} />
            <SiteDistribution sites={sites} />
          </div>

          {/* Lower Section: OS Distribution & Recent Activity Feed */}
          <div className="dash-row two-col">
            <OSDistribution metrics={osMetrics} />

            <div className="dash-card">
              <div className="card-header">
                <div className="card-title-group">
                  <Activity size={18} className="card-icon" />
                  <h3 className="card-title">Recent Fleet Activity & Audit</h3>
                </div>
                <span className="card-badge">{activity.length} Events</span>
              </div>
              <div className="activity-body">
                {activity.length === 0 ? (
                  <div className="empty-state">No recent activity recorded</div>
                ) : (
                  <div className="activity-stream">
                    {activity.map((item) => (
                      <div key={item.id} className="activity-item">
                        <div className="activity-icon-wrap">
                          {item.action.includes('enroll') ? (
                            <CheckCircle2 size={16} className="text-success" />
                          ) : item.action.includes('command') ? (
                            <Clock size={16} className="text-primary" />
                          ) : (
                            <ShieldAlert size={16} className="text-warning" />
                          )}
                        </div>
                        <div className="activity-details">
                          <div className="activity-line">
                            <span className="activity-actor">{item.actor_type}:{item.actor_id}</span>
                            <span className="activity-action">{item.action}</span>
                            <span className="activity-target">{item.target_id}</span>
                          </div>
                          <span className="activity-time">
                            {new Date(item.created_at).toLocaleString()}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  )
}

import React, { useEffect, useState } from 'react'
import {
  Activity,
  ArrowUpCircle,
  Bell,
  Calendar,
  FileText,
  Globe,
  HardDrive,
  Laptop,
  LogOut,
  Package,
  Shield,
  ShieldCheck,
  Terminal,
  Users,
} from 'lucide-react'
import { useAuth } from '../context/AuthContext'

interface NavbarProps {
  currentTab: string
  onTabChange: (tab: string) => void
}

export const Navbar: React.FC<NavbarProps> = ({ currentTab, onTabChange }) => {
  const { user, logout } = useAuth()
  const [agentsOnline, setAgentsOnline] = useState<number | null>(null)

  useEffect(() => {
    const checkHealth = async () => {
      try {
        const res = await fetch('/healthz')
        if (res.ok) {
          const data = await res.json()
          setAgentsOnline(data.agents_online ?? 0)
        }
      } catch {
        setAgentsOnline(null)
      }
    }
    checkHealth()
    const interval = setInterval(checkHealth, 10000)
    return () => clearInterval(interval)
  }, [])

  return (
    <header className="app-navbar">
      <div className="nav-container">
        <div className="nav-left">
          <div className="brand-badge">
            <div className="brand-icon-wrap">
              <Terminal size={20} />
            </div>
            <div>
              <div className="brand-title">Endpoint Manager</div>
              <span className="brand-tag">ENTERPRISE</span>
            </div>
          </div>

          <nav className="nav-scroll-wrap">
            <button
              type="button"
              className={`nav-item ${currentTab === 'dashboard' ? 'active' : ''}`}
              onClick={() => onTabChange('dashboard')}
              title="Fleet Overview & Live Metrics"
            >
              <Activity size={16} />
              <span>Dashboard</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'devices' ? 'active' : ''}`}
              onClick={() => onTabChange('devices')}
              title="Enrolled Endpoints, Hardware Specs, Remote Shell & Desktop"
            >
              <Laptop size={16} />
              <span>Devices</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'software' ? 'active' : ''}`}
              onClick={() => onTabChange('software')}
              title="Software Deployment & Silent Package Rollout"
            >
              <Package size={16} />
              <span>Software</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'patches' ? 'active' : ''}`}
              onClick={() => onTabChange('patches')}
              title="OS Patch Management & Security Compliance"
            >
              <ShieldCheck size={16} />
              <span>Patches</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'scheduler' ? 'active' : ''}`}
              onClick={() => onTabChange('scheduler')}
              title="Script Repository & Recurring Maintenance Jobs"
            >
              <Calendar size={16} />
              <span>Scheduler</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'filter' ? 'active' : ''}`}
              onClick={() => onTabChange('filter')}
              title="DNS Sinkhole & Corporate Domain Filtering"
            >
              <Globe size={16} />
              <span>Web Filter</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'alerts' ? 'active' : ''}`}
              onClick={() => onTabChange('alerts')}
              title="Fleet Anomaly Alerts & Deduplicated Incidents"
            >
              <Bell size={16} />
              <span>Alerts</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'assets' ? 'active' : ''}`}
              onClick={() => onTabChange('assets')}
              title="Hardware Valuation (HAM) & Software License Compliance (SAM)"
            >
              <HardDrive size={16} />
              <span>Assets & SAM</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'updates' ? 'active' : ''}`}
              onClick={() => onTabChange('updates')}
              title="Agent Binary Releases & Phased Canary Upgrades"
            >
              <ArrowUpCircle size={16} />
              <span>Updates</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'reports' ? 'active' : ''}`}
              onClick={() => onTabChange('reports')}
              title="On-Demand Streaming CSV & JSON Audit Exports"
            >
              <FileText size={16} />
              <span>Reports</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'users' ? 'active' : ''}`}
              onClick={() => onTabChange('users')}
              title="User Accounts & Role-Based Access Control"
            >
              <Users size={16} />
              <span>Users</span>
            </button>

            <button
              type="button"
              className={`nav-item ${currentTab === 'audit' ? 'active' : ''}`}
              onClick={() => onTabChange('audit')}
              title="Immutable Security Audit Trail"
            >
              <Shield size={16} />
              <span>Audit</span>
            </button>
          </nav>
        </div>

        <div className="nav-right">
          {agentsOnline !== null ? (
            <div className="live-agent-badge" title="Active fleet agent WebSocket connections">
              <span className="status-dot"></span>
              <span>{agentsOnline} live {agentsOnline === 1 ? 'agent' : 'agents'}</span>
            </div>
          ) : (
            <div className="server-status offline" title="Server connection unreachable">
              <span className="status-dot"></span>
              <span>Connecting...</span>
            </div>
          )}

          {user && (
            <div className="user-profile-badge">
              <div className="user-info">
                <span className="user-name">{user.usr}</span>
                <span className={`role-pill ${user.rol}`}>{user.rol}</span>
              </div>
              <button
                type="button"
                className="btn-signout"
                onClick={logout}
                title="Sign out of console"
              >
                <LogOut size={15} />
                <span>Logout</span>
              </button>
            </div>
          )}
        </div>
      </div>
    </header>
  )
}

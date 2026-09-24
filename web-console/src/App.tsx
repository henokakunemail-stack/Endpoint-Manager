import React, { useState } from 'react'
import { InteractiveTerminalModal } from './components/InteractiveTerminalModal'
import { Navbar } from './components/Navbar'
import { RemoteControlModal } from './components/RemoteControlModal'
import { RemoteExecModal } from './components/RemoteExecModal'
import { AuthProvider, useAuth } from './context/AuthContext'
import { ToastProvider } from './context/ToastContext'
import { AgentUpdatesPage } from './pages/AgentUpdatesPage'
import { AlertsPage } from './pages/AlertsPage'
import { AssetLicensePage } from './pages/AssetLicensePage'
import { AuditPage } from './pages/AuditPage'
import { DashboardPage } from './pages/DashboardPage'
import { DevicesPage } from './pages/DevicesPage'
import { LoginPage } from './pages/LoginPage'
import { NetworkFilterPage } from './pages/NetworkFilterPage'
import { PatchesPage } from './pages/PatchesPage'
import { ReportsPage } from './pages/ReportsPage'
import { SoftwarePage } from './pages/SoftwarePage'
import { TasksSchedulerPage } from './pages/TasksSchedulerPage'
import { UsersPage } from './pages/UsersPage'
import type { DeviceDTO } from './types/api'

type ConsoleTab =
  | 'dashboard'
  | 'devices'
  | 'software'
  | 'patches'
  | 'scheduler'
  | 'filter'
  | 'alerts'
  | 'assets'
  | 'updates'
  | 'reports'
  | 'users'
  | 'audit'

const ConsoleRoot: React.FC = () => {
  const { isAuthenticated, loading } = useAuth()
  const [activeTab, setActiveTab] = useState<ConsoleTab>('dashboard')
  const [execDevice, setExecDevice] = useState<DeviceDTO | null>(null)
  const [termDevice, setTermDevice] = useState<DeviceDTO | null>(null)
  const [termShell, setTermShell] = useState<string>('powershell')
  const [rcDevice, setRcDevice] = useState<DeviceDTO | null>(null)

  if (loading) {
    return (
      <div className="app-loading-screen">
        <div className="app-loading-spinner"></div>
        <p className="app-loading-text">Initializing Enterprise Console...</p>
      </div>
    )
  }

  if (!isAuthenticated) {
    return <LoginPage />
  }

  return (
    <div className="app-layout">
      <Navbar currentTab={activeTab} onTabChange={(t) => setActiveTab(t as ConsoleTab)} />
      <main className="main-viewport">
        {activeTab === 'dashboard' && (
          <DashboardPage onNavigateToDevices={() => setActiveTab('devices')} />
        )}
        {activeTab === 'devices' && (
          <DevicesPage
            onOpenExec={(d) => setExecDevice(d)}
            onOpenTerminal={(d, shell) => {
              setTermShell(shell)
              setTermDevice(d)
            }}
            onOpenRemoteControl={(d) => setRcDevice(d)}
          />
        )}
        {activeTab === 'software' && <SoftwarePage />}
        {activeTab === 'patches' && <PatchesPage />}
        {activeTab === 'scheduler' && <TasksSchedulerPage />}
        {activeTab === 'filter' && <NetworkFilterPage />}
        {activeTab === 'alerts' && <AlertsPage />}
        {activeTab === 'assets' && <AssetLicensePage />}
        {activeTab === 'updates' && <AgentUpdatesPage />}
        {activeTab === 'reports' && <ReportsPage />}
        {activeTab === 'users' && <UsersPage />}
        {activeTab === 'audit' && <AuditPage />}
      </main>

      <RemoteExecModal device={execDevice} onClose={() => setExecDevice(null)} />
      <InteractiveTerminalModal
        device={termDevice}
        shell={termShell}
        onClose={() => setTermDevice(null)}
      />
      <RemoteControlModal
        device={rcDevice}
        onClose={() => setRcDevice(null)}
      />
    </div>
  )
}

export const App: React.FC = () => {
  return (
    <AuthProvider>
      <ToastProvider>
        <ConsoleRoot />
      </ToastProvider>
    </AuthProvider>
  )
}

export default App

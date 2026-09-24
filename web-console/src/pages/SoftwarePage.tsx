import React, { useCallback, useEffect, useState } from 'react'
import {
  Package,
  UploadCloud,
  Play,
  CheckCircle2,
  XCircle,
  Clock,
  Trash2,
  Terminal,
  Layers,
  Search,
  RefreshCw,
  AlertCircle,
  X,
  Server,
} from 'lucide-react'
import { useAuth } from '../context/AuthContext'
import { api } from '../services/api'
import type {
  SoftwarePackageDTO,
  SoftwareDeploymentDTO,
  DeploymentTaskDTO,
} from '../types/api'

export const SoftwarePage: React.FC = () => {
  const { user } = useAuth()
  const [activeTab, setActiveTab] = useState<'packages' | 'deployments'>('packages')
  const [packages, setPackages] = useState<SoftwarePackageDTO[]>([])
  const [deployments, setDeployments] = useState<SoftwareDeploymentDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [statusMsg, setStatusMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  // Upload Modal State
  const [showUploadModal, setShowUploadModal] = useState(false)
  const [uploadName, setUploadName] = useState('')
  const [uploadVersion, setUploadVersion] = useState('')
  const [uploadOS, setUploadOS] = useState<'windows' | 'linux' | 'macos'>('windows')
  const [uploadType, setUploadType] = useState<'msi' | 'exe' | 'deb' | 'rpm' | 'pkg' | 'script'>('msi')
  const [uploadInstallArgs, setUploadInstallArgs] = useState('')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [uploading, setUploading] = useState(false)

  // Deployment Wizard Modal State
  const [showDeployModal, setShowDeployModal] = useState(false)
  const [deployPackageId, setDeployPackageId] = useState('')
  const [deployName, setDeployName] = useState('')
  const [deployTargetType, setDeployTargetType] = useState<'all' | 'group' | 'device'>('all')
  const [deployTargetId, setDeployTargetId] = useState('')
  const [deploying, setDeploying] = useState(false)

  // Task Details Modal State
  const [viewingDeployment, setViewingDeployment] = useState<SoftwareDeploymentDTO | null>(null)
  const [deploymentTasks, setDeploymentTasks] = useState<DeploymentTaskDTO[]>([])
  const [loadingTasks, setLoadingTasks] = useState(false)
  const [expandedTaskId, setExpandedTaskId] = useState<string | null>(null)

  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const [pkgs, deps] = await Promise.all([
        api.getPackages(),
        api.getDeployments(),
      ])
      setPackages(pkgs)
      setDeployments(deps)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to fetch data'
      setStatusMsg({ type: 'error', text: msg })
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleUpload = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedFile) {
      setStatusMsg({ type: 'error', text: 'Please select an installer file' })
      return
    }

    setUploading(true)
    setStatusMsg(null)
    try {
      const formData = new FormData()
      formData.append('name', uploadName)
      formData.append('version', uploadVersion)
      formData.append('os_target', uploadOS)
      formData.append('package_type', uploadType)
      formData.append('install_args', uploadInstallArgs)
      formData.append('file', selectedFile)

      await api.uploadPackage(formData)
      setStatusMsg({ type: 'success', text: `Package ${uploadName} uploaded successfully` })
      setShowUploadModal(false)
      setUploadName('')
      setUploadVersion('')
      setUploadInstallArgs('')
      setSelectedFile(null)
      fetchData()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Upload failed'
      setStatusMsg({ type: 'error', text: msg })
    } finally {
      setUploading(false)
    }
  }

  const handleDeletePackage = async (id: string, name: string) => {
    if (!window.confirm(`Are you sure you want to delete package "${name}"? This action cannot be undone.`)) {
      return
    }
    try {
      await api.deletePackage(id)
      setStatusMsg({ type: 'success', text: `Package ${name} deleted successfully` })
      fetchData()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete package'
      setStatusMsg({ type: 'error', text: msg })
    }
  }

  const handleCreateDeployment = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!deployPackageId || !deployName) {
      setStatusMsg({ type: 'error', text: 'Please complete all required fields' })
      return
    }

    setDeploying(true)
    setStatusMsg(null)
    try {
      const res = await api.createDeployment({
        name: deployName,
        package_id: deployPackageId,
        target_type: deployTargetType,
        target_id: deployTargetId,
      })
      setStatusMsg({
        type: 'success',
        text: `Deployment initiated! ${res.tasks_total} target(s) queued, ${res.dispatched_live} dispatched live.`,
      })
      setShowDeployModal(false)
      setDeployName('')
      setDeployPackageId('')
      setDeployTargetId('')
      setActiveTab('deployments')
      fetchData()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to trigger deployment'
      setStatusMsg({ type: 'error', text: msg })
    } finally {
      setDeploying(false)
    }
  }

  const handleOpenTasks = async (dep: SoftwareDeploymentDTO) => {
    setViewingDeployment(dep)
    setLoadingTasks(true)
    try {
      const tasks = await api.getDeploymentTasks(dep.id)
      setDeploymentTasks(tasks)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to load tasks'
      setStatusMsg({ type: 'error', text: msg })
    } finally {
      setLoadingTasks(false)
    }
  }

  const filteredPackages = packages.filter((p) =>
    p.name.toLowerCase().includes(search.toLowerCase()) ||
    p.file_name.toLowerCase().includes(search.toLowerCase()) ||
    p.version.toLowerCase().includes(search.toLowerCase())
  )

  const filteredDeployments = deployments.filter((d) =>
    d.name.toLowerCase().includes(search.toLowerCase()) ||
    (d.package_name && d.package_name.toLowerCase().includes(search.toLowerCase()))
  )

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  }

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
            <Package className="h-6 w-6 text-cyan-400" />
            Software Deployment
          </h1>
          <p className="text-sm text-slate-400 mt-1">
            Silent multi-platform distribution with integrity verification and progress tracking
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => fetchData()}
            className="p-2 text-slate-400 hover:text-white bg-slate-800 hover:bg-slate-750 border border-slate-700 rounded-lg transition-colors"
            title="Refresh"
          >
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          {user?.rol !== 'viewer' && (
            <>
              <button
                onClick={() => setShowUploadModal(true)}
                className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-white bg-slate-800 hover:bg-slate-700 border border-slate-600 rounded-lg transition-colors"
              >
                <UploadCloud className="h-4 w-4 text-cyan-400" />
                Upload Package
              </button>
              <button
                onClick={() => {
                  setDeployPackageId(packages[0]?.id || '')
                  setDeployName(`Deploy ${packages[0]?.name || 'Package'} - ${new Date().toLocaleDateString()}`)
                  setShowDeployModal(true)
                }}
                disabled={packages.length === 0}
                className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-white bg-cyan-600 hover:bg-cyan-500 rounded-lg transition-colors disabled:opacity-50"
              >
                <Play className="h-4 w-4" />
                New Deployment
              </button>
            </>
          )}
        </div>
      </div>

      {/* Notifications */}
      {statusMsg && (
        <div
          className={`p-4 rounded-lg flex items-center justify-between border ${
            statusMsg.type === 'success'
              ? 'bg-emerald-950/40 text-emerald-300 border-emerald-800/60'
              : 'bg-rose-950/40 text-rose-300 border-rose-800/60'
          }`}
        >
          <div className="flex items-center gap-2 text-sm">
            {statusMsg.type === 'success' ? <CheckCircle2 className="h-4 w-4" /> : <AlertCircle className="h-4 w-4" />}
            {statusMsg.text}
          </div>
          <button onClick={() => setStatusMsg(null)} className="text-slate-400 hover:text-white">
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Stats Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-slate-800/80 border border-slate-700 rounded-xl p-4">
          <div className="flex items-center justify-between text-slate-400 text-xs font-semibold uppercase tracking-wider">
            Repository Packages
            <Package className="h-4 w-4 text-cyan-400" />
          </div>
          <div className="mt-2 text-2xl font-bold text-white">{packages.length}</div>
          <div className="text-xs text-slate-400 mt-1">Cross-platform installers ready</div>
        </div>

        <div className="bg-slate-800/80 border border-slate-700 rounded-xl p-4">
          <div className="flex items-center justify-between text-slate-400 text-xs font-semibold uppercase tracking-wider">
            Active Deployments
            <Play className="h-4 w-4 text-amber-400" />
          </div>
          <div className="mt-2 text-2xl font-bold text-white">
            {deployments.filter((d) => d.status === 'running').length}
          </div>
          <div className="text-xs text-slate-400 mt-1">In-progress rollout jobs</div>
        </div>

        <div className="bg-slate-800/80 border border-slate-700 rounded-xl p-4">
          <div className="flex items-center justify-between text-slate-400 text-xs font-semibold uppercase tracking-wider">
            Total Endpoints Targeted
            <Server className="h-4 w-4 text-indigo-400" />
          </div>
          <div className="mt-2 text-2xl font-bold text-white">
            {deployments.reduce((acc, d) => acc + d.total_tasks, 0)}
          </div>
          <div className="text-xs text-slate-400 mt-1">Across all deployment tasks</div>
        </div>

        <div className="bg-slate-800/80 border border-slate-700 rounded-xl p-4">
          <div className="flex items-center justify-between text-slate-400 text-xs font-semibold uppercase tracking-wider">
            Execution Success Rate
            <CheckCircle2 className="h-4 w-4 text-emerald-400" />
          </div>
          <div className="mt-2 text-2xl font-bold text-white">
            {(() => {
              const total = deployments.reduce((acc, d) => acc + d.total_tasks, 0)
              const success = deployments.reduce((acc, d) => acc + d.success_tasks, 0)
              return total > 0 ? `${((success / total) * 100).toFixed(1)}%` : '100%'
            })()}
          </div>
          <div className="text-xs text-emerald-400/80 mt-1">Cryptographic verified runs</div>
        </div>
      </div>

      {/* Tabs & Search */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-slate-800 pb-4">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setActiveTab('packages')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              activeTab === 'packages'
                ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/30'
                : 'text-slate-400 hover:text-white hover:bg-slate-800/60'
            }`}
          >
            Package Repository ({packages.length})
          </button>
          <button
            onClick={() => setActiveTab('deployments')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              activeTab === 'deployments'
                ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/30'
                : 'text-slate-400 hover:text-white hover:bg-slate-800/60'
            }`}
          >
            Deployments & Rollouts ({deployments.length})
          </button>
        </div>

        <div className="relative">
          <Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
          <input
            type="text"
            placeholder={activeTab === 'packages' ? 'Search packages...' : 'Search deployments...'}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full sm:w-64 pl-9 pr-4 py-1.5 text-sm bg-slate-900 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-cyan-500"
          />
        </div>
      </div>

      {/* TAB 1: Package Repository */}
      {activeTab === 'packages' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm text-slate-300">
              <thead className="bg-slate-800/80 text-xs font-semibold uppercase tracking-wider text-slate-400 border-b border-slate-700">
                <tr>
                  <th className="px-6 py-3">Package Name</th>
                  <th className="px-6 py-3">Target OS</th>
                  <th className="px-6 py-3">Type</th>
                  <th className="px-6 py-3">File & Size</th>
                  <th className="px-6 py-3">SHA-256 Hash</th>
                  <th className="px-6 py-3">Install Flags</th>
                  <th className="px-6 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {filteredPackages.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="px-6 py-8 text-center text-slate-500">
                      No software packages in repository. Click "Upload Package" to add your first installer.
                    </td>
                  </tr>
                ) : (
                  filteredPackages.map((pkg) => (
                    <tr key={pkg.id} className="hover:bg-slate-800/30 transition-colors">
                      <td className="px-6 py-4 font-medium text-white">
                        <div className="flex items-center gap-2">
                          <Package className="h-4 w-4 text-cyan-400 shrink-0" />
                          <div>
                            <div>{pkg.name}</div>
                            <div className="text-xs text-slate-400">v{pkg.version}</div>
                          </div>
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium uppercase bg-slate-800 text-slate-300 border border-slate-700">
                          {pkg.os_target}
                        </span>
                      </td>
                      <td className="px-6 py-4">
                        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-mono font-medium uppercase bg-indigo-950/60 text-indigo-300 border border-indigo-800/50">
                          {pkg.package_type}
                        </span>
                      </td>
                      <td className="px-6 py-4 text-slate-300">
                        <div className="truncate max-w-[160px]" title={pkg.file_name}>
                          {pkg.file_name}
                        </div>
                        <div className="text-xs text-slate-500">{formatBytes(pkg.file_size)}</div>
                      </td>
                      <td className="px-6 py-4">
                        <span
                          className="font-mono text-xs text-slate-400 bg-slate-950 px-2 py-1 rounded border border-slate-800 select-all"
                          title={pkg.sha256}
                        >
                          {pkg.sha256.substring(0, 10)}...{pkg.sha256.substring(pkg.sha256.length - 8)}
                        </span>
                      </td>
                      <td className="px-6 py-4">
                        <code className="text-xs bg-slate-950 px-2 py-1 rounded text-cyan-300 border border-slate-800">
                          {pkg.install_args || '(default silent)'}
                        </code>
                      </td>
                      <td className="px-6 py-4 text-right space-x-2">
                        {user?.rol !== 'viewer' && (
                          <button
                            onClick={() => {
                              setDeployPackageId(pkg.id)
                              setDeployName(`Deploy ${pkg.name} v${pkg.version}`)
                              setShowDeployModal(true)
                            }}
                            className="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-medium text-cyan-400 bg-cyan-950/40 hover:bg-cyan-900/50 border border-cyan-800/60 rounded-md transition-colors"
                          >
                            <Play className="h-3 w-3" />
                            Deploy
                          </button>
                        )}
                        {user?.rol === 'admin' && (
                          <button
                            onClick={() => handleDeletePackage(pkg.id, pkg.name)}
                            className="inline-flex items-center p-1 text-slate-400 hover:text-rose-400 hover:bg-rose-950/30 rounded-md transition-colors"
                            title="Delete Package"
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        )}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* TAB 2: Deployments & Rollouts */}
      {activeTab === 'deployments' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm text-slate-300">
              <thead className="bg-slate-800/80 text-xs font-semibold uppercase tracking-wider text-slate-400 border-b border-slate-700">
                <tr>
                  <th className="px-6 py-3">Deployment Name</th>
                  <th className="px-6 py-3">Package</th>
                  <th className="px-6 py-3">Target Scope</th>
                  <th className="px-6 py-3">Status</th>
                  <th className="px-6 py-3">Progress</th>
                  <th className="px-6 py-3">Started</th>
                  <th className="px-6 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800">
                {filteredDeployments.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="px-6 py-8 text-center text-slate-500">
                      No deployments initiated yet. Select a package and click "Deploy" to start a rollout.
                    </td>
                  </tr>
                ) : (
                  filteredDeployments.map((dep) => {
                    const pct = dep.total_tasks > 0 ? (dep.success_tasks / dep.total_tasks) * 100 : 0
                    return (
                      <tr key={dep.id} className="hover:bg-slate-800/30 transition-colors">
                        <td className="px-6 py-4 font-medium text-white">
                          <div className="flex items-center gap-2">
                            <Layers className="h-4 w-4 text-cyan-400 shrink-0" />
                            {dep.name}
                          </div>
                        </td>
                        <td className="px-6 py-4 text-slate-300">
                          <div>{dep.package_name || dep.package_id}</div>
                          <div className="text-xs text-slate-500">v{dep.package_version || 'latest'}</div>
                        </td>
                        <td className="px-6 py-4">
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium uppercase bg-slate-800 text-slate-300 border border-slate-700">
                            {dep.target_type} {dep.target_id ? `(${dep.target_id})` : ''}
                          </span>
                        </td>
                        <td className="px-6 py-4">
                          {dep.status === 'completed' ? (
                            <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-950/60 text-emerald-400 border border-emerald-800/60">
                              <CheckCircle2 className="h-3 w-3" /> Completed
                            </span>
                          ) : dep.status === 'running' ? (
                            <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-cyan-950/60 text-cyan-400 border border-cyan-800/60">
                              <Clock className="h-3 w-3 animate-spin" /> In Progress
                            </span>
                          ) : (
                            <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-rose-950/60 text-rose-400 border border-rose-800/60">
                              <XCircle className="h-3 w-3" /> Failed
                            </span>
                          )}
                        </td>
                        <td className="px-6 py-4">
                          <div className="w-36">
                            <div className="flex justify-between text-xs text-slate-400 mb-1">
                              <span>
                                {dep.success_tasks}/{dep.total_tasks}
                              </span>
                              <span>{pct.toFixed(0)}%</span>
                            </div>
                            <div className="w-full bg-slate-800 rounded-full h-1.5 overflow-hidden">
                              <div
                                className={`h-1.5 rounded-full ${
                                  dep.failed_tasks > 0 ? 'bg-amber-500' : 'bg-emerald-500'
                                }`}
                                style={{ width: `${pct}%` }}
                              />
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 text-xs text-slate-400">
                          {new Date(dep.created_at).toLocaleString()}
                        </td>
                        <td className="px-6 py-4 text-right">
                          <button
                            onClick={() => handleOpenTasks(dep)}
                            className="inline-flex items-center gap-1.5 px-3 py-1 text-xs font-medium text-white bg-slate-800 hover:bg-slate-700 border border-slate-700 rounded-md transition-colors"
                          >
                            <Terminal className="h-3.5 w-3.5 text-cyan-400" />
                            View Tasks
                          </button>
                        </td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* UPLOAD PACKAGE MODAL */}
      {showUploadModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm">
          <div className="bg-slate-900 border border-slate-800 rounded-xl shadow-2xl max-w-lg w-full p-6 space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800">
              <h2 className="text-lg font-bold text-white flex items-center gap-2">
                <UploadCloud className="h-5 w-5 text-cyan-400" />
                Upload Software Package
              </h2>
              <button onClick={() => setShowUploadModal(false)} className="text-slate-400 hover:text-white">
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleUpload} className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">Package Name</label>
                  <input
                    type="text"
                    required
                    placeholder="e.g. 7-Zip Archiver"
                    value={uploadName}
                    onChange={(e) => setUploadName(e.target.value)}
                    className="w-full px-3 py-2 text-sm bg-slate-950 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">Version</label>
                  <input
                    type="text"
                    required
                    placeholder="e.g. 24.08"
                    value={uploadVersion}
                    onChange={(e) => setUploadVersion(e.target.value)}
                    className="w-full px-3 py-2 text-sm bg-slate-950 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">Target Operating System</label>
                  <select
                    value={uploadOS}
                    onChange={(e) => setUploadOS(e.target.value as any)}
                    className="w-full px-3 py-2 text-sm bg-slate-950 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                  >
                    <option value="windows">Windows</option>
                    <option value="linux">Linux</option>
                    <option value="macos">macOS</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">Package Format</label>
                  <select
                    value={uploadType}
                    onChange={(e) => setUploadType(e.target.value as any)}
                    className="w-full px-3 py-2 text-sm bg-slate-950 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500 font-mono text-xs"
                  >
                    {uploadOS === 'windows' && (
                      <>
                        <option value="msi">MSI (Windows Installer)</option>
                        <option value="exe">EXE (Executable)</option>
                        <option value="script">PowerShell Script (.ps1)</option>
                      </>
                    )}
                    {uploadOS === 'linux' && (
                      <>
                        <option value="deb">DEB (Debian/Ubuntu)</option>
                        <option value="rpm">RPM (RHEL/CentOS)</option>
                        <option value="script">Shell Script (.sh)</option>
                      </>
                    )}
                    {uploadOS === 'macos' && (
                      <>
                        <option value="pkg">PKG (Apple Installer)</option>
                        <option value="script">Shell Script (.sh)</option>
                      </>
                    )}
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">Silent Install Arguments</label>
                <input
                  type="text"
                  placeholder={uploadType === 'msi' ? '/qn /norestart' : uploadType === 'exe' ? '/S' : ''}
                  value={uploadInstallArgs}
                  onChange={(e) => setUploadInstallArgs(e.target.value)}
                  className="w-full px-3 py-2 text-sm bg-slate-950 border border-slate-700 rounded-lg text-white font-mono text-xs focus:outline-none focus:border-cyan-500"
                />
                <span className="text-[11px] text-slate-500">Leave blank for automatic default silent flags</span>
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">Installer Binary File</label>
                <input
                  type="file"
                  required
                  onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
                  className="w-full text-xs text-slate-400 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-xs file:font-semibold file:bg-slate-800 file:text-cyan-400 hover:file:bg-slate-700 cursor-pointer bg-slate-950 border border-slate-700 rounded-lg p-1.5"
                />
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowUploadModal(false)}
                  className="px-4 py-2 text-sm text-slate-400 hover:text-white"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={uploading}
                  className="px-4 py-2 text-sm font-semibold text-white bg-cyan-600 hover:bg-cyan-500 rounded-lg transition-colors flex items-center gap-2 disabled:opacity-50"
                >
                  {uploading ? (
                    <>
                      <RefreshCw className="h-4 w-4 animate-spin" />
                      Computing SHA-256 & Uploading...
                    </>
                  ) : (
                    'Upload & Index'
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* DEPLOYMENT WIZARD MODAL */}
      {showDeployModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm">
          <div className="bg-slate-900 border border-slate-800 rounded-xl shadow-2xl max-w-lg w-full p-6 space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800">
              <h2 className="text-lg font-bold text-white flex items-center gap-2">
                <Play className="h-5 w-5 text-cyan-400" />
                Launch Software Deployment
              </h2>
              <button onClick={() => setShowDeployModal(false)} className="text-slate-400 hover:text-white">
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleCreateDeployment} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">Deployment Name</label>
                <input
                  type="text"
                  required
                  value={deployName}
                  onChange={(e) => setDeployName(e.target.value)}
                  className="w-full px-3 py-2 text-sm bg-slate-950 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                />
              </div>

              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">Software Package</label>
                <select
                  value={deployPackageId}
                  onChange={(e) => setDeployPackageId(e.target.value)}
                  className="w-full px-3 py-2 text-sm bg-slate-950 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                >
                  {packages.map((pkg) => (
                    <option key={pkg.id} value={pkg.id}>
                      {pkg.name} v{pkg.version} ({pkg.os_target} - {pkg.package_type})
                    </option>
                  ))}
                </select>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">Target Scope</label>
                  <select
                    value={deployTargetType}
                    onChange={(e) => setDeployTargetType(e.target.value as any)}
                    className="w-full px-3 py-2 text-sm bg-slate-950 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                  >
                    <option value="all">All Compatible Devices</option>
                    <option value="group">Static Device Group</option>
                    <option value="device">Single Endpoint</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">Target ID</label>
                  <input
                    type="text"
                    disabled={deployTargetType === 'all'}
                    placeholder={deployTargetType === 'all' ? '(Applies to all fleet)' : 'Enter Group ID or Device ID'}
                    value={deployTargetId}
                    onChange={(e) => setDeployTargetId(e.target.value)}
                    className="w-full px-3 py-2 text-sm bg-slate-950 border border-slate-700 rounded-lg text-white disabled:opacity-50 focus:outline-none focus:border-cyan-500"
                  />
                </div>
              </div>

              <div className="p-3 bg-cyan-950/20 border border-cyan-800/40 rounded-lg text-xs text-cyan-300 flex items-start gap-2">
                <AlertCircle className="h-4 w-4 shrink-0 mt-0.5 text-cyan-400" />
                <span>
                  Online endpoints receive execution triggers immediately via persistent WebSocket transport.
                  Packages are cryptographically validated against SHA-256 before silent background installation.
                </span>
              </div>

              <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowDeployModal(false)}
                  className="px-4 py-2 text-sm text-slate-400 hover:text-white"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={deploying}
                  className="px-4 py-2 text-sm font-semibold text-white bg-cyan-600 hover:bg-cyan-500 rounded-lg transition-colors flex items-center gap-2 disabled:opacity-50"
                >
                  {deploying ? (
                    <>
                      <RefreshCw className="h-4 w-4 animate-spin" />
                      Dispatching Jobs...
                    </>
                  ) : (
                    'Start Deployment'
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* VIEW TASKS MODAL */}
      {viewingDeployment && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm">
          <div className="bg-slate-900 border border-slate-800 rounded-xl shadow-2xl max-w-4xl w-full p-6 space-y-4 max-h-[90vh] flex flex-col">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800">
              <div>
                <h2 className="text-lg font-bold text-white flex items-center gap-2">
                  <Terminal className="h-5 w-5 text-cyan-400" />
                  Deployment Tasks: {viewingDeployment.name}
                </h2>
                <p className="text-xs text-slate-400 mt-0.5">
                  Package: {viewingDeployment.package_name} | Target: {viewingDeployment.target_type}
                </p>
              </div>
              <button onClick={() => setViewingDeployment(null)} className="text-slate-400 hover:text-white">
                <X className="h-5 w-5" />
              </button>
            </div>

            <div className="flex-1 overflow-y-auto space-y-2 pr-1">
              {loadingTasks ? (
                <div className="py-12 text-center text-slate-500 flex flex-col items-center gap-2">
                  <RefreshCw className="h-6 w-6 animate-spin text-cyan-400" />
                  Loading execution logs...
                </div>
              ) : deploymentTasks.length === 0 ? (
                <div className="py-8 text-center text-slate-500">No tasks generated for this deployment.</div>
              ) : (
                deploymentTasks.map((t) => (
                  <div key={t.id} className="bg-slate-950 border border-slate-800 rounded-lg p-3 space-y-2">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2 font-mono text-sm text-white">
                        <Server className="h-4 w-4 text-slate-400" />
                        <span>{t.hostname || t.device_id}</span>
                        {t.site && (
                          <span className="text-xs text-slate-500 bg-slate-900 px-1.5 py-0.5 rounded">
                            {t.site}
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-3">
                        <span
                          className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-semibold ${
                            t.status === 'success'
                              ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-800/60'
                              : t.status === 'failed'
                              ? 'bg-rose-950/60 text-rose-400 border border-rose-800/60'
                              : t.status === 'installing' || t.status === 'downloading'
                              ? 'bg-cyan-950/60 text-cyan-400 border border-cyan-800/60 animate-pulse'
                              : 'bg-slate-900 text-slate-400 border border-slate-700'
                          }`}
                        >
                          {t.status}
                        </span>
                        {(t.output_log || t.error_message) && (
                          <button
                            onClick={() => setExpandedTaskId(expandedTaskId === t.id ? null : t.id)}
                            className="text-xs text-cyan-400 hover:text-cyan-300 font-medium"
                          >
                            {expandedTaskId === t.id ? 'Hide Logs' : 'View Logs'}
                          </button>
                        )}
                      </div>
                    </div>

                    {expandedTaskId === t.id && (
                      <div className="mt-2 pt-2 border-t border-slate-800 space-y-2">
                        {t.exit_code !== undefined && t.exit_code !== null && (
                          <div className="text-xs text-slate-400 font-mono">
                            Exit Code: <span className="text-white">{t.exit_code}</span>
                          </div>
                        )}
                        {t.error_message && (
                          <div className="p-2 bg-rose-950/40 border border-rose-800/50 rounded text-xs text-rose-300 font-mono">
                            Error: {t.error_message}
                          </div>
                        )}
                        {t.output_log && (
                          <pre className="p-3 bg-slate-900 border border-slate-800 rounded text-xs text-slate-300 font-mono overflow-x-auto max-h-48 whitespace-pre-wrap">
                            {t.output_log}
                          </pre>
                        )}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>

            <div className="flex justify-end pt-3 border-t border-slate-800">
              <button
                onClick={() => setViewingDeployment(null)}
                className="px-4 py-2 text-sm font-semibold text-white bg-slate-800 hover:bg-slate-700 rounded-lg transition-colors"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

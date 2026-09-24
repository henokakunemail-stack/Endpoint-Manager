import React, { useEffect, useState } from 'react'
import {
  Calendar,
  Clock,
  Code2,
  FileCode,
  FileText,
  Plus,
  RefreshCw,
  Trash2,
  X,
} from 'lucide-react'
import { api } from '../services/api'
import type { ScheduleDTO, ScriptDTO, TaskRunDTO } from '../types/api'

export const TasksSchedulerPage: React.FC = () => {
  const [subTab, setSubTab] = useState<'scripts' | 'schedules' | 'runs'>('scripts')
  const [scripts, setScripts] = useState<ScriptDTO[]>([])
  const [schedules, setSchedules] = useState<ScheduleDTO[]>([])
  const [runs, setRuns] = useState<TaskRunDTO[]>([])
  const [loading, setLoading] = useState(true)

  // Modals
  const [isScriptModalOpen, setIsScriptModalOpen] = useState(false)
  const [isScheduleModalOpen, setIsScheduleModalOpen] = useState(false)
  const [viewLogRun, setViewLogRun] = useState<TaskRunDTO | null>(null)

  // Form states
  const [newScript, setNewScript] = useState({
    name: '',
    description: '',
    shell_type: 'powershell',
    script_content: '',
  })
  const [newSchedule, setNewSchedule] = useState({
    name: '',
    script_id: '',
    target_type: 'all',
    target_id: '',
    schedule_type: 'interval',
    interval_seconds: 3600,
    cron_expr: '0 0 * * *',
  })
  const [msg, setMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const loadData = async () => {
    setLoading(true)
    try {
      const [scList, schList, runList] = await Promise.all([
        api.getScripts().catch(() => []),
        api.getSchedules().catch(() => []),
        api.getTaskRuns().catch(() => []),
      ])
      setScripts(scList)
      setSchedules(schList)
      setRuns(runList)
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to load task scheduler data' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  const handleCreateScript = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.createScript(newScript)
      setMsg({ type: 'success', text: `Script '${newScript.name}' created successfully with SHA-256 hash.` })
      setIsScriptModalOpen(false)
      setNewScript({ name: '', description: '', shell_type: 'powershell', script_content: '' })
      loadData()
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to create script' })
    }
  }

  const handleDeleteScript = async (id: string, name: string) => {
    if (!confirm(`Delete script '${name}'? This cannot be undone.`)) return
    try {
      await api.deleteScript(id)
      setMsg({ type: 'success', text: 'Script deleted' })
      loadData()
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to delete script' })
    }
  }

  const handleCreateSchedule = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.createSchedule(newSchedule)
      setMsg({ type: 'success', text: `Schedule '${newSchedule.name}' created and activated.` })
      setIsScheduleModalOpen(false)
      loadData()
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to create schedule' })
    }
  }

  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <h2 className="page-title">Task Scheduler & Script Repository</h2>
          <p className="page-subtitle">
            Automated maintenance scripts, recurring background jobs, and distributed fleet executions
          </p>
        </div>
        <div className="header-controls">
          <button type="button" className="btn btn-secondary" onClick={loadData} disabled={loading}>
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            <span>Refresh</span>
          </button>
          {subTab === 'scripts' && (
            <button type="button" className="btn btn-primary" onClick={() => setIsScriptModalOpen(true)}>
              <Plus size={16} />
              <span>New Script</span>
            </button>
          )}
          {subTab === 'schedules' && (
            <button type="button" className="btn btn-primary" onClick={() => setIsScheduleModalOpen(true)}>
              <Plus size={16} />
              <span>New Schedule</span>
            </button>
          )}
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

      {/* Sub Tabs */}
      <div className="tabs-nav">
        <button
          type="button"
          className={`tab-btn ${subTab === 'scripts' ? 'active' : ''}`}
          onClick={() => setSubTab('scripts')}
        >
          <Code2 size={16} />
          <span>Script Repository ({scripts.length})</span>
        </button>
        <button
          type="button"
          className={`tab-btn ${subTab === 'schedules' ? 'active' : ''}`}
          onClick={() => setSubTab('schedules')}
        >
          <Calendar size={16} />
          <span>Schedules ({schedules.length})</span>
        </button>
        <button
          type="button"
          className={`tab-btn ${subTab === 'runs' ? 'active' : ''}`}
          onClick={() => setSubTab('runs')}
        >
          <Clock size={16} />
          <span>Execution History ({runs.length})</span>
        </button>
      </div>

      {/* Tab 1: Script Repository */}
      {subTab === 'scripts' && (
        <div className="table-card">
          <div className="table-responsive">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Script Name</th>
                  <th>Shell Type</th>
                  <th>Description</th>
                  <th>SHA-256 Checksum</th>
                  <th>Author</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {scripts.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="text-center py-8 text-muted">
                      No scripts in repository. Click "New Script" to create one.
                    </td>
                  </tr>
                ) : (
                  scripts.map((s) => (
                    <tr key={s.id}>
                      <td>
                        <div className="font-semibold text-main flex items-center gap-2">
                          <FileCode size={16} className="text-primary" />
                          <span>{s.name}</span>
                        </div>
                      </td>
                      <td>
                        <span className="os-badge">{s.shell_type}</span>
                      </td>
                      <td>{s.description || '—'}</td>
                      <td>
                        <span className="font-mono text-sm text-dim" title={s.sha256_hash}>
                          {s.sha256_hash ? s.sha256_hash.slice(0, 16) + '...' : '—'}
                        </span>
                      </td>
                      <td>{s.created_by}</td>
                      <td>
                        <button
                          type="button"
                          className="btn-action text-danger"
                          onClick={() => handleDeleteScript(s.id, s.name)}
                          title="Delete script"
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
      )}

      {/* Tab 2: Schedules */}
      {subTab === 'schedules' && (
        <div className="table-card">
          <div className="table-responsive">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Schedule Name</th>
                  <th>Script</th>
                  <th>Target Fleet</th>
                  <th>Frequency</th>
                  <th>Status</th>
                  <th>Next Execution</th>
                </tr>
              </thead>
              <tbody>
                {schedules.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="text-center py-8 text-muted">
                      No recurring schedules configured.
                    </td>
                  </tr>
                ) : (
                  schedules.map((sch) => (
                    <tr key={sch.id}>
                      <td className="font-semibold text-main">{sch.name}</td>
                      <td>{sch.script_name || sch.script_id.slice(0, 12)}</td>
                      <td>
                        <span className="badge-target">{sch.target_type}</span>
                      </td>
                      <td>
                        {sch.schedule_type === 'cron' ? `Cron: ${sch.cron_expr}` : `Every ${sch.interval_seconds}s`}
                      </td>
                      <td>
                        <span className={`status-pill ${sch.is_active ? 'online' : 'offline'}`}>
                          {sch.is_active ? 'Active' : 'Paused'}
                        </span>
                      </td>
                      <td className="text-sm text-muted">
                        {sch.next_run_at ? new Date(sch.next_run_at).toLocaleString() : '—'}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Tab 3: Execution History */}
      {subTab === 'runs' && (
        <div className="table-card">
          <div className="table-responsive">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Job ID</th>
                  <th>Device</th>
                  <th>Status</th>
                  <th>Exit Code</th>
                  <th>Executed At</th>
                  <th>Log Output</th>
                </tr>
              </thead>
              <tbody>
                {runs.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="text-center py-8 text-muted">
                      No execution logs recorded yet.
                    </td>
                  </tr>
                ) : (
                  runs.map((r) => (
                    <tr key={r.id}>
                      <td className="font-mono text-sm">{r.id.slice(0, 12)}...</td>
                      <td>{r.hostname || r.device_id.slice(0, 10)}</td>
                      <td>
                        <span className={`status-pill ${r.status === 'success' ? 'online' : r.status === 'failed' ? 'danger' : 'warning'}`}>
                          {r.status}
                        </span>
                      </td>
                      <td className="font-mono">{r.exit_code !== undefined ? r.exit_code : '—'}</td>
                      <td className="text-sm text-muted">{new Date(r.started_at).toLocaleString()}</td>
                      <td>
                        <button
                          type="button"
                          className="btn-action"
                          onClick={() => setViewLogRun(r)}
                        >
                          <FileText size={14} />
                          <span>View Log</span>
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Create Script Modal */}
      {isScriptModalOpen && (
        <div className="modal-overlay" onClick={() => setIsScriptModalOpen(false)}>
          <div className="modal-dialog modal-md" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Create Maintenance Script</h3>
              <button type="button" className="btn-close" onClick={() => setIsScriptModalOpen(false)}>
                <X size={18} />
              </button>
            </div>
            <form onSubmit={handleCreateScript}>
              <div className="modal-body space-y-4">
                <div className="form-group">
                  <label className="form-label">Script Title</label>
                  <input
                    type="text"
                    className="form-input"
                    required
                    placeholder="e.g. Flush DNS & Restart Spooler"
                    value={newScript.name}
                    onChange={(e) => setNewScript({ ...newScript, name: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Shell Environment</label>
                  <select
                    className="form-select"
                    value={newScript.shell_type}
                    onChange={(e) => setNewScript({ ...newScript, shell_type: e.target.value })}
                  >
                    <option value="powershell">PowerShell (Windows)</option>
                    <option value="cmd">Command Prompt (CMD)</option>
                    <option value="bash">Bash (Linux / macOS)</option>
                    <option value="sh">POSIX Shell (/bin/sh)</option>
                  </select>
                </div>
                <div className="form-group">
                  <label className="form-label">Description (Optional)</label>
                  <input
                    type="text"
                    className="form-input"
                    placeholder="Purpose of this script"
                    value={newScript.description}
                    onChange={(e) => setNewScript({ ...newScript, description: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Script Body</label>
                  <textarea
                    className="form-textarea font-mono"
                    rows={8}
                    required
                    placeholder="# Enter script commands here..."
                    value={newScript.script_content}
                    onChange={(e) => setNewScript({ ...newScript, script_content: e.target.value })}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setIsScriptModalOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Save Script
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Create Schedule Modal */}
      {isScheduleModalOpen && (
        <div className="modal-overlay" onClick={() => setIsScheduleModalOpen(false)}>
          <div className="modal-dialog modal-md" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Schedule Recurring Maintenance</h3>
              <button type="button" className="btn-close" onClick={() => setIsScheduleModalOpen(false)}>
                <X size={18} />
              </button>
            </div>
            <form onSubmit={handleCreateSchedule}>
              <div className="modal-body space-y-4">
                <div className="form-group">
                  <label className="form-label">Schedule Name</label>
                  <input
                    type="text"
                    className="form-input"
                    required
                    placeholder="e.g. Daily Temp File Cleanup"
                    value={newSchedule.name}
                    onChange={(e) => setNewSchedule({ ...newSchedule, name: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Target Script</label>
                  <select
                    className="form-select"
                    required
                    value={newSchedule.script_id}
                    onChange={(e) => setNewSchedule({ ...newSchedule, script_id: e.target.value })}
                  >
                    <option value="">-- Select Script --</option>
                    {scripts.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name} ({s.shell_type})
                      </option>
                    ))}
                  </select>
                </div>
                <div className="form-group">
                  <label className="form-label">Target Fleet</label>
                  <select
                    className="form-select"
                    value={newSchedule.target_type}
                    onChange={(e) => setNewSchedule({ ...newSchedule, target_type: e.target.value })}
                  >
                    <option value="all">All Enrolled Devices</option>
                    <option value="group">Branch Site / Group</option>
                  </select>
                </div>
                <div className="form-group">
                  <label className="form-label">Schedule Interval (Seconds)</label>
                  <input
                    type="number"
                    className="form-input"
                    value={newSchedule.interval_seconds}
                    onChange={(e) => setNewSchedule({ ...newSchedule, interval_seconds: Number(e.target.value) })}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setIsScheduleModalOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Activate Schedule
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* View Output Log Modal */}
      {viewLogRun && (
        <div className="modal-overlay" onClick={() => setViewLogRun(null)}>
          <div className="modal-dialog modal-lg" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Job Execution Output: {viewLogRun.id}</h3>
              <button type="button" className="btn-close" onClick={() => setViewLogRun(null)}>
                <X size={18} />
              </button>
            </div>
            <div className="modal-body">
              <pre className="terminal-log-output">
                {viewLogRun.output_log || viewLogRun.error_message || '(No output recorded)'}
              </pre>
            </div>
            <div className="modal-footer">
              <button type="button" className="btn btn-secondary" onClick={() => setViewLogRun(null)}>
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

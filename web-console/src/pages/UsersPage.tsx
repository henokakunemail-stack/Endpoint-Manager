import React, { useEffect, useState } from 'react'
import {
  Edit2,
  Key,
  RefreshCw,
  UserPlus,
  Users,
  UserX,
  X,
} from 'lucide-react'
import { api } from '../services/api'
import { useToast } from '../context/ToastContext'
import type { UserDTO } from '../types/api'

export const UsersPage: React.FC = () => {
  const [users, setUsers] = useState<UserDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [msg, setMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const toast = useToast()

  // Modals
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [isEditModalOpen, setIsEditModalOpen] = useState(false)
  const [isPasswordModalOpen, setIsPasswordModalOpen] = useState(false)
  const [selectedUser, setSelectedUser] = useState<UserDTO | null>(null)

  // Create Form
  const [newUsername, setNewUsername] = useState('')
  const [newDisplayName, setNewDisplayName] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newRole, setNewRole] = useState<'admin' | 'technician' | 'viewer'>('technician')

  // Edit Form
  const [editDisplayName, setEditDisplayName] = useState('')
  const [editRole, setEditRole] = useState<'admin' | 'technician' | 'viewer'>('technician')

  // Password Reset Form
  const [resetPassword, setResetPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')

  const loadUsers = async () => {
    setLoading(true)
    try {
      const uList = await api.getUsers()
      setUsers(uList)
    } catch (err: any) {
      setMsg({ type: 'error', text: err.message || 'Failed to load user accounts' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadUsers()
  }, [])

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      await api.createUser({
        username: newUsername,
        display_name: newDisplayName,
        password: newPassword,
        role: newRole,
      })
      const successText = `User account '${newUsername}' created successfully.`
      setMsg({ type: 'success', text: successText })
      toast.success(successText, 'User Created')
      setIsCreateModalOpen(false)
      setNewUsername('')
      setNewDisplayName('')
      setNewPassword('')
      loadUsers()
    } catch (err: any) {
      const errorText = err.message || 'Failed to create user'
      setMsg({ type: 'error', text: errorText })
      toast.error(errorText, 'Creation Failed')
    }
  }

  const handleOpenEdit = (u: UserDTO) => {
    setSelectedUser(u)
    setEditDisplayName(u.display_name)
    setEditRole(u.role)
    setIsEditModalOpen(true)
  }

  const handleSaveEdit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedUser) return
    try {
      await api.updateUser(selectedUser.id, {
        display_name: editDisplayName,
        role: editRole,
      })
      const successText = `User '${selectedUser.username}' updated.`
      setMsg({ type: 'success', text: successText })
      toast.success(successText, 'User Updated')
      setIsEditModalOpen(false)
      loadUsers()
    } catch (err: any) {
      const errorText = err.message || 'Failed to update user'
      setMsg({ type: 'error', text: errorText })
      toast.error(errorText, 'Update Failed')
    }
  }

  const handleOpenPasswordReset = (u: UserDTO) => {
    setSelectedUser(u)
    setResetPassword('')
    setConfirmPassword('')
    setIsPasswordModalOpen(true)
  }

  const handleSavePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedUser) return
    if (resetPassword !== confirmPassword) {
      setMsg({ type: 'error', text: 'Passwords do not match!' })
      toast.warning('Passwords do not match', 'Validation Error')
      return
    }
    if (resetPassword.length < 8) {
      setMsg({ type: 'error', text: 'Password must be at least 8 characters long.' })
      toast.warning('Password must be at least 8 characters long.', 'Validation Error')
      return
    }

    try {
      await api.adminResetPassword(selectedUser.id, resetPassword)
      const successText = `Password for '${selectedUser.username}' updated successfully.`
      setMsg({ type: 'success', text: successText })
      toast.success(successText, 'Password Reset')
      setIsPasswordModalOpen(false)
    } catch (err: any) {
      const errorText = err.message || 'Failed to reset password'
      setMsg({ type: 'error', text: errorText })
      toast.error(errorText, 'Reset Failed')
    }
  }

  const handleDeactivate = async (u: UserDTO) => {
    if (!confirm(`Are you sure you want to deactivate user account '${u.username}'?`)) return
    try {
      await api.deactivateUser(u.id)
      const infoText = `User '${u.username}' deactivated.`
      setMsg({ type: 'success', text: infoText })
      toast.info(infoText, 'Account Deactivated')
      loadUsers()
    } catch (err: any) {
      const errorText = err.message || 'Failed to deactivate user'
      setMsg({ type: 'error', text: errorText })
      toast.error(errorText, 'Deactivation Failed')
    }
  }

  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <h2 className="page-title">User Accounts & RBAC Permissions</h2>
          <p className="page-subtitle">
            Enterprise role-based access control, operator identity management, and credential provisioning
          </p>
        </div>
        <div className="header-controls">
          <button type="button" className="btn btn-secondary" onClick={loadUsers} disabled={loading}>
            <RefreshCw size={16} className={loading ? 'animate-spin' : ''} />
            <span>Refresh</span>
          </button>
          <button type="button" className="btn btn-primary" onClick={() => setIsCreateModalOpen(true)}>
            <UserPlus size={16} />
            <span>Create User</span>
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

      {/* Users Table */}
      <div className="table-card">
        <div className="table-responsive">
          <table className="data-table">
            <thead>
              <tr>
                <th>Username</th>
                <th>Display Name</th>
                <th>Assigned Role</th>
                <th>Account Status</th>
                <th>Created At</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {users.length === 0 ? (
                <tr>
                  <td colSpan={6} className="text-center py-8 text-muted">
                    No users found.
                  </td>
                </tr>
              ) : (
                users.map((u) => (
                  <tr key={u.id}>
                    <td>
                      <div className="font-semibold text-main flex items-center gap-2">
                        <Users size={16} className="text-primary" />
                        <span>{u.username}</span>
                      </div>
                    </td>
                    <td>{u.display_name}</td>
                    <td>
                      <span
                        className={`badge-role ${
                          u.role === 'admin'
                            ? 'role-admin'
                            : u.role === 'technician'
                            ? 'role-technician'
                            : 'role-viewer'
                        }`}
                      >
                        {u.role.toUpperCase()}
                      </span>
                    </td>
                    <td>
                      <span className={`status-pill ${u.status === 'active' ? 'online' : 'offline'}`}>
                        {u.status}
                      </span>
                    </td>
                    <td className="text-sm text-muted">{new Date(u.created_at).toLocaleDateString()}</td>
                    <td>
                      <div className="action-buttons">
                        <button
                          type="button"
                          className="btn-action"
                          onClick={() => handleOpenEdit(u)}
                          title="Edit role / name"
                        >
                          <Edit2 size={14} />
                          <span>Edit</span>
                        </button>
                        <button
                          type="button"
                          className="btn-action text-warning"
                          onClick={() => handleOpenPasswordReset(u)}
                          title="Reset password"
                        >
                          <Key size={14} />
                          <span>Password</span>
                        </button>
                        {u.status === 'active' && u.username !== 'admin' && (
                          <button
                            type="button"
                            className="btn-action text-danger"
                            onClick={() => handleDeactivate(u)}
                            title="Deactivate account"
                          >
                            <UserX size={14} />
                            <span>Deactivate</span>
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

      {/* Create User Modal */}
      {isCreateModalOpen && (
        <div className="modal-overlay" onClick={() => setIsCreateModalOpen(false)}>
          <div className="modal-dialog modal-md" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Create Console Operator Account</h3>
              <button type="button" className="btn-close" onClick={() => setIsCreateModalOpen(false)}>
                <X size={18} />
              </button>
            </div>
            <form onSubmit={handleCreateUser}>
              <div className="modal-body space-y-4">
                <div className="form-group">
                  <label className="form-label">Username</label>
                  <input
                    type="text"
                    className="form-input"
                    required
                    placeholder="e.g. jdoe"
                    value={newUsername}
                    onChange={(e) => setNewUsername(e.target.value)}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Display Name</label>
                  <input
                    type="text"
                    className="form-input"
                    required
                    placeholder="e.g. John Doe"
                    value={newDisplayName}
                    onChange={(e) => setNewDisplayName(e.target.value)}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Temporary Password</label>
                  <input
                    type="password"
                    className="form-input"
                    required
                    minLength={8}
                    placeholder="Min 8 characters"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">RBAC Role</label>
                  <select
                    className="form-select"
                    value={newRole}
                    onChange={(e) => setNewRole(e.target.value as any)}
                  >
                    <option value="technician">Technician / Operator (Deploy, Exec, Patches)</option>
                    <option value="admin">Administrator (Full Access & User Mgmt)</option>
                    <option value="viewer">Viewer (Read-only)</option>
                  </select>
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setIsCreateModalOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Create User
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Edit User Modal */}
      {isEditModalOpen && selectedUser && (
        <div className="modal-overlay" onClick={() => setIsEditModalOpen(false)}>
          <div className="modal-dialog modal-md" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Edit User: {selectedUser.username}</h3>
              <button type="button" className="btn-close" onClick={() => setIsEditModalOpen(false)}>
                <X size={18} />
              </button>
            </div>
            <form onSubmit={handleSaveEdit}>
              <div className="modal-body space-y-4">
                <div className="form-group">
                  <label className="form-label">Display Name</label>
                  <input
                    type="text"
                    className="form-input"
                    required
                    value={editDisplayName}
                    onChange={(e) => setEditDisplayName(e.target.value)}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Role</label>
                  <select
                    className="form-select"
                    value={editRole}
                    onChange={(e) => setEditRole(e.target.value as any)}
                  >
                    <option value="technician">Technician / Operator</option>
                    <option value="admin">Administrator</option>
                    <option value="viewer">Viewer (Read-only)</option>
                  </select>
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setIsEditModalOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Save Changes
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Password Reset Modal */}
      {isPasswordModalOpen && selectedUser && (
        <div className="modal-overlay" onClick={() => setIsPasswordModalOpen(false)}>
          <div className="modal-dialog modal-md" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Reset Password: {selectedUser.username}</h3>
              <button type="button" className="btn-close" onClick={() => setIsPasswordModalOpen(false)}>
                <X size={18} />
              </button>
            </div>
            <form onSubmit={handleSavePassword}>
              <div className="modal-body space-y-4">
                <div className="form-group">
                  <label className="form-label">New Password</label>
                  <input
                    type="password"
                    className="form-input"
                    required
                    minLength={8}
                    placeholder="Enter new password"
                    value={resetPassword}
                    onChange={(e) => setResetPassword(e.target.value)}
                  />
                </div>
                <div className="form-group">
                  <label className="form-label">Confirm New Password</label>
                  <input
                    type="password"
                    className="form-input"
                    required
                    placeholder="Re-type new password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                  />
                </div>
              </div>
              <div className="modal-footer">
                <button type="button" className="btn btn-secondary" onClick={() => setIsPasswordModalOpen(false)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Update Password
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

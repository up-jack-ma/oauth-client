import { useState, useEffect } from 'react'
import { api } from '../api'

const PRESET_PROVIDERS = {
  github: {
    name: 'github', display_name: 'GitHub', icon: 'GH',
    auth_url: 'https://github.com/login/oauth/authorize',
    token_url: 'https://github.com/login/oauth/access_token',
    userinfo_url: 'https://api.github.com/user',
    scopes: 'read:user user:email',
  },
  google: {
    name: 'google', display_name: 'Google', icon: 'G',
    auth_url: 'https://accounts.google.com/o/oauth2/v2/auth',
    token_url: 'https://oauth2.googleapis.com/token',
    userinfo_url: 'https://www.googleapis.com/oauth2/v2/userinfo',
    scopes: 'openid email profile',
    extra_params: JSON.stringify({ access_type: 'offline', prompt: 'consent' }),
  },
  gitlab: {
    name: 'gitlab', display_name: 'GitLab', icon: 'GL',
    auth_url: 'https://gitlab.com/oauth/authorize',
    token_url: 'https://gitlab.com/oauth/token',
    userinfo_url: 'https://gitlab.com/api/v4/user',
    scopes: 'read_user',
  },
  discord: {
    name: 'discord', display_name: 'Discord', icon: 'DC',
    auth_url: 'https://discord.com/api/oauth2/authorize',
    token_url: 'https://discord.com/api/oauth2/token',
    userinfo_url: 'https://discord.com/api/users/@me',
    scopes: 'identify email',
  },
}

const emptyProvider = {
  name: '', display_name: '', icon: '', client_id: '', client_secret: '',
  auth_url: '', token_url: '', userinfo_url: '', scopes: '', enabled: true, extra_params: '{}',
}

export default function AdminPage() {
  const [tab, setTab] = useState('providers')
  const [stats, setStats] = useState(null)
  const [providers, setProviders] = useState([])
  const [users, setUsers] = useState([])
  const [showModal, setShowModal] = useState(false)
  const [editProvider, setEditProvider] = useState(null)
  const [form, setForm] = useState({ ...emptyProvider })
  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => { loadData() }, [tab])

  const loadData = async () => {
    setLoading(true)
    try {
      const s = await api.adminGetStats()
      setStats(s)
      if (tab === 'providers') {
        const p = await api.adminGetProviders()
        setProviders(p)
      } else {
        const u = await api.adminGetUsers()
        setUsers(u)
      }
    } catch (err) {
      setMessage(`Error: ${err.message}`)
    } finally {
      setLoading(false)
    }
  }

  const showMsg = (msg) => {
    setMessage(msg)
    setTimeout(() => setMessage(''), 3000)
  }

  const openCreate = (preset) => {
    if (preset && PRESET_PROVIDERS[preset]) {
      setForm({ ...emptyProvider, ...PRESET_PROVIDERS[preset] })
    } else {
      setForm({ ...emptyProvider })
    }
    setEditProvider(null)
    setShowModal(true)
  }

  const openEdit = (p) => {
    setForm({ ...p })
    setEditProvider(p)
    setShowModal(true)
  }

  const handleSave = async () => {
    try {
      if (editProvider) {
        await api.adminUpdateProvider(editProvider.id, form)
        showMsg('Provider updated')
      } else {
        await api.adminCreateProvider(form)
        showMsg('Provider created')
      }
      setShowModal(false)
      loadData()
    } catch (err) {
      showMsg(`Error: ${err.message}`)
    }
  }

  const handleDelete = async (id) => {
    if (!confirm('Delete this provider? All linked accounts using it will be removed.')) return
    try {
      await api.adminDeleteProvider(id)
      showMsg('Provider deleted')
      loadData()
    } catch (err) {
      showMsg(`Error: ${err.message}`)
    }
  }

  const handleRoleChange = async (userId, newRole) => {
    try {
      await api.adminUpdateUserRole(userId, newRole)
      showMsg('Role updated')
      loadData()
    } catch (err) {
      showMsg(`Error: ${err.message}`)
    }
  }

  return (
    <div className="container">
      {stats && (
        <div className="stat-grid">
          <div className="stat-card">
            <div className="stat-value">{stats.total_users}</div>
            <div className="stat-label">Users</div>
          </div>
          <div className="stat-card">
            <div className="stat-value">{stats.total_providers}</div>
            <div className="stat-label">Providers</div>
          </div>
          <div className="stat-card">
            <div className="stat-value">{stats.total_linked_accounts}</div>
            <div className="stat-label">Linked Accounts</div>
          </div>
        </div>
      )}

      {message && (
        <div className={`alert ${message.startsWith('Error') ? 'alert-error' : 'alert-success'}`}>
          {message}
        </div>
      )}

      <div className="tab-bar">
        <button className={tab === 'providers' ? 'active' : ''} onClick={() => setTab('providers')}>
          OAuth Providers
        </button>
        <button className={tab === 'users' ? 'active' : ''} onClick={() => setTab('users')}>
          Users
        </button>
      </div>

      {tab === 'providers' && (
        <div className="card">
          <div className="card-header">
            <h3>OAuth Providers</h3>
            <div className="flex gap-2">
              <select className="btn btn-outline btn-sm" onChange={e => { if (e.target.value) openCreate(e.target.value); e.target.value = '' }}>
                <option value="">Quick Add...</option>
                {Object.entries(PRESET_PROVIDERS).map(([key, p]) => (
                  <option key={key} value={key}>{p.display_name}</option>
                ))}
              </select>
              <button className="btn btn-primary btn-sm" onClick={() => openCreate()}>
                + Custom Provider
              </button>
            </div>
          </div>

          {loading ? (
            <div className="text-muted">Loading...</div>
          ) : providers.length === 0 ? (
            <div className="empty-state">
              <div className="icon">&#9881;</div>
              <h3>No providers configured</h3>
              <p className="text-muted mt-2">Add an OAuth provider to get started</p>
            </div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Provider</th>
                  <th>Client ID</th>
                  <th>Status</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {providers.map(p => (
                  <tr key={p.id}>
                    <td>
                      <div className="flex items-center gap-2">
                        <span>{p.icon || p.name[0].toUpperCase()}</span>
                        <div>
                          <div style={{ fontWeight: 600 }}>{p.display_name}</div>
                          <div className="text-sm text-muted">{p.name}</div>
                        </div>
                      </div>
                    </td>
                    <td><code className="text-sm">{p.client_id.substring(0, 20)}...</code></td>
                    <td>
                      <span className={`badge ${p.enabled ? 'badge-enabled' : 'badge-disabled'}`}>
                        {p.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </td>
                    <td>
                      <div className="flex gap-2">
                        <button className="btn btn-outline btn-sm" onClick={() => openEdit(p)}>Edit</button>
                        <button className="btn btn-danger btn-sm" onClick={() => handleDelete(p.id)}>Delete</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {tab === 'users' && (
        <div className="card">
          <div className="card-header">
            <h3>Users</h3>
          </div>
          {loading ? (
            <div className="text-muted">Loading...</div>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>User</th>
                  <th>Email</th>
                  <th>Role</th>
                  <th>Linked</th>
                  <th>Joined</th>
                </tr>
              </thead>
              <tbody>
                {users.map(u => (
                  <tr key={u.id}>
                    <td style={{ fontWeight: 500 }}>{u.display_name}</td>
                    <td className="text-sm">{u.email}</td>
                    <td>
                      <select
                        value={u.role}
                        onChange={e => handleRoleChange(u.id, e.target.value)}
                        className="btn btn-outline btn-sm"
                        style={{ padding: '4px 8px' }}
                      >
                        <option value="user">User</option>
                        <option value="admin">Admin</option>
                      </select>
                    </td>
                    <td>{u.linked_count}</td>
                    <td className="text-sm text-muted">
                      {new Date(u.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* Provider Edit Modal */}
      {showModal && (
        <div className="modal-overlay" onClick={e => { if (e.target === e.currentTarget) setShowModal(false) }}>
          <div className="modal">
            <h2>{editProvider ? 'Edit Provider' : 'Add Provider'}</h2>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
              <div className="form-group">
                <label>Name (slug)</label>
                <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })}
                  placeholder="github" disabled={!!editProvider} />
              </div>
              <div className="form-group">
                <label>Display Name</label>
                <input value={form.display_name} onChange={e => setForm({ ...form, display_name: e.target.value })}
                  placeholder="GitHub" />
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
              <div className="form-group">
                <label>Client ID</label>
                <input value={form.client_id} onChange={e => setForm({ ...form, client_id: e.target.value })}
                  placeholder="your-client-id" />
              </div>
              <div className="form-group">
                <label>Client Secret</label>
                <input value={form.client_secret} onChange={e => setForm({ ...form, client_secret: e.target.value })}
                  placeholder="your-client-secret" type="password" />
              </div>
            </div>

            <div className="form-group">
              <label>Auth URL</label>
              <input value={form.auth_url} onChange={e => setForm({ ...form, auth_url: e.target.value })}
                placeholder="https://provider.com/oauth/authorize" />
            </div>
            <div className="form-group">
              <label>Token URL</label>
              <input value={form.token_url} onChange={e => setForm({ ...form, token_url: e.target.value })}
                placeholder="https://provider.com/oauth/token" />
            </div>
            <div className="form-group">
              <label>Userinfo URL</label>
              <input value={form.userinfo_url} onChange={e => setForm({ ...form, userinfo_url: e.target.value })}
                placeholder="https://provider.com/api/user" />
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '12px' }}>
              <div className="form-group">
                <label>Scopes</label>
                <input value={form.scopes} onChange={e => setForm({ ...form, scopes: e.target.value })}
                  placeholder="openid email profile" />
              </div>
              <div className="form-group">
                <label>Icon (emoji or text)</label>
                <input value={form.icon} onChange={e => setForm({ ...form, icon: e.target.value })}
                  placeholder="GH" />
              </div>
            </div>

            <div className="form-group">
              <label>Extra Auth Params (JSON)</label>
              <input value={form.extra_params} onChange={e => setForm({ ...form, extra_params: e.target.value })}
                placeholder='{"access_type":"offline"}' />
            </div>

            <div className="form-group">
              <label className="flex items-center gap-2" style={{ cursor: 'pointer' }}>
                <input type="checkbox" checked={form.enabled}
                  onChange={e => setForm({ ...form, enabled: e.target.checked })} />
                Enabled
              </label>
            </div>

            <div className="modal-actions">
              <button className="btn btn-outline" onClick={() => setShowModal(false)}>Cancel</button>
              <button className="btn btn-primary" onClick={handleSave}>
                {editProvider ? 'Update' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

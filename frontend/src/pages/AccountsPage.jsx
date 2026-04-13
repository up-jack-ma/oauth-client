import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../api'
import { useAuth } from '../App'

function TokenStatus({ expiry, label }) {
  if (!expiry) return <span className="badge badge-disabled">N/A</span>

  const expiryDate = new Date(expiry)
  const now = new Date()
  const diff = expiryDate - now
  const isExpired = diff <= 0

  const formatRemaining = (ms) => {
    if (ms <= 0) return 'Expired'
    const hours = Math.floor(ms / 3600000)
    const mins = Math.floor((ms % 3600000) / 60000)
    if (hours > 24) {
      const days = Math.floor(hours / 24)
      return `${days}d ${hours % 24}h`
    }
    return `${hours}h ${mins}m`
  }

  return (
    <span className={`badge ${isExpired ? 'badge-disabled' : 'badge-enabled'}`}>
      {isExpired ? 'Expired' : formatRemaining(diff)}
    </span>
  )
}

function TokenField({ label, value, mono }) {
  const [visible, setVisible] = useState(false)
  if (!value) return (
    <div className="token-field">
      <div className="token-label">{label}</div>
      <div className="token-value text-muted">-</div>
    </div>
  )

  return (
    <div className="token-field">
      <div className="token-label">{label}</div>
      <div className="token-value" style={{ fontFamily: mono ? 'monospace' : 'inherit', fontSize: mono ? '12px' : '13px' }}>
        {value}
      </div>
    </div>
  )
}

export default function AccountsPage() {
  const [accounts, setAccounts] = useState([])
  const [providers, setProviders] = useState([])
  const [loading, setLoading] = useState(true)
  const [message, setMessage] = useState('')
  const [expandedId, setExpandedId] = useState(null)
  const [refreshingId, setRefreshingId] = useState(null)
  const [rawViewId, setRawViewId] = useState(null)
  const { user } = useAuth()
  const [searchParams] = useSearchParams()

  useEffect(() => {
    if (searchParams.get('linked') === 'true') {
      setMessage('Account linked successfully!')
      setTimeout(() => setMessage(''), 3000)
    }
  }, [searchParams])

  const loadData = async () => {
    setLoading(true)
    try {
      const [accts, provs] = await Promise.all([
        api.getAccounts(),
        api.getProviders(),
      ])
      setAccounts(accts)
      setProviders(provs)
    } catch (err) {
      console.error(err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadData() }, [])

  const handleLink = async (provider) => {
    try {
      const data = await api.linkAccount(provider.name)
      window.location.href = data.auth_url
    } catch (err) {
      setMessage(`Error: ${err.message}`)
    }
  }

  const handleUnlink = async (id) => {
    if (!confirm('Are you sure you want to unlink this account?')) return
    try {
      await api.unlinkAccount(id)
      setMessage('Account unlinked')
      loadData()
      setTimeout(() => setMessage(''), 3000)
    } catch (err) {
      setMessage(`Error: ${err.message}`)
    }
  }

  const handleRefresh = async (id) => {
    setRefreshingId(id)
    try {
      const result = await api.refreshAccountToken(id)
      setMessage('Token refreshed successfully!')

      // Update the account in local state with new token info
      setAccounts(prev => prev.map(a => {
        if (a.id === id) {
          return {
            ...a,
            access_token: result.access_token,
            refresh_token: result.refresh_token,
            token_expiry: result.token_expiry,
            refresh_token_expiry: result.refresh_token_expiry,
            scopes_granted: result.scopes_granted,
            raw_token_response: result.raw_token_response,
          }
        }
        return a
      }))
      setTimeout(() => setMessage(''), 3000)
    } catch (err) {
      setMessage(`Error: ${err.message}`)
    } finally {
      setRefreshingId(null)
    }
  }

  const formatDate = (d) => {
    if (!d) return '-'
    return new Date(d).toLocaleString('zh-CN', {
      year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    })
  }

  const formatRawJSON = (raw) => {
    if (!raw || raw === '{}') return '{}'
    try {
      return JSON.stringify(JSON.parse(raw), null, 2)
    } catch {
      return raw
    }
  }

  const linkedProviderIds = new Set(accounts.map(a => a.provider_id))
  const unlinkedProviders = providers.filter(p => !linkedProviderIds.has(p.id))

  if (loading) return <div className="container"><div className="text-muted">Loading...</div></div>

  return (
    <div className="container">
      <div className="card">
        <div className="card-header">
          <h2>Linked Accounts</h2>
          <span className="text-sm text-muted">{accounts.length} connected</span>
        </div>

        {message && (
          <div className={`alert ${message.startsWith('Error') ? 'alert-error' : 'alert-success'}`}>
            {message}
          </div>
        )}

        {accounts.length === 0 ? (
          <div className="empty-state">
            <div className="icon">&#128279;</div>
            <h3>No linked accounts yet</h3>
            <p className="text-muted mt-2">Connect your third-party accounts below</p>
          </div>
        ) : (
          accounts.map(account => (
            <div key={account.id} className="account-detail-card">
              {/* Header row */}
              <div className="account-card" onClick={() => setExpandedId(expandedId === account.id ? null : account.id)} style={{ cursor: 'pointer', marginBottom: 0, border: 'none' }}>
                <div className="avatar">
                  {account.provider_avatar ? (
                    <img src={account.provider_avatar} alt="" />
                  ) : (
                    account.provider_icon || account.provider_display_name?.[0] || '?'
                  )}
                </div>
                <div className="info">
                  <div className="provider">{account.provider_display_name}</div>
                  <div className="name">{account.provider_name || 'Unknown'}</div>
                  <div className="email">{account.provider_email}</div>
                </div>
                <div className="flex items-center gap-2">
                  <TokenStatus expiry={account.token_expiry} label="Token" />
                  <span className="expand-arrow" style={{ transition: 'transform 0.2s', transform: expandedId === account.id ? 'rotate(180deg)' : 'rotate(0deg)', fontSize: '12px', color: '#9ca3af' }}>
                    &#9660;
                  </span>
                </div>
              </div>

              {/* Expanded token details */}
              {expandedId === account.id && (
                <div className="token-details">
                  <div className="token-grid">
                    <TokenField label="Access Token" value={account.access_token} mono />
                    <TokenField label="Refresh Token" value={account.refresh_token || '-'} mono />

                    <div className="token-field">
                      <div className="token-label">Token Expiry</div>
                      <div className="token-value">
                        <span>{formatDate(account.token_expiry)}</span>
                        {' '}
                        <TokenStatus expiry={account.token_expiry} />
                      </div>
                    </div>

                    <div className="token-field">
                      <div className="token-label">Refresh Token Expiry</div>
                      <div className="token-value">
                        <span>{formatDate(account.refresh_token_expiry)}</span>
                        {' '}
                        <TokenStatus expiry={account.refresh_token_expiry} />
                      </div>
                    </div>

                    <div className="token-field" style={{ gridColumn: '1 / -1' }}>
                      <div className="token-label">Scopes</div>
                      <div className="token-value">
                        {account.scopes_granted ? (
                          <div className="scope-tags">
                            {account.scopes_granted.split(/[\s,]+/).filter(Boolean).map((s, i) => (
                              <span key={i} className="scope-tag">{s}</span>
                            ))}
                          </div>
                        ) : (
                          <span className="text-muted">-</span>
                        )}
                      </div>
                    </div>

                    {/* OAuth User Info */}
                    {account.raw_userinfo && account.raw_userinfo !== '{}' && (() => {
                      let info = {}
                      try { info = JSON.parse(account.raw_userinfo) } catch {}
                      const entries = Object.entries(info)
                      if (entries.length === 0) return null
                      return (
                        <div className="token-field" style={{ gridColumn: '1 / -1' }}>
                          <div className="token-label">User Info (from {account.provider_display_name})</div>
                          <div className="userinfo-table-wrap">
                            <table className="userinfo-table">
                              <tbody>
                                {entries.map(([key, val]) => (
                                  <tr key={key}>
                                    <td className="userinfo-key">{key}</td>
                                    <td className="userinfo-val">
                                      {typeof val === 'object' ? JSON.stringify(val) : String(val)}
                                    </td>
                                  </tr>
                                ))}
                              </tbody>
                            </table>
                          </div>
                        </div>
                      )
                    })()}

                    <div className="token-field" style={{ gridColumn: '1 / -1' }}>
                      <div className="token-label">
                        Raw Token Response
                        <button className="btn-text" onClick={() => setRawViewId(rawViewId === account.id ? null : account.id)}>
                          {rawViewId === account.id ? 'Hide' : 'Show'}
                        </button>
                      </div>
                      {rawViewId === account.id && (
                        <pre className="raw-json">{formatRawJSON(account.raw_token_response)}</pre>
                      )}
                    </div>

                    <div className="token-field" style={{ gridColumn: '1 / -1' }}>
                      <div className="token-label">Linked At</div>
                      <div className="token-value">{formatDate(account.created_at)}</div>
                    </div>

                    <div className="token-field" style={{ gridColumn: '1 / -1' }}>
                      <div className="token-label">Last Updated</div>
                      <div className="token-value">{formatDate(account.updated_at)}</div>
                    </div>
                  </div>

                  <div className="token-actions">
                    <button
                      className="btn btn-primary btn-sm"
                      onClick={() => handleRefresh(account.id)}
                      disabled={refreshingId === account.id || !account.refresh_token}
                      title={!account.refresh_token ? 'No refresh token available' : 'Refresh access token'}
                    >
                      {refreshingId === account.id ? 'Refreshing...' : 'Refresh Token'}
                    </button>
                    <button className="btn btn-danger btn-sm" onClick={() => handleUnlink(account.id)}>
                      Unlink
                    </button>
                  </div>
                </div>
              )}
            </div>
          ))
        )}
      </div>

      {unlinkedProviders.length > 0 && (
        <div className="card">
          <h3 className="mb-4">Link More Accounts</h3>
          {unlinkedProviders.map(p => (
            <button key={p.id} className="btn-oauth" onClick={() => handleLink(p)}>
              <span className="icon">{p.icon || p.display_name[0]}</span>
              <span>Connect {p.display_name}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

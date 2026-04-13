const API_BASE = '/api'

function getToken() {
  // Try cookie first (set by server), fallback to localStorage
  return localStorage.getItem('token') || ''
}

function setToken(token) {
  localStorage.setItem('token', token)
}

function clearToken() {
  localStorage.removeItem('token')
}

async function request(path, options = {}) {
  const token = getToken()
  const headers = { 'Content-Type': 'application/json', ...options.headers }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const resp = await fetch(`${API_BASE}${path}`, { ...options, headers })

  if (resp.status === 401) {
    clearToken()
    if (window.location.pathname !== '/' && window.location.pathname !== '/login') {
      window.location.href = '/login'
    }
    throw new Error('Unauthorized')
  }

  const data = await resp.json()
  if (!resp.ok) {
    throw new Error(data.error || 'Request failed')
  }
  return data
}

export const api = {
  getToken,
  setToken,
  clearToken,

  // Public
  getProviders: () => request('/providers'),

  // Auth
  login: (email, password) => request('/auth/login', {
    method: 'POST', body: JSON.stringify({ email, password })
  }).then(data => { setToken(data.token); return data }),

  register: (email, password, name) => request('/auth/register', {
    method: 'POST', body: JSON.stringify({ email, password, name })
  }).then(data => { setToken(data.token); return data }),

  logout: () => request('/logout', { method: 'POST' }).finally(() => clearToken()),

  // User
  getMe: () => request('/me'),
  getAccounts: () => request('/accounts'),
  linkAccount: (provider) => request(`/auth/${provider}/link`, { method: 'POST' }),
  refreshAccountToken: (id) => request(`/accounts/${id}/refresh`, { method: 'POST' }),
  unlinkAccount: (id) => request(`/accounts/${id}`, { method: 'DELETE' }),

  // Admin
  adminGetProviders: () => request('/admin/providers'),
  adminCreateProvider: (data) => request('/admin/providers', {
    method: 'POST', body: JSON.stringify(data)
  }),
  adminUpdateProvider: (id, data) => request(`/admin/providers/${id}`, {
    method: 'PUT', body: JSON.stringify(data)
  }),
  adminDeleteProvider: (id) => request(`/admin/providers/${id}`, { method: 'DELETE' }),
  adminGetUsers: () => request('/admin/users'),
  adminUpdateUserRole: (id, role) => request(`/admin/users/${id}/role`, {
    method: 'PUT', body: JSON.stringify({ role })
  }),
  adminGetStats: () => request('/admin/stats'),
}

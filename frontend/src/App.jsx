import { useState, useEffect, createContext, useContext } from 'react'
import { Routes, Route, Link, useNavigate, useLocation, Navigate } from 'react-router-dom'
import { api } from './api'
import LoginPage from './pages/LoginPage'
import AccountsPage from './pages/AccountsPage'
import AdminPage from './pages/AdminPage'

const AuthContext = createContext(null)
export const useAuth = () => useContext(AuthContext)

function App() {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()
  const location = useLocation()

  useEffect(() => {
    // Check URL params first (from OAuth callback redirect)
    let token = new URLSearchParams(location.search).get('token')
    if (token) {
      api.setToken(token)
      // Clean token from URL without reload
      window.history.replaceState({}, '', location.pathname)
    }

    // Fallback to localStorage
    if (!token) {
      token = api.getToken()
    }

    if (token) {
      api.getMe()
        .then(u => setUser(u))
        .catch(() => { api.clearToken() })
        .finally(() => setLoading(false))
    } else {
      setLoading(false)
    }
  }, [location.pathname, location.search])

  const logout = () => {
    api.logout().finally(() => {
      setUser(null)
      navigate('/')
    })
  }

  if (loading) {
    return <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}>
      <div className="text-muted">Loading...</div>
    </div>
  }

  return (
    <AuthContext.Provider value={{ user, setUser, logout }}>
      {user && (
        <nav className="nav">
          <Link to="/accounts" className="nav-brand">OAuth Client</Link>
          <div className="nav-links">
            <Link to="/accounts" className={location.pathname === '/accounts' ? 'active' : ''}>
              My Accounts
            </Link>
            {user.role === 'admin' && (
              <Link to="/admin" className={location.pathname.startsWith('/admin') ? 'active' : ''}>
                Admin
              </Link>
            )}
            <span className="text-sm text-muted" style={{ padding: '8px' }}>
              {user.display_name || user.email}
            </span>
            <button className="btn btn-outline btn-sm" onClick={logout}>Logout</button>
          </div>
        </nav>
      )}

      <Routes>
        <Route path="/" element={user ? <Navigate to="/accounts" /> : <LoginPage />} />
        <Route path="/login" element={user ? <Navigate to="/accounts" /> : <LoginPage />} />
        <Route path="/accounts" element={user ? <AccountsPage /> : <Navigate to="/login" />} />
        <Route path="/admin" element={
          user?.role === 'admin' ? <AdminPage /> : <Navigate to="/" />
        } />
        <Route path="/admin/*" element={
          user?.role === 'admin' ? <AdminPage /> : <Navigate to="/" />
        } />
        <Route path="*" element={<Navigate to="/" />} />
      </Routes>
    </AuthContext.Provider>
  )
}

export default App

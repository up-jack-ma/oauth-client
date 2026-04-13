import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import { useAuth } from '../App'

const PROVIDER_ICONS = {
  github: 'GH',
  google: 'G',
  gitlab: 'GL',
  discord: 'DC',
  twitter: 'TW',
  facebook: 'FB',
  apple: 'AP',
  microsoft: 'MS',
  slack: 'SL',
  linkedin: 'IN',
}

export default function LoginPage() {
  const [providers, setProviders] = useState([])
  const [mode, setMode] = useState('login') // login | register
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { setUser } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    api.getProviders().then(setProviders).catch(() => {})
  }, [])

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const data = mode === 'login'
        ? await api.login(email, password)
        : await api.register(email, password, name)
      setUser(data.user)
      navigate('/accounts')
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleOAuth = (provider) => {
    window.location.href = `/api/auth/${provider.name}?redirect=/accounts`
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <h1>{mode === 'login' ? 'Welcome Back' : 'Create Account'}</h1>
        <p className="subtitle">
          {mode === 'login'
            ? 'Sign in to manage your linked accounts'
            : 'Register a new account'}
        </p>

        {error && <div className="alert alert-error">{error}</div>}

        {providers.length > 0 && (
          <>
            {providers.map(p => (
              <button key={p.id} className="btn-oauth" onClick={() => handleOAuth(p)}>
                <span className="icon">{p.icon || PROVIDER_ICONS[p.name] || p.name[0].toUpperCase()}</span>
                <span>Continue with {p.display_name}</span>
              </button>
            ))}
            <div className="divider">or</div>
          </>
        )}

        <form onSubmit={handleSubmit}>
          {mode === 'register' && (
            <div className="form-group">
              <label>Name</label>
              <input type="text" value={name} onChange={e => setName(e.target.value)}
                placeholder="Your name" />
            </div>
          )}
          <div className="form-group">
            <label>Email</label>
            <input type="email" value={email} onChange={e => setEmail(e.target.value)}
              placeholder="you@example.com" required />
          </div>
          <div className="form-group">
            <label>Password</label>
            <input type="password" value={password} onChange={e => setPassword(e.target.value)}
              placeholder="••••••" required minLength={6} />
          </div>
          <button type="submit" className="btn btn-primary" style={{ width: '100%' }}
            disabled={loading}>
            {loading ? 'Please wait...' : (mode === 'login' ? 'Sign In' : 'Create Account')}
          </button>
        </form>

        <p className="mt-4" style={{ textAlign: 'center', fontSize: '14px' }}>
          {mode === 'login' ? (
            <>Don't have an account? <a href="#" onClick={e => { e.preventDefault(); setMode('register'); setError('') }}>Register</a></>
          ) : (
            <>Already have an account? <a href="#" onClick={e => { e.preventDefault(); setMode('login'); setError('') }}>Sign in</a></>
          )}
        </p>
      </div>
    </div>
  )
}

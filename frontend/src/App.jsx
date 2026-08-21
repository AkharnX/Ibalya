import { useCallback, useEffect, useState } from 'react'
import { NavLink, Route, Routes, useLocation } from 'react-router-dom'
import { api, AuthError, getToken, setToken } from './api'
import Synthese from './pages/Synthese'
import Suivi from './pages/Suivi'
import AValider from './pages/AValider'
import Alertes from './pages/Alertes'
import Agent from './pages/Agent'
import Reglages from './pages/Reglages'

function Toaster() {
  const [state, setState] = useState(null)
  useEffect(() => {
    let timer
    const onToast = (e) => {
      setState(e.detail)
      clearTimeout(timer)
      timer = setTimeout(() => setState(null), 3200)
    }
    window.addEventListener('ibalya:toast', onToast)
    return () => { window.removeEventListener('ibalya:toast', onToast); clearTimeout(timer) }
  }, [])
  if (!state) return null
  return <div className={'toast' + (state.isError ? ' error-toast' : '')}>{state.message}</div>
}

const Logo = () => (
  <div className="logo"><div className="logo-mark">IB</div><span>Ibalya</span></div>
)

function Login({ error, onSubmit }) {
  const [value, setValue] = useState('')
  return (
    <div className="login">
      <div className="login-box">
        <Logo />
        <p>Entrez votre jeton d'accès administrateur.</p>
        <input type="password" placeholder="Jeton d'accès" value={value} autoFocus
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && onSubmit(value.trim())} />
        <button className="btn primary" onClick={() => onSubmit(value.trim())}>Se connecter</button>
        {error && <p className="error">{error}</p>}
      </div>
    </div>
  )
}

// Navigation groupée, reprise du redesign : Opérations / Extraction / Système.
const NAV = [
  ['Opérations', [
    ['/', 'Synthèse', null],
    ['/a-valider', 'À valider', 'messages_a_valider'],
  ]],
  ['Extraction', [
    ['/suivi', 'Engagements', null],
    ['/alertes', 'Alertes', 'alertes'],
  ]],
  ['Système', [
    ['/agent', 'Règles métier', null],
    ['/reglages', 'Réglages', null],
  ]],
]

const TITRES = {
  '/': 'Synthèse', '/a-valider': 'À valider', '/suivi': 'Engagements',
  '/alertes': 'Alertes', '/agent': 'Règles métier', '/reglages': 'Réglages',
}

export default function App() {
  const [authed, setAuthed] = useState(null)
  const [loginError, setLoginError] = useState('')
  const [status, setStatus] = useState(null)
  const [counts, setCounts] = useState({})
  const [dark, setDark] = useState(() => localStorage.getItem('ibalya_theme') !== 'light')
  const [menuOpen, setMenuOpen] = useState(false)
  const { pathname } = useLocation()

  useEffect(() => {
    document.documentElement.classList.toggle('light', !dark)
    localStorage.setItem('ibalya_theme', dark ? 'dark' : 'light')
  }, [dark])

  const check = useCallback(() => {
    if (!getToken()) { setAuthed(false); return }
    api('/status')
      .then((st) => { setStatus(st); setAuthed(true) })
      .catch((e) => {
        setAuthed(false)
        setLoginError(e instanceof AuthError ? 'Jeton invalide.' : e.message)
      })
  }, [])
  useEffect(() => { check() }, [check])

  // compteurs de la navigation, rafraîchis à chaque changement de page
  const refreshCounts = useCallback(() => {
    if (!getToken()) return
    api('/status').then((st) => {
      setStatus(st)
      setCounts({
        messages_a_valider: st.compteurs?.brouillons_proposes || 0,
        alertes: st.compteurs?.detections_actives || 0,
      })
    }).catch(() => {})
  }, [])
  useEffect(() => { if (authed) refreshCounts() }, [authed, pathname, refreshCounts])
  useEffect(() => { setMenuOpen(false) }, [pathname])

  if (authed === null) return null
  if (!authed) return <Login error={loginError} onSubmit={(t) => { setToken(t); setLoginError(''); check() }} />

  const agentOk = status?.canal_connecte && status?.service_llm_ok

  return (
    <div className="shell">
      <aside className={'sidebar' + (menuOpen ? ' open' : '')}>
        <div className="sidebar-head"><Logo /></div>
        <nav className="sidebar-nav">
          {NAV.map(([groupe, items]) => (
            <div key={groupe}>
              <div className="nav-group">{groupe}</div>
              {items.map(([path, label, countKey]) => (
                <NavLink key={path} to={path} end={path === '/'}
                  className={({ isActive }) => 'nav-item' + (isActive ? ' active' : '')}>
                  <span>{label}</span>
                  {countKey && counts[countKey] > 0 && <span className="nav-count">{counts[countKey]}</span>}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <div className="sidebar-foot">
          <div className="sidebar-user">
            <b>{status?.compte ? status.compte.split('@')[0] : 'Dirigeant'}</b>
            <span>{status?.compte || status?.canal || '—'}</span>
          </div>
          <button className="icon-btn" title={dark ? 'Thème clair' : 'Thème sombre'}
            onClick={() => setDark(!dark)}>{dark ? '☀' : '☾'}</button>
        </div>
      </aside>

      <div className="content">
        <header className="topbar">
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <button className="icon-btn burger" onClick={() => setMenuOpen(!menuOpen)}>☰</button>
            <h1>{TITRES[pathname] || 'Ibalya'}</h1>
          </div>
          <div className={'agent-live' + (agentOk ? '' : ' off')}>
            <i />{agentOk ? 'Agent actif' : (status?.canal_connecte ? 'LLM injoignable' : 'Canal non connecté')}
          </div>
        </header>
        <main>
          <Routes>
            <Route path="/" element={<Synthese />} />
            <Route path="/a-valider" element={<AValider />} />
            <Route path="/suivi" element={<Suivi />} />
            <Route path="/alertes" element={<Alertes />} />
            <Route path="/agent" element={<Agent />} />
            <Route path="/reglages" element={<Reglages />} />
            <Route path="*" element={<Synthese />} />
          </Routes>
        </main>
      </div>
      <Toaster />
    </div>
  )
}

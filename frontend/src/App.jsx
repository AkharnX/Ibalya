import { useCallback, useEffect, useState } from 'react'
import { NavLink, Route, Routes, useLocation } from 'react-router-dom'
import { api, AuthError, login as apiLogin, logout as apiLogout, toast } from './api'
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

// Symbole Ibalya : trois pièces formant un cadre. currentColor pour qu'il
// suive la couleur du thème, sur la barre latérale comme sur l'écran de connexion.
const Symbole = ({ className = 'logo-mark' }) => (
  <svg className={className} viewBox="0 0 100 100" fill="currentColor" aria-hidden="true">
    <path d="M0 0h64v24H0z" />
    <path d="M76 0h24v64H76z" />
    <path d="M0 36h24v40h76v24H0z" />
  </svg>
)

const Logo = () => (
  <div className="logo"><Symbole /><span>Ibalya</span></div>
)

function Login({ onConnecte }) {
  // Un échec de connexion Google revient ici via ?erreur= (redirection serveur).
  const [erreurGoogle] = useState(() => {
    const p = new URLSearchParams(window.location.search).get('erreur')
    if (p) window.history.replaceState({}, '', window.location.pathname)
    return p || ''
  })
  const [email, setEmail] = useState('')
  const [motDePasse, setMotDePasse] = useState('')
  const [erreur, setErreur] = useState('')
  const [busy, setBusy] = useState(false)

  const soumettre = async (e) => {
    e?.preventDefault()
    if (busy || !email || !motDePasse) return
    setBusy(true); setErreur('')
    try {
      const u = await apiLogin(email.trim(), motDePasse)
      onConnecte(u)
    } catch (err) {
      setErreur(err.message === 'Session expirée' ? 'Identifiants incorrects' : err.message)
      setMotDePasse('')
    } finally { setBusy(false) }
  }

  return (
    <div className="login">
      <form className="login-box" onSubmit={soumettre}>
        <Logo />
        <p>Connectez-vous pour accéder au suivi de vos engagements.</p>
        <label htmlFor="email">Adresse email</label>
        <input id="email" type="email" autoComplete="username" autoFocus
          value={email} onChange={(e) => setEmail(e.target.value)} />
        <label htmlFor="mdp">Mot de passe</label>
        <input id="mdp" type="password" autoComplete="current-password"
          value={motDePasse} onChange={(e) => setMotDePasse(e.target.value)} />
        <button className="btn primary" type="submit" disabled={busy}>
          {busy ? 'Connexion…' : 'Se connecter'}
        </button>
        {(erreur || erreurGoogle) && <p className="error">{erreur || erreurGoogle}</p>}

        <div className="separateur"><span>ou</span></div>

        <a className="btn btn-google" href="/api/oauth/google/login">
          <svg viewBox="0 0 18 18" width="16" height="16" aria-hidden="true">
            <path fill="#4285F4" d="M17.64 9.2c0-.64-.06-1.25-.16-1.84H9v3.48h4.84a4.14 4.14 0 0 1-1.8 2.72v2.26h2.92c1.7-1.57 2.68-3.88 2.68-6.62z"/>
            <path fill="#34A853" d="M9 18c2.43 0 4.47-.8 5.96-2.18l-2.92-2.26c-.8.54-1.84.86-3.04.86-2.34 0-4.32-1.58-5.03-3.7H.96v2.33A9 9 0 0 0 9 18z"/>
            <path fill="#FBBC05" d="M3.97 10.72a5.4 5.4 0 0 1 0-3.44V4.95H.96a9 9 0 0 0 0 8.1l3.01-2.33z"/>
            <path fill="#EA4335" d="M9 3.58c1.32 0 2.5.45 3.44 1.35l2.58-2.58C13.46.9 11.43 0 9 0A9 9 0 0 0 .96 4.95l3.01 2.33C4.68 5.16 6.66 3.58 9 3.58z"/>
          </svg>
          Continuer avec Google
        </a>
        <p className="login-note">
          La connexion Google est réservée aux adresses déjà autorisées : elle ne crée aucun compte.
        </p>
      </form>
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
  const [user, setUser] = useState(null)
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
    api('/me')
      .then((u) => { setUser(u); setAuthed(true) })
      .catch((e) => {
        setAuthed(false)
        if (!(e instanceof AuthError)) setLoginError(e.message)
      })
  }, [])
  useEffect(() => { check() }, [check])

  // compteurs de la navigation, rafraîchis à chaque changement de page
  const refreshCounts = useCallback(() => {
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
  if (!authed) return <Login onConnecte={(u) => { setUser(u); setLoginError(''); setAuthed(true) }} />

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
            <b>{user?.nom || user?.email || '—'}</b>
            <span>{user?.email}</span>
          </div>
          <div style={{ display: 'flex', gap: 2 }}>
            <button className="icon-btn" title={dark ? 'Thème clair' : 'Thème sombre'}
              onClick={() => setDark(!dark)}>{dark ? '☀' : '☾'}</button>
            <button className="icon-btn" title="Se déconnecter" onClick={async () => {
              try { await apiLogout() } catch { /* session déjà close */ }
              setAuthed(false); setUser(null)
            }}>⏻</button>
          </div>
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

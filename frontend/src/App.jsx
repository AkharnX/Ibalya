import { useCallback, useEffect, useState } from 'react'
import { NavLink, Route, Routes, useLocation } from 'react-router-dom'
import { api, AuthError, login as apiLogin, logout as apiLogout, toast } from './api'
import { FournisseurEtatAgent, libelleCycle, useEtatAgent } from './etatAgent'
import Synthese from './pages/Synthese'
import Miroir from './pages/Miroir'
import Digest from './pages/Digest'
import Suivi from './pages/Suivi'
import Liens from './pages/Liens'
import Interlocuteurs from './pages/Interlocuteurs'
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
    ['/miroir', 'Miroir d’activité', null],
    ['/digest', 'Digest', null],
    ['/a-valider', 'À valider', 'messages_a_valider'],
  ]],
  ['Extraction', [
    ['/suivi', 'Engagements', null],
    ['/alertes', 'Alertes', 'alertes'],
    ['/liens', 'Dépendances', 'liens_a_confirmer'],
  ]],
  ['Système', [
    ['/agent', 'Règles métier', null],
    ['/interlocuteurs', 'Interlocuteurs', null],
    ['/reglages', 'Réglages', null],
  ]],
]

const TITRES = {
  '/': 'Synthèse', '/miroir': 'Miroir d’activité', '/digest': 'Digest',
  '/a-valider': 'À valider', '/suivi': 'Engagements', '/alertes': 'Alertes',
  '/liens': 'Dépendances', '/agent': 'Règles métier',
  '/interlocuteurs': 'Interlocuteurs', '/reglages': 'Réglages',
}

// Indicateur d'activité de la barre du haut. Il annonçait seulement que
// l'agent était joignable ; il montre maintenant ce qu'il fait, y compris
// quand le cycle a été lancé par le scheduler et non par le dirigeant.
function IndicateurAgent() {
  const { statut, cycle } = useEtatAgent()
  const ok = statut?.canal_connecte && statut?.service_llm_ok

  if (cycle?.en_cours) {
    return (
      <div className="agent-live travaille" title={cycle.origine === 'dirigeant'
        ? 'Analyse que vous avez lancée' : 'Analyse automatique, toutes les 15 minutes'}>
        <span className="rotor" aria-hidden="true" />
        <span>{libelleCycle(cycle)}</span>
      </div>
    )
  }
  return (
    <div className={'agent-live' + (ok ? '' : ' off')}>
      <i />{ok ? 'Agent actif' : (statut?.canal_connecte ? 'LLM injoignable' : 'Canal non connecté')}
    </div>
  )
}

// Pastille de la navigation. Elle lit le sondage partagé plutôt que de
// déclencher sa propre requête à chaque changement de page.
function PastilleNav({ cle }) {
  const { statut } = useEtatAgent()
  const c = statut?.compteurs || {}
  const valeurs = {
    messages_a_valider: c.brouillons_proposes || 0,
    alertes: c.detections_actives || 0,
    liens_a_confirmer: c.liens_a_confirmer || 0,
  }
  const n = valeurs[cle] || 0
  return n > 0 ? <span className="nav-count">{n}</span> : null
}

export default function App() {
  const [authed, setAuthed] = useState(null)
  const [loginError, setLoginError] = useState('')
  const [user, setUser] = useState(null)
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

  useEffect(() => {
    document.title = TITRES[pathname] ? `${TITRES[pathname]} · Ibalya` : 'Ibalya'
  }, [pathname])
  useEffect(() => { setMenuOpen(false) }, [pathname])

  if (authed === null) return null
  if (!authed) return <Login onConnecte={(u) => { setUser(u); setLoginError(''); setAuthed(true) }} />

  return (
    <FournisseurEtatAgent actif={authed}>
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
                  {countKey && <PastilleNav cle={countKey} />}
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
          <div className="topbar-gauche">
            <button className="icon-btn burger" onClick={() => setMenuOpen(!menuOpen)}>☰</button>
            {/* La marque n'apparaît ici qu'en petite largeur, là où la barre
                latérale est escamotée : sur grand écran le logo y est déjà. */}
            <Logo />
          </div>
          <IndicateurAgent />
        </header>
        <main>
          <Routes>
            <Route path="/" element={<Synthese />} />
            <Route path="/miroir" element={<Miroir />} />
            <Route path="/digest" element={<Digest />} />
            <Route path="/a-valider" element={<AValider />} />
            <Route path="/suivi" element={<Suivi />} />
            <Route path="/liens" element={<Liens />} />
            <Route path="/interlocuteurs" element={<Interlocuteurs />} />
            <Route path="/alertes" element={<Alertes />} />
            <Route path="/agent" element={<Agent />} />
            <Route path="/reglages" element={<Reglages />} />
            <Route path="*" element={<Synthese />} />
          </Routes>
        </main>
      </div>
      <Toaster />
    </div>
    </FournisseurEtatAgent>
  )
}

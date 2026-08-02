import { useEffect, useState } from 'react'
import { NavLink, Route, Routes } from 'react-router-dom'
import { api, AuthError, getToken, setToken } from './api'
import Pilotage from './pages/Pilotage'
import Engagements from './pages/Engagements'
import Alertes from './pages/Alertes'
import Miroir from './pages/Miroir'
import Agent from './pages/Agent'
import Reglages from './pages/Reglages'

function Toaster() {
  const [state, setState] = useState(null)
  useEffect(() => {
    let timer
    const onToast = (e) => {
      setState(e.detail)
      clearTimeout(timer)
      timer = setTimeout(() => setState(null), 4000)
    }
    window.addEventListener('agentops:toast', onToast)
    return () => { window.removeEventListener('agentops:toast', onToast); clearTimeout(timer) }
  }, [])
  if (!state) return null
  return <div className={'toast' + (state.isError ? ' error-toast' : '')}>{state.message}</div>
}

function Login({ error, onSubmit }) {
  const [value, setValue] = useState('')
  return (
    <div className="login">
      <div className="login-box">
        <h1>Agent<span className="accent">Ops</span></h1>
        <p>Entrez votre jeton d'accès administrateur.</p>
        <input
          type="password" placeholder="Jeton d'accès" value={value} autoFocus
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && onSubmit(value.trim())}
        />
        <button className="primary" onClick={() => onSubmit(value.trim())}>Se connecter</button>
        {error && <p className="error">{error}</p>}
      </div>
    </div>
  )
}

function StatusPill() {
  const [st, setSt] = useState(null)
  useEffect(() => {
    const load = () => api('/status').then(setSt).catch(() => {})
    load()
    const t = setInterval(load, 60_000)
    return () => clearInterval(t)
  }, [])
  if (!st) return <div className="pill">…</div>
  const ok = st.canal_connecte && st.service_llm_ok
  return (
    <div className={'pill ' + (ok ? 'ok' : 'warn')}>
      {(st.canal_connecte ? '● ' : '○ ') + st.canal}
      {st.compte ? ' · ' + st.compte : ''}
      {st.service_llm_ok ? '' : ' · LLM injoignable'}
    </div>
  )
}

const PAGES = [
  ['/', 'Pilotage'],
  ['/engagements', 'Engagements'],
  ['/alertes', 'Alertes'],
  ['/miroir', 'Miroir'],
  ['/agent', 'Agent'],
  ['/reglages', 'Réglages'],
]

export default function App() {
  const [authed, setAuthed] = useState(null) // null = vérification en cours
  const [loginError, setLoginError] = useState('')

  const check = () => {
    if (!getToken()) { setAuthed(false); return }
    api('/status')
      .then(() => setAuthed(true))
      .catch((e) => {
        setAuthed(false)
        setLoginError(e instanceof AuthError ? 'Jeton invalide.' : e.message)
      })
  }
  useEffect(check, [])

  if (authed === null) return null
  if (!authed) return <Login error={loginError} onSubmit={(t) => { setToken(t); setLoginError(''); check() }} />

  return (
    <>
      <header>
        <h1>Agent<span className="accent">Ops</span></h1>
        <nav>
          {PAGES.map(([path, label]) => (
            <NavLink key={path} to={path} end={path === '/'}>{label}</NavLink>
          ))}
        </nav>
        <StatusPill />
      </header>
      <main>
        <Routes>
          <Route path="/" element={<Pilotage />} />
          <Route path="/engagements" element={<Engagements />} />
          <Route path="/alertes" element={<Alertes />} />
          <Route path="/miroir" element={<Miroir />} />
          <Route path="/agent" element={<Agent />} />
          <Route path="/reglages" element={<Reglages />} />
          <Route path="*" element={<Pilotage />} />
        </Routes>
      </main>
      <Toaster />
    </>
  )
}

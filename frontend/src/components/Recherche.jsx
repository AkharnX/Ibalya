// Recherche transverse depuis la barre du haut.
//
// Avec cent cinquante interlocuteurs et six cents messages, retrouver un
// dossier supposait de deviner dans quelle page chercher : les filtres de
// chaque écran ne portent que sur ce que l'écran affiche déjà.
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import Icone from './Icone'

const LIBELLE = { engagement: 'Engagement', interlocuteur: 'Interlocuteur', conversation: 'Conversation' }

export default function Recherche() {
  const [q, setQ] = useState('')
  const [res, setRes] = useState([])
  const [ouvert, setOuvert] = useState(false)
  const [actif, setActif] = useState(0)
  const zone = useRef(null)
  const navigate = useNavigate()

  // Un appel par frappe saturerait le serveur pour rien : on attend une pause.
  useEffect(() => {
    if (q.trim().length < 3) { setRes([]); return }
    const t = setTimeout(() => {
      api(`/recherche?q=${encodeURIComponent(q.trim())}`)
        .then((r) => { setRes(r || []); setActif(0); setOuvert(true) })
        .catch(() => setRes([]))
    }, 220)
    return () => clearTimeout(t)
  }, [q])

  useEffect(() => {
    const ailleurs = (e) => { if (zone.current && !zone.current.contains(e.target)) setOuvert(false) }
    document.addEventListener('mousedown', ailleurs)
    return () => document.removeEventListener('mousedown', ailleurs)
  }, [])

  const aller = useCallback((r) => {
    setOuvert(false); setQ('')
    if (r.type === 'engagement') navigate(`/suivi?ouvrir=${r.id}`)
    else if (r.type === 'interlocuteur') navigate(`/interlocuteurs?fiche=${r.id}`)
    else navigate(`/miroir?fil=${r.id}`)
  }, [navigate])

  const clavier = (e) => {
    if (!ouvert || !res.length) return
    if (e.key === 'ArrowDown') { e.preventDefault(); setActif((i) => (i + 1) % res.length) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setActif((i) => (i - 1 + res.length) % res.length) }
    else if (e.key === 'Enter') { e.preventDefault(); aller(res[actif]) }
    else if (e.key === 'Escape') setOuvert(false)
  }

  return (
    <div className="recherche" ref={zone}>
      <Icone nom="action-rechercher" />
      <input type="search" value={q} placeholder="Rechercher un client, un engagement…"
        onChange={(e) => setQ(e.target.value)} onFocus={() => res.length && setOuvert(true)}
        onKeyDown={clavier} aria-label="Recherche" />
      {ouvert && q.trim().length >= 3 && (
        <div className="recherche-resultats" role="listbox">
          {!res.length ? (
            <p className="recherche-vide">Aucun résultat pour « {q.trim()} ».</p>
          ) : res.map((r, i) => (
            <button key={r.type + r.id} role="option" aria-selected={i === actif}
              className={'recherche-item' + (i === actif ? ' actif' : '')}
              onMouseEnter={() => setActif(i)} onClick={() => aller(r)}>
              <span className="recherche-type">{LIBELLE[r.type]}</span>
              <span className="recherche-titre">{r.titre}</span>
              {r.detail && <span className="recherche-detail">{r.detail}</span>}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

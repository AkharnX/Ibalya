// Recherche transverse depuis la barre du haut.
//
// Avec cent cinquante interlocuteurs et six cents messages, retrouver un
// dossier supposait de deviner dans quelle page chercher : les filtres de
// chaque écran ne portent que sur ce que l'écran affiche déjà.
//
// Deux modes : « Tout » (engagements, interlocuteurs, conversations) et
// « Pièces jointes » (nom, contenu, expéditeur, objet des documents reçus,
// robots compris). Un résultat de pièce renvoie au mail dans la messagerie :
// on ne garde jamais le fichier, l'utilisateur le télécharge là-bas.
import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import Icone from './Icone'

const LIBELLE = { engagement: 'Engagement', interlocuteur: 'Interlocuteur', conversation: 'Conversation' }
const CLE_MODE = 'recherche-mode'

function modeInitial() {
  try { return sessionStorage.getItem(CLE_MODE) === 'pieces' ? 'pieces' : 'tout' } catch { return 'tout' }
}

const fmtJour = (s) => {
  const d = new Date(s)
  return isNaN(d) ? '' : d.toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit' })
}

// Libellé du geste d'ouverture : la messagerie quand on a un lien, le fil sinon.
function ouverture(p) {
  if (!p.url_web) return 'Voir le fil'
  if (p.url_web.includes('mail.google.com')) return 'Ouvrir dans Gmail'
  if (p.url_web.includes('outlook')) return 'Ouvrir dans Outlook'
  return 'Ouvrir dans la messagerie'
}

export default function Recherche() {
  const [q, setQ] = useState('')
  const [mode, setMode] = useState(modeInitial)
  const [res, setRes] = useState([])
  const [ouvert, setOuvert] = useState(false)
  const [actif, setActif] = useState(0)
  const [canal, setCanal] = useState(null)
  const zone = useRef(null)
  const navigate = useNavigate()

  const changerMode = (m) => {
    setMode(m)
    try { sessionStorage.setItem(CLE_MODE, m) } catch { /* stockage indisponible : le mode vit le temps de la page */ }
  }

  // Le type de boîte sert au message vide : Outlook n'a pas encore la lecture
  // des pièces jointes, et il faut le dire plutôt qu'afficher « aucun résultat ».
  useEffect(() => {
    if (mode !== 'pieces' || canal) return
    api('/canal').then((c) => setCanal(c || {})).catch(() => setCanal({}))
  }, [mode, canal])

  // Un appel par frappe saturerait le serveur pour rien : on attend une pause.
  useEffect(() => {
    if (q.trim().length < 3) { setRes([]); return }
    const chemin = mode === 'pieces' ? '/recherche/pieces-jointes' : '/recherche'
    const t = setTimeout(() => {
      api(`${chemin}?q=${encodeURIComponent(q.trim())}`)
        .then((r) => { setRes(r || []); setActif(0); setOuvert(true) })
        .catch(() => setRes([]))
    }, 220)
    return () => clearTimeout(t)
  }, [q, mode])

  useEffect(() => {
    const ailleurs = (e) => { if (zone.current && !zone.current.contains(e.target)) setOuvert(false) }
    document.addEventListener('mousedown', ailleurs)
    return () => document.removeEventListener('mousedown', ailleurs)
  }, [])

  const aller = useCallback((r) => {
    setOuvert(false); setQ('')
    if (mode === 'pieces') {
      if (r.url_web) window.open(r.url_web, '_blank', 'noopener')
      else navigate(`/miroir?fil=${r.fil_id}`)
      return
    }
    if (r.type === 'engagement') navigate(`/suivi?ouvrir=${r.id}`)
    else if (r.type === 'interlocuteur') navigate(`/interlocuteurs?fiche=${r.id}`)
    else navigate(`/miroir?fil=${r.id}`)
  }, [navigate, mode])

  const clavier = (e) => {
    if (!ouvert || !res.length) return
    if (e.key === 'ArrowDown') { e.preventDefault(); setActif((i) => (i + 1) % res.length) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setActif((i) => (i - 1 + res.length) % res.length) }
    else if (e.key === 'Enter') { e.preventDefault(); aller(res[actif]) }
    else if (e.key === 'Escape') setOuvert(false)
  }

  const texteVide = mode === 'pieces'
    ? (canal?.type === 'outlook'
      ? 'Les pièces jointes des boîtes Outlook ne sont pas encore lues.'
      : `Aucune pièce jointe pour « ${q.trim()} ».`)
    : `Aucun résultat pour « ${q.trim()} ».`

  return (
    <div className="recherche" ref={zone}>
      <Icone nom="action-rechercher" />
      <input type="search" value={q} placeholder="Rechercher un client, un engagement, une pièce jointe…"
        onChange={(e) => setQ(e.target.value)} onFocus={() => (res.length || q.trim().length >= 3) && setOuvert(true)}
        onKeyDown={clavier} aria-label="Recherche" />
      {ouvert && q.trim().length >= 3 && (
        <div className="recherche-resultats">
          <div className="recherche-modes" role="group" aria-label="Chercher dans">
            <button type="button" className="recherche-chip" aria-pressed={mode === 'tout'} onClick={() => changerMode('tout')}>Tout</button>
            <button type="button" className="recherche-chip" aria-pressed={mode === 'pieces'} onClick={() => changerMode('pieces')}>Pièces jointes</button>
          </div>
          <div role="listbox">
            {!res.length ? (
              <p className="recherche-vide">{texteVide}</p>
            ) : mode === 'pieces' ? res.map((p, i) => (
              <button key={'pj' + p.id} role="option" aria-selected={i === actif}
                className={'recherche-item recherche-pj' + (i === actif ? ' actif' : '')}
                onMouseEnter={() => setActif(i)} onClick={() => aller(p)}>
                <span className="recherche-type">{p.type_libelle || 'Fichier'}</span>
                <span className="recherche-titre">{p.nom}</span>
                <span className="recherche-date">{fmtJour(p.date)}</span>
                <span className="recherche-detail">{[p.expediteur, p.objet].filter(Boolean).join(' · ')}</span>
                {p.extrait && <span className="recherche-extrait">« {p.extrait} »</span>}
                <span className="recherche-ouvrir">{ouverture(p)}</span>
              </button>
            )) : res.map((r, i) => (
              <button key={r.type + r.id} role="option" aria-selected={i === actif}
                className={'recherche-item' + (i === actif ? ' actif' : '')}
                onMouseEnter={() => setActif(i)} onClick={() => aller(r)}>
                <span className="recherche-type">{LIBELLE[r.type]}</span>
                <span className="recherche-titre">{r.titre}</span>
                {r.detail && <span className="recherche-detail">{r.detail}</span>}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

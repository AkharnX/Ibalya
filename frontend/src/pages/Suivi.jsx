import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, toast } from '../api'
import { DraftPanel, useDraft } from '../components/DraftPanel'
import SourcePanel from '../components/SourcePanel'
import { TYPE_LABELS, fmtDate } from '../components/ui'
import { SqueletteTable } from '../components/Squelette'

const CATEGORIES = [
  ['all', 'Tous'],
  ['encours', 'Engagements en cours'],
  ['retard', 'Retards probables'],
  ['risque', 'À risque'],
]
const TYPES = ['all', 'livraison', 'devis', 'facturation', 'rendez_vous', 'prise_de_contact']

// Corrections d'un geste (CDC 11) : chaque geste devient une règle explicite,
// lisible et révocable depuis « Règles métier ». Pas de boîte noire.
const CORRECTIONS = [
  ['pas_un_engagement', 'Ce n’est pas un engagement'],
  ['priorite_haute', 'Priorité haute pour cet interlocuteur'],
  ['ignorer_interlocuteur', 'Ne plus rien extraire de cet interlocuteur'],
  ['ne_plus_alerter', 'Ne plus m’alerter sur ce fil'],
]

export default function Suivi() {
  const [rows, setRows] = useState(null)
  const [params, setParams] = useSearchParams()
  const [type, setType] = useState('all')
  const [search, setSearch] = useState('')
  const cat = params.get('cat') || 'all'

  const load = useCallback(() => {
    api('/suivi').then((r) => setRows(r || [])).catch((e) => { setRows([]); toast(e.message, true) })
  }, [])
  useEffect(load, [load])

  const d = useDraft(load)
  const [sourceId, setSourceId] = useState(null)
  const [menuId, setMenuId] = useState(null)      // menu de correction ouvert
  const [menuPos, setMenuPos] = useState(null)    // ancrage écran du menu
  const [dateId, setDateId] = useState(null)      // échéance en cours de confirmation
  const [dateVal, setDateVal] = useState('')
  const zone = useRef(null)

  // Le tableau défile horizontalement (.tbl-wrap est en overflow-x:auto), ce qui
  // force le navigateur à rogner aussi la verticale : un menu en position absolue
  // y était coupé net. On l'ancre donc à l'écran, à partir de la position du
  // bouton — et on le referme dès que cette position n'est plus valable.
  const ouvrirMenu = (e, id) => {
    if (menuId === id) { setMenuId(null); return }
    const r = e.currentTarget.getBoundingClientRect()
    setMenuPos({ top: r.bottom + 4, right: Math.max(8, window.innerWidth - r.right) })
    setMenuId(id)
  }

  useEffect(() => {
    if (menuId === null) return
    const ailleurs = (e) => { if (zone.current && !zone.current.contains(e.target)) setMenuId(null) }
    const fermer = () => setMenuId(null)
    document.addEventListener('mousedown', ailleurs)
    window.addEventListener('resize', fermer)
    window.addEventListener('scroll', fermer, true) // true : capte aussi le défilement du tableau
    return () => {
      document.removeEventListener('mousedown', ailleurs)
      window.removeEventListener('resize', fermer)
      window.removeEventListener('scroll', fermer, true)
    }
  }, [menuId])

  const counts = useMemo(() => {
    const c = { all: (rows || []).length }
    ;(rows || []).forEach((r) => { c[r.categorie] = (c[r.categorie] || 0) + 1 })
    return c
  }, [rows])

  const shown = (rows || []).filter((r) => {
    const okCat = cat === 'all' || r.categorie === cat
    const okType = type === 'all' || r.type === type
    const q = search.trim().toLowerCase()
    const okSearch = !q || [r.objet, r.contact, r.emetteur_email, r.destinataire_email]
      .filter(Boolean).some((v) => v.toLowerCase().includes(q))
    return okCat && okType && okSearch
  })

  const patch = async (id, body, msg) => {
    try { await api(`/engagements/${id}`, { method: 'PATCH', body: JSON.stringify(body) }); toast(msg); load() }
    catch (e) { toast(e.message, true) }
  }
  const correct = async (id, action) => {
    setMenuId(null)
    try {
      const r = await api(`/engagements/${id}/correct`, { method: 'POST', body: JSON.stringify({ action }) })
      toast(r.regle ? 'Règle apprise : ' + r.regle : 'Correction enregistrée'); load()
    } catch (e) { toast(e.message, true) }
  }

  // Une échéance inférée par le modèle n'alimente les détecteurs qu'une fois
  // confirmée (CDC 7.3) : cet écran est le seul endroit où la confirmer.
  const ouvrirDate = (r) => {
    setDateId(r.id)
    setDateVal(r.echeance ? new Date(r.echeance).toISOString().slice(0, 10) : '')
  }
  const confirmerEcheance = async (id) => {
    if (!dateVal) return
    try {
      await api(`/engagements/${id}`, { method: 'PATCH', body: JSON.stringify({ echeance: dateVal }) })
      toast('Échéance confirmée, les alertes la prennent en compte')
      setDateId(null); load()
    } catch (e) { toast(e.message, true) }
  }

  const statusClass = (r) => (r.categorie === 'retard' ? 'late' : r.categorie === 'risque' ? 'risk' : 'open')
  const statusLabel = (r) => (r.categorie === 'retard' ? 'En retard' : r.categorie === 'risque' ? 'À risque' : 'Ouvert')

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Suivi des engagements</h1>
          <p>Tout ce qui a été promis — par vous ou à vous — détecté dans vos échanges. La photographie de vos 30 derniers jours, à corriger d'un geste.</p>
        </div>
      </div>

      <div className="chip-row">
        {CATEGORIES.map(([key, label]) => (
          <div key={key} className={'chip' + (cat === key ? ' active' : '')}
            onClick={() => setParams(key === 'all' ? {} : { cat: key })}>
            {label} <span className="n">{counts[key] || 0}</span>
          </div>
        ))}
      </div>

      <div className="filter-bar">
        {TYPES.map((t) => (
          <div key={t} className={'type-pill' + (type === t ? ' active' : '')} onClick={() => setType(t)}>
            {t === 'all' ? 'Tous les types' : TYPE_LABELS[t]}
          </div>
        ))}
        <div className="search-wrap">
          <input type="text" placeholder="Rechercher un client..." value={search}
            onChange={(e) => setSearch(e.target.value)} />
        </div>
      </div>

      {rows === null ? (
        <SqueletteTable lignes={6} colonnes={6} />
      ) : !shown.length ? (
        <div className="empty">Aucun engagement ne correspond à ces filtres.</div>
      ) : (
        <div className="tbl-wrap">
          <table>
            <thead>
              <tr><th>Type</th><th>Engagement</th><th>Échéance</th><th>Fiabilité</th><th>Statut</th><th><span className="sr-only">Actions</span></th></tr>
            </thead>
            <tbody>
              {shown.map((r) => (
                <tr key={r.id}>
                  <td><span className={'badge ' + (r.type || 'autre')}>{TYPE_LABELS[r.type] || 'Autre'}</span></td>
                  <td>
                    <p className="eng-title">
                      <button className="lien-source" title="Voir la conversation d'origine"
                        onClick={() => setSourceId(r.id)}>{r.objet}</button>
                      {r.echeance && r.echeance_inferee && !r.echeance_confirmee && <span className="tag-confirm">à confirmer</span>}
                    </p>
                    <p className="eng-flow">{r.emetteur_email || '?'} → {r.destinataire_email || '?'}</p>
                    {r.blocage && <p className="eng-flow">⛓ bloqué par : {r.blocage.amont_objet}</p>}
                    {r.action && (
                      <button className="eng-action" onClick={() => d.openForEngagement(r.id, r.action)}>
                        → {r.action.label}
                      </button>
                    )}
                  </td>
                  <td>
                    {dateId === r.id ? (
                      <div className="echeance-edit">
                        <input type="date" value={dateVal} onChange={(e) => setDateVal(e.target.value)} />
                        <button className="btn-icon primary" title="Confirmer cette échéance"
                          onClick={() => confirmerEcheance(r.id)}>✓</button>
                        <button className="btn-icon" title="Annuler" onClick={() => setDateId(null)}>✕</button>
                      </div>
                    ) : r.echeance ? (
                      r.echeance_inferee && !r.echeance_confirmee ? (
                        <button className="echeance-a-confirmer" title="Échéance déduite par l’agent : à confirmer"
                          onClick={() => ouvrirDate(r)}>{fmtDate(r.echeance)}</button>
                      ) : fmtDate(r.echeance)
                    ) : <span className="mono">—</span>}
                  </td>
                  <td><div className="reli"><span style={{ width: `${Math.round(r.confiance * 100)}%` }} /></div></td>
                  <td><div className={'status ' + statusClass(r)}><span className="dot" />{statusLabel(r)}</div></td>
                  <td>
                    <div className="row-actions">
                      {r.action && (
                        <button className="btn-icon primary" title={r.action.label}
                          onClick={() => d.openForEngagement(r.id, r.action)}>✉</button>
                      )}
                      <button className="btn-icon" title="Marquer livré" onClick={() => patch(r.id, { statut: 'livre' }, 'Marqué comme livré')}>✓</button>
                      <div className="menu-wrap" ref={menuId === r.id ? zone : null}>
                        <button className="btn-icon" title="Corriger l’agent"
                          aria-haspopup="menu" aria-expanded={menuId === r.id}
                          onClick={(e) => ouvrirMenu(e, r.id)}>⋯</button>
                        {menuId === r.id && menuPos && (
                          <div className="menu-corr" role="menu" style={{ top: menuPos.top, right: menuPos.right }}>
                            <p className="menu-titre">Corriger l’agent</p>
                            {CORRECTIONS.map(([action, label]) => (
                              <button key={action} role="menuitem" onClick={() => correct(r.id, action)}>{label}</button>
                            ))}
                          </div>
                        )}
                      </div>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <DraftPanel draft={d.draft} loading={d.loading} title={d.meta.title} hint={d.meta.hint}
        onClose={d.close} onSent={d.onSent} />
      <SourcePanel engagementId={sourceId} onClose={() => setSourceId(null)} />
    </section>
  )
}

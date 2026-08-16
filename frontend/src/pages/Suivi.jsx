import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, toast } from '../api'
import { DraftPanel, useDraft } from '../components/DraftPanel'
import { TYPE_LABELS, fmtDate } from '../components/ui'

const CATEGORIES = [
  ['all', 'Tous'],
  ['encours', 'Engagements en cours'],
  ['retard', 'Retards probables'],
  ['risque', 'À risque'],
]
const TYPES = ['all', 'livraison', 'devis', 'facturation', 'rendez_vous', 'prise_de_contact']

export default function Suivi() {
  const [rows, setRows] = useState([])
  const [params, setParams] = useSearchParams()
  const [type, setType] = useState('all')
  const [search, setSearch] = useState('')
  const cat = params.get('cat') || 'all'

  const load = useCallback(() => {
    api('/suivi').then((r) => setRows(r || [])).catch((e) => toast(e.message, true))
  }, [])
  useEffect(load, [load])

  const d = useDraft(load)

  const counts = useMemo(() => {
    const c = { all: rows.length }
    rows.forEach((r) => { c[r.categorie] = (c[r.categorie] || 0) + 1 })
    return c
  }, [rows])

  const shown = rows.filter((r) => {
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
  const correct = async (id) => {
    try {
      const r = await api(`/engagements/${id}/correct`, { method: 'POST', body: JSON.stringify({ action: 'pas_un_engagement' }) })
      toast(r.regle ? 'Règle apprise : ' + r.regle : 'Écarté'); load()
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

      {!shown.length ? (
        <div className="empty">Aucun engagement ne correspond à ces filtres.</div>
      ) : (
        <div className="tbl-wrap">
          <table>
            <thead>
              <tr><th>Type</th><th>Engagement</th><th>Échéance</th><th>Fiabilité</th><th>Statut</th><th></th></tr>
            </thead>
            <tbody>
              {shown.map((r) => (
                <tr key={r.id}>
                  <td><span className={'badge ' + (r.type || 'autre')}>{TYPE_LABELS[r.type] || 'Autre'}</span></td>
                  <td>
                    <p className="eng-title">
                      {r.objet}
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
                  <td>{r.echeance ? fmtDate(r.echeance) : <span className="mono">—</span>}</td>
                  <td><div className="reli"><span style={{ width: `${Math.round(r.confiance * 100)}%` }} /></div></td>
                  <td><div className={'status ' + statusClass(r)}><span className="dot" />{statusLabel(r)}</div></td>
                  <td>
                    <div className="row-actions">
                      {r.action && (
                        <button className="btn-icon primary" title={r.action.label}
                          onClick={() => d.openForEngagement(r.id, r.action)}>✉</button>
                      )}
                      <button className="btn-icon" title="Marquer livré" onClick={() => patch(r.id, { statut: 'livre' }, 'Marqué comme livré')}>✓</button>
                      <button className="btn-icon" title="Pas un engagement" onClick={() => correct(r.id)}>✕</button>
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
    </section>
  )
}

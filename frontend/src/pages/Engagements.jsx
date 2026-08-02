import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import { EngTable, TYPE_LABELS } from '../components/ui'

const STATUT_FILTERS = [
  ['ouvert,confirme,en_retard', 'Actifs'],
  ['en_retard', 'En retard'],
  ['livre', 'Livrés'],
  ['abandonne', 'Écartés'],
  ['', 'Tous'],
]

export default function Engagements() {
  const [engs, setEngs] = useState([])
  const [statut, setStatut] = useState(STATUT_FILTERS[0][0])
  const [typeFilter, setTypeFilter] = useState('')

  const load = useCallback(() => {
    api('/engagements' + (statut ? '?statut=' + statut : ''))
      .then((r) => setEngs(r || []))
      .catch((e) => toast(e.message, true))
  }, [statut])
  useEffect(load, [load])

  const counts = {}
  engs.forEach((e) => { counts[e.type] = (counts[e.type] || 0) + 1 })
  const shown = typeFilter ? engs.filter((e) => e.type === typeFilter) : engs

  return (
    <section>
      <div className="page-head">
        <div>
          <h2>Engagements</h2>
          <p className="help">Tout ce qui a été promis — par vous ou à vous — détecté dans vos échanges. Corrigez d'un geste : l'agent apprend de chaque correction.</p>
        </div>
      </div>
      <div className="filters">
        <div className="chips">
          <span className={'chip clickable' + (!typeFilter ? ' on' : '')} onClick={() => setTypeFilter('')}>
            Tous <b>{engs.length}</b>
          </span>
          {Object.entries(counts).sort((a, b) => b[1] - a[1]).map(([t, n]) => (
            <span key={t} className={'chip clickable' + (typeFilter === t ? ' on' : '')} onClick={() => setTypeFilter(t)}>
              {TYPE_LABELS[t] || t} <b>{n}</b>
            </span>
          ))}
        </div>
        <select value={statut} onChange={(e) => setStatut(e.target.value)}>
          {STATUT_FILTERS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
        </select>
      </div>
      <EngTable engs={shown} refresh={load} />
    </section>
  )
}

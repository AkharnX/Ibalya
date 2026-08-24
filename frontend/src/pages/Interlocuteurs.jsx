import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, toast } from '../api'
import { SqueletteTable } from '../components/Squelette'
import FichePersonne from '../components/FichePersonne'
import SourcePanel from '../components/SourcePanel'

// Personnes et organisations (CDC 5.3). Le type et l'indicateur de sensibilité
// nourrissent la capsule : un interlocuteur marqué comme sensible obtient une
// attention accrue au scoring des alertes.
const TYPES = [
  ['interne', 'Interne'],
  ['client', 'Client'],
  ['fournisseur', 'Fournisseur'],
  ['autre', 'Autre'],
]

export default function Interlocuteurs() {
  const [rows, setRows] = useState(null)
  const [filtre, setFiltre] = useState('')
  const [search, setSearch] = useState('')
  const [ficheId, setFicheId] = useState(null)
  const [params, setParams] = useSearchParams()

  // Cible d'un résultat de recherche.
  useEffect(() => {
    const id = Number(params.get('fiche'))
    if (!id) return
    setFicheId(id)
    setParams((p) => { p.delete('fiche'); return p }, { replace: true })
  }, [params, setParams])
  const [sourceId, setSourceId] = useState(null)
  const [filId, setFilId] = useState(null)

  const load = useCallback(() => {
    api('/persons').then((r) => setRows(r || [])).catch((e) => { setRows([]); toast(e.message, true) })
  }, [])
  useEffect(load, [load])

  const compte = useMemo(() => {
    const c = { '': (rows || []).length, sensible: 0 }
    ;(rows || []).forEach((p) => {
      c[p.type] = (c[p.type] || 0) + 1
      if (p.sensitive) c.sensible += 1
    })
    return c
  }, [rows])

  const shown = (rows || []).filter((p) => {
    const okType = !filtre || (filtre === 'sensible' ? p.sensitive : p.type === filtre)
    const q = search.trim().toLowerCase()
    const okSearch = !q || [p.name, p.email].filter(Boolean).some((v) => v.toLowerCase().includes(q))
    return okType && okSearch
  })

  // L'API attend le couple complet : on renvoie toujours type ET sensibilité.
  const mettreAJour = async (p, champs) => {
    const corps = { type: p.type, sensitive: p.sensitive, ...champs }
    setRows((rs) => rs.map((x) => (x.id === p.id ? { ...x, ...corps } : x)))
    try { await api(`/persons/${p.id}`, { method: 'PATCH', body: JSON.stringify(corps) }) }
    catch (e) { toast(e.message, true); load() }
  }

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Interlocuteurs</h1>
          <p>Les personnes déduites de vos échanges. Indiquez qui est client, qui est fournisseur, et qui mérite une vigilance particulière.</p>
        </div>
      </div>

      <div className="chip-row">
        <div className={'chip' + (filtre === '' ? ' active' : '')} onClick={() => setFiltre('')}>
          Tous <span className="n">{compte[''] || 0}</span>
        </div>
        {TYPES.map(([k, label]) => (
          <div key={k} className={'chip' + (filtre === k ? ' active' : '')} onClick={() => setFiltre(k)}>
            {label} <span className="n">{compte[k] || 0}</span>
          </div>
        ))}
        <div className={'chip' + (filtre === 'sensible' ? ' active' : '')} onClick={() => setFiltre('sensible')}>
          À surveiller <span className="n">{compte.sensible || 0}</span>
        </div>
      </div>

      <div className="filter-bar">
        <div className="search-wrap">
          <input type="text" placeholder="Rechercher un nom ou une adresse..." value={search}
            onChange={(e) => setSearch(e.target.value)} />
        </div>
      </div>

      {rows === null ? <SqueletteTable lignes={6} colonnes={4} /> : !shown.length ? (
        <div className="empty">Aucun interlocuteur ne correspond.</div>
      ) : (
        <div className="tbl-wrap">
          <table>
            <thead><tr><th>Nom</th><th>Adresse</th><th>Type</th><th>À surveiller</th></tr></thead>
            <tbody>
              {shown.map((p) => (
                <tr key={p.id}>
                  <td className="obj">
                    <button className="lien-source" title="Ouvrir la fiche"
                      onClick={() => setFicheId(p.id)}>
                      {p.name || p.email.split('@')[0]}
                    </button>
                  </td>
                  <td className="sub mono">{p.email}</td>
                  <td>
                    <select value={p.type || 'autre'} onChange={(e) => mettreAJour(p, { type: e.target.value })}
                      aria-label={`Type de ${p.email}`}>
                      {TYPES.map(([k, label]) => <option key={k} value={k}>{label}</option>)}
                    </select>
                  </td>
                  <td>
                    <label className="switch-inline">
                      <input type="checkbox" checked={!!p.sensitive}
                        onChange={(e) => mettreAJour(p, { sensitive: e.target.checked })} />
                      <span>{p.sensitive ? 'Vigilance accrue' : 'Standard'}</span>
                    </label>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <FichePersonne personneId={ficheId} onClose={() => setFicheId(null)}
        onEngagement={(id) => { setFicheId(null); setSourceId(id) }}
        onFil={(id) => { setFicheId(null); setFilId(id) }} />
      <SourcePanel engagementId={sourceId} onClose={() => setSourceId(null)} />
      <SourcePanel threadId={filId} onClose={() => setFilId(null)} />
    </section>
  )
}

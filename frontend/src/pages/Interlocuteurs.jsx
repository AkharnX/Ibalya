import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, toast } from '../api'
import { SqueletteTable } from '../components/Squelette'
import FichePersonne from '../components/FichePersonne'
import SourcePanel from '../components/SourcePanel'
import { useTri, EnteteTri } from '../components/tri'

// Personnes et organisations (CDC 5.3). Le type et l'indicateur de sensibilité
// nourrissent la capsule : un interlocuteur marqué comme sensible obtient une
// attention accrue au scoring des alertes.
// Messageries grand public : jamais de typage par domaine.
const GRAND_PUBLIC = new Set(['gmail.com', 'googlemail.com', 'outlook.com', 'outlook.fr', 'hotmail.com', 'hotmail.fr', 'live.com', 'live.fr', 'msn.com', 'yahoo.com', 'yahoo.fr', 'orange.fr', 'wanadoo.fr', 'free.fr', 'sfr.fr', 'neuf.fr', 'laposte.net', 'bbox.fr', 'icloud.com', 'me.com', 'mac.com', 'protonmail.com', 'proton.me', 'pm.me', 'gmx.com', 'gmx.fr', 'aol.com', 'numericable.fr'])
const domaineDe = (email) => (email || '').toLowerCase().split('@')[1] || ''
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

  const { tri, trierPar, trier } = useTri()
  const lignes = trier(shown, {
    nom: (p) => (p.name || p.email || '').toLowerCase(),
    email: (p) => (p.email || '').toLowerCase(),
    type: (p) => (p.type || 'autre'),
    sensitive: (p) => (p.sensitive ? 1 : 0),
  })

  // L'API attend le couple complet : on renvoie toujours type ET sensibilité.
  const mettreAJour = async (p, champs) => {
    const corps = { type: p.type, sensitive: p.sensitive, ...champs }
    setRows((rs) => rs.map((x) => (x.id === p.id ? { ...x, ...corps } : x)))
    try { await api(`/persons/${p.id}`, { method: 'PATCH', body: JSON.stringify(corps) }) }
    catch (e) { toast(e.message, true); load() }
    // Après un changement de type, proposer d'appliquer au domaine entier
    // (pas pour une messagerie grand public : gmail.com n'est pas une entreprise).
    if (champs.type) setProposition({ id: p.id, email: p.email, type: champs.type, domaine: domaineDe(p.email) })
  }
  const [proposition, setProposition] = useState(null)
  const appliquerDomaine = async () => {
    const { id, type, domaine } = proposition
    try {
      const r = await api(`/persons/${id}`, { method: 'PATCH', body: JSON.stringify({ type, sensitive: !!(rows || []).find((x) => x.id === id)?.sensitive, appliquer_domaine: true }) })
      toast(`${r.modifies} interlocuteur${r.modifies > 1 ? 's' : ''} @${domaine} ${r.modifies > 1 ? 'typés' : 'typé'}`)
      load()
    } catch (e) { toast(e.message, true) }
    setProposition(null)
  }
  const [typageEnCours, setTypageEnCours] = useState(false)
  const typerAuto = async () => {
    setTypageEnCours(true)
    try {
      const r = await api('/persons/typer-auto', { method: 'POST' })
      toast(r.modifies ? `${r.modifies} interlocuteur${r.modifies > 1 ? 's' : ''} typé${r.modifies > 1 ? 's' : ''} automatiquement` : 'Rien à typer : complétez les clients récurrents et fournisseurs critiques dans Règles métier')
      load()
    } catch (e) { toast(e.message, true) }
    setTypageEnCours(false)
  }

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Interlocuteurs</h1>
          <p>Les personnes déduites de vos échanges. Indiquez qui est client, qui est fournisseur, et qui mérite une vigilance particulière.</p>
        </div>
        <div className="page-actions">
          <button className="btn" disabled={typageEnCours} onClick={typerAuto}
            title="Interne par le domaine de votre entreprise, client et fournisseur d'après vos règles métier">
            {typageEnCours ? 'Typage…' : 'Typer automatiquement'}
          </button>
        </div>
      </div>
      {proposition && !GRAND_PUBLIC.has(proposition.domaine) && (
        <p className="help proposition-domaine">
          Appliquer « {TYPES.find(([k]) => k === proposition.type)?.[1] || proposition.type} » à tous les interlocuteurs @{proposition.domaine} ?{' '}
          <button className="ghost" onClick={appliquerDomaine}>Oui, tout le domaine</button>{' '}
          <button className="ghost" onClick={() => setProposition(null)}>Non</button>
        </p>
      )}

      <div className="chip-row">
        <button type="button" className={'chip' + (filtre === '' ? ' active' : '')} onClick={() => setFiltre('')}>
          Tous <span className="n">{compte[''] || 0}</span>
        </button>
        {TYPES.map(([k, label]) => (
          <button type="button" key={k} className={'chip' + (filtre === k ? ' active' : '')} onClick={() => setFiltre(k)}>
            {label} <span className="n">{compte[k] || 0}</span>
          </button>
        ))}
        <button type="button" className={'chip' + (filtre === 'sensible' ? ' active' : '')} onClick={() => setFiltre('sensible')}>
          À surveiller <span className="n">{compte.sensible || 0}</span>
        </button>
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
            <thead><tr>
              <EnteteTri col="nom" tri={tri} trierPar={trierPar}>Nom</EnteteTri>
              <EnteteTri col="email" tri={tri} trierPar={trierPar}>Adresse</EnteteTri>
              <EnteteTri col="type" tri={tri} trierPar={trierPar}>Type</EnteteTri>
              <EnteteTri col="sensitive" tri={tri} trierPar={trierPar}>À surveiller</EnteteTri>
            </tr></thead>
            <tbody>
              {lignes.map((p) => (
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

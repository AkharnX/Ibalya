import { useCallback, useEffect, useMemo, useState } from 'react'
import { api, toast } from '../api'
import { fmtDate } from '../components/ui'
import SourcePanel from '../components/SourcePanel'
import { SqueletteTable } from '../components/Squelette'

// Graphe de dépendances (CDC 8.1). Les liens sont proposés par heuristique,
// jamais décidés par le modèle. Tant qu'un lien reste « candidat », le
// détecteur de contradiction l'ignore : c'est cet écran qui le confirme.
const FILTRES = [
  ['candidat', 'À confirmer'],
  ['confirme', 'Confirmés'],
  ['rejete', 'Rejetés'],
  ['', 'Tous'],
]

export default function Liens() {
  const [rows, setRows] = useState(null)
  const [filtre, setFiltre] = useState('candidat')
  const [sourceId, setSourceId] = useState(null)
  const [busy, setBusy] = useState(0)

  const load = useCallback(() => {
    setRows(null)
    api('/links').then((r) => setRows(r || [])).catch((e) => { setRows([]); toast(e.message, true) })
  }, [])
  useEffect(load, [load])

  const compte = useMemo(() => {
    const c = { '': (rows || []).length }
    ;(rows || []).forEach((l) => { c[l.statut] = (c[l.statut] || 0) + 1 })
    return c
  }, [rows])

  const shown = (rows || []).filter((l) => !filtre || l.statut === filtre)

  const decider = async (id, action) => {
    if (busy) return
    setBusy(id)
    try {
      await api(`/links/${id}/${action}`, { method: 'POST' })
      toast(action === 'confirm' ? 'Lien confirmé, le détecteur de contradiction le prend en compte' : 'Lien rejeté')
      load()
    } catch (e) { toast(e.message, true) }
    finally { setBusy(0) }
  }

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Dépendances</h1>
          <p>Quand une promesse que vous avez faite dépend de quelqu'un d'autre, l'agent propose le lien. À vous de trancher : il ne le décide jamais seul.</p>
        </div>
      </div>

      <div className="note">
        Un lien confirmé permet à l'agent de vous prévenir avant la casse : « votre fournisseur est
        silencieux depuis 8 jours et vous avez promis une livraison au client le 15 ». Tant qu'un lien
        reste à confirmer, cette alerte croisée ne se déclenche pas.
      </div>

      <div className="chip-row">
        {FILTRES.map(([k, label]) => (
          <div key={k || 'all'} className={'chip' + (filtre === k ? ' active' : '')} onClick={() => setFiltre(k)}>
            {label} <span className="n">{compte[k] || 0}</span>
          </div>
        ))}
      </div>

      {rows === null ? <SqueletteTable lignes={4} colonnes={4} /> : !shown.length ? (
        <div className="empty">
          {filtre === 'candidat' ? 'Aucun lien en attente de décision.' : 'Aucun lien dans cette catégorie.'}
        </div>
      ) : (
        <div className="tbl-wrap">
          <table>
            <thead>
              <tr><th>Dépend de (amont)</th><th>Engagement concerné (aval)</th><th>Pourquoi ce lien</th><th>Proposé le</th><th></th></tr>
            </thead>
            <tbody>
              {shown.map((l) => (
                <tr key={l.id}>
                  <td className="obj">
                    <button className="lien-source" onClick={() => setSourceId(l.amont_id)}>{l.amont_objet}</button>
                  </td>
                  <td className="obj">
                    <button className="lien-source" onClick={() => setSourceId(l.aval_id)}>{l.aval_objet}</button>
                  </td>
                  <td className="sub">{l.raison || '—'}</td>
                  <td className="sub">{fmtDate(l.created_at)}</td>
                  <td>
                    {l.statut === 'candidat' ? (
                      <div className="row-actions">
                        <button className="btn-icon primary" title="Confirmer ce lien"
                          disabled={busy === l.id} onClick={() => decider(l.id, 'confirm')}>✓</button>
                        <button className="btn-icon" title="Ce lien n'existe pas"
                          disabled={busy === l.id} onClick={() => decider(l.id, 'reject')}>✕</button>
                      </div>
                    ) : (
                      <div className={'status ' + (l.statut === 'confirme' ? 'open' : 'late')}>
                        <span className="dot" />{l.statut === 'confirme' ? 'Confirmé' : 'Rejeté'}
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <SourcePanel engagementId={sourceId} onClose={() => setSourceId(null)} />
    </section>
  )
}

// File d'attente des messages proposés par l'agent.
// Marche 3 de l'escalier d'agentivité : rien ne part sans un clic du dirigeant.
import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import { DraftPanel } from '../components/DraftPanel'
import { Empty, fmtDT } from '../components/ui'
import { SqueletteTable } from '../components/Squelette'

export default function AValider() {
  const [drafts, setDrafts] = useState(null)
  const [selected, setSelected] = useState(null)

  const load = useCallback(() => {
    api('/drafts?statut=propose').then((r) => setDrafts(r || [])).catch((e) => toast(e.message, true))
  }, [])
  useEffect(load, [load])

  const reject = async (id) => {
    try { await api(`/drafts/${id}/reject`, { method: 'POST' }); toast('Message rejeté'); load() }
    catch (e) { toast(e.message, true) }
  }

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>À valider</h1>
          <p>Les messages que l'agent a pré-rédigés à partir de vos engagements. Relisez, ajustez si besoin, puis envoyez — rien ne part sans votre validation.</p>
        </div>
      </div>

      {drafts === null ? (
        <SqueletteTable lignes={3} colonnes={5} />
      ) : !drafts.length ? (
        <Empty>Aucun message en attente. Tout est traité.</Empty>
      ) : (
        <div className="tbl-wrap">
          <table>
            <thead>
              <tr><th>Destinataire</th><th>Message proposé</th><th>Motif</th><th>Créé</th><th><span className="sr-only">Actions</span></th></tr>
            </thead>
            <tbody>
              {drafts.map((d) => (
                <tr key={d.id}>
                  <td className="sub">{d.to_email}</td>
                  <td>
                    <p className="eng-title">{d.subject}</p>
                    <p className="eng-flow">{d.body.slice(0, 90).replace(/\n/g, ' ')}…</p>
                  </td>
                  <td style={{ maxWidth: 260 }}>
                    <span className="sub">{d.detection_titre || d.engagement_objet || '—'}</span>
                  </td>
                  <td className="sub">{fmtDT(d.created_at)}</td>
                  <td>
                    <div className="row-actions">
                      <button className="btn-icon primary" title="Relire et envoyer"
                        onClick={() => setSelected(d)}>✉</button>
                      <button className="btn-icon" title="Rejeter" onClick={() => reject(d.id)}>✕</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <DraftPanel
        draft={selected} loading={false}
        title={selected?.detection_titre || 'Proposition de message'}
        hint={selected?.detection_detail || "Générée par l'agent à partir du contexte de l'engagement"}
        onClose={() => setSelected(null)}
        onSent={() => { setSelected(null); load() }}
      />
    </section>
  )
}

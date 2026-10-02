// File d'attente des messages proposés par l'agent.
// Marche 3 de l'escalier d'agentivité : rien ne part sans un clic du dirigeant.
import { useCallback, useEffect, useState } from 'react'
import Icone from '../components/Icone'
import { api, toast, estVueCombinee, definirVueCombinee } from '../api'
import { DraftPanel } from '../components/DraftPanel'
import { Empty, fmtDT } from '../components/ui'
import { SqueletteTable } from '../components/Squelette'

export default function AValider() {
  const [drafts, setDrafts] = useState(null)
  const [selected, setSelected] = useState(null)
  const combinee = estVueCombinee()

  const load = useCallback(() => {
    api(combinee ? '/drafts/global' : '/drafts?statut=propose').then((r) => setDrafts(r || [])).catch((e) => toast(e.message, true))
  }, [combinee])
  useEffect(load, [load])

  const reject = async (id) => {
    try { await api(`/drafts/${id}/reject`, { method: 'POST' }); toast('Message rejeté'); load() }
    catch (e) { toast(e.message, true) }
  }

  // Vue combinée : chaque message vit dans sa boîte (tenant isolé). Pour le lire
  // et l'envoyer, on bascule la session sur cette boîte (et on quitte la combinée).
  const ouvrirBoite = async (boiteId) => {
    definirVueCombinee(false)
    try { await api(`/espaces/${boiteId}/activer`, { method: 'POST' }); window.location.reload() }
    catch (e) { toast(e.message, true) }
  }

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>À valider{combinee ? ' · toutes les boîtes' : ''}</h1>
          <p>{combinee
            ? "Tous les messages en attente, agrégés sur vos boîtes. Chaque ligne porte sa boîte ; cliquez pour l'ouvrir dans celle-ci et l'envoyer."
            : "Les messages que l'agent a pré-rédigés à partir de vos engagements. Relisez, ajustez si besoin, puis envoyez — rien ne part sans votre validation."}</p>
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
              <tr><th>Destinataire</th><th>Message proposé</th>{combinee ? <th>Boîte</th> : <th>Motif</th>}<th>Créé</th><th><span className="sr-only">Actions</span></th></tr>
            </thead>
            <tbody>
              {drafts.map((d) => (
                <tr key={(d.boite_id || 0) + '-' + d.id}>
                  <td className="sub">{d.to_email}</td>
                  <td>
                    {combinee ? (
                      <button type="button" className="msg-open" title={'Ouvrir dans ' + d.boite}
                        onClick={() => ouvrirBoite(d.boite_id)} aria-label={`Ouvrir dans ${d.boite} : ${d.subject}`}>
                        <span className="eng-title">{d.subject}</span>
                        <span className="eng-flow">{d.body.slice(0, 90).replace(/\n/g, ' ')}…</span>
                      </button>
                    ) : (
                      <button type="button" className="msg-open" title="Lire le message"
                        onClick={() => setSelected(d)} aria-label={`Lire le message : ${d.subject}`}>
                        <span className="eng-title">{d.subject}</span>
                        <span className="eng-flow">{d.body.slice(0, 90).replace(/\n/g, ' ')}…</span>
                      </button>
                    )}
                  </td>
                  {combinee ? (
                    <td><button className="boite-tag" title={'Ouvrir dans ' + d.boite} onClick={() => ouvrirBoite(d.boite_id)}>{d.boite}</button></td>
                  ) : (
                    <td style={{ maxWidth: 260 }}><span className="sub">{d.detection_titre || d.engagement_objet || '—'}</span></td>
                  )}
                  <td className="sub">{fmtDT(d.created_at)}</td>
                  <td>
                    {combinee ? (
                      <div className="row-actions">
                        <button className="btn-icon primary" title={'Ouvrir dans ' + d.boite}
                          onClick={() => ouvrirBoite(d.boite_id)}><Icone nom="action-valider-envoyer" /></button>
                      </div>
                    ) : (
                      <div className="row-actions">
                        <button className="btn-icon primary" title="Relire et envoyer"
                          onClick={() => setSelected(d)}><Icone nom="action-valider-envoyer" /></button>
                        <button className="btn-icon" title="Rejeter" onClick={() => reject(d.id)}><Icone nom="action-rejeter" /></button>
                      </div>
                    )}
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

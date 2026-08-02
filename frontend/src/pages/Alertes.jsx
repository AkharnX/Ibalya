import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import { ConfBar, DET_LABELS, Empty, fmtDT } from '../components/ui'

function DraftCard({ d, refresh }) {
  const [editing, setEditing] = useState(false)
  const [form, setForm] = useState({ to_email: d.to_email, subject: d.subject, body: d.body })
  const [busy, setBusy] = useState(false)

  const save = async () => {
    try {
      await api(`/drafts/${d.id}`, { method: 'PATCH', body: JSON.stringify(form) })
      toast('Message enregistré'); setEditing(false); refresh()
    } catch (e) { toast(e.message, true) }
  }
  const validate = async () => {
    if (busy || !confirm('Envoyer ce message maintenant ?')) return
    setBusy(true)
    try { await api(`/drafts/${d.id}/validate`, { method: 'POST' }); toast('Message envoyé ✓'); refresh() }
    catch (e) { toast(e.message, true) }
    finally { setBusy(false) }
  }
  const reject = async () => {
    try { await api(`/drafts/${d.id}/reject`, { method: 'POST' }); toast('Message rejeté'); refresh() }
    catch (e) { toast(e.message, true) }
  }

  return (
    <div className="item">
      {d.detection_titre && (
        <p className="context">💡 {d.detection_titre}{d.engagement_objet ? ' · engagement : ' + d.engagement_objet : ''}</p>
      )}
      {!editing ? (
        <>
          <b>À : {d.to_email}</b> — {d.subject}
          <pre>{d.body}</pre>
          <div className="row-actions">
            <button className="primary" disabled={busy} onClick={validate}>✓ Valider et envoyer</button>
            <button onClick={() => setEditing(true)}>✎ Modifier</button>
            <button onClick={reject}>✗ Rejeter</button>
          </div>
        </>
      ) : (
        <div className="draft-edit">
          <label>Destinataire</label>
          <input value={form.to_email} onChange={(e) => setForm({ ...form, to_email: e.target.value })} />
          <label>Objet</label>
          <input value={form.subject} onChange={(e) => setForm({ ...form, subject: e.target.value })} />
          <label>Message</label>
          <textarea rows={8} value={form.body} onChange={(e) => setForm({ ...form, body: e.target.value })} />
          <div className="row-actions">
            <button className="primary" onClick={save}>Enregistrer</button>
            <button onClick={() => setEditing(false)}>Annuler</button>
          </div>
        </div>
      )}
    </div>
  )
}

export default function Alertes() {
  const [dets, setDets] = useState([])
  const [drafts, setDrafts] = useState([])

  const load = useCallback(() => {
    api('/detections').then((r) => setDets(r || [])).catch((e) => toast(e.message, true))
    api('/drafts?statut=propose').then((r) => setDrafts(r || [])).catch((e) => toast(e.message, true))
  }, [])
  useEffect(load, [load])

  const dismiss = async (id) => {
    try { await api(`/detections/${id}/dismiss`, { method: 'POST' }); toast('Alerte écartée'); load() }
    catch (e) { toast(e.message, true) }
  }

  return (
    <section>
      <div className="page-head">
        <div>
          <h2>Alertes</h2>
          <p className="help">L'agent surveille vos engagements en continu et vous alerte dans 5 situations. Chaque alerte fiable s'accompagne, quand c'est possible, d'un message prêt à envoyer.</p>
        </div>
      </div>
      <details className="legend">
        <summary>Les 5 types d'alerte expliqués</summary>
        <ul>
          <li><b>Échéance à risque</b> — une promesse arrive à échéance sans signe d'avancement récent.</li>
          <li><b>Silence anormal</b> — un fil avec un engagement en cours ne répond plus depuis plus longtemps que son rythme habituel.</li>
          <li><b>Contradiction</b> — une promesse à un client devient intenable à cause d'un retard amont (fournisseur, prestataire).</li>
          <li><b>Orphelin</b> — une promesse jamais suivie d'effet : ni confirmation, ni livraison, ni relance. Les oublis.</li>
          <li><b>Surcharge</b> — trop d'échéances concentrées sur la même semaine.</li>
        </ul>
      </details>

      {!dets.length ? <Empty>Aucune alerte active. 👌</Empty> : (
        <div className="tbl-wrap">
          <table className="tbl">
            <thead>
              <tr><th>Alerte</th><th>Détail</th><th>Fiabilité</th><th>Date</th><th></th></tr>
            </thead>
            <tbody>
              {dets.map((d) => (
                <tr key={d.id}>
                  <td><b>{d.critique ? '⚠ ' : ''}{DET_LABELS[d.type] || d.type}</b></td>
                  <td className="obj">{d.titre}<div className="sub">{d.detail}</div></td>
                  <td><ConfBar value={d.score} /></td>
                  <td className="sub">{fmtDT(d.created_at)}</td>
                  <td><button className="ghost" onClick={() => dismiss(d.id)}>✗ Écarter</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <h3>✉ Messages proposés <span className="muted">— rien ne part sans votre validation</span></h3>
      {!drafts.length ? <Empty>Aucun message en attente de validation.</Empty> :
        drafts.map((d) => <DraftCard key={d.id} d={d} refresh={load} />)}
    </section>
  )
}

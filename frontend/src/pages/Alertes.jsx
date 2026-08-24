import { useCallback, useEffect, useState } from 'react'
import Icone from '../components/Icone'
import { Link } from 'react-router-dom'
import { api, toast } from '../api'
import { DET_LABELS, Empty, FiltreFiabilite, Reli, fmtDT, niveauFiabilite } from '../components/ui'
import { SqueletteTable } from '../components/Squelette'

export default function Alertes() {
  const [dets, setDets] = useState(null)
  const [enAttente, setEnAttente] = useState(0)
  const [fiabilite, setFiabilite] = useState('')

  const load = useCallback(() => {
    api('/detections').then((r) => setDets(r || [])).catch((e) => toast(e.message, true))
    // On ne compte que les messages en attente : leur traitement se fait sur
    // « À valider », inutile de reproduire la même file sur deux pages.
    api('/drafts?statut=propose').then((r) => setEnAttente((r || []).length)).catch(() => {})
  }, [])
  useEffect(load, [load])

  const visibles = (dets || []).filter((d) => !fiabilite || niveauFiabilite(d.score) === fiabilite)

  const dismiss = async (id) => {
    try { await api(`/detections/${id}/dismiss`, { method: 'POST' }); toast('Alerte écartée'); load() }
    catch (e) { toast(e.message, true) }
  }

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Alertes</h1>
          <p>L'agent surveille vos engagements en continu et vous alerte dans 5 situations. Chaque alerte fiable s'accompagne, quand c'est possible, d'un message prêt à envoyer.</p>
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

      {dets !== null && dets.length > 0 && (
        <FiltreFiabilite valeur={fiabilite} onChange={setFiabilite} champ="score" rows={dets} />
      )}

      {dets === null ? <SqueletteTable lignes={4} colonnes={5} /> : !visibles.length ? (
        <Empty>{dets.length ? 'Aucune alerte à ce niveau de fiabilité.' : 'Aucune alerte active.'}</Empty>
      ) : (
        <div className="tbl-wrap">
          <table>
            <thead>
              <tr><th>Alerte</th><th>Détail</th><th>Fiabilité</th><th>Date</th><th><span className="sr-only">Actions</span></th></tr>
            </thead>
            <tbody>
              {visibles.map((d) => (
                <tr key={d.id}>
                  <td><b>{d.critique ? '⚠ ' : ''}{DET_LABELS[d.type] || d.type}</b></td>
                  <td className="obj">{d.titre}<div className="sub">{d.detail}</div></td>
                  <td><Reli value={d.score} /></td>
                  <td className="sub">{fmtDT(d.created_at)}</td>
                  <td><button className="ghost" onClick={() => dismiss(d.id)}>
                    <Icone nom="action-rejeter" /> Écarter</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {enAttente > 0 && (
        <div className="renvoi">
          <span>
            <Icone nom="action-valider-envoyer" /> {enAttente} message{enAttente > 1 ? 's' : ''} pré-rédigé{enAttente > 1 ? 's' : ''} attend
            {enAttente > 1 ? 'ent' : ''} votre validation. Rien ne part sans votre clic.
          </span>
          <Link className="btn" to="/a-valider">Ouvrir « À valider »</Link>
        </div>
      )}
    </section>
  )
}

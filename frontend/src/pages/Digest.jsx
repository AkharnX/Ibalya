import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import { DET_LABELS, Reli, TYPE_LABELS, fmtDT, fmtDate } from '../components/ui'
import SourcePanel from '../components/SourcePanel'
import { SqueletteLignes } from '../components/Squelette'

// Digest (CDC 9.3, EF-6). Règle anti-churn : seules les détections au-dessus du
// seuil de publication y figurent, les autres restent consultables ailleurs.
const TYPES = [['quotidien', 'Quotidien'], ['hebdo', 'Hebdomadaire']]

export default function Digest() {
  const [type, setType] = useState('quotidien')
  const [rep, setRep] = useState(undefined)
  const [busy, setBusy] = useState(false)
  const [sourceId, setSourceId] = useState(null)

  const load = useCallback((t) => {
    setRep(undefined)
    api(`/digest/latest?type=${t}`).then(setRep).catch(() => setRep(null))
  }, [])
  useEffect(() => { load(type) }, [load, type])

  const generer = async () => {
    if (busy) return
    setBusy(true)
    try {
      await api('/digest/generate', { method: 'POST', body: JSON.stringify({ type }) })
      toast('Digest généré'); load(type)
    } catch (e) { toast(e.message, true) }
    finally { setBusy(false) }
  }

  const d = rep?.content

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Digest</h1>
          <p>Le récapitulatif que l'agent vous adresse : engagements triés par risque, alertes du jour, messages déjà rédigés. Rien sous le seuil de fiabilité n'y entre.</p>
        </div>
        <div className="page-actions">
          <button className="btn" disabled={busy} onClick={generer}>{busy ? 'Génération…' : '⟳ Générer maintenant'}</button>
        </div>
      </div>

      <div className="chip-row">
        {TYPES.map(([k, label]) => (
          <div key={k} className={'chip' + (type === k ? ' active' : '')} onClick={() => setType(k)}>{label}</div>
        ))}
      </div>

      {rep === undefined && <SqueletteLignes nombre={4} />}

      {rep === null && (
        <div className="empty">
          Aucun digest {type === 'hebdo' ? 'hebdomadaire' : 'quotidien'} n'a encore été produit.
        </div>
      )}

      {d && (
        <>
          <p className="help">Produit le {fmtDT(d.genere_le || rep.created_at)}</p>

          <div className="section-title">
            <h2>Alertes retenues <span className="muted">{(d.detections || []).length}</span></h2>
          </div>
          {!(d.detections || []).length ? <div className="empty">Aucune alerte au-dessus du seuil.</div> : (
            <div className="tbl-wrap">
              <table>
                <thead><tr><th>Alerte</th><th>Détail</th><th>Fiabilité</th></tr></thead>
                <tbody>
                  {d.detections.map((x) => (
                    <tr key={x.id}>
                      <td><b>{x.critique ? '⚠ ' : ''}{DET_LABELS[x.type] || x.type}</b></td>
                      <td className="obj">{x.titre}<div className="sub">{x.detail}</div></td>
                      <td><Reli value={x.score} /></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div className="section-title">
            <h2>Engagements à risque <span className="muted">{(d.engagements_a_risque || []).length}</span></h2>
          </div>
          {!(d.engagements_a_risque || []).length ? <div className="empty">Aucun engagement à risque.</div> : (
            <div className="tbl-wrap">
              <table>
                <thead><tr><th>Type</th><th>Engagement</th><th>Échéance</th></tr></thead>
                <tbody>
                  {d.engagements_a_risque.map((e) => (
                    <tr key={e.id}>
                      <td><span className={'badge ' + (e.type || 'autre')}>{TYPE_LABELS[e.type] || 'Autre'}</span></td>
                      <td>
                        <p className="eng-title">
                          <button className="lien-source" onClick={() => setSourceId(e.id)}>{e.objet}</button>
                        </p>
                        <p className="eng-flow">{e.emetteur_email || '?'} → {e.destinataire_email || '?'}</p>
                      </td>
                      <td>{e.echeance ? fmtDate(e.echeance) : <span className="mono">—</span>}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <div className="section-title">
            <h2>Messages proposés <span className="muted">{(d.brouillons_proposes || []).length}</span></h2>
          </div>
          <p className="help">Rien ne part sans votre validation. Le traitement se fait depuis la page « À valider ».</p>
          {!(d.brouillons_proposes || []).length ? <div className="empty">Aucun message proposé.</div> :
            d.brouillons_proposes.map((b) => (
              <div className="item" key={b.id}>
                <b>À : {b.to_email}</b> — {b.subject}
                <pre>{b.body}</pre>
              </div>
            ))}
        </>
      )}

      <SourcePanel engagementId={sourceId} onClose={() => setSourceId(null)} />
    </section>
  )
}

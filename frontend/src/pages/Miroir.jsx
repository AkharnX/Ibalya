import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import { Reli, TYPE_LABELS, fmtDT, fmtDate } from '../components/ui'
import SourcePanel from '../components/SourcePanel'
import { SqueletteKpi, SqueletteLignes } from '../components/Squelette'

// Miroir d'activité (CDC 9.1, EF-2) : le premier rapport, produit après lecture
// des 30 derniers jours. Le CDC en fait le livrable d'activation, à montrer
// AVANT de demander quoi que ce soit au dirigeant.
function Liste({ titre, aide, rows, onSource }) {
  return (
    <div className="miroir-bloc">
      <div className="section-title"><h2>{titre} <span className="muted">{rows.length}</span></h2></div>
      <p className="help">{aide}</p>
      {!rows.length ? <div className="empty">Rien dans cette catégorie.</div> : (
        <div className="tbl-wrap">
          <table>
            <thead><tr><th>Type</th><th>Engagement</th><th>Échéance</th><th>Fiabilité</th></tr></thead>
            <tbody>
              {rows.map((e) => (
                <tr key={e.id}>
                  <td><span className={'badge ' + (e.type || 'autre')}>{TYPE_LABELS[e.type] || 'Autre'}</span></td>
                  <td>
                    <p className="eng-title">
                      <button className="lien-source" onClick={() => onSource(e.id)}>{e.objet}</button>
                    </p>
                    <p className="eng-flow">{e.emetteur_email || '?'} → {e.destinataire_email || '?'}</p>
                  </td>
                  <td>{e.echeance ? fmtDate(e.echeance) : <span className="mono">—</span>}</td>
                  <td><Reli value={e.confiance} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

export default function Miroir() {
  const [rep, setRep] = useState(undefined) // undefined = chargement, null = jamais généré
  const [busy, setBusy] = useState(false)
  const [sourceId, setSourceId] = useState(null)

  const load = useCallback(() => {
    api('/miroir').then(setRep).catch(() => setRep(null))
  }, [])
  useEffect(load, [load])

  const generer = async () => {
    if (busy) return
    setBusy(true)
    toast('Lecture des 30 derniers jours…')
    try { await api('/miroir/generate', { method: 'POST' }); toast('Miroir généré'); load() }
    catch (e) { toast(e.message, true) }
    finally { setBusy(false) }
  }

  const m = rep?.content

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Miroir d'activité</h1>
          <p>Ce que l'agent a compris de vos 30 derniers jours, sans que vous ayez rien saisi. Corrigez-le : chaque correction le calibre sur votre réalité.</p>
        </div>
        <div className="page-actions">
          <button className="btn" disabled={busy} onClick={generer}>
            {busy ? 'Lecture…' : '⟳ Régénérer'}
          </button>
        </div>
      </div>

      {rep === undefined && <><SqueletteKpi nombre={3} /><SqueletteLignes nombre={4} /></>}

      {rep === null && (
        <div className="empty">
          Aucun miroir n'a encore été généré. Lancez la lecture de votre historique pour obtenir le premier rapport.
        </div>
      )}

      {m && (
        <>
          <p className="context">{m.note}</p>

          <div className="kpi-row">
            <div className="kpi static">
              <span className="lbl">Messages lus</span><span className="num">{m.messages_lus}</span>
            </div>
            <div className="kpi static">
              <span className="lbl">Engagements en cours</span><span className="num">{(m.engagements_ouverts || []).length}</span>
            </div>
            <div className="kpi static">
              <span className="lbl">Retards probables</span><span className="num">{(m.en_retard_probable || []).length}</span>
            </div>
            <div className="kpi static">
              <span className="lbl">Fils sans réponse</span><span className="num">{(m.fils_sans_reponse || []).length}</span>
            </div>
          </div>

          <p className="help">
            Période lue : {m.periode_jours} jours · rapport produit le {fmtDT(m.genere_le || rep.created_at)}
          </p>

          <Liste titre="Engagements en cours" onSource={setSourceId}
            aide="Les promesses détectées dans vos échanges, encore ouvertes à ce jour."
            rows={m.engagements_ouverts || []} />

          <Liste titre="Retards probables" onSource={setSourceId}
            aide="Une échéance est passée sans signe de livraison dans le fil."
            rows={m.en_retard_probable || []} />

          <div className="miroir-bloc">
            <div className="section-title">
              <h2>Fils sans réponse <span className="muted">{(m.fils_sans_reponse || []).length}</span></h2>
            </div>
            <p className="help">Des conversations où plus personne n'a écrit depuis un moment.</p>
            {!(m.fils_sans_reponse || []).length ? (
              <div className="empty">Aucun fil en attente de réponse.</div>
            ) : (
              <div className="tbl-wrap">
                <table>
                  <thead><tr><th>Sujet</th><th>Interlocuteur</th><th>Silence</th></tr></thead>
                  <tbody>
                    {m.fils_sans_reponse.map((f) => (
                      <tr key={f.thread_id}>
                        <td className="obj">{f.sujet || '(sans objet)'}</td>
                        <td className="sub">{f.interlocuteur || '—'}</td>
                        <td><span className="mono">{f.jours_silence} j</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </>
      )}

      <SourcePanel engagementId={sourceId} onClose={() => setSourceId(null)} />
    </section>
  )
}

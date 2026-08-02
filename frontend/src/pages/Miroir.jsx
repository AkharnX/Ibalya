import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import { Empty, EngTable, fmtDT } from '../components/ui'

export default function Miroir() {
  const [miroir, setMiroir] = useState(null)
  const [missing, setMissing] = useState(false)

  const load = useCallback(() => {
    api('/miroir')
      .then((rep) => { setMiroir(rep.content); setMissing(false) })
      .catch(() => setMissing(true))
  }, [])
  useEffect(load, [load])

  const regen = async () => {
    try { await api('/miroir/generate', { method: 'POST' }); toast('Miroir régénéré'); load() }
    catch (e) { toast(e.message, true) }
  }

  return (
    <section>
      <div className="page-head">
        <div>
          <h2>Miroir d'activité</h2>
          <p className="help">La photographie de vos 30 derniers jours, générée automatiquement à l'activation — avant toute question.</p>
        </div>
        <div className="page-actions"><button onClick={regen}>⟳ Régénérer</button></div>
      </div>

      {missing && <Empty>Aucun miroir généré. Connectez un canal puis relancez l'onboarding (Réglages).</Empty>}
      {miroir && (
        <>
          <p className="help">{miroir.note}</p>
          <p className="muted">Généré le {fmtDT(miroir.genere_le)} — {miroir.messages_lus} messages lus sur {miroir.periode_jours} jours.</p>
          <h3>Engagements en cours ({(miroir.engagements_ouverts || []).length})</h3>
          <EngTable engs={miroir.engagements_ouverts} compact />
          <h3>En retard probable ({(miroir.en_retard_probable || []).length})</h3>
          <EngTable engs={miroir.en_retard_probable} compact />
          <h3>Fils sans réponse ({(miroir.fils_sans_reponse || []).length})</h3>
          {!(miroir.fils_sans_reponse || []).length ? <Empty>Aucun.</Empty> : (
            <div className="tbl-wrap">
              <table className="tbl">
                <thead><tr><th>Sujet</th><th>Interlocuteur</th><th>Silence</th></tr></thead>
                <tbody>
                  {miroir.fils_sans_reponse.map((f) => (
                    <tr key={f.thread_id}>
                      <td className="obj">{f.sujet}</td>
                      <td>{f.interlocuteur}</td>
                      <td>{f.jours_silence} jours</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </section>
  )
}

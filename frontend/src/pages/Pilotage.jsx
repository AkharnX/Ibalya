import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, toast } from '../api'
import { ConfBar, DET_LABELS, EngTable, Empty, TypeBadge, confirmEcheance, fmtDate } from '../components/ui'

export default function Pilotage() {
  const [status, setStatus] = useState(null)
  const [p, setP] = useState(null)
  const navigate = useNavigate()

  const load = useCallback(() => {
    api('/status').then(setStatus).catch(() => {})
    api('/pilotage').then(setP).catch((e) => toast(e.message, true))
  }, [])
  useEffect(load, [load])

  const runCycle = async () => {
    toast('Analyse en cours…')
    try {
      const r = await api('/cycle/run', { method: 'POST', body: JSON.stringify({ since_days: 2 }) })
      toast(r.erreur ? 'Terminé avec erreur : ' + r.erreur : `Analyse terminée (${r.duree})`, !!r.erreur)
      load()
    } catch (e) { toast(e.message, true) }
  }
  const genDigest = async () => {
    try {
      await api('/digest/generate', { method: 'POST', body: JSON.stringify({ type: localStorage.getItem('digest_type') || 'quotidien' }) })
      toast('Digest généré'); load()
    } catch (e) { toast(e.message, true) }
  }
  const validateDraft = async (id) => {
    if (!confirm('Envoyer ce message maintenant ?')) return
    try { await api(`/drafts/${id}/validate`, { method: 'POST' }); toast('Message envoyé ✓'); load() }
    catch (e) { toast(e.message, true) }
  }
  const linkAction = async (id, action) => {
    try { await api(`/links/${id}/${action}`, { method: 'POST' }); toast('Dépendance mise à jour'); load() }
    catch (e) { toast(e.message, true) }
  }

  const c = status?.compteurs
  const qw = p?.quick_wins || {}
  const quickWinCount = (qw.brouillons_a_valider?.length || 0) + (qw.echeances_a_confirmer?.length || 0) + (qw.liens_a_trancher?.length || 0)

  return (
    <section>
      <div className="page-head">
        <div>
          <h2>Pilotage</h2>
          <p className="help">L'essentiel de votre activité : ce qui bloque, ce qui arrive, ce que vous pouvez traiter en un clic.</p>
        </div>
        <div className="page-actions">
          <button onClick={runCycle}>⟳ Analyser maintenant</button>
          <button onClick={genDigest}>Générer le digest</button>
        </div>
      </div>

      {c && (
        <div className="cards">
          {[['Messages lus', c.messages], ['Filtrés sans coût IA', c.messages_exclus],
            ['Engagements suivis', c.engagements], ['Alertes actives', c.detections_actives],
            ['Messages à valider', c.brouillons_proposes]].map(([l, v]) => (
            <div className="card" key={l}><div className="num">{v}</div><div className="lbl">{l}</div></div>
          ))}
        </div>
      )}

      {(p?.alertes_critiques || []).map((d) => (
        <div className="item critical" key={d.id}>
          <span className="tag">{DET_LABELS[d.type] || d.type}</span> <b>{d.titre}</b>
          <p>{d.detail}</p>
        </div>
      ))}

      <h3>Répartition des engagements en cours</h3>
      <div className="chips">
        {Object.entries(p?.par_type || {}).sort((a, b) => b[1] - a[1]).map(([t, n]) => (
          <span className="chip clickable" key={t} onClick={() => navigate('/engagements')}>
            <TypeBadge type={t} /> <b>{n}</b>
          </span>
        ))}
        {!Object.keys(p?.par_type || {}).length && <Empty>Aucun engagement actif.</Empty>}
      </div>

      <div className="grid-2">
        <div>
          <h3>🔴 En retard</h3>
          <EngTable engs={p?.en_retard} refresh={load} emptyText="Rien en retard. 👌" />
        </div>
        <div>
          <h3>📅 Jalons — 14 prochains jours</h3>
          <EngTable engs={p?.jalons_14_jours} compact emptyText="Aucun jalon confirmé sur 14 jours." />
        </div>
      </div>

      <h3>⚡ Actions rapides</h3>
      {!quickWinCount && <Empty>Aucune action en attente — tout est traité.</Empty>}
      {(qw.brouillons_a_valider || []).map((d) => (
        <div className="item" key={'d' + d.id}>
          <span className="tag">✉ message prêt</span> <b>{d.subject}</b>
          <p>À {d.to_email}{d.detection_titre ? ' · ' + d.detection_titre : ''}</p>
          <div className="row-actions">
            <button className="primary" onClick={() => validateDraft(d.id)}>✓ Valider et envoyer</button>
            <button onClick={() => navigate('/alertes')}>Voir / modifier</button>
          </div>
        </div>
      ))}
      {(qw.echeances_a_confirmer || []).map((e) => (
        <div className="item" key={'e' + e.id}>
          <span className="tag warn-tag">échéance à confirmer</span> <b>{e.objet}</b>
          <p>Date déduite : {fmtDate(e.echeance)} — confirmez pour activer la surveillance. <ConfBar value={e.confiance} /></p>
          <div className="row-actions">
            <button className="primary" onClick={() => confirmEcheance(e.id, (e.echeance || '').slice(0, 10), load)}>📅 Confirmer</button>
          </div>
        </div>
      ))}
      {(qw.liens_a_trancher || []).map((l) => (
        <div className="item" key={'l' + l.id}>
          <span className="tag candidat">dépendance à trancher</span>
          <p><b>{l.amont_objet}</b> conditionne-t-il <b>{l.aval_objet}</b> ?</p>
          <div className="row-actions">
            <button className="primary" onClick={() => linkAction(l.id, 'confirm')}>Oui</button>
            <button onClick={() => linkAction(l.id, 'reject')}>Non</button>
          </div>
        </div>
      ))}
    </section>
  )
}

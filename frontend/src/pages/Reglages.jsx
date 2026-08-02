import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import { fmtDT } from '../components/ui'

export default function Reglages() {
  const [settings, setSettings] = useState({ seuil_publication: '0.6', digest_type: 'quotidien', digest_email: '0' })
  const [status, setStatus] = useState(null)
  const [kpis, setKpis] = useState(null)
  const [audit, setAudit] = useState([])

  const load = useCallback(() => {
    api('/settings').then((s) => setSettings((p) => ({ ...p, ...s }))).catch((e) => toast(e.message, true))
    api('/status').then(setStatus).catch(() => {})
    api('/kpis').then(setKpis).catch(() => {})
    api('/audit?limit=100').then((r) => setAudit(r || [])).catch(() => {})
  }, [])
  useEffect(load, [load])

  const save = async () => {
    const seuil = parseFloat(String(settings.seuil_publication).replace(',', '.'))
    if (isNaN(seuil) || seuil < 0 || seuil > 1) { toast('Le seuil doit être un nombre entre 0 et 1', true); return }
    try {
      await api('/settings', { method: 'PUT', body: JSON.stringify({ ...settings, seuil_publication: String(seuil) }) })
      localStorage.setItem('digest_type', settings.digest_type)
      toast('Réglages enregistrés')
    } catch (e) { toast(e.message, true) }
  }
  const connectGmail = async () => {
    try { const r = await api('/oauth/google/start', { method: 'POST' }); window.location.href = r.url }
    catch (e) { toast(e.message, true) }
  }
  const onboard = async () => {
    try { await api('/onboarding/run', { method: 'POST' }); toast('Onboarding lancé — le miroir sera prêt dans quelques minutes.') }
    catch (e) { toast(e.message, true) }
  }

  const pct = (v) => (v * 100).toFixed(0) + ' %'
  const kpiRows = kpis ? [
    ['Précision estimée', pct(kpis.precision_estimee), 'cible > 85 %', kpis.precision_estimee > 0.85],
    ['Faux positifs', pct(kpis.taux_faux_positifs), 'cible < 10 %', kpis.taux_faux_positifs < 0.10],
    ['Actions validées', pct(kpis.taux_validation_actions), 'cible > 40 %', kpis.taux_validation_actions > 0.40],
    ['Corrections / 7 j', kpis.corrections_7_jours, 'cible < 3', kpis.corrections_7_jours < 3],
    ['Exclusion pré-filtre', pct(kpis.taux_exclusion_prefiltre), 'économie IA', true],
    ['Incidents critiques', kpis.incidents_critiques, 'cible : 0', kpis.incidents_critiques === 0],
  ] : []

  const set = (k) => (e) => setSettings({ ...settings, [k]: e.target.value })

  return (
    <section>
      <div className="page-head"><div><h2>Réglages</h2></div></div>
      <div className="grid-2">
        <div className="panel">
          <h3>Connexion</h3>
          <div className="setting">
            <label>Canal email</label>
            <div className="muted">{status ? (status.canal_connecte ? `Connecté (${status.canal}${status.compte ? ' : ' + status.compte : ''})` : 'Non connecté') : '…'}</div>
            <button onClick={connectGmail}>Connecter Gmail</button>
          </div>
          <div className="setting">
            <label>Onboarding (relire 30 jours + miroir + capsule)</label>
            <button onClick={onboard}>Relancer l'onboarding</button>
          </div>
        </div>
        <div className="panel">
          <h3>Comportement</h3>
          <div className="setting">
            <label>Seuil de publication (0–1) — sous ce score, rien n'est présenté proactivement</label>
            <input type="number" step="0.05" min="0" max="1" value={settings.seuil_publication} onChange={set('seuil_publication')} />
          </div>
          <div className="setting">
            <label>Rythme du digest</label>
            <select value={settings.digest_type} onChange={set('digest_type')}>
              <option value="quotidien">Quotidien</option>
              <option value="hebdo">Hebdomadaire</option>
            </select>
          </div>
          <div className="setting">
            <label>Recevoir le digest par email</label>
            <select value={settings.digest_email} onChange={set('digest_email')}>
              <option value="0">Non — tableau de bord uniquement</option>
              <option value="1">Oui — sur ma boîte</option>
            </select>
          </div>
          <button className="primary" onClick={save}>Enregistrer</button>
        </div>
      </div>

      <h3>Indicateurs de réussite</h3>
      <div className="cards">
        {kpiRows.map(([l, v, cible, ok]) => (
          <div className={'card ' + (ok ? 'ok' : 'warn')} key={l}>
            <div className="num">{v}</div>
            <div className="lbl">{l} · {cible}</div>
          </div>
        ))}
      </div>

      <h3>Journal d'audit <span className="muted">— chaque lecture, détection et action, horodatée</span></h3>
      <div className="tbl-wrap">
        <table className="tbl">
          <thead><tr><th>Date</th><th>Acteur</th><th>Événement</th><th>Détails</th></tr></thead>
          <tbody>
            {audit.map((e) => (
              <tr key={e.id}>
                <td className="sub">{fmtDT(e.ts)}</td>
                <td>{e.actor}</td>
                <td>{e.event_type}</td>
                <td className="mono">{JSON.stringify(e.payload).slice(0, 140)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}

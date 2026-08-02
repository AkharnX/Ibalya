import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import { Empty, fmtDate } from '../components/ui'

export default function Agent() {
  const [facts, setFacts] = useState('{}')
  const [intentions, setIntentions] = useState({ priorites: '', surveillance: '', energie: '' })
  const [rules, setRules] = useState([])

  const load = useCallback(() => {
    api('/capsule').then((c) => {
      setFacts(JSON.stringify(c.facts || {}, null, 2))
      const i = c.intentions || {}
      setIntentions({ priorites: i.priorites || '', surveillance: i.surveillance || '', energie: i.energie || '' })
    }).catch((e) => toast(e.message, true))
    api('/rules').then((r) => setRules(r || [])).catch((e) => toast(e.message, true))
  }, [])
  useEffect(load, [load])

  const save = async () => {
    let parsed
    try { parsed = JSON.parse(facts) } catch { toast('Le texte des faits doit rester un JSON valide', true); return }
    try {
      await api('/capsule', { method: 'PUT', body: JSON.stringify({ facts: parsed, intentions }) })
      toast("Enregistré — l'agent en tiendra compte dès le prochain cycle")
    } catch (e) { toast(e.message, true) }
  }
  const infer = async () => {
    toast('Inférence en cours…')
    try { await api('/capsule/infer', { method: 'POST' }); toast('Faits ré-inférés'); load() }
    catch (e) { toast(e.message, true) }
  }
  const delRule = async (id) => {
    try { await api('/rules/' + id, { method: 'DELETE' }); toast('Règle désactivée'); load() }
    catch (e) { toast(e.message, true) }
  }

  const setInt = (k) => (e) => setIntentions({ ...intentions, [k]: e.target.value })

  return (
    <section>
      <div className="page-head">
        <div>
          <h2>Votre agent</h2>
          <p className="help">Ce que l'agent sait de votre activité, et ce qu'il a appris de vos corrections. Tout est lisible et réversible — pas de boîte noire.</p>
        </div>
        <div className="page-actions"><button onClick={infer}>⟳ Ré-inférer les faits</button></div>
      </div>

      <h3>Ce que je comprends de votre activité</h3>
      <p className="help">Déduit de vos emails — corrigez ce qui est faux, cela guide l'extraction et la priorisation.</p>
      <textarea rows={12} spellCheck={false} value={facts} onChange={(e) => setFacts(e.target.value)} />

      <h3>Vos intentions</h3>
      <label>Quelles sont vos 2-3 priorités ou inquiétudes du moment ?</label>
      <textarea rows={2} value={intentions.priorites} onChange={setInt('priorites')} />
      <label>Y a-t-il des clients, dossiers ou personnes à surveiller particulièrement ?</label>
      <textarea rows={2} value={intentions.surveillance} onChange={setInt('surveillance')} />
      <label>Qu'est-ce qui vous coûte le plus de temps ou d'énergie aujourd'hui ?</label>
      <textarea rows={2} value={intentions.energie} onChange={setInt('energie')} />
      <div className="page-actions"><button className="primary" onClick={save}>Enregistrer</button></div>

      <h3>Ce que j'ai appris de vos corrections</h3>
      {!rules.length ? (
        <Empty>Aucune règle pour l'instant — corrigez un engagement d'un geste pour en créer une.</Empty>
      ) : (
        <div className="tbl-wrap">
          <table className="tbl">
            <thead><tr><th>Règle</th><th>Portée</th><th>Apprise le</th><th></th></tr></thead>
            <tbody>
              {rules.map((r) => (
                <tr key={r.id} style={r.active ? undefined : { opacity: 0.45 }}>
                  <td className="obj">{r.note}</td>
                  <td>{r.portee_type}{r.portee_cible ? ' · ' + r.portee_cible : ''}</td>
                  <td className="sub">{fmtDate(r.created_at)}</td>
                  <td>{r.active
                    ? <button className="ghost" onClick={() => delRule(r.id)}>Désactiver</button>
                    : <span className="sub">désactivée</span>}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}

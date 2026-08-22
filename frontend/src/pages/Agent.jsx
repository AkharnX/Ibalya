import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import { Empty, fmtDate } from '../components/ui'
import ListEditor from '../components/ListEditor'

// Champs connus de la capsule, présentés en formulaire. Les clés inconnues
// renvoyées par le modèle sont conservées telles quelles à l'enregistrement.
const CHAMPS_CONNUS = [
  'secteur', 'description', 'cycle_type', 'clients_recurrents',
  'fournisseurs_critiques', 'interlocuteurs_cles', 'horizon_jours', 'silence_defaut_heures',
]

export default function Agent() {
  const [facts, setFacts] = useState({})
  const [intentions, setIntentions] = useState({ priorites: '', surveillance: '', energie: '' })
  const [rules, setRules] = useState([])
  const [saving, setSaving] = useState(false)

  const load = useCallback(() => {
    api('/capsule').then((c) => {
      setFacts(c.facts && typeof c.facts === 'object' ? c.facts : {})
      const i = c.intentions || {}
      setIntentions({ priorites: i.priorites || '', surveillance: i.surveillance || '', energie: i.energie || '' })
    }).catch((e) => toast(e.message, true))
    api('/rules').then((r) => setRules(r || [])).catch((e) => toast(e.message, true))
  }, [])
  useEffect(load, [load])

  const set = (k) => (e) => setFacts({ ...facts, [k]: e.target.value })
  const setNum = (k) => (e) => setFacts({ ...facts, [k]: e.target.value === '' ? '' : Number(e.target.value) })
  const setList = (k) => (v) => setFacts({ ...facts, [k]: v })
  const setInt = (k) => (e) => setIntentions({ ...intentions, [k]: e.target.value })

  const save = async () => {
    setSaving(true)
    try {
      // on renvoie l'objet complet : les clés non affichées sont préservées
      const payload = { ...facts }
      for (const k of ['horizon_jours', 'silence_defaut_heures']) {
        if (payload[k] === '' || payload[k] == null) delete payload[k]
      }
      await api('/capsule', { method: 'PUT', body: JSON.stringify({ facts: payload, intentions }) })
      toast("Enregistré — l'agent en tiendra compte dès le prochain cycle")
    } catch (e) { toast(e.message, true) } finally { setSaving(false) }
  }

  const infer = async () => {
    toast("L'agent relit vos échanges…")
    try { await api('/capsule/infer', { method: 'POST' }); toast('Compréhension mise à jour'); load() }
    catch (e) { toast(e.message, true) }
  }

  const delRule = async (id) => {
    try { await api('/rules/' + id, { method: 'DELETE' }); toast('Règle désactivée'); load() }
    catch (e) { toast(e.message, true) }
  }

  const heures = Number(facts.silence_defaut_heures)
  const inconnues = Object.keys(facts).filter((k) => !CHAMPS_CONNUS.includes(k))

  return (
    <section>
      <div className="page-head">
        <div>
          <h1>Règles métier</h1>
          <p>Ce que l'agent a compris de votre activité, et ce qu'il a appris de vos corrections. Tout est modifiable et réversible.</p>
        </div>
        <div className="page-actions">
          <button onClick={infer}>⟳ Relire mes échanges</button>
        </div>
      </div>

      <h3>Ce que je comprends de votre activité</h3>
      <p className="help">
        Déduit automatiquement de vos échanges — corrigez ce qui est faux. Ces informations guident
        l'extraction des engagements et le niveau de priorité des alertes.
      </p>

      <div className="panel form-grid">
        <div className="field">
          <label>Votre activité</label>
          <p className="field-hint">En une ligne : le métier de l'entreprise.</p>
          <input value={facts.secteur || ''} onChange={set('secteur')}
            placeholder="ex. menuiserie et agencement sur mesure" />
        </div>

        <div className="field span-2">
          <label>En quelques mots</label>
          <p className="field-hint">La description que l'agent utilise comme contexte général.</p>
          <textarea rows={3} value={facts.description || ''} onChange={set('description')}
            placeholder="ex. L'entreprise fabrique et pose des éléments en bois pour des particuliers et des collectivités…" />
        </div>

        <div className="field span-2">
          <label>Rythme habituel de vos affaires</label>
          <p className="field-hint">Sert à juger si un délai est normal ou anormal.</p>
          <input value={facts.cycle_type || ''} onChange={set('cycle_type')}
            placeholder="ex. chantiers de 2 à 8 semaines, de la prise de mesures à la pose" />
        </div>

        <ListEditor
          label="Clients récurrents"
          hint="Les clients avec qui vous travaillez régulièrement."
          placeholder="nom ou adresse email"
          value={facts.clients_recurrents} onChange={setList('clients_recurrents')} />

        <ListEditor
          label="Fournisseurs critiques"
          hint="Ceux dont un retard bloque vos propres engagements."
          placeholder="nom ou adresse email"
          value={facts.fournisseurs_critiques} onChange={setList('fournisseurs_critiques')} />

        <ListEditor
          label="Interlocuteurs clés"
          hint="Les personnes qui comptent, chez vous ou chez vos partenaires."
          placeholder="nom ou adresse email"
          value={facts.interlocuteurs_cles} onChange={setList('interlocuteurs_cles')} />

        <div className="field">
          <label>Me prévenir combien de jours avant une échéance</label>
          <p className="field-hint">Un chantier long se surveille plus tôt qu'une livraison express.</p>
          <input type="number" min="1" max="60" value={facts.horizon_jours ?? ''} onChange={setNum('horizon_jours')} />
        </div>

        <div className="field">
          <label>Silence anormal au-delà de (heures)</label>
          <p className="field-hint">
            Valeur de départ, le temps que l'agent apprenne le rythme propre à chaque échange.
            {heures > 0 && ` Soit environ ${(heures / 24).toFixed(heures % 24 ? 1 : 0)} jour(s).`}
          </p>
          <input type="number" min="1" max="720" value={facts.silence_defaut_heures ?? ''} onChange={setNum('silence_defaut_heures')} />
        </div>
      </div>

      <h3>Vos intentions</h3>
      <p className="help">L'agent devine les faits, pas vos priorités. Ces trois réponses pondèrent ses alertes.</p>
      <div className="panel form-grid">
        <div className="field span-2">
          <label>Vos 2-3 priorités ou inquiétudes du moment</label>
          <textarea rows={2} value={intentions.priorites} onChange={setInt('priorites')} />
        </div>
        <div className="field span-2">
          <label>Clients, dossiers ou personnes à surveiller particulièrement</label>
          <textarea rows={2} value={intentions.surveillance} onChange={setInt('surveillance')} />
        </div>
        <div className="field span-2">
          <label>Ce qui vous coûte le plus de temps ou d'énergie</label>
          <textarea rows={2} value={intentions.energie} onChange={setInt('energie')} />
        </div>
      </div>

      <div className="page-actions" style={{ marginTop: 16 }}>
        <button className="btn primary" disabled={saving} onClick={save}>
          {saving ? 'Enregistrement…' : 'Enregistrer'}
        </button>
      </div>

      {inconnues.length > 0 && (
        <details className="legend" style={{ marginTop: 20 }}>
          <summary>Informations supplémentaires retenues par l'agent ({inconnues.length})</summary>
          <ul>
            {inconnues.map((k) => (
              <li key={k}><b>{k.replace(/_/g, ' ')}</b> : {
                typeof facts[k] === 'object' ? JSON.stringify(facts[k]) : String(facts[k])
              }</li>
            ))}
          </ul>
          <p className="field-hint">Ces informations sont conservées telles quelles à l'enregistrement.</p>
        </details>
      )}

      <h3>Ce que j'ai appris de vos corrections</h3>
      <p className="help">
        Chaque correction d'un geste devient une règle écrite en clair. C'est ainsi que l'agent
        se cale sur votre réalité en deux à trois semaines.
      </p>
      {!rules.length ? (
        <Empty>Aucune règle pour l'instant — corrigez un engagement pour en créer une.</Empty>
      ) : (
        <div className="tbl-wrap">
          <table>
            <thead><tr><th>Règle</th><th>Portée</th><th>Apprise le</th><th><span className="sr-only">Actions</span></th></tr></thead>
            <tbody>
              {rules.map((r) => (
                <tr key={r.id} style={r.active ? undefined : { opacity: 0.45 }}>
                  <td className="obj">{r.note}</td>
                  <td className="sub">{r.portee_type}{r.portee_cible ? ' · ' + r.portee_cible : ''}</td>
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

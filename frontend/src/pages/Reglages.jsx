import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import { fmtDT } from '../components/ui'
import Canal from '../components/Canal'

// Signature composée depuis les quatre champs. Doit reproduire exactement la
// règle du serveur (engine.SignatureComposee), sans quoi l'aperçu mentirait.
export function signatureComposee(s) {
  const nom = [s.identite_prenom, s.identite_nom].map((x) => (x || '').trim()).filter(Boolean).join(' ')
  if (!nom) return ''
  const role = [s.identite_fonction, s.identite_societe].map((x) => (x || '').trim()).filter(Boolean).join(', ')
  return [nom, role].filter(Boolean).join('\n')
}

// Éditeur de signature. Les quatre champs donnent un point de départ, mais ils
// ne couvrent pas une mention légale, un téléphone ou une seconde adresse :
// le texte reste modifiable, et prime alors sur les champs.
function EditeurSignature({ s, onChange }) {
  const composee = signatureComposee(s)
  const libre = s.identite_signature || ''
  const effective = libre || composee
  return (
    <div className="signature-bloc">
      <label htmlFor="signature">Signature apposée aux messages</label>
      <textarea id="signature" rows={4} value={libre} placeholder={composee || 'Renseignez au moins un prénom'}
        onChange={(e) => onChange(e.target.value)} />
      <div className="signature-pied">
        <span className="sub">
          {libre
            ? 'Texte personnalisé : il remplace les champs ci-dessus.'
            : 'Composée depuis les champs ci-dessus. Écrivez ici pour la remplacer.'}
        </span>
        {libre && (
          <button className="ghost" type="button" onClick={() => onChange('')}>
            Repartir des champs
          </button>
        )}
      </div>
      {effective && (
        <div className="apercu-signature">
          <span className="sub">Aperçu en fin de message</span>
          <pre>{'Cordialement,\n\n' + effective}</pre>
        </div>
      )}
    </div>
  )
}


// Catégories de courrier écartées avant toute inférence. Le CDC les exige
// (« filtres de catégories (RH, juridique, santé), exclusion configurable »),
// et l'argument commercial est direct : un dirigeant qui confie sa messagerie
// demande toujours ce qui arrive aux messages qui ne regardent personne.
const CATEGORIES = [
  { cle: 'sante', titre: 'Santé',
    aide: "Arrêts de travail, certificats médicaux, résultats d'analyses." },
  { cle: 'rh', titre: 'Ressources humaines',
    aide: 'Candidatures, CV, bulletins de paie, ruptures de contrat.' },
  { cle: 'juridique', titre: 'Juridique',
    aide: "Mises en demeure, huissiers, procédures. Souvent porteur d'échéances réelles, donc filtré seulement si vous le demandez." },
]

function Confidentialite({ valeur, onChange, onEnregistrer }) {
  let actives = {}
  try { actives = JSON.parse(valeur || '{}') } catch { actives = {} }
  const bascule = (cle) => () =>
    onChange(JSON.stringify({ ...actives, [cle]: !actives[cle] }))
  return (
    <div className="panel">
      <h3>Confidentialité</h3>
      <p className="help">
        Un message reconnu dans une catégorie cochée est écarté avant l'analyse :
        il n'est jamais envoyé au modèle, et son contenu n'est pas conservé par
        Ibalya. Seuls l'expéditeur et la date restent, pour garder trace de
        l'échange. Votre messagerie, elle, n'est pas touchée.
      </p>
      {CATEGORIES.map((c) => (
        <label className="setting bascule" key={c.cle}>
          <input type="checkbox" checked={!!actives[c.cle]} onChange={bascule(c.cle)} />
          <span>
            <b>{c.titre}</b>
            <span className="help">{c.aide}</span>
          </span>
        </label>
      ))}
      <button className="primary" onClick={onEnregistrer}>Enregistrer</button>
    </div>
  )
}

// Fautes de frappe courantes sur les domaines grand public : une adresse comme
// « dsgh@gmial.com » a un format valide mais un domaine qui n'existe pas. On
// propose la correction plutôt que d'enregistrer une adresse morte.
const DOMAINES_TYPO = {
  'gmial.com': 'gmail.com', 'gmai.com': 'gmail.com', 'gmil.com': 'gmail.com',
  'gnail.com': 'gmail.com', 'gmaill.com': 'gmail.com', 'gmail.co': 'gmail.com',
  'gmail.fr': 'gmail.com', 'hotmial.com': 'hotmail.com', 'hotmai.com': 'hotmail.com',
  'hotmil.com': 'hotmail.com', 'hotmail.fr': 'hotmail.com', 'outlok.com': 'outlook.com',
  'outloo.com': 'outlook.com', 'yaho.com': 'yahoo.com', 'yahooo.com': 'yahoo.com',
  'yahoo.co': 'yahoo.com', 'ornage.fr': 'orange.fr', 'orane.fr': 'orange.fr',
  'wanadou.fr': 'wanadoo.fr', 'oulook.com': 'outlook.com',
}

// Champ d'adresses à pastilles : on tape une adresse, Entrée (ou virgule) la
// transforme en pastille retirable. Plus lisible qu'un texte libre, et on voit
// d'un coup ce qui est enregistré. La valeur reste une chaîne (une adresse par
// ligne) pour ne rien changer côté serveur.
function ChampAdresses({ value, onChange }) {
  const liste = (value || '').split(/[\s,;]+/).map((s) => s.trim()).filter(Boolean)
  const [saisie, setSaisie] = useState('')
  const estEmail = (e) => /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(e)
  const maj = (arr) => onChange(arr.join('\n'))
  const ajouter = (txt) => {
    const e = (txt || '').trim().replace(/[,;]+$/, '')
    if (!e) return true
    if (!estEmail(e)) { toast('Adresse invalide : ' + e, true); return false }
    const domaine = e.split('@')[1].toLowerCase()
    if (DOMAINES_TYPO[domaine]) {
      const propose = e.slice(0, e.lastIndexOf('@') + 1) + DOMAINES_TYPO[domaine]
      toast('Domaine inconnu : vouliez-vous dire ' + propose + ' ? Corrigez l’adresse.', true)
      return false
    }
    if (!liste.some((x) => x.toLowerCase() === e.toLowerCase())) maj([...liste, e])
    setSaisie('')
    return true
  }
  const retirer = (e) => maj(liste.filter((x) => x !== e))
  const onKey = (ev) => {
    if (ev.key === 'Enter' || ev.key === ',' || ev.key === ';') { ev.preventDefault(); ajouter(saisie) }
    else if (ev.key === 'Backspace' && !saisie && liste.length) retirer(liste[liste.length - 1])
  }
  return (
    <div className="adr-champ" onClick={(e) => e.currentTarget.querySelector('input')?.focus()}>
      {liste.map((e) => (
        <span className="adr-chip" key={e}>{e}
          <button type="button" className="adr-x" aria-label={'Retirer ' + e} onClick={() => retirer(e)}>×</button>
        </span>
      ))}
      <input className="adr-input" type="email" value={saisie} autoComplete="off"
        placeholder={liste.length ? 'Ajouter…' : 'perso@gmail.com'}
        onChange={(e) => setSaisie(e.target.value)} onKeyDown={onKey}
        onBlur={() => ajouter(saisie)} />
    </div>
  )
}

export default function Reglages() {
  const [settings, setSettings] = useState({
    seuil_publication: '0.6', digest_type: 'quotidien', digest_email: '0', digest_expediteur: '',
    identite_signature: '',
    identite_prenom: '', identite_nom: '', identite_fonction: '', identite_societe: '',
    adresses_soi: '',
    categories_sensibles: '{"sante":true,"rh":true,"juridique":false}',
    google_mode_test: '1',
  })
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
  const onboard = async () => {
    try { await api('/onboarding/run', { method: 'POST' }); toast('Onboarding lancé, le miroir sera prêt dans quelques minutes.') }
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
      <div className="page-head">
        <div>
          <h1>Réglages</h1>
          <p>La connexion au canal, le seuil à partir duquel l'agent vous parle, et la trace de tout ce qu'il a fait.</p>
        </div>
        <div className="page-actions">
          <button className="btn" onClick={() => window.dispatchEvent(new Event('ibalya:assistant-config'))}>
            Assistant de configuration
          </button>
        </div>
      </div>
      <div className="reglages-grille">
        <div className="panel panel-large">
          <h3>Connexion</h3>
          <Canal statut={status} onChange={load} />
          <div className="setting">
            <label>Onboarding (relire 30 jours + miroir + capsule)</label>
            <button onClick={onboard}>Relancer l'onboarding</button>
          </div>
        </div>
        <div className="panel panel-large">
          <h3>Votre identité</h3>
          <p className="help">
            Elle signe les messages que l'agent prépare pour vous. Sans elle, le modèle
            invente un nom : sur douze brouillons il en a produit cinq variantes, dont
            deux qui n'étaient pas le bon prénom.
          </p>
          <div className="form-grid">
            <div className="setting">
              <label htmlFor="prenom">Prénom</label>
              <input id="prenom" value={settings.identite_prenom} onChange={set('identite_prenom')} />
            </div>
            <div className="setting">
              <label htmlFor="nom">Nom</label>
              <input id="nom" value={settings.identite_nom} onChange={set('identite_nom')} />
            </div>
            <div className="setting">
              <label htmlFor="fonction">Fonction</label>
              <input id="fonction" placeholder="Gérant, CTO…" value={settings.identite_fonction} onChange={set('identite_fonction')} />
            </div>
            <div className="setting">
              <label htmlFor="societe">Société</label>
              <input id="societe" value={settings.identite_societe} onChange={set('identite_societe')} />
            </div>
          </div>
          <EditeurSignature s={settings} onChange={(v) => setSettings((p) => ({ ...p, identite_signature: v }))} />
          <div className="setting">
            <label htmlFor="adresses_soi">Mes autres adresses</label>
            <ChampAdresses value={settings.adresses_soi}
              onChange={(v) => setSettings((p) => ({ ...p, adresses_soi: v }))} />
            <p className="help">Vos autres adresses (perso, alias) : tapez-en une et appuyez sur Entrée. L’agent ne vous
              prendra plus pour un interlocuteur quand vous apparaissez via l’une d’elles.</p>
          </div>
          <button className="primary" onClick={save}>Enregistrer</button>
        </div>
        <div className="panel">
          <h3>Comportement</h3>
          <div className="setting">
            <label>Seuil de publication (0–1) : sous ce score, rien n'est présenté proactivement</label>
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
            <label>Mode de l'application Google</label>
            <select value={settings.google_mode_test} onChange={set('google_mode_test')}>
              <option value="1">Test : jeton à renouveler tous les 7 jours</option>
              <option value="0">Production : connexion durable</option>
            </select>
            <p className="help">Reflète le statut de publication dans Google Cloud. En mode Test, un rappel de reconnexion s'affiche avant l'expiration.</p>
          </div>
          <div className="setting">
            <label>Recevoir le digest par email</label>
            <select value={settings.digest_email} onChange={set('digest_email')}>
              <option value="0">Non : tableau de bord uniquement</option>
              <option value="1">Oui : sur ma boîte</option>
            </select>
          </div>
          <div className="setting">
            <label htmlFor="expediteur">Adresse d'expédition du digest</label>
            <input id="expediteur" type="email" placeholder="digest@ibalya.com"
              value={settings.digest_expediteur} onChange={set('digest_expediteur')} />
            <p className="help">Doit être vérifiée dans Gmail, sinon Google refuse l'envoi. Vide, le digest part de votre adresse.</p>
          </div>
          <button className="primary" onClick={save}>Enregistrer</button>
        </div>
        <Confidentialite
          valeur={settings.categories_sensibles}
          onChange={(v) => setSettings((p) => ({ ...p, categories_sensibles: v }))}
          onEnregistrer={save} />
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

      <h3>Journal d'audit <span className="muted">: chaque lecture, détection et action, horodatée</span></h3>
      <div className="tbl-wrap">
        <table>
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

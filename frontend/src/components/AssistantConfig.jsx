// Assistant de configuration : un pas-à-pas pour les nouveaux venus. Une question
// par écran, l'utilisateur remplit, écran suivant. À la fin, tout est enregistré
// d'un coup dans l'identité (/settings) et les règles métier (/capsule). Il
// s'ouvre tout seul quand la capsule est vide, et se relance depuis les Réglages
// (événement « ibalya:assistant-config »).
import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'

const CLE_FAIT = 'ibalya_assistant_config_fait'
const CLE_PASSE = 'ibalya_assistant_config_passe' // ne pas re-proposer dans la même session

// Clés par destination, pour router l'enregistrement.
const CLES_IDENTITE = ['identite_prenom', 'identite_nom', 'identite_fonction', 'identite_societe']
const CLES_FACTS_TEXTE = ['secteur', 'cycle_type', 'description']
const CLES_FACTS_LISTE = ['clients_recurrents', 'fournisseurs_critiques', 'interlocuteurs_cles']
const CLES_FACTS_NUM = ['horizon_jours', 'silence_defaut_heures']
const CLES_INTENTIONS = ['priorites', 'surveillance', 'energie']

const ETAPES = [
  { type: 'intro' },
  { type: 'duo', q: 'Comment vous présentez-vous ?', aide: "Sert à signer les messages que l'agent prépare. Sans votre nom, il en inventerait un.",
    a: { k: 'identite_prenom', ph: 'Prénom' }, b: { k: 'identite_nom', ph: 'Nom' } },
  { type: 'duo', q: 'Votre fonction et votre société ?', aide: 'Apparaît sous votre nom dans la signature.',
    a: { k: 'identite_fonction', ph: 'ex. Gérante' }, b: { k: 'identite_societe', ph: 'ex. Menuiserie Mercier' } },
  { type: 'text', k: 'secteur', q: 'Quel est votre métier ?', aide: "En une ligne. Cadre toute la compréhension de vos échanges.", ph: 'ex. menuiserie et agencement sur mesure' },
  { type: 'text', k: 'cycle_type', q: 'Le rythme habituel de vos affaires ?', aide: "Sert à juger si un délai est normal ou anormal.", ph: 'ex. chantiers de 2 à 8 semaines' },
  { type: 'textarea', k: 'description', q: 'En quelques mots, votre activité ?', aide: 'Deux ou trois phrases de contexte général.', ph: 'ex. Nous fabriquons et posons des éléments bois pour des particuliers et des collectivités ; les devis partent sous une semaine.' },
  { type: 'tags', k: 'clients_recurrents', q: 'Vos clients récurrents ?', aide: 'Ceux avec qui vous travaillez régulièrement. Un par entrée, validez avec Entrée.', ph: 'nom ou adresse email' },
  { type: 'tags', k: 'fournisseurs_critiques', q: 'Vos fournisseurs critiques ?', aide: 'Ceux dont un retard bloque vos propres engagements.', ph: 'nom ou adresse email' },
  { type: 'tags', k: 'interlocuteurs_cles', q: 'Vos interlocuteurs clés ?', aide: 'Les personnes qui comptent, chez vous ou chez vos partenaires. Optionnel.', ph: 'nom ou adresse email' },
  { type: 'number', k: 'horizon_jours', q: 'Combien de jours avant une échéance vous prévenir ?', aide: 'Un chantier long se surveille plus tôt qu\'une livraison express.', ph: '7', min: 1, max: 60 },
  { type: 'number', k: 'silence_defaut_heures', q: 'Au-delà de combien d\'heures un silence devient-il anormal ?', aide: "Valeur de départ ; l'agent affine ensuite au rythme de chaque échange.", ph: '72', min: 1, max: 720 },
  { type: 'textarea', k: 'priorites', q: 'Vos deux ou trois priorités du moment ?', aide: "L'agent devine les faits, pas vos priorités. Ces réponses pèsent sur l'ordre des alertes.", ph: 'ex. boucler le chantier de la mairie ; relancer les devis en attente' },
  { type: 'textarea', k: 'surveillance', q: 'Qui ou quoi surveiller de près ?', aide: '', ph: 'ex. la scierie Legrand, souvent en retard' },
  { type: 'textarea', k: 'energie', q: 'Ce qui vous coûte le plus de temps ?', aide: '', ph: 'ex. courir après les validations de devis' },
  { type: 'fin' },
]

const estFait = () => { try { return localStorage.getItem(CLE_FAIT) === '1' } catch { return false } }
const estPasse = () => { try { return sessionStorage.getItem(CLE_PASSE) === '1' } catch { return false } }

function ChampTags({ valeur, onChange, placeholder }) {
  const [draft, setDraft] = useState('')
  const items = Array.isArray(valeur) ? valeur : []
  const ajouter = () => { const v = draft.trim(); if (!v) return; onChange([...items, v]); setDraft('') }
  return (
    <div className="wiz-tags">
      <div className="tag-list">
        {items.map((it, i) => (
          <span className="tag-edit" key={i}>{typeof it === 'string' ? it : (it.nom || it.email || '')}
            <button type="button" aria-label="Retirer" title="Retirer" onClick={() => onChange(items.filter((_, j) => j !== i))}>×</button>
          </span>
        ))}
        {!items.length && <span className="field-hint">Aucun pour l'instant.</span>}
      </div>
      <div className="tag-add">
        <input value={draft} placeholder={placeholder} onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); ajouter() } }} autoFocus />
        <button type="button" onClick={ajouter}>Ajouter</button>
      </div>
    </div>
  )
}

export default function AssistantConfig() {
  const [ouvert, setOuvert] = useState(false)
  const [idx, setIdx] = useState(0)
  const [val, setVal] = useState({})
  const [saving, setSaving] = useState(false)

  const prefill = useCallback((capsule, settings) => {
    const f = capsule?.facts || {}
    const i = capsule?.intentions || {}
    const s = settings || {}
    setVal({
      identite_prenom: s.identite_prenom || '', identite_nom: s.identite_nom || '',
      identite_fonction: s.identite_fonction || '', identite_societe: s.identite_societe || '',
      secteur: f.secteur || '', cycle_type: f.cycle_type || '', description: f.description || '',
      clients_recurrents: Array.isArray(f.clients_recurrents) ? f.clients_recurrents : [],
      fournisseurs_critiques: Array.isArray(f.fournisseurs_critiques) ? f.fournisseurs_critiques : [],
      interlocuteurs_cles: Array.isArray(f.interlocuteurs_cles) ? f.interlocuteurs_cles : [],
      horizon_jours: f.horizon_jours ?? '', silence_defaut_heures: f.silence_defaut_heures ?? '',
      priorites: i.priorites || '', surveillance: i.surveillance || '', energie: i.energie || '',
    })
  }, [])

  useEffect(() => {
    let annule = false
    const ouvrir = async () => {
      try {
        const [c, s] = await Promise.all([api('/capsule').catch(() => ({})), api('/settings').catch(() => ({}))])
        if (annule) return
        prefill(c, s)
        setIdx(0); setOuvert(true)
      } catch { /* ignoré */ }
    }
    window.addEventListener('ibalya:assistant-config', ouvrir)
    // Auto pour les nouveaux : capsule vide, jamais terminé, pas passé cette session.
    if (!estFait() && !estPasse()) {
      api('/capsule').then((c) => {
        if (annule) return
        const f = c?.facts || {}
        const vide = !f.secteur && !f.description && !(f.clients_recurrents || []).length
        if (vide) api('/settings').then((s) => { if (!annule) { prefill(c, s); setOuvert(true) } }).catch(() => { if (!annule) { prefill(c, {}); setOuvert(true) } })
      }).catch(() => {})
    }
    return () => { annule = true; window.removeEventListener('ibalya:assistant-config', ouvrir) }
  }, [prefill])

  const set = (k) => (e) => setVal((v) => ({ ...v, [k]: e.target.value }))
  const setListe = (k) => (liste) => setVal((v) => ({ ...v, [k]: liste }))

  const fermer = (passe) => {
    if (passe) { try { sessionStorage.setItem(CLE_PASSE, '1') } catch { /* sans effet */ } }
    setOuvert(false)
  }

  const terminer = async () => {
    setSaving(true)
    try {
      const s = await api('/settings').catch(() => ({}))
      const identite = {}
      for (const k of CLES_IDENTITE) identite[k] = (val[k] || '').trim()
      await api('/settings', { method: 'PUT', body: JSON.stringify({ ...s, ...identite }) })

      const c = await api('/capsule').catch(() => ({}))
      const facts = { ...(c.facts || {}) }
      for (const k of CLES_FACTS_TEXTE) { const t = (val[k] || '').trim(); if (t) facts[k] = t }
      for (const k of CLES_FACTS_LISTE) { if (Array.isArray(val[k]) && val[k].length) facts[k] = val[k] }
      for (const k of CLES_FACTS_NUM) { const n = Number(val[k]); if (val[k] !== '' && val[k] != null && !isNaN(n)) facts[k] = n }
      const intentions = { ...(c.intentions || {}) }
      for (const k of CLES_INTENTIONS) intentions[k] = (val[k] || '').trim()
      await api('/capsule', { method: 'PUT', body: JSON.stringify({ facts, intentions }) })

      try { localStorage.setItem(CLE_FAIT, '1') } catch { /* sans effet */ }
      toast("Configuration enregistrée, votre agent en tient compte dès le prochain cycle.")
      setOuvert(false)
    } catch (e) { toast(e.message, true) } finally { setSaving(false) }
  }

  if (!ouvert) return null
  const et = ETAPES[idx]
  const dernier = idx === ETAPES.length - 1
  // Progression sur les écrans à question (hors intro et fin).
  const totalQ = ETAPES.length - 2
  const numQ = Math.min(Math.max(idx, 1), totalQ)

  return (
    <div className="overlay open wiz-overlay">
      <div className="wiz" role="dialog" aria-modal="true" aria-label="Assistant de configuration">
        {et.type !== 'intro' && et.type !== 'fin' && (
          <button type="button" className="wiz-fermer" onClick={() => fermer(true)}>Plus tard</button>
        )}
        {et.type === 'intro' ? (
          <div className="wiz-corps">
            <div className="wiz-kicker">Configuration</div>
            <h2 className="wiz-q">Configurons votre agent</h2>
            <p className="wiz-aide">Quelques questions, cinq minutes, une fois pour toutes. Elles disent à Ibalya qui vous êtes et comment tourne votre activité : ses alertes et ses messages n'en seront que plus justes. Tout reste modifiable ensuite dans les Réglages.</p>
            <div className="wiz-foot">
              <button className="ghost" onClick={() => fermer(true)}>Plus tard</button>
              <button className="btn primary" onClick={() => setIdx(1)}>Commencer</button>
            </div>
          </div>
        ) : et.type === 'fin' ? (
          <div className="wiz-corps">
            <div className="wiz-kicker">Presque fini</div>
            <h2 className="wiz-q">Tout est prêt</h2>
            <p className="wiz-aide">J'enregistre vos réponses dans votre identité et vos règles métier. Vous pourrez tout revoir et compléter dans les Réglages à tout moment.</p>
            <div className="wiz-foot">
              <button className="ghost" onClick={() => setIdx(idx - 1)} disabled={saving}>Précédent</button>
              <button className="btn primary" onClick={terminer} disabled={saving}>{saving ? 'Enregistrement…' : 'Terminer'}</button>
            </div>
          </div>
        ) : (
          <div className="wiz-corps">
            <div className="wiz-progress"><span>Étape {numQ} / {totalQ}</span>
              <div className="wiz-barre"><div style={{ width: `${(numQ / totalQ) * 100}%` }} /></div>
            </div>
            <h2 className="wiz-q">{et.q}</h2>
            {et.aide && <p className="wiz-aide">{et.aide}</p>}
            <div className="wiz-champ">
              {et.type === 'duo' && (
                <div className="wiz-duo">
                  <input value={val[et.a.k] || ''} onChange={set(et.a.k)} placeholder={et.a.ph} autoFocus />
                  <input value={val[et.b.k] || ''} onChange={set(et.b.k)} placeholder={et.b.ph} />
                </div>
              )}
              {et.type === 'text' && (
                <input value={val[et.k] || ''} onChange={set(et.k)} placeholder={et.ph} autoFocus />
              )}
              {et.type === 'textarea' && (
                <textarea rows={3} value={val[et.k] || ''} onChange={set(et.k)} placeholder={et.ph} autoFocus />
              )}
              {et.type === 'number' && (
                <input type="number" min={et.min} max={et.max} value={val[et.k] ?? ''} onChange={set(et.k)} placeholder={et.ph} autoFocus />
              )}
              {et.type === 'tags' && (
                <ChampTags valeur={val[et.k]} onChange={setListe(et.k)} placeholder={et.ph} />
              )}
            </div>
            <div className="wiz-foot">
              <button className="ghost" onClick={() => setIdx(idx - 1)}>Précédent</button>
              <div className="wiz-foot-droite">
                <button className="ghost" onClick={() => setIdx(idx + 1)}>Passer</button>
                <button className="btn primary" onClick={() => setIdx(idx + 1)}>{dernier ? 'Terminer' : 'Suivant'}</button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

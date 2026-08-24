// Raccordement de la boîte email depuis l'interface.
//
// Le connecteur IMAP existait sans porte d'entrée : brancher autre chose que
// Gmail supposait d'éditer le fichier d'environnement sur le serveur et de
// redémarrer. Un client ne peut pas faire ça.
import { useCallback, useEffect, useState } from 'react'
import { api, toast } from '../api'
import Icone from './Icone'

// Les hôtes courants, pour éviter au dirigeant d'aller les chercher.
const PRESETS = [
  { cle: 'ovh', nom: 'OVH', hote: 'ssl0.ovh.net', smtp: 'ssl0.ovh.net' },
  { cle: 'gandi', nom: 'Gandi', hote: 'mail.gandi.net', smtp: 'mail.gandi.net' },
  { cle: 'yahoo', nom: 'Yahoo', hote: 'imap.mail.yahoo.com', smtp: 'smtp.mail.yahoo.com' },
  { cle: 'orange', nom: 'Orange', hote: 'imap.orange.fr', smtp: 'smtp.orange.fr' },
  { cle: 'free', nom: 'Free', hote: 'imap.free.fr', smtp: 'smtp.free.fr' },
  { cle: 'autre', nom: 'Autre', hote: '', smtp: '' },
]

const VIDE = {
  type: 'imap', hote: '', port: 993, utilisateur: '', mot_de_passe: '',
  dossier: 'INBOX', smtp_hote: '', smtp_port: 587,
}

export default function Canal({ statut, onChange }) {
  const [conf, setConf] = useState(null)
  const [mode, setMode] = useState('gmail')
  const [essai, setEssai] = useState(null)   // { ok, message }
  const [busy, setBusy] = useState('')

  const charger = useCallback(() => {
    api('/canal').then((c) => {
      setConf({ ...VIDE, ...c, mot_de_passe: '' })
      setMode(c.type === 'imap' ? 'imap' : 'gmail')
    }).catch((e) => toast(e.message, true))
  }, [])
  useEffect(charger, [charger])

  if (!conf) return <p className="help">Chargement…</p>

  const set = (champ) => (e) => {
    const v = e.target.type === 'number' ? Number(e.target.value) : e.target.value
    setConf((p) => ({ ...p, [champ]: v }))
    setEssai(null) // toute modification invalide l'épreuve précédente
  }

  const appliquerPreset = (cle) => {
    const p = PRESETS.find((x) => x.cle === cle)
    if (!p || !p.hote) return
    setConf((c) => ({ ...c, hote: p.hote, smtp_hote: p.smtp }))
    setEssai(null)
  }

  const connecterGmail = async () => {
    try { const r = await api('/oauth/google/start', { method: 'POST' }); window.location.href = r.url }
    catch (e) { toast(e.message, true) }
  }

  const tester = async () => {
    setBusy('test'); setEssai(null)
    try {
      const r = await api('/canal/tester', { method: 'POST', body: JSON.stringify(conf) })
      setEssai({ ok: true, message: r.message })
    } catch (e) {
      setEssai({ ok: false, message: e.message })
    } finally { setBusy('') }
  }

  const enregistrer = async () => {
    setBusy('save')
    try {
      const r = await api('/canal', { method: 'PUT', body: JSON.stringify(conf) })
      setConf({ ...VIDE, ...r, mot_de_passe: '' })
      setEssai({ ok: true, message: 'Boîte raccordée. L’agent la lit dès le prochain cycle.' })
      toast('Boîte raccordée')
      onChange?.()
    } catch (e) {
      setEssai({ ok: false, message: e.message })
    } finally { setBusy('') }
  }

  return (
    <>
      <div className="setting">
        <label>État</label>
        <div className="muted">
          {statut
            ? statut.canal_connecte
              ? `Connecté (${statut.canal}${statut.compte ? ' : ' + statut.compte : ''})`
              : 'Aucune boîte raccordée'
            : '…'}
        </div>
      </div>

      <div className="setting">
        <label>Fournisseur</label>
        <div className="choix-canal">
          <label className={mode === 'gmail' ? 'actif' : ''}>
            <input type="radio" checked={mode === 'gmail'} onChange={() => setMode('gmail')} />
            Gmail
          </label>
          <label className={mode === 'imap' ? 'actif' : ''}>
            <input type="radio" checked={mode === 'imap'} onChange={() => setMode('imap')} />
            Autre boîte (IMAP)
          </label>
        </div>
      </div>

      {mode === 'gmail' ? (
        <>
          <p className="help">
            Le raccordement passe par Google : Ibalya n’a jamais votre mot de passe.
          </p>
          <button className="primary" onClick={connecterGmail}>Connecter Gmail</button>
        </>
      ) : (
        <>
          <div className="setting">
            <label htmlFor="preset">Hébergeur</label>
            <select id="preset" defaultValue="" onChange={(e) => appliquerPreset(e.target.value)}>
              <option value="">Choisir pour préremplir…</option>
              {PRESETS.map((p) => <option key={p.cle} value={p.cle}>{p.nom}</option>)}
            </select>
          </div>

          <div className="form-grid">
            <div className="setting">
              <label htmlFor="hote">Serveur IMAP</label>
              <input id="hote" value={conf.hote} onChange={set('hote')} placeholder="imap.exemple.fr" />
            </div>
            <div className="setting">
              <label htmlFor="port">Port</label>
              <input id="port" type="number" value={conf.port} onChange={set('port')} />
            </div>
            <div className="setting">
              <label htmlFor="user">Adresse email</label>
              <input id="user" type="email" value={conf.utilisateur} onChange={set('utilisateur')} />
            </div>
            <div className="setting">
              <label htmlFor="mdp">Mot de passe</label>
              <input id="mdp" type="password" value={conf.mot_de_passe} onChange={set('mot_de_passe')}
                placeholder={conf.mot_de_passe_enregistre ? '•••••••• (déjà enregistré)' : ''} />
            </div>
            <div className="setting">
              <label htmlFor="smtp">Serveur SMTP</label>
              <input id="smtp" value={conf.smtp_hote} onChange={set('smtp_hote')} placeholder="déduit du serveur IMAP" />
            </div>
            <div className="setting">
              <label htmlFor="smtpport">Port SMTP</label>
              <input id="smtpport" type="number" value={conf.smtp_port} onChange={set('smtp_port')} />
            </div>
          </div>

          <p className="help">
            La plupart des fournisseurs exigent un <b>mot de passe d’application</b>, distinct
            de celui de votre compte, à créer depuis leurs réglages de sécurité. Outlook
            professionnel n’accepte plus ce mode et n’est pas encore pris en charge.
          </p>

          {essai && (
            <div className={'essai-canal' + (essai.ok ? ' ok' : ' echec')} role="status">
              <Icone nom={essai.ok ? 'etat-livre' : 'action-rejeter'} />
              <span>{essai.message}</span>
            </div>
          )}

          <div className="row-actions">
            <button onClick={tester} disabled={!!busy || !conf.hote || !conf.utilisateur}>
              {busy === 'test' ? 'Connexion…' : 'Tester la connexion'}
            </button>
            <button className="primary" onClick={enregistrer} disabled={!!busy || !conf.hote || !conf.utilisateur}>
              {busy === 'save' ? 'Enregistrement…' : 'Enregistrer et raccorder'}
            </button>
          </div>
        </>
      )}
    </>
  )
}

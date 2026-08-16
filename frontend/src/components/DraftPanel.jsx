// Panneau latéral de proposition de message : aperçu → modification → envoi.
// Aucun message ne part sans validation explicite (marche 3 du CDC).
import { useEffect, useState } from 'react'
import { api, toast } from '../api'

export function DraftPanel({ draft, loading, title, hint, onClose, onSent }) {
  const [body, setBody] = useState('')
  const [editing, setEditing] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setBody(draft?.body || '')
    setEditing(false)
  }, [draft?.id, draft?.body])

  const open = loading || !!draft

  const save = async () => {
    await api(`/drafts/${draft.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ to_email: draft.to_email, subject: draft.subject, body }),
    })
  }

  const send = async () => {
    if (busy || !draft) return
    setBusy(true)
    try {
      if (body !== draft.body) await save()
      await api(`/drafts/${draft.id}/validate`, { method: 'POST' })
      toast('Message envoyé à ' + draft.to_email)
      onSent?.()
      onClose()
    } catch (e) {
      toast(e.message, true)
    } finally {
      setBusy(false)
    }
  }

  const toggleEdit = async () => {
    if (editing && draft && body !== draft.body) {
      try { await save(); toast('Modifications enregistrées') } catch (e) { toast(e.message, true) }
    }
    setEditing(!editing)
  }

  return (
    <>
      <div className={'overlay' + (open ? ' open' : '')} onClick={onClose} />
      <div className={'draft-panel' + (open ? ' open' : '')}>
        <div className="draft-head">
          <div>
            <h3>{title || 'Proposition de message'}</h3>
            <p>{hint || "Générée par l'agent à partir du contexte de l'engagement"}</p>
          </div>
          <button className="draft-close" onClick={onClose}>✕</button>
        </div>
        <div className="draft-body">
          {loading && <p className="draft-loading">L'agent rédige le message…</p>}
          {!loading && draft && (
            <>
              <span className={'draft-mode-badge ' + (editing ? 'edit' : 'preview')}>
                {editing ? 'Modification' : 'Aperçu'}
              </span>
              <div className="draft-field"><label>À</label><div className="val">{draft.to_email}</div></div>
              <div className="draft-field"><label>Objet</label><div className="val">{draft.subject}</div></div>
              <div className="draft-field">
                <label>Message</label>
                <textarea
                  className="draft-textarea" readOnly={!editing} value={body}
                  onChange={(e) => setBody(e.target.value)}
                />
              </div>
            </>
          )}
        </div>
        {!loading && draft && (
          <div className="draft-foot">
            <button className="btn" onClick={toggleEdit}>{editing ? "Revenir à l'aperçu" : 'Modifier'}</button>
            <button className="btn primary" disabled={busy} onClick={send}>Valider et envoyer</button>
          </div>
        )}
      </div>
    </>
  )
}

export function QueuePanel({ open, drafts, onPick, onClose }) {
  return (
    <>
      <div className={'overlay' + (open ? ' open' : '')} onClick={onClose} />
      <div className={'draft-panel' + (open ? ' open' : '')}>
        <div className="draft-head">
          <div>
            <h3>Messages à valider</h3>
            <p>{drafts.length} brouillon(s) en attente de votre validation</p>
          </div>
          <button className="draft-close" onClick={onClose}>✕</button>
        </div>
        <div className="draft-body">
          {!drafts.length && <p className="draft-loading">Aucun message en attente.</p>}
          {drafts.map((d) => (
            <button className="queue-row" key={d.id} onClick={() => onPick(d)}>
              <div>
                <p className="qt">{d.detection_titre || d.subject}</p>
                <p className="qs">{d.to_email} — {d.subject}</p>
              </div>
              <span className="qgo">→</span>
            </button>
          ))}
        </div>
      </div>
    </>
  )
}

// Hook partagé : ouverture d'un brouillon (existant ou généré à la demande).
export function useDraft(refresh) {
  const [draft, setDraft] = useState(null)
  const [loading, setLoading] = useState(false)
  const [meta, setMeta] = useState({})

  const openForEngagement = async (engagementId, action) => {
    setLoading(true)
    setMeta({ title: action?.label, hint: action?.hint })
    try {
      const d = await api(`/engagements/${engagementId}/draft`, {
        method: 'POST', body: JSON.stringify({ intent: action?.intent || '' }),
      })
      setDraft(d)
    } catch (e) {
      toast(e.message, true)
      setMeta({})
    } finally {
      setLoading(false)
    }
  }

  const openExisting = (d) => {
    setMeta({ title: d.detection_titre || 'Proposition de message', hint: d.detection_detail })
    setDraft(d)
  }

  const close = () => { setDraft(null); setLoading(false); setMeta({}) }

  return {
    draft, loading, meta, openForEngagement, openExisting, close,
    onSent: () => { setDraft(null); refresh?.() },
  }
}

// Panneau latéral de proposition de message : aperçu → modification → envoi.
// Aucun message ne part sans validation explicite (marche 3 du CDC).
import { useEffect, useState } from 'react'
import { api, toast } from '../api'

const REMARQUE_LABEL = { factuel: 'Fait à vérifier', manque: 'Élément manquant', risque: 'Risque', ton: 'Ton' }

export function DraftPanel({ draft, loading, title, hint, onClose, onSent }) {
  const [body, setBody] = useState('')
  const [editing, setEditing] = useState(false)
  const [busy, setBusy] = useState(false)
  const [review, setReview] = useState(null)
  const [reviewing, setReviewing] = useState(false)

  useEffect(() => {
    setBody(draft?.body || '')
    setEditing(false)
    setReview(null)
  }, [draft?.id, draft?.body])

  // Relecture par l'agent de la version en cours d'édition.
  const askReview = async () => {
    if (reviewing || !draft) return
    setReviewing(true)
    setReview(null)
    try {
      const r = await api(`/drafts/${draft.id}/review`, {
        method: 'POST', body: JSON.stringify({ subject: draft.subject, body }),
      })
      setReview(r)
    } catch (e) { toast(e.message, true) } finally { setReviewing(false) }
  }

  const applySuggestion = () => {
    if (!review?.suggestion) return
    setBody(review.suggestion)
    setReview({ ...review, suggestion: '', applied: true })
    toast('Version de l\'agent appliquée — relisez avant d\'envoyer')
  }

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
                  onChange={(e) => { setBody(e.target.value); if (review) setReview(null) }}
                />
              </div>

              {editing && (
                <button className="btn" disabled={reviewing} onClick={askReview}>
                  {reviewing ? "L'agent relit…" : "⟲ Demander l'avis de l'agent"}
                </button>
              )}

              {review && (
                <div className={'review ' + (review.verdict === 'pret_a_envoyer' ? 'ok' : 'todo')}>
                  <p className="review-verdict">
                    {review.verdict === 'pret_a_envoyer'
                      ? '✓ Rien à signaler — le message peut partir.'
                      : `${review.remarques.length} point(s) à regarder avant d'envoyer`}
                  </p>
                  {review.remarques.map((r, i) => (
                    <p className="review-item" key={i}>
                      <span className={'review-tag ' + r.type}>{REMARQUE_LABEL[r.type] || r.type}</span>
                      {r.message}
                    </p>
                  ))}
                  {review.suggestion && (
                    <>
                      <p className="review-sub">L'agent propose une version corrigée :</p>
                      <pre className="review-suggestion">{review.suggestion}</pre>
                      <button className="btn" onClick={applySuggestion}>Utiliser cette version</button>
                    </>
                  )}
                </div>
              )}
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

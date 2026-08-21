// Panneau de traçabilité : d'où vient cet engagement ?
// Affiche la conversation complète, message d'origine mis en évidence, avec
// un lien direct vers Gmail pour répondre dans le fil réel.
import { useEffect, useState } from 'react'
import { api, toast } from '../api'
import { fmtDT } from './ui'

export default function SourcePanel({ engagementId, onClose }) {
  const [data, setData] = useState(null)
  const [erreur, setErreur] = useState('')

  useEffect(() => {
    if (!engagementId) { setData(null); setErreur(''); return }
    setData(null); setErreur('')
    api(`/engagements/${engagementId}/source`)
      .then(setData)
      .catch((e) => setErreur(e.message))
  }, [engagementId])

  const ouvert = !!engagementId

  return (
    <>
      <div className={'overlay' + (ouvert ? ' open' : '')} onClick={onClose} />
      <div className={'draft-panel source-panel' + (ouvert ? ' open' : '')}>
        <div className="draft-head">
          <div>
            <h3>{data?.sujet || 'Conversation d’origine'}</h3>
            <p>{data ? `${data.messages.length} message(s) dans ce fil` : 'Chargement…'}</p>
          </div>
          <button className="draft-close" onClick={onClose}>✕</button>
        </div>

        <div className="draft-body">
          {erreur && <div className="empty">{erreur}</div>}
          {data && (
            <>
              <p className="context">🔗 Engagement extrait : {data.objet}</p>
              {data.messages.map((m) => (
                <div className={'msg' + (m.est_source ? ' msg-source' : '')} key={m.id}>
                  <div className="msg-head">
                    <div>
                      <b>{m.sender}</b>
                      <span className="msg-to">→ {(m.recipients || []).join(', ') || '—'}</span>
                    </div>
                    <span className="msg-date">{fmtDT(m.sent_at)}</span>
                  </div>
                  {m.est_source && <span className="msg-badge">Message d’origine</span>}
                  {m.status === 'excluded' && (
                    <span className="msg-badge exclu">Écarté par le pré-filtre — non analysé</span>
                  )}
                  <p className="msg-subject">{m.subject}</p>
                  <pre className="msg-body">{m.body || '(corps vide)'}</pre>
                  {m.url_gmail && (
                    <a className="msg-lien" href={m.url_gmail} target="_blank" rel="noreferrer">
                      Ouvrir ce message dans Gmail ↗
                    </a>
                  )}
                </div>
              ))}
            </>
          )}
        </div>

        {data?.url_gmail_fil && (
          <div className="draft-foot">
            <a className="btn primary" href={data.url_gmail_fil} target="_blank" rel="noreferrer">
              Ouvrir le fil dans Gmail ↗
            </a>
          </div>
        )}
      </div>
    </>
  )
}

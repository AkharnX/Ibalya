// Panneau de traçabilité : d'où vient cet engagement ?
// Affiche la conversation complète, message d'origine mis en évidence, avec
// un lien direct vers Gmail pour répondre dans le fil réel.
import { useEffect, useState } from 'react'
import Icone from './Icone'
import { api } from '../api'
import { EVT_LABELS, fmtDT } from './ui'

// Le panneau s'ouvre sur un engagement ou directement sur une conversation.
// Les fils sans réponse du miroir n'ont produit aucun engagement : sans la
// seconde entrée, ils n'étaient consultables nulle part.
export default function SourcePanel({ engagementId, threadId, action, onAction, onClose }) {
  const [data, setData] = useState(null)
  const [events, setEvents] = useState([])
  const [erreur, setErreur] = useState('')

  useEffect(() => {
    if (!engagementId && !threadId) { setData(null); setEvents([]); setErreur(''); return }
    setData(null); setEvents([]); setErreur('')
    const source = engagementId ? `/engagements/${engagementId}/source` : `/threads/${threadId}/source`
    api(source).then(setData).catch((e) => setErreur(e.message))
    if (engagementId) {
      // Journal d'événements (CDC 5.2) : l'état d'un engagement se reconstruit
      // à tout instant, et c'est la matière de l'audit trail.
      api(`/engagements/${engagementId}/events`)
        .then((r) => setEvents(r || []))
        .catch(() => setEvents([]))
    }
  }, [engagementId, threadId])

  const ouvert = !!engagementId || !!threadId

  return (
    <>
      <div className={'overlay' + (ouvert ? ' open' : '')} onClick={onClose} />
      <div className={'draft-panel source-panel' + (ouvert ? ' open' : '')}>
        <div className="draft-head">
          <div>
            <h3>{data?.sujet || 'Conversation d’origine'}</h3>
            <p>{data ? `${data.messages.length} message(s) dans ce fil` : 'Chargement…'}</p>
          </div>
          <button className="draft-close" title="Fermer" onClick={onClose}><Icone nom="action-fermer-panneau" /></button>
        </div>

        <div className="draft-body">
          {erreur && <div className="empty">{erreur}</div>}
          {data && (
            <>
              {data.objet && <p className="context">Engagement extrait : {data.objet}</p>}

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
                      Ouvrir ce message dans Gmail <Icone nom="action-ouvrir-gmail" taille={13} />
                    </a>
                  )}
                </div>
              ))}

              {/* Le fil d'abord : c'est ce qu'on vient lire. L'historique et
                  l'action viennent après, dans l'ordre où on s'en sert. */}
              {events.length > 0 && (
                <details className="journal">
                  <summary>Journal de l'engagement <span className="n">{events.length}</span></summary>
                  <ol className="journal-liste">
                    {events.map((ev) => (
                      <li key={ev.id}>
                        <span className="journal-date">{fmtDT(ev.horodatage)}</span>
                        <span className="journal-type">{EVT_LABELS[ev.type] || ev.type}</span>
                      </li>
                    ))}
                  </ol>
                </details>
              )}

              {/* L'action suggérée s'affichait sur chaque ligne du tableau, ce qui
                  répétait la même phrase à l'identique sur tous les engagements.
                  Elle a sa place ici, où le fil donne le contexte qui la justifie. */}
              {action && onAction && (
                <div className="suggestion">
                  <div>
                    <b>{action.label}</b>
                    {action.hint && <span className="sub">{action.hint}</span>}
                  </div>
                  <button className="btn primary" onClick={() => onAction(action)}>
                    Préparer le message
                  </button>
                </div>
              )}
            </>
          )}
        </div>

        {data?.url_gmail_fil && (
          <div className="draft-foot">
            <a className="btn primary" href={data.url_gmail_fil} target="_blank" rel="noreferrer">
              Ouvrir le fil dans Gmail <Icone nom="action-ouvrir-gmail" taille={14} />
            </a>
          </div>
        )}
      </div>
    </>
  )
}

import { useEffect, useRef, useState } from 'react'
import { api } from '../api'
import Icone from '../components/Icone'
import SourcePanel from '../components/SourcePanel'

// Rendu markdown minimal et sûr : le modèle répond avec du gras (**…**), des
// listes à puces (- …) et des paragraphes. On construit des éléments React
// (jamais de HTML injecté) : pas de dépendance, aucun risque XSS.
function enGras(texte) {
  // Les segments impairs de la découpe sur **…** sont le contenu à mettre en gras.
  return texte.split(/\*\*(.+?)\*\*/g).map((seg, i) =>
    i % 2 === 1 ? <strong key={i}>{seg}</strong> : seg)
}

function renduMarkdown(texte) {
  const blocs = []
  let puces = null
  const viderPuces = (cle) => { if (puces) { blocs.push(<ul key={cle}>{puces}</ul>); puces = null } }
  texte.split('\n').forEach((ligne, idx) => {
    const puce = ligne.match(/^\s*[-*]\s+(.*)$/)
    if (puce) {
      ;(puces ||= []).push(<li key={'li' + idx}>{enGras(puce[1])}</li>)
      return
    }
    viderPuces('ul' + idx)
    if (ligne.trim() !== '') blocs.push(<p key={'p' + idx}>{enGras(ligne)}</p>)
  })
  viderPuces('ul-fin')
  return blocs
}

// Assistant conversationnel : le dirigeant interroge sa boîte en langage
// naturel. L'assistant répond à partir des données déjà extraites (engagements,
// alertes, messages), cloisonnées par tenant côté serveur. Il informe, il
// n'agit pas : aucun envoi ne part d'ici (règle de l'escalier d'agentivité).

const SUGGESTIONS = [
  'Combien de devis sont en attente ?',
  'Qui ne m’a pas répondu depuis plus d’une semaine ?',
  'Quels engagements sont en retard ?',
  'Où en est mon dossier le plus urgent ?',
]

export default function Chat() {
  const [tours, setTours] = useState([]) // { role: 'user'|'assistant', content, sources? }
  const [question, setQuestion] = useState('')
  const [busy, setBusy] = useState(false)
  const [charge, setCharge] = useState(false) // historique chargé ?
  const [filSource, setFilSource] = useState(null) // fil ouvert dans le panneau
  const finRef = useRef(null)
  const champRef = useRef(null)

  // Historisation : au chargement, on relit la conversation persistée côté
  // serveur pour la restaurer telle quelle après un rechargement de page.
  useEffect(() => {
    api('/chat/historique')
      .then((h) => setTours(Array.isArray(h) ? h : []))
      .catch(() => {})
      .finally(() => setCharge(true))
  }, [])

  useEffect(() => {
    finRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [tours, busy])

  const nouvelleConversation = async () => {
    if (busy) return
    try { await api('/chat/historique', { method: 'DELETE' }) } catch (e) { /* on efface localement quand même */ }
    setTours([])
    champRef.current?.focus()
  }

  const envoyer = async (texte) => {
    const q = (texte ?? question).trim()
    if (!q || busy) return
    // On n'envoie que les tours de dialogue au serveur, pas les sources.
    const historique = tours.map((t) => ({ role: t.role, content: t.content }))
    setTours((t) => [...t, { role: 'user', content: q }])
    setQuestion('')
    setBusy(true)
    try {
      const r = await api('/chat', {
        method: 'POST',
        body: JSON.stringify({ question: q, historique }),
      })
      setTours((t) => [...t, { role: 'assistant', content: r.reponse, sources: r.sources || [] }])
    } catch (e) {
      setTours((t) => [...t, {
        role: 'assistant',
        content: 'Je n’ai pas pu répondre (assistant indisponible). Réessaie dans un instant.',
        sources: [],
      }])
    } finally {
      setBusy(false)
      champRef.current?.focus()
    }
  }

  const surTouche = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      envoyer()
    }
  }

  return (
    <div className="chat">
      {tours.length > 0 && (
        <div className="chat-entete">
          <button className="chat-nouvelle" onClick={nouvelleConversation} disabled={busy}>
            <Icone nom="action-rejeter" taille={14} /> Nouvelle conversation
          </button>
        </div>
      )}
      <div className="chat-fil">
        {charge && tours.length === 0 && (
          <div className="chat-accueil">
            <div className="chat-accueil-ic"><Icone nom="nav-assistant" taille={28} /></div>
            <h2>Pose ta question sur ta boîte</h2>
            <p className="help">
              L’assistant répond à partir de ce qu’Ibalya a déjà lu : engagements,
              relances, silences, dossiers. Il informe, il n’envoie rien.
            </p>
            <div className="chat-suggestions">
              {SUGGESTIONS.map((s) => (
                <button key={s} className="chat-chip" onClick={() => envoyer(s)}>{s}</button>
              ))}
            </div>
          </div>
        )}

        {tours.map((t, i) => (
          <div key={i} className={'chat-bulle ' + t.role}>
            <div className="chat-texte">
              {t.role === 'assistant' ? renduMarkdown(t.content) : t.content}
            </div>
            {t.role === 'assistant' && t.sources?.length > 0 && (
              <div className="chat-sources">
                <span className="chat-sources-lbl">Sources</span>
                {t.sources.map((s, j) => s.thread_id ? (
                  <button key={j} className="chat-source lien"
                    title="Voir la conversation d'origine"
                    onClick={() => setFilSource(s.thread_id)}>
                    {s.label}
                  </button>
                ) : (
                  <span key={j} className="chat-source">{s.label}</span>
                ))}
              </div>
            )}
          </div>
        ))}

        {busy && (
          <div className="chat-bulle assistant">
            <div className="chat-texte chat-attente"><span></span><span></span><span></span></div>
          </div>
        )}
        <div ref={finRef} />
      </div>

      <div className="chat-saisie">
        <textarea
          ref={champRef}
          rows={1}
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          onKeyDown={surTouche}
          placeholder="Ex. Quels engagements sont en retard cette semaine ?"
          disabled={busy}
        />
        <button className="primary" onClick={() => envoyer()} disabled={busy || !question.trim()}>
          {busy ? '…' : 'Demander'}
        </button>
      </div>
      <p className="help chat-avertissement">
        L’assistant peut se tromper : vérifie les points importants dans le fil source.
      </p>

      {filSource && (
        <SourcePanel threadId={filSource} onClose={() => setFilSource(null)} />
      )}
    </div>
  )
}

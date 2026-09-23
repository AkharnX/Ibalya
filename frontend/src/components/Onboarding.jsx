// Bandeau d'avancement de l'onboarding.
// La lecture de trente jours de messagerie prend plusieurs minutes : on montre
// des compteurs réels plutôt qu'une animation, parce qu'un chiffre qui monte
// prouve que le travail avance.
import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import Aide from './Aide'
import { api } from '../api'

const ETAPES = ['lecture', 'analyse', 'miroir', 'capsule']

export default function Onboarding() {
  const [etat, setEtat] = useState(null)

  const lire = useCallback(() => {
    api('/onboarding/status').then(setEtat).catch(() => {})
  }, [])

  useEffect(() => {
    lire()
    const t = setInterval(lire, 3000)
    return () => clearInterval(t)
  }, [lire])

  // on cesse d'interroger dès que c'est fini
  useEffect(() => {
    if (etat && !etat.en_cours && etat.phase !== 'erreur') {
      const t = setTimeout(lire, 30000)
      return () => clearTimeout(t)
    }
  }, [etat, lire])

  if (!etat?.phase) return null

  const masquer = async () => {
    try { await api('/onboarding/ack', { method: 'POST' }) } catch { /* sans effet */ }
    setEtat(null)
  }

  const indice = ETAPES.indexOf(etat.phase)
  const erreur = etat.phase === 'erreur'
  const fini = etat.phase === 'termine'

  return (
    <div className={'onb' + (erreur ? ' onb-erreur' : fini ? ' onb-fini' : '')}>
      <div className="onb-tete">
        <div>
          <b>{erreur ? 'Analyse interrompue' : fini ? 'Votre agent est prêt' : etat.libelle}</b>
          <p>
            {erreur ? etat.erreur
              : fini ? <>Vos engagements sont extraits. <Link to="/miroir">Le miroir d’activité</Link> vous attend.</>
              : 'Première lecture de vos trente derniers jours. Vous pouvez continuer à naviguer.'}
          </p>
        </div>
        {(fini || erreur) && <button className="btn" onClick={masquer}>Fermer</button>}
      </div>

      {!erreur && (
        <ol className="onb-etapes">
          {ETAPES.map((e, i) => (
            <li key={e} className={i < indice || fini ? 'faite' : i === indice ? 'active' : ''}>
              {['Lecture', 'Analyse', 'Miroir', 'Compréhension'][i]}
            </li>
          ))}
        </ol>
      )}

      <div className="onb-compteurs">
        <div><b>{etat.messages_lus}</b><span>messages lus<Aide texte="Messages parcourus sur les 30 derniers jours de votre boîte." /></span></div>
        <div><b>{etat.messages_filtres}</b><span>filtrés sans coût d’IA<Aide texte="Messages écartés en amont (newsletters, notifications automatiques…) sans appeler l’IA : ils n’ont aucun coût d’analyse." /></span></div>
        <div><b>{etat.messages_analyses}</b><span>analysés<Aide texte="Messages réellement passés à l’IA pour en extraire des engagements." /></span></div>
        <div><b>{etat.engagements}</b><span>engagements extraits<Aide texte="Promesses et échéances détectées dans les messages analysés (qui a promis quoi, à qui, pour quand)." /></span></div>
      </div>
    </div>
  )
}

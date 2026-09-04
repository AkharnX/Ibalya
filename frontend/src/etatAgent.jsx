// État de l'agent, partagé par toute l'application.
//
// Un cycle dure de trente secondes à deux minutes et le scheduler en lance un
// toutes les quinze minutes. Sans sondage, l'interface reste figée pendant ce
// temps : rien ne distingue « l'agent travaille » de « il ne se passe rien ».
// Un seul sondage vit ici, les pages s'y abonnent — deux sondages
// indépendants doubleraient les requêtes pour la même information.
import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import { api } from './api'

const Contexte = createContext({ statut: null, cycle: {}, rafraichir: () => {} })

const LENT = 12000   // veille : on guette un cycle d'arrière-plan
const RAPIDE = 2000  // cycle en cours : on suit sa progression

export function FournisseurEtatAgent({ actif, children }) {
  const [statut, setStatut] = useState(null)
  const enCoursPrecedent = useRef(false)
  const [finiA, setFiniA] = useState(0) // horodatage de la dernière fin, pour réveiller les pages

  const rafraichir = useCallback(async () => {
    try {
      const st = await api('/status')
      setStatut(st)
      const enCours = !!st.cycle?.en_cours
      // Front montant → descendant : le cycle vient de se terminer, les pages
      // qui affichent des données doivent se recharger.
      if (enCoursPrecedent.current && !enCours) setFiniA(Date.now())
      enCoursPrecedent.current = enCours
      return st
    } catch { return null }
  }, [])

  useEffect(() => {
    if (!actif) return
    let arrete = false
    let minuteur
    const boucle = async () => {
      const st = await rafraichir()
      if (arrete) return
      minuteur = setTimeout(boucle, st?.cycle?.en_cours ? RAPIDE : LENT)
    }
    boucle()
    return () => { arrete = true; clearTimeout(minuteur) }
  }, [actif, rafraichir])

  // Le serveur ne renvoie « secondes » qu'à chaque sondage (toutes les 2 s), si
  // bien que le compteur paraissait figé, surtout sur un cycle court. On le fait
  // avancer localement chaque seconde, calculé depuis l'heure de début fournie
  // par le serveur : précis et vivant, sans dérive.
  const [, tic] = useState(0)
  const enCours = !!statut?.cycle?.en_cours
  useEffect(() => {
    if (!enCours) return
    const id = setInterval(() => tic((n) => n + 1), 1000)
    return () => clearInterval(id)
  }, [enCours])

  const cycleBrut = statut?.cycle || {}
  const secondes = enCours && cycleBrut.debut
    ? Math.max(0, Math.floor((Date.now() - new Date(cycleBrut.debut)) / 1000))
    : cycleBrut.secondes || 0
  const cycle = { ...cycleBrut, secondes }
  return (
    <Contexte.Provider value={{ statut, cycle, finiA, rafraichir }}>
      {children}
    </Contexte.Provider>
  )
}

export const useEtatAgent = () => useContext(Contexte)

// Libellé de l'activité en cours, prêt à afficher.
export function libelleCycle(cycle) {
  if (!cycle?.en_cours) return ''
  const phase = cycle.phase || 'Analyse en cours'
  const s = cycle.secondes || 0
  const duree = s < 60 ? `${s} s` : `${Math.floor(s / 60)} min ${String(s % 60).padStart(2, '0')}`
  return `${phase} · ${duree}`
}

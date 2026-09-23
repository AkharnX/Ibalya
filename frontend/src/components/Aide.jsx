// Petite pastille d'aide : au survol (ou focus clavier), révèle une explication.
// Retour de Stewe : chaque métrique doit pouvoir être expliquée simplement,
// sans quitter l'écran (« une fenêtre liée à la souris indiquant ce que ça
// représente »).
export default function Aide({ texte }) {
  return (
    <span className="aide" tabIndex={0} role="note" aria-label={texte} data-aide={texte}>
      i
    </span>
  )
}

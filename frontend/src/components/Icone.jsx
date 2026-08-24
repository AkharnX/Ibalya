// Icône du jeu Ibalya.
//
// Les tracés sont inlinés plutôt que chargés en <img> : dans une image, le SVG
// est isolé et `currentColor` ne voit plus la couleur du texte environnant, si
// bien que l'icône resterait noire quel que soit le thème.
//
// Le contrat graphique — 24×24, trait 1,5, extrémités droites — vit ici et non
// dans chaque fichier : une icône ne peut pas s'en écarter par accident.
import { ICONES } from './icones'

// dangerouslySetInnerHTML est sans risque ici : le contenu vient d'un module
// généré à la construction depuis des fichiers du dépôt, jamais d'une saisie.

export default function Icone({ nom, taille = 16, titre, className = '' }) {
  const ic = ICONES[nom]
  if (!ic) return null
  // Sans titre explicite, l'icône est décorative : elle accompagne toujours un
  // libellé visible ou un bouton déjà nommé, et n'a rien à annoncer de plus.
  const etiquette = titre === undefined ? null : titre || ic.titre
  return (
    <svg
      className={'ic ' + className}
      width={taille} height={taille} viewBox="0 0 24 24"
      fill="none" stroke="currentColor" strokeWidth="1.5"
      strokeLinecap="butt" strokeLinejoin="miter"
      role={etiquette ? 'img' : undefined}
      aria-label={etiquette || undefined}
      aria-hidden={etiquette ? undefined : 'true'}
      dangerouslySetInnerHTML={{ __html: ic.corps }}
    />
  )
}

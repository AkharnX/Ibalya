# Refonte de la charte graphique Ibalya — vitrine + produit

**Date** : 2026-11-04
**État** : brouillon de conception (en attente de validation)

## Intention et pourquoi

Yacou préfère le design et la charte graphique du dossier
`dossier-2-roi-simulateurs-ultime/` à l'identité actuelle du site. La décision
est une **préférence d'équipe assumée**, pas la résolution d'un problème UX
mesuré. L'objectif : adopter **toute** cette identité, sur **tout** le produit
Ibalya — la vitrine publique *et* l'application React — pour que l'ensemble
parle d'une seule voix visuelle.

Formulation du commanditaire : « on touche à tout même le produit », « toute la
vitrine copier-coller », « on rajoute un mode sombre à ça ».

## Ce qu'est réellement la source

Le fichier `dossier-2-roi-simulateurs-ultime/index.html` (2,4 Mo) n'est **pas**
du HTML statique déposable tel quel : c'est un **artifact React compilé**
(titre « React Artifact », JS minifié inline, 11 images en base64). Il n'est ni
maintenable ni compatible avec la CSP stricte de la landing actuelle
(`script-src 'self'`).

La charte réutilisable vit dans les fichiers propres fournis à côté :
- `animations-curated.css` (9 Ko) : **les tokens de la charte** + 20 keyframes
  d'animation.
- `animations.js` (6 Ko) : logique des interactions (curseur fluide, magnétisme,
  compteurs, etc.).
- `logo.png`, `shield.png`, `rocket.png` : assets.

**Conséquence** : on ne copie-colle pas le bundle. On **extrait la charte** et on
**réimplémente** proprement la vitrine (statique, CSP-propre) et le produit
(tokens React).

## La charte (source de vérité extraite)

### Palette — clair (« paper »)
```
--paper: #F7F4F1   --paper-deep: #F1EDE9   --paper-border: #E6DED7
--ink: #22272E     --ink-mute: #746D73     --ink-fade: #938B90
--teal: #0E7490    --teal-hover: #0B637A   --teal-light: #E5F0F2
--ok: #3A7D5C      --warn: #A86523         --risk: #B5432A
```
Le canard (`--teal`) est un **accent rare**, pas une couleur de remplissage.

### Palette — sombre (à dériver, voir plus bas)
N'existe pas dans la source. À créer (voir « Décisions »).

### Typo
- **Fraunces** (serif variable) : titres.
- **JetBrains Mono** : étiquettes, chiffres, mono.
- **Instrument Serif** : accents éditoriaux ponctuels.

### Signature de forme
- Rayons **arrondis** : 12 px (champs), 16 px (icônes), 24 px (cartes), `9999px`
  (pastilles).
- Ombres douces : `0 8px 32px rgba(34,39,46,0.08)`.
- Courbe d'animation maison : `cubic-bezier(0.16,1,0.3,1)`.

## Portée

### A. Charte centrale (fondation)
Un jeu de tokens unique (couleurs clair + sombre, polices, rayons, échelle
typographique, ombres, styles de composants partagés) servant de source de
vérité pour la vitrine et le produit.

### B. Vitrine (`landing/`)
Réimplémenter la page d'accueil au nouveau design, en **statique propre** (HTML
+ CSS + JS vanilla, CSP-compatible) :
- Trame narrative actuelle conservée (héro, problème, fonctionnement, assistant,
  sécurité, FAQ, CTA, pied de page) + les **6 simulateurs ROI** du nouveau design.
- Reporter la plomberie existante : SEO/OG/Twitter, `canonical`, `favicon.svg`,
  `og.png`, CSP, bascule de thème (`theme.js`), `annee.js`.
- Re-skinner `cgu.html`, `confidentialite.html` et `guide/` à la charte.
- Ajouter le **mode sombre** (voir Décisions).

#### Les 6 simulateurs ROI (du README source)
1. **ROI Calculator Ultra** — 6 curseurs → temps gagné, engagements sécurisés,
   retards évités €, économies/an, ROI %, payback jours ; bar chart « sans vs
   avec » + line chart cumul 12 mois.
2. **Coût de l'oubli** — coût moyen × fréquence × taux transfo → coût annuel
   direct/indirect/total + jauge demi-cercle vert→rouge.
3. **Temps perdu** — messages/j × minutes de recherche × jours/an → heures/jours/€
   + heatmap calendaire 12×31.
4. **Risque (heatmap)** — engagements × % retard × % silence → heatmap 7×24 +
   score de risque + top 3 risques.
5. **Validation (swipe)** — pile de 5 messages draggables (seuil 100 px) +
   compteur validés / temps gagné.
6. **Fiabilité** — 5 signaux à activer → entonnoir 3 étapes + précision %.

### C. Produit (`frontend/`, React + Vite)
Appliquer la charte à l'app. L'app est **stylée par tokens CSS centralisés**
(`src/styles.css`, blocs `:root` clair/sombre), donc l'essentiel est un
**remplacement de tokens** + un polissage composant par composant là où la
charte introduit de nouveaux motifs (titres serif, cartes arrondies, champs de
formulaire). Surface : 11 pages + ~12 composants.

**Les fioritures marketing ne passent pas dans le produit** (ni confettis, ni
curseur fluide, ni metaballs, ni boutons magnétiques, ni CTA collant) : un outil
de travail, pas une page de vente. On garde les transitions sobres et
`prefers-reduced-motion`.

## Non-objectifs / anti-objectifs
- **Non-objectif** : refondre les fonctionnalités, la navigation ou le contenu
  textuel du produit. On repeint, on ne reconstruit pas.
- **Non-objectif** : porter les simulateurs ROI dans l'app (ils restent sur la
  vitrine).
- **Anti-objectif (échec même si « ça marche »)** : une app ralentie ou
  distrayante à cause d'animations marketing ; une CSP affaiblie pour faire
  entrer des polices/scripts tiers ; une divergence où vitrine et produit ne se
  ressemblent toujours pas.

## Contraintes
- CSP de la vitrine : `script-src 'self'` → pas de JS inline, pas de CDN de
  polices. **Auto-héberger** les polices (woff2) et garder le JS en fichiers
  `self`.
- Déploiement inchangé : `deployer.sh` build front + sert `landing/`. Ne rien
  casser du câblage.
- Accessibilité : contraste AA sur paper/ink/teal, focus visibles, respect de
  `prefers-reduced-motion`.
- Le travail se fait **en branche**, PR + CI verte avant merge (convention du
  dépôt).

## Décisions

### Tranchées par le commanditaire (2026-11-04)
1. **Chiffres des simulateurs = estimation assumée.** Aucune source lisible
   n'existe (les formules sont perdues dans le bundle minifié). On produit des
   chiffres **estimés et transparents** : chaque simulateur affiche ses
   hypothèses et sa formule, présentés comme une estimation paramétrable — pas
   comme une promesse chiffrée garantie. Les hypothèses-tête (coût horaire par
   défaut, % de retard typique, minutes perdues/jour, « 3 min gagnées par
   validation », coût d'une livraison oubliée) sont **soumises à Yacou pour
   relecture avant mise en ligne**. Cela respecte la règle « ne jamais inventer
   de chiffres/prix » : rien n'est présenté comme un fait, tout est un modèle
   explicite que le visiteur ajuste. *(réponse : « fais une estimation »)*
2. **Vitrine = sous-ensemble curaté d'animations.** On garde l'identité (crème,
   serif, arrondis, canard en touches) et des animations **sobres** (révélation
   au scroll, compteurs, transitions douces). On **jette** les effets tape-à-l'œil
   de la source (curseur fluide, metaballs, confettis, boutons magnétiques, tilt
   3D, CTA collant) qui contredisent la charte « anti-IA » mémorisée. *(réponse :
   « sous-ensemble curaté »)*
3. **Produit = reshape complet.** Arrondis (12–24px) + ombres douces appliqués à
   chaque carte, champ, bouton et panneau, sur les 11 pages + ~12 composants. La
   signature « angles droits » actuelle est abandonnée. C'est une passe
   composant par composant, pas un simple changement de couleurs. *(réponse :
   « reshape complet »)*

### Mes choix (techniques, à signaler)
4. **Réimplémentation, pas copier-coller du bundle** : vitrine statique propre,
   charte extraite depuis `animations-curated.css`. *(technique)*
5. **Mode sombre dérivé** : variante sombre « chaude » cohérente avec la charte
   (réutilise l'amorce `#17151c`/`#f4f2f0` de l'app), bascule gardée sur vitrine
   + produit. *(demandé : « on rajoute un mode sombre »)*
6. **Polices** : auto-héberger **JetBrains Mono** (charte) à la place de Geist
   Mono (actuel), + Fraunces (déjà présent) + Instrument Serif. *(réconcilie
   charte et CSP `self`)*
7. **Lot 2 = évolution, pas réécriture à zéro.** La landing actuelle partage déjà
   la palette paper, le canard « rare » et Fraunces (son `theme-color` est déjà
   `#f7f4f1`). Le vrai delta visuel est : arrondis, Instrument Serif, la couche
   d'animation curatée, et les 6 simulateurs. On fait donc **évoluer** la landing
   existante plutôt que de la reconstruire. *(mon choix, réduit le risque et
   l'effort)*

## Séquencement proposé (à valider)
- **Lot 1 — Fondation + produit** : extraire la charte en tokens, l'appliquer au
  produit (clair + sombre), polir les composants. Livrable visible : l'app à la
  nouvelle identité.
- **Lot 2 — Vitrine (coquille)** : faire évoluer la landing existante vers le
  nouveau design (héro, narratif, FAQ, CTA, arrondis, Instrument Serif,
  animations curatées, dark), re-skin CGU/confidentialité/guide, **sans** les
  simulateurs.
- **Lot 3 — Les 6 simulateurs ROI** : les réimplémenter proprement un par un sur
  la vitrine, chacun avec ses hypothèses affichées (chiffres estimés, voir
  Décision 1), et faire valider les hypothèses-tête par Yacou avant go-live.

Ce découpage permet de livrer de la valeur tôt et d'isoler le gros morceau (les
simulateurs). L'ordre lot 1 → 2 → 3 est une proposition ; il peut s'inverser si
la priorité de Yacou est la vitrine.

## Laissé au constructeur
- Échelle typographique précise et rythme vertical (dérivés de la charte).
- Mapping exact ancien token → nouveau token, page par page.
- Choix des graphes (bar/line/jauge/heatmap/entonnoir) : lib légère vs SVG maison,
  en respectant le budget perf et la CSP.
- Structure des fichiers de la vitrine réimplémentée.

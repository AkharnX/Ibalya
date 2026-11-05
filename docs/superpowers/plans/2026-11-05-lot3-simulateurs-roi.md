# Lot 3 — 6 simulateurs ROI (vitrine) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ajouter à la home (`landing/index.html`) une section « simulateurs » contenant les **6 simulateurs ROI interactifs** du design source, réimplémentés proprement en statique (SVG maison + JS vanilla, CSP-safe), à la charte « paper », avec des **hypothèses éditables et transparentes** (jamais présentées comme des faits garantis — à valider par Yacou avant go-live).

**Architecture :** Vitrine statique sans build. Une nouvelle `<section id="simulateurs">` dans `landing/index.html`, un fichier `landing/simulateurs.js` (externe, `self`) qui porte un petit framework partagé (helpers de curseur, formatage FR, init paresseuse via IntersectionObserver, garde reduced-motion) puis la logique des 6 simulateurs, et des styles ajoutés à `landing/style.css`. Les graphes sont des **SVG construits en JS** (aucune lib, aucun CDN). Chaque simulateur **dégrade proprement sans JS** (inputs visibles + note, pas d'écran cassé).

**Tech Stack :** HTML/CSS/JS vanilla, SVG inline généré en JS, `<input type=range>` natifs, `IntersectionObserver`, `requestAnimationFrame` pour les compteurs. Aucune dépendance.

**Spec :** `docs/superpowers/specs/2026-11-04-refonte-charte-ibalya-design.md` (section B « Les 6 simulateurs ROII » + Décision 1 : chiffres = estimation assumée, transparente, éditable, validée par Yacou avant go-live).

## Global Constraints

- **CSP (nginx) :** `script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'`. → `simulateurs.js` est un fichier externe `self` ; **aucun JS inline**, aucun `onclick`, aucun CDN, aucune lib de graphes tierce, aucune police distante.
- **Pas d'étape de build** : fichiers de `landing/` servis tels quels. Pas de bundler.
- **Jamais inventer de chiffres présentés comme des faits (règle projet + Décision 1).** Toute hypothèse-tête est un **champ éditable** avec une valeur par défaut, un libellé « estimation / hypothèse », et la **formule affichée ou accessible**. Les résultats portent la mention qu'il s'agit d'une estimation paramétrable. Les valeurs par défaut du tableau ci-dessous sont **proposées et soumises à Yacou** ; elles ne sont pas des mesures.
- **Dégradation sans JS :** sans JavaScript, la section reste lisible — titres, descriptions et champs visibles, avec une note « activez JavaScript pour le calcul en direct ». Rien ne doit être coincé invisible.
- **Performance :** les 6 simulateurs s'**initialisent paresseusement** (IntersectionObserver) quand ils entrent dans le viewport — pas de calcul ni de construction SVG des 6 au chargement de la page.
- **Accessibilité :** curseurs `<input type=range>` natifs (clavier), `<label>`/`aria-label` sur chaque entrée, `aria-live="polite"` sur les sorties chiffrées, contrastes AA, focus visibles (anneau canard de la charte). `prefers-reduced-motion: reduce` → compteurs et transitions instantanés, pas d'animation de graphe.
- **Charte :** réutiliser les tokens existants de `style.css` (`--background/-foreground/-primary/-card/-border/-radius-*/-shadow-*`, Fraunces titres, JetBrains Mono chiffres, `--ok/-warn/-risk`). Graphes aux couleurs de la charte (canard accent, vert→ambre→brique pour les gravités). Le canard reste un accent, pas un aplat de fond.
- **Ne rien casser de la plomberie** (SEO/OG, theme.js, annee.js, animations.js, CSP) ni des Lots 1-2.

## Hypothèses par défaut — PROPOSÉES, à valider par Yacou avant go-live

Valeurs de départ des champs éditables. Chacune est modifiable par le visiteur ; aucune n'est présentée comme un fait.

| Hypothèse | Défaut proposé | Où |
|---|---|---|
| Coût horaire chargé (dirigeant/cadre) | 50 €/h | Sim 1, 3 |
| Part des messages qui sont des engagements | 15 % | Sim 1 |
| Temps manuel pour suivre 1 engagement | 2 min | Sim 1 |
| Temps gagné par validation en 1 clic | 3 min *(du README source)* | Sim 1, 5 |
| Réduction des oublis avec l'outil | 70 % | Sim 1 |
| Coût moyen d'un engagement/livraison oublié | 400 € | Sim 1, 2 |
| Engagements oubliés / mois sans outil | 4 | Sim 1 |
| Minutes perdues par recherche « qui a promis quoi » | 2 min | Sim 3 |
| Recherches de ce type / jour | 10 | Sim 3 |
| **Coût mensuel estimé de la solution** (pour ROI %/payback) | **champ éditable, défaut à confirmer avec Yacou** | Sim 1 |

> ⚠️ **Décision sensible (prix) :** le ROI % et le « payback » du Sim 1 exigent un coût de la solution. Ibalya est en bêta privée **sans prix public**. Plan : en faire un **champ éditable** « coût mensuel estimé de la solution » dont le défaut est un placeholder explicitement marqué « à confirmer », jamais présenté comme le tarif réel. À trancher avec Yacou au moment du go-live (ou masquer ROI %/payback si on préfère ne rien afficher côté prix).

## Review Focus

- **Chiffre présenté comme un fait** : un résultat sans mention « estimation », une hypothèse non éditable, ou un prix de solution en dur présenté comme le tarif → échec de la Décision 1. Chaque simulateur doit exposer ses hypothèses éditables + le cadre « estimation ». *(test : Task de chaque sim + Task 8)*
- **Casse sans JS** : avec JS désactivé, la section doit rester lisible (inputs + note), rien coincé à `opacity:0` ni écran blanc. *(test : Task 1 + Task 8)*
- **CSP** : un `onchange=`/`oninput=` inline, un `<script>` inline, ou un CDN casserait la page en prod. Tout le câblage se fait dans `simulateurs.js` (addEventListener). *(test : Task 1, vérif console sans violation à chaque task)*
- **Clavier & lecteurs d'écran** : curseurs opérables au clavier, sorties en `aria-live`, pas de piège au focus dans le swipe (Sim 5 doit avoir une alternative clavier : boutons Valider/Passer). *(test : Task 6 + Task 8)*
- **Perf au chargement** : les 6 sims ne calculent/ne construisent leurs SVG qu'à l'entrée dans le viewport ; pas de jank au scroll. *(test : Task 1 pose l'IO ; Task 8 vérifie)*

---

## Task 1 : Fondation — section, framework partagé, dégradation, styles

**Files :**
- Create: `landing/simulateurs.js`
- Modify: `landing/index.html` (nav : lien « Votre ROI » ; `<section id="simulateurs">` insérée après la section `#assistant` et avant `#securite`, contenant l'intro + le disclaimer « estimation » + 6 conteneurs `<div class="sim" id="sim-1..6" data-sim="…">` vides avec titre/description statiques ; `<script src="/simulateurs.js"></script>` après `animations.js`)
- Modify: `landing/style.css` (styles de la section et des composants partagés : `.sim`, `.sim-grille`, curseurs `.sim-range`, cartes de sortie `.sim-out`, `.sim-note`, conteneurs de graphe `.sim-chart`)

**Interfaces :**
- Produces (dans `simulateurs.js`, pour les Tasks 2-7) :
  - `registerSim(id, initFn)` — enregistre l'init d'un simulateur ; l'IO appelle `initFn(rootEl)` une seule fois à l'entrée dans le viewport.
  - `fmtEUR(n)`, `fmtInt(n)`, `fmtH(n)` — formatage FR (espace fine, « € », « h »).
  - `animateCount(el, from, to, fmt)` — compteur `requestAnimationFrame`, instantané si reduced-motion.
  - `bindRange(inputEl, onInput)` — abonne un `<input type=range>` + met à jour une bulle de valeur ; renvoie un lecteur de valeur.
  - `svg(tag, attrs, children)` — mini-fabrique d'éléments SVG (namespacée).
  - `prefersReducedMotion` — booléen.

- [ ] **Step 1 : Scaffold HTML.** Dans `index.html`, ajouter le lien de nav et la `<section id="simulateurs" class="lp-section">` : un `.lp-section-tete` (titre Fraunces « Combien Ibalya vous fait gagner », chapeau), un bloc disclaimer `.sim-disclaimer` (« Estimations paramétrables : ajustez les hypothèses, rien n'est garanti »), et une grille de 6 blocs `<div class="sim" id="sim-N" data-sim="roi|oubli|temps|risque|validation|fiabilite">` chacun avec un `<h3>` et un `<p>` descriptif **statiques** (lisibles sans JS). Charger `/simulateurs.js` en fin de `<body>`.

- [ ] **Step 2 : Framework `simulateurs.js`.** IIFE strict, `self`. Implémenter les fonctions de l'interface ci-dessus. `registerSim` empile dans une table ; un `IntersectionObserver` (seuil ~0.2) appelle l'`initFn` du `.sim` correspondant une fois puis `unobserve`. Garde `prefersReducedMotion`. Aucun calcul avant intersection. Si `IntersectionObserver` manque, initialiser tout de suite (dégradation).

- [ ] **Step 3 : Styles partagés.** Dans `style.css`, styler `.sim`/`.sim-grille`/`.sim-range` (piste + pouce canard, `--radius-pill`)/`.sim-out` (cartes de résultat arrondies, JetBrains Mono, `aria-live`)/`.sim-note`/`.sim-chart`/`.sim-disclaimer`, clair + sombre, via tokens. Curseurs stylés pour WebKit + Firefox. Focus visibles.

- [ ] **Step 4 : Dégradation sans JS.** Confirmer (JS désactivé) que la section montre titres/descriptions/disclaimer et une note « activez JavaScript pour le calcul en direct » (posée en HTML, masquée par JS une fois initialisé). Rien coincé invisible.

- [ ] **Step 5 : Vérifier.** Servir `landing/` (`python3 -m http.server`), rendre au navigateur (Chromium installé) clair + sombre : section présente, curseurs stylés, **console sans violation CSP**, et JS-off = lisible. Joindre captures.

---

## Task 2 : Simulateur 1 — ROI Calculator Ultra

**Files :** Modify `landing/simulateurs.js` (bloc `registerSim('sim-1', …)`), et au besoin `landing/style.css` (spécifique bar/line chart).

**Interfaces :** Consumes le framework Task 1 (`registerSim`, `fmt*`, `animateCount`, `bindRange`, `svg`).

**Entrées (curseurs, min–max, défaut) :** `boites` 1–20 (3) · `msgJour` 10–300 (80) · `tauxRetard` 0–50 % (15) · `coutHoraire` 20–150 € (50) · `coutOubli` 50–5000 € (400) · `freqOubli` 0–20/mois (4). Champ éditable **`coutSolution`** (€/mois, défaut placeholder marqué « à confirmer », voir décision prix).

**Formule (affichable) :**
- `engagements/mois = boites × msgJour × 0.15 × 21`
- `tempsGagné h/mois = engagements/mois × 2 min ÷ 60`
- `valeurTemps €/an = tempsGagné × 12 × coutHoraire`
- `oublisÉvités €/an = freqOubli × 12 × coutOubli × 0.70`
- `économies €/an = valeurTemps + oublisÉvités`
- `coûtOutil €/an = coutSolution × 12`
- `ROI % = (économies − coûtOutil) ÷ coûtOutil × 100` *(affiché seulement si `coutSolution` renseigné ; sinon « — »)*
- `payback jours = coûtOutil ÷ (économies ÷ 365)` *(idem)*

**Sorties (`.sim-out`, `aria-live`, compteurs) :** temps gagné (h/mois), engagements sécurisés (/mois), oublis évités (€/an), économies (€/an), ROI %, payback (jours).

**Graphes (SVG maison) :** (a) **barres** « Sans Ibalya vs Avec » (coût annuel subi vs économies) ; (b) **ligne** cumul des économies sur 12 mois.

- [ ] Step 1 : init — construire curseurs + sorties + 2 SVG dans `#sim-1`.
- [ ] Step 2 : brancher `oninput` (recalcul + `animateCount` + redraw SVG).
- [ ] Step 3 : afficher la formule (détails repliables `<details>`).
- [ ] Step 4 : ROI/payback = « — » tant que `coutSolution` vide ; libellé « estimation ».
- [ ] Step 5 : vérifier (render clair+sombre, clavier sur curseurs, reduced-motion, console CSP propre). Captures.

---

## Task 3 : Simulateur 2 — Coût de l'oubli (jauge)

**Files :** Modify `landing/simulateurs.js` (`registerSim('sim-2', …)`), `style.css` si besoin.

**Entrées :** `coutMoyenOubli` 50–5000 € (400) · `frequenceMois` 0–30/mois (4) · `tauxTransfo` 0–100 % (30).

**Formule :** `direct €/an = coutMoyenOubli × frequenceMois × 12` · `indirect €/an = direct × tauxTransfo%` (opportunités perdues) · `total = direct + indirect`.

**Sorties :** coût direct, indirect, total (€/an). **Graphe :** **jauge demi-cercle** SVG, dégradé vert→ambre→brique (charte `--ok/--warn/--risk`), aiguille positionnée sur `total` normalisé par un plafond raisonnable.

- [ ] Step 1 : curseurs + sorties + SVG jauge demi-cercle. 
- [ ] Step 2 : brancher recalcul + aiguille animée (instantanée si reduced-motion).
- [ ] Step 3 : mention « estimation » + formule repliable.
- [ ] Step 4 : vérifier (render, a11y, CSP). Captures.

---

## Task 4 : Simulateur 3 — Temps perdu (heatmap calendaire)

**Files :** Modify `landing/simulateurs.js` (`registerSim('sim-3', …)`), `style.css` si besoin.

**Entrées :** `msgJour` 10–300 (80) · `minRecherche` 1–10 min (2) · `recherchesJour` 1–40 (10) · `coutHoraire` 20–150 € (50).

**Formule :** `minParJour = recherchesJour × minRecherche` · `hParAn = minParJour × 230 jours ÷ 60` · `coutParAn = hParAn × coutHoraire`.

**Sorties :** heures/an, jours ouvrés équivalents, €/an. **Graphe :** **heatmap calendaire** SVG ~12×20 (une case = un jour ouvré), intensité ∝ minutes perdues ce jour (variation déterministe autour de la moyenne, pas de hasard présenté comme donnée), échelle crème→canard.

- [ ] Step 1 : curseurs + sorties + grille SVG.
- [ ] Step 2 : recalcul + remplissage des cases.
- [ ] Step 3 : « estimation » + formule.
- [ ] Step 4 : vérifier. Captures.

---

## Task 5 : Simulateur 4 — Risque (heatmap 7×24)

**Files :** Modify `landing/simulateurs.js` (`registerSim('sim-4', …)`), `style.css` si besoin.

**Entrées :** `engagements` 1–200 (40) · `tauxRetard` 0–60 % (20) · `tauxSilence` 0–60 % (15).

**Formule :** `scoreRisque = clamp( (tauxRetard×0.6 + tauxSilence×0.4) × log(engagements) …)` normalisé 0–100 ; `aRisque = engagements × (tauxRetard+tauxSilence)/100`. Top 3 = libellés de risque dérivés des entrées (échéance à risque / silence anormal / promesse menacée), sans donnée inventée.

**Sorties :** score de risque (0–100 + libellé faible/moyen/élevé), nombre d'engagements à risque, top 3. **Graphe :** **heatmap 7×24** (jours × heures) SVG, intensité ∝ densité de risque (profil déterministe : plus chaud en journée/semaine), échelle vert→brique.

- [ ] Step 1 : curseurs + sorties + grille 7×24.
- [ ] Step 2 : recalcul + score + top 3 + remplissage.
- [ ] Step 3 : « estimation » + explication du score (repliable).
- [ ] Step 4 : vérifier. Captures.

---

## Task 6 : Simulateur 5 — Validation (swipe + alternative clavier)

**Files :** Modify `landing/simulateurs.js` (`registerSim('sim-5', …)`), `style.css` (pile de cartes).

**Contenu :** pile de 5 cartes « message » (exemples génériques, pas de vraie donnée client ; contenu fictif clairement illustratif). **Interaction :** glisser (pointer events, seuil 100px) pour valider/écarter ; **ET** deux boutons « Valider » / « Passer » (obligatoires — alternative clavier/lecteur d'écran, pas de dépendance au drag). Compteur : validés, temps gagné = validés × 3 min.

**Sorties :** compteur validés, temps gagné (min). **Pas de graphe** (l'interaction est la démo).

- [ ] Step 1 : construire la pile + boutons + compteur.
- [ ] Step 2 : brancher pointer-drag (seuil) + boutons ; `touch-action:none` sur la carte ; relâche propre.
- [ ] Step 3 : **alternative clavier** vérifiée (Tab vers les boutons, Entrée valide) ; focus visible ; pas de piège.
- [ ] Step 4 : reduced-motion = pas d'anim de ressort (transition instantanée). Mobile OK (`touch-action`).
- [ ] Step 5 : vérifier (drag souris + clavier + mobile viewport, CSP). Captures.

---

## Task 7 : Simulateur 6 — Fiabilité (entonnoir)

**Files :** Modify `landing/simulateurs.js` (`registerSim('sim-6', …)`), `style.css` si besoin.

**Entrées :** 5 interrupteurs (`<input type=checkbox>` stylés) : signaux `date`, `engagement`, `interlocuteur`, `contexte`, `formulation` (tous activés par défaut).

**Formule :** `précision % = base + Σ(poids des signaux actifs)` plafonnée à une valeur < 100 (p.ex. base 55 %, +9/+9/+8/+8/+6 ; max ~95 %) — **modèle illustratif**, pas une mesure, libellé explicite. Entonnoir 3 étapes : messages lus → engagements détectés → engagements fiables, les volumes dérivant de la précision.

**Sorties :** précision % (compteur), 3 étapes de l'entonnoir. **Graphe :** **entonnoir** SVG 3 segments (largeurs ∝ volumes), couleurs charte.

- [ ] Step 1 : toggles + sortie + SVG entonnoir.
- [ ] Step 2 : recalcul au toggle + redraw.
- [ ] Step 3 : mention « modèle illustratif de pondération » + détail repliable.
- [ ] Step 4 : vérifier (render, clavier sur toggles, CSP). Captures.

---

## Task 8 : Intégration, accessibilité & polish

**Files :** Modify `landing/index.html` (nav active state, ancrage), `landing/style.css` (responsive de la grille de sims, cohérence), `landing/simulateurs.js` (nettoyages).

- [ ] **Step 1 : Responsive.** Grille des 6 sims en 1 colonne sous ~900px ; curseurs et SVG fluides ; aucun débordement horizontal à 360/390px.
- [ ] **Step 2 : Accessibilité.** Passe complète : chaque curseur a `<label>`/`aria-label` et une sortie `aria-live`, contrastes AA des couleurs de graphe et des libellés, focus visibles partout, le swipe a son alternative clavier, les SVG décoratifs en `aria-hidden` avec un résumé textuel des résultats à côté.
- [ ] **Step 3 : Perf & reduced-motion.** Confirmer l'init paresseuse (les 6 ne calculent qu'à l'entrée viewport), pas de jank au scroll, `prefers-reduced-motion` coupe compteurs/anim de graphe partout.
- [ ] **Step 4 : Dégradation & CSP finales.** JS-off : section lisible ; console **sans violation CSP** sur toute la page ; aucune police 404.
- [ ] **Step 5 : Rendu final.** Captures clair + sombre de la section complète (desktop + mobile). Lister dans le rapport les hypothèses-tête à faire valider par Yacou (le tableau ci-dessus) pour le go-live.

---

## Self-Review

- **Spec coverage (6 simulateurs)** : Sim 1 ROI (Task 2), Sim 2 Coût oubli (Task 3), Sim 3 Temps perdu (Task 4), Sim 4 Risque (Task 5), Sim 5 Validation (Task 6), Sim 6 Fiabilité (Task 7) ; fondation (Task 1), intégration/a11y (Task 8). Décision 1 (estimations éditables + Yacou) : portée par la contrainte globale, le disclaimer Task 1, et une étape « estimation » dans chaque sim. ✅
- **Décision sensible prix** : ROI %/payback via champ éditable `coutSolution` flaggé — explicitement surfacé pour arbitrage Yacou (Task 2 Step 4 + tableau). N'invente pas de tarif présenté comme réel. ✅
- **Type consistency** : les helpers `registerSim/fmtEUR/fmtInt/fmtH/animateCount/bindRange/svg/prefersReducedMotion` sont définis Task 1 et consommés tels quels Tasks 2-7. ✅
- **Review Focus** : chiffre-comme-fait, no-JS, CSP, clavier/swipe, perf paresseuse — chacun a une étape de vérif dans la task propriétaire + Task 8. ✅
- **Proportion** : formules et plages de curseurs spécifiées (valeurs décidées), corps JS laissés au constructeur ; graphes décrits par type + données, pas codés. Longueur proportionnée aux 6 sims. ✅

## Laissé au constructeur
- Détail de construction des SVG (viewBox, échelles, graduations).
- Micro-copie exacte des descriptions et des libellés de résultats.
- Choix fin des pondérations illustratives (dans les bornes données), à soumettre à Yacou.
- Disposition fine de la grille et rythme vertical.

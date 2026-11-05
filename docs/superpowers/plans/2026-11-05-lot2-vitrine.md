# Lot 2 — Vitrine (coquille) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Faire évoluer la landing statique `landing/` vers la charte « paper » complète (arrondis, ombres douces, JetBrains Mono, Instrument Serif, couche d'animation curatée, dark aligné), et re-skinner CGU/confidentialité/guide — **sans** les 6 simulateurs ROI (Lot 3).

**Architecture :** La landing est un site **statique sans build** (`landing/index.html` + `/style.css` + `/theme.js` + `/annee.js`, servi tel quel par le backend Go via `http.FileServer`, et par nginx en prod). Elle partage déjà la palette paper, le canard « rare » et Fraunces. Le travail est une **évolution** de `style.css` (tokens + formes + animations) + un nouveau `landing/animations.js` (CSP-safe, externe), + une passe sur 3 pages légales à `<style>` inline. Aucun framework, aucune étape de compilation.

**Tech Stack :** HTML/CSS/JS vanilla. Polices woff2 auto-hébergées. IntersectionObserver pour les révélations au scroll. Aucune dépendance tierce.

**Spec :** `docs/superpowers/specs/2026-11-04-refonte-charte-ibalya-design.md` (section B « Vitrine » + Décisions 2, 5, 6, 7). Validée par le commanditaire.

## Global Constraints

- **CSP (nginx, prod + beta)** : `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'`. → Aucun JS inline (scripts en fichiers `self`). Inline `<style>` toléré. **Polices auto-hébergées** (woff2 dans `landing/fonts/`), jamais de CDN de polices ni `@import` Google Fonts.
- **Pas d'étape de build** pour la vitrine : les fichiers de `landing/` sont servis tels quels. Ne pas introduire de bundler.
- **Ne rien casser de la plomberie** : balises SEO/OG/Twitter, `<link rel=canonical>`, `favicon.svg`, `og.png`, `theme.js` (bascule + anti-flash), `annee.js`, la CSP. `deployer.sh` sert `landing/` inchangé.
- **Accessibilité** : contraste AA sur paper/ink/teal (texte ≥ 4.5:1), focus clavier visibles, `prefers-reduced-motion: reduce` coupe toutes les animations.
- **Anti-IA (mémoire projet + Décision 2)** : animations **sobres uniquement**. Sont **proscrits** : curseur fluide, boutons magnétiques, tilt 3D/bento-glow, metaballs, particules, confetti, CTA collant, carousel qui dérive, morphing d'icône à 180°. Garder : révélation au scroll, compteurs, accordéon FAQ fluide, transitions douces `cubic-bezier(.16,1,.3,1)`.
- **Cohérence avec le produit (Lot 1, déjà sur `main`)** : réutiliser **verbatim** les mêmes valeurs de tokens que `frontend/src/styles.css` pour la palette sombre chaude, le canard sombre (`#3FA7C0`), l'échelle de rayons et les ombres, pour que vitrine et produit parlent d'une seule voix.
- **`landing/` n'a pas de runner de test.** Vérification = rendu réel : la vitrine est **statique**, donc un sous-agent peut la servir (`python3 -m http.server` dans `landing/`) et la capturer au navigateur (Playwright/chromium déjà installé sous `~/.cache/ms-playwright/`). Joindre captures clair + sombre.

## Review Focus

- **Flash de thème au chargement** (déjà géré par `theme.js` dans le `<head>`) : toute nouvelle feuille/police ne doit pas réintroduire un flash clair→sombre. → vérifié en Task 1 (chargement police `font-display:swap`, pas de FOIT bloquant) et au rendu dark.
- **CSP** : un `@import` de police, un `onclick=` inline, ou un `<script>` inline casserait la page en prod. → chaque task qui ajoute du JS/police le fait en fichier `self` ; vérif console sans violation CSP en Task 3 et Task 5.
- **`prefers-reduced-motion`** : les révélations au scroll doivent laisser le contenu **visible** (pas coincé à `opacity:0`) quand l'utilisateur coupe les animations, et quand JS échoue. → l'état par défaut du contenu est visible ; l'animation ne fait que *re-jouer* une entrée. Testé en Task 3.
- **Contenu sans JS** : si `animations.js` ne charge pas, la page reste entièrement lisible (révélations = progressive enhancement, jamais un prérequis d'affichage). → Task 3 : classes de révélation posées par JS, pas dans le HTML au repos.
- **Responsive mobile** (≤ 560px) : le header masque déjà nav/CTA ; les arrondis et ombres ne doivent pas introduire de débordement horizontal. → Task 2 + Task 5 capture mobile.

---

## Task 1 : Tokens & polices de la vitrine (fondation)

**Files :**
- Modify: `landing/style.css` (bloc `:root` ~l.6-14, bloc `:root[data-theme="dark"]` ~l.307-313, bloc `@font-face` ~l.274-280, en-tête commentaire l.1-5)
- Create: `landing/fonts/jetbrains-mono-latin.woff2`, `landing/fonts/instrument-serif-400.woff2`
- (Les woff2 existent déjà côté produit : `frontend/src/assets/fonts/jetbrains-mono-latin.woff2` et `instrument-serif-400.woff2` — **copier ces fichiers**, ne pas re-télécharger.)

**Interfaces :**
- Produces : les tokens `--radius-sm/md/lg/pill`, `--shadow-card`, `--shadow-soft`, `--mono` (JetBrains), `--accent` (Instrument Serif) que Task 2 et Task 3 consomment. Palette sombre alignée que Task 2 (ombres dark) consomme.

- [ ] **Step 1 : Copier les woff2 depuis le produit.**
  `cp frontend/src/assets/fonts/jetbrains-mono-latin.woff2 frontend/src/assets/fonts/instrument-serif-400.woff2 landing/fonts/`

- [ ] **Step 2 : `@font-face`.** Dans `landing/style.css`, remplacer le `@font-face` Geist Mono (~l.279-280) par JetBrains Mono (variable, `font-weight:100 800`, `src:url('/fonts/jetbrains-mono-latin.woff2')`), et ajouter Instrument Serif 400 (`src:url('/fonts/instrument-serif-400.woff2')`). Garder les 3 Fraunces. `font-display:swap` partout. Supprimer le fichier `landing/fonts/geist-mono-400.woff2` ET son `@font-face`.

- [ ] **Step 3 : Tokens de police.** `--mono: "JetBrains Mono", ui-monospace, monospace;` (retirer Geist). Ajouter `--accent: "Instrument Serif", Georgia, serif;`. Garder `--sans` et la famille Fraunces des titres.

- [ ] **Step 4 : Échelle de formes (invariants `:root`).** Ajouter, **valeurs verbatim du produit** : `--radius-sm:12px; --radius-md:16px; --radius-lg:24px; --radius-pill:9999px; --shadow-card:0 8px 32px rgba(34,39,46,.08); --shadow-soft:0 2px 8px rgba(34,39,46,.06);`

- [ ] **Step 5 : Aligner la palette sombre sur le produit.** Dans `:root[data-theme="dark"]`, passer `--primary` de `#22a4c4` à **`#3FA7C0`** (valeur produit). Vérifier que `--background:#17151c`, `--foreground:#f4f2f0`, `--card:#221f2a`, `--secondary:#2c2836`, `--border:#39333f` correspondent au produit ; ajuster sinon. Redéfinir `--shadow-card`/`--shadow-soft` en sombre avec une ombre lisible sur fond sombre (base `rgba(0,0,0,.45)` comme le produit).

- [ ] **Step 6 : Contraste AA des statuts.** Vérifier `--risk` sur paper et en sombre ≥ 4.5:1 pour du texte ; aligner sur les valeurs produit si besoin. Noter les ratios dans le rapport.

- [ ] **Step 7 : Nettoyer l'en-tête.** Corriger le commentaire l.1-5 (« angles droits, bleu cobalt, Geist » — devenu faux : arrondis, teal, JetBrains).

- [ ] **Step 8 : Vérifier.** Servir `landing/` (`cd landing && python3 -m http.server 8181`), ouvrir au navigateur clair + sombre : titres Fraunces, mono JetBrains (pas de repli Courier), aucun flash, aucune violation CSP en console (vérifier `font-src`). Joindre 1 capture.

---

## Task 2 : Formes — arrondis + ombres douces sur les composants

**Files :**
- Modify: `landing/style.css` (sélecteurs de composants : `.lp-btn`, `.lp-carte`, `.lp-carte-mate`, `.lp-terminal`, `.lp-terminal-barre i`, `.lp-demo`, `.lp-demo-carte`, `.lp-config`, `.lp-theme`, `.lp-skip`, `.lp-faq-item`, `.lp-extraction`, `mark`, focus-visible)

**Interfaces :**
- Consumes : `--radius-*`, `--shadow-*` (Task 1). Palette sombre alignée (Task 1).
- Produces : rien pour les tasks suivantes (purement visuel).

- [ ] **Step 1 : Contrôles.** `.lp-btn` + `.lp-btn-primary` + `.lp-theme` + `.lp-skip` → `border-radius: var(--radius-sm)`. Garder `.lp-btn-primary` en **aplat canard** (cohérent avec le produit livré ; l'arbitrage « aplat vs contour » est une décision Yacou globale, hors périmètre de cette task — ne pas le changer unilatéralement).

- [ ] **Step 2 : Cartes & conteneurs.** `.lp-carte`, `.lp-carte-mate`, `.lp-terminal`, `.lp-demo` (et ses enfants `.lp-demo > div` aux coins externes), `.lp-demo-carte`, `.lp-config` → `border-radius: var(--radius-lg)` (cartes/terminal) ou `var(--radius-sm)` (petites cartes `.lp-demo-carte`). Ajouter `box-shadow: var(--shadow-card)` sur `.lp-terminal` et `.lp-carte` (pas sur `.lp-carte-mate` qui reste mate/transparente). Les points du terminal `.lp-terminal-barre i` sont déjà `border-radius:50%` — OK.

- [ ] **Step 3 : Petits rayons.** `.lp-faq-item` reste à bords simples (liste), mais `mark`/`.lp-demo-tag`/badges éventuels → `var(--radius-pill)` ou `--radius-sm` selon la forme. `.lp-extraction` (bord gauche) inchangé.

- [ ] **Step 4 : Focus & transitions.** Harmoniser l'anneau de focus avec le produit : `a:focus-visible,button:focus-visible,.lp-btn:focus-visible` → anneau canard robuste (`outline:2px solid transparent; box-shadow:0 0 0 2px var(--card),0 0 0 4px var(--primary);` ou équivalent lisible en forced-colors). Ajouter `transition: box-shadow .2s, border-color .2s` sur `.lp-carte`/`.lp-btn`.

- [ ] **Step 5 : Ombres thématiques.** Confirmer que `.lp-terminal`/`.lp-carte` portent une ombre visible en clair ET en sombre (la variable `--shadow-card` sombre vient de Task 1).

- [ ] **Step 6 : Vérifier.** Servir `landing/`, ouvrir clair + sombre + mobile (≤560px) : arrondis cohérents, ombres douces, **aucun débordement horizontal**, focus visibles. Joindre captures clair + sombre (page dense : héro + fonctionnement).

---

## Task 3 : Couche d'animation curatée (CSP-safe, sobre)

**Files :**
- Create: `landing/animations.js` (externe, chargé en fin de `<body>` comme `annee.js`)
- Modify: `landing/index.html` (ajouter `<script src="/animations.js"></script>` près de `annee.js` ; poser les attributs/hooks de révélation sur les sections/cartes), `landing/style.css` (keyframes + classes de révélation/compteur, bloc `prefers-reduced-motion`)

**Interfaces :**
- Consumes : la courbe `cubic-bezier(.16,1,.3,1)`, tokens Task 1.
- Produces : rien (feuille terminale).

**Périmètre animations (Décision 2) — GARDER uniquement :**
- **Révélation au scroll** : IntersectionObserver ajoute `.is-visible` aux `[data-reveal]` (sections, cartes, demo). CSS : état repos `opacity:1` **par défaut**, et l'animation d'entrée ne joue que si JS a posé `.reveal` (progressive enhancement — si JS échoue, contenu visible).
- **Compteurs** : sur les rares nombres-clés affichés (p.ex. « 92 % » de fiabilité, « 5 détecteurs »), animer de 0 à la valeur via JS quand l'élément entre dans le viewport (`.counter`, `tabular-nums`). N'inventer aucun chiffre : n'animer que des nombres **déjà présents** dans le HTML.
- **Accordéon FAQ** (optionnel, sobre) : convertir `.lp-faq-item` (h3 + p) en `<details>`/`<summary>` natif (accessible, **sans JS**, CSP-safe) OU laisser statique + révélation. Choix laissé au constructeur ; si accordéon, garder tout le texte dans le DOM (SEO).

**PROSCRIT (ne pas implémenter) :** curseur fluide, `[data-magnetic]`, `.bento-tilt`, metaballs, particules, `.top-progress`, confetti, `.sticky-cta`, carousel, icon-morph. Ne **pas** importer `animations-curated.css` tel quel (il contient ces effets + un `@import` Google Fonts interdit par la CSP).

- [ ] **Step 1 : `animations.js`.** Écrire un script `self` : garde `if (matchMedia('(prefers-reduced-motion: reduce)').matches) return;` tôt ; IntersectionObserver qui ajoute `.is-visible` aux `[data-reveal]` ; fonction compteur sur `[data-count]`. Idempotent, sans dépendance, dégrade proprement si un sélecteur manque.

- [ ] **Step 2 : CSS de révélation.** Ajouter keyframe `lp-reveal` (translateY(10px)→0, opacity 0→1) et règles : `[data-reveal]` repos = visible ; `[data-reveal].is-visible` = joue l'entrée. Compléter le bloc `@media (prefers-reduced-motion: reduce)` existant (~l.264) pour neutraliser les nouvelles animations.

- [ ] **Step 3 : Poser les hooks HTML.** Ajouter `data-reveal` sur les `.lp-carte`, `.lp-section-tete`, `.lp-demo` (sans casser l'anim séquencée existante `lp-t1..t5` — soit on la garde telle quelle, soit on la ré-exprime via `data-reveal`, au choix). Ajouter `data-count` sur les nombres déjà écrits qu'on veut animer.

- [ ] **Step 4 : Charger le script.** Dans `index.html`, `<script src="/animations.js"></script>` juste avant `</body>` (après `annee.js`). Pas d'inline.

- [ ] **Step 5 : Vérifier.** Servir `landing/`, ouvrir : scroller → révélations douces ; activer `prefers-reduced-motion` (devtools) → contenu **visible, animations coupées** ; désactiver JS → page lisible ; console **sans violation CSP**. Joindre une capture + une note sur le comportement reduced-motion.

---

## Task 4 : Re-skin des pages légales + guide

**Files :**
- Modify: `landing/cgu.html`, `landing/confidentialite.html`, `landing/guide/index.html` (chacune a un `<style>` **inline** propre — CSP `style-src 'unsafe-inline'` OK)

**Interfaces :**
- Consumes : la charte (mêmes valeurs de tokens que `style.css` : paper, ink, teal, radii, Fraunces, JetBrains, dark `data-theme`).

- [ ] **Step 1 : Auditer le `<style>` de chaque page.** Relever les couleurs/polices/rayons en dur qui divergent de la charte.

- [ ] **Step 2 : Aligner les tokens.** Dans chaque `<style>`, poser le même `:root` paper + `:root[data-theme="dark"]` que `style.css` (ou, plus DRY, lier `/style.css` et ne garder en inline que les règles propres à la page — **au choix du constructeur**, tant que les deux thèmes rendent bien). Titres Fraunces, mono JetBrains, arrondis `--radius-*`, ombres douces.

- [ ] **Step 3 : Bascule de thème.** Si ces pages ont déjà un bouton + `theme.js`, le conserver/aligner ; sinon, au minimum respecter `data-theme` (anti-flash via `theme.js` dans le `<head>`) pour que le thème choisi sur la home s'applique ici aussi.

- [ ] **Step 4 : Vérifier.** Servir `landing/`, ouvrir `/cgu.html`, `/confidentialite.html`, `/guide/` en clair + sombre : charte cohérente avec la home, lisibles, pas de zone illisible. Joindre 1 capture par page (sombre).

---

## Self-Review

- **Spec coverage (section B)** : héro/narratif/FAQ/CTA re-skinnés → Tasks 1-3 ; arrondis → Task 2 ; Instrument Serif → Task 1 (token) + usage ponctuel possible ; animations curatées → Task 3 ; dark → Task 1 ; CGU/confidentialité/guide → Task 4 ; plomberie SEO/OG/CSP/theme.js/annee.js **préservée** (contrainte globale). Les **6 simulateurs = Lot 3**, explicitement hors périmètre ici (non-objectif spec). ✅
- **Instrument Serif** : défini en Task 1 ; son application (accents éditoriaux ponctuels) est laissée au constructeur comme touche — non bloquant, cohérent avec « laissé au constructeur » de la spec.
- **Type consistency** : `--radius-sm/md/lg/pill`, `--shadow-card/-soft`, `--mono`, `--accent` nommés identiquement de Task 1 (produit) à Task 2/3. ✅
- **Review Focus** : flash thème, CSP, reduced-motion, dégradation sans JS, responsive — chacun a une étape de vérif dans la task propriétaire. ✅
- **Proportion** : plan ~ longueur raisonnable vs delta réel (évolution, pas réécriture) ; aucun bloc de code recopié, seulement signatures/valeurs/fichiers. ✅

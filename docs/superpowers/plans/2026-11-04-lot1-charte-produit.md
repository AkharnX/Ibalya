# Lot 1 — Fondation (charte) + reskin du produit — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Implementation aids:** this is a redesign of an existing app. Invoke the project skills `redesign-existing-projects` and `design-taste-frontend` (under `frontend/src/.claude/skills`) during execution to audit and polish — they carry out the visual work the plan scopes.

**Goal:** Repeindre l'application React Ibalya à la nouvelle charte « paper » (crème, encre, canard, titres serif, coins arrondis), en clair et en sombre, sans toucher aux fonctionnalités.

**Architecture:** Le style de l'app est centralisé dans `frontend/src/styles.css` via des variables CSS par thème (`:root` sombre, `:root.light` clair). On remplace les valeurs de tokens par la charte, on introduit une échelle de rayons + ombres douces, on auto-héberge les polices de la charte, puis on applique arrondis/ombres composant par composant. La bascule de thème existante (classe `.light`, `App.jsx`) est conservée telle quelle.

**Tech Stack:** React 19, Vite, CSS vanilla (tokens), oxlint. Pas de framework de test front → vérification par `npm run build`, `npm run lint`, et contrôle visuel (dev server + capture).

**Spec:** `docs/superpowers/specs/2026-11-04-refonte-charte-ibalya-design.md`

## Global Constraints

- Palette claire « paper » (verbatim charte) : `--paper #F7F4F1`, `--paper-deep #F1EDE9`, `--paper-border #E6DED7`, `--ink #22272E`, `--ink-mute #746D73`, `--ink-fade #938B90`, `--teal #0E7490`, `--teal-hover #0B637A`, `--teal-light #E5F0F2`, `--ok #3A7D5C`, `--warn #A86523`, `--risk #B5432A`.
- Canard (`--teal`) = accent **rare**, jamais couleur de remplissage.
- Rayons charte : 12px (champs/boutons/chips), 16px (icônes), 24px (cartes/panneaux), 9999px (pastilles). Ombre carte : `0 8px 32px rgba(34,39,46,0.08)`.
- Polices auto-hébergées, CSP-propre : **Fraunces** (titres, déjà en woff2 dans `landing/fonts/`), **JetBrains Mono** (mono), **Instrument Serif** (accents). Aucune dépendance CDN.
- Zéro fioriture marketing dans le produit (pas de curseur fluide, metaballs, confettis, boutons magnétiques). Transitions sobres uniquement.
- Respect de `prefers-reduced-motion`.
- Aucune modification de logique/fonctionnalité : on repeint, on ne reconstruit pas. La bascule de thème reste la classe `.light`.
- Travail en branche dédiée, `npm run build` + `npm run lint` verts avant toute PR.

## Review Focus

- **Parité clair/sombre** : toute surface doit avoir ses tokens dans les DEUX thèmes ; une couleur codée en dur casse un thème. → vérifié en Task 4 (audit `grep` des couleurs hex hors `:root`).
- **Contraste AA** : `--ink` sur `--paper` et `--teal` sur `--paper` doivent passer AA (texte), `--teal` sur blanc pour les boutons. → vérifié en Task 1.
- **Couleurs de catégorie (`--t-*`)** lisibles et distinctes sur fond crème (elles ne doivent pas jurer avec l'identité chaude). → vérifié en Task 1.
- **Chargement des polices** : fallback propre si un woff2 manque (pas de page nue), `font-display: swap`. → vérifié en Task 2.
- **`prefers-reduced-motion`** : les transitions ajoutées se neutralisent. → vérifié en Task 3.

## File Structure

- Modify `frontend/src/styles.css` : blocs de tokens (clair/sombre), échelle de rayons/ombres, `@font-face`, application arrondis+ombres aux composants. Cœur du lot.
- Create `frontend/src/fonts/` : woff2 auto-hébergés (Fraunces repris de `landing/fonts/`, JetBrains Mono + Instrument Serif récupérés depuis Google Fonts OFL).
- Modify `frontend/index.html` : `<link rel="preload">` des polices critiques (Fraunces titres, JetBrains Mono).
- `frontend/src/App.jsx` : **inchangé** (bascule de thème conservée) — référencé seulement.

---

### Task 1 : Tokens de la charte (couleurs + rayons/ombres), clair & sombre

**Files:**
- Modify: `frontend/src/styles.css` (blocs `:root` lignes 8-42, `:root.light` 44-74, et le bloc « variante chaleureuse beta » ~1172)

**Interfaces:**
- Produces : le vocabulaire de tokens que toutes les tâches suivantes consomment — noms conservés (`--background`, `--card`, `--foreground`, `--muted-foreground`, `--border`, `--secondary`, `--primary`, `--primary-foreground`, `--risk/-bg`, `--delay/-bg`, `--healthy/-bg`, `--sidebar*`, `--logo-ink`, `--t-*`) + nouveaux : `--radius-sm:12px`, `--radius-md:16px`, `--radius-lg:24px`, `--radius-pill:9999px`, `--shadow-card`, `--shadow-soft`.

- [ ] **Step 1 — Thème clair = charte paper.** Dans `:root.light`, mapper : `--background:#F7F4F1`, `--secondary:#F1EDE9`, `--card:#FFFFFF`, `--foreground:#22272E`, `--muted-foreground:#746D73`, `--border:#E6DED7`, `--primary:#0E7490`, `--primary-foreground:#FFFFFF`, `--risk:#B5432A`, `--delay:#A86523`, `--healthy:#3A7D5C` (avec `-bg` clairs dérivés : teintes pâles de chaque), sidebar sur blanc/paper (`--sidebar:#FFFFFF`, `--sidebar-hover:#F1EDE9`, `--sidebar-border:#E6DED7`, textes en ink/ink-mute), `--logo-ink:#22272E`. Re-dériver `--t-*` pour rester distincts ET lisibles (AA) sur crème.

- [ ] **Step 2 — Thème sombre = variante chaude.** Dans le bloc beta `~1172`, garder les neutres chauds (`#17151c`/`#f4f2f0`/`#221f2a`…), basculer l'accent sur le canard (`--primary` = version éclaircie du teal pour contraste sur fond sombre, p.ex. `#3FA7C0`), aligner `--risk/--delay/--healthy` sur des versions lisibles des couleurs charte. Supprimer la duplication froide devenue morte du `:root` ligne 8 (ou la ramener à la variante chaude) pour qu'il ne reste qu'une définition de sombre.

- [ ] **Step 3 — Échelle de formes.** Ajouter dans les invariants : `--radius-sm/md/lg/pill` et `--shadow-card:0 8px 32px rgba(34,39,46,.08)`, `--shadow-soft:0 2px 8px rgba(34,39,46,.06)`. Conserver `--radius` (compat) en l'aliasant sur `--radius-sm`.

- [ ] **Step 4 — Vérifier le build + le contraste.** Run : `npm run build && npm run lint`. Expected : build OK, lint sans erreur. Puis contrôle contraste (outil au choix) : `--ink`/`--paper` ≥ 4.5:1, `--teal`/`--paper` ≥ 4.5:1 (texte) ou usage gros/bouton, `--t-*`/`--paper` lisibles. Noter les ratios dans le message de commit.

- [ ] **Step 5 — Commit.**
```bash
git add frontend/src/styles.css
git commit -m "charte: tokens paper (clair) + sombre chaud + échelle de rayons/ombres"
```

---

### Task 2 : Polices de la charte (auto-hébergées) + titres serif

**Files:**
- Create: `frontend/src/fonts/*.woff2`
- Modify: `frontend/src/styles.css` (`@font-face`, tokens de police, titres), `frontend/index.html` (preload)

**Interfaces:**
- Consumes : tokens de Task 1.
- Produces : `--font-serif` (Fraunces, titres), `--font-sans` (corps), `--font-mono` (JetBrains Mono) ; titres `h1..h3`/classes de titre en serif.

- [ ] **Step 1 — Récupérer les woff2.** Copier `landing/fonts/fraunces-*.woff2` vers `frontend/src/fonts/`. Récupérer **JetBrains Mono** (400,500) et **Instrument Serif** (400) en woff2 (Google Fonts, licence OFL). Si réseau indisponible à l'exécution : le signaler et s'arrêter (ne pas inventer de fichiers).

- [ ] **Step 2 — `@font-face`.** Déclarer chaque police dans `styles.css` avec `font-display:swap` et le bon `format('woff2')`. Pointer `--font-serif:"Fraunces", Georgia, serif`, `--font-mono:"JetBrains Mono", ui-monospace, monospace`, garder `--font-sans` (Inter + fallback système). Ajouter `--font-accent:"Instrument Serif", serif`.

- [ ] **Step 3 — Titres en serif.** Appliquer `font-family:var(--font-serif)` aux titres de page/section (repérer les classes de titre existantes dans `styles.css` et les composants). Ne pas serifier les étiquettes mono ni le corps.

- [ ] **Step 4 — Preload.** Dans `index.html`, `<link rel="preload" as="font" type="font/woff2" crossorigin>` pour Fraunces (titres) et JetBrains Mono.

- [ ] **Step 5 — Vérifier.** Run : `npm run build && npm run lint`. Puis `npm run dev`, ouvrir l'app, confirmer visuellement : titres en Fraunces, mono en JetBrains, aucun flash de page nue (fallback OK). Capture d'écran d'une page (p.ex. Synthèse) jointe au retour.

- [ ] **Step 6 — Commit.**
```bash
git add frontend/src/fonts frontend/src/styles.css frontend/index.html
git commit -m "charte: polices auto-hébergées (Fraunces/JetBrains Mono/Instrument Serif) + titres serif"
```

---

### Task 3 : Reshape des formes (arrondis + ombres douces) sur les composants partagés

**Files:**
- Modify: `frontend/src/styles.css` (les ~26 `border-radius` + surfaces sans rayon : cartes, champs, boutons, chips, kpi, sidebar, modales, panneaux)

**Interfaces:**
- Consumes : `--radius-*`, `--shadow-*` de Task 1.

- [ ] **Step 1 — Cartes & panneaux.** Passer les conteneurs (`.card`, `.kpi`, `.cat-*`, panneaux latéraux `SourcePanel`/`DraftPanel`, modales) à `border-radius:var(--radius-lg)` + `box-shadow:var(--shadow-card)` sur fond `--card`, bordure `--paper-border`/`--border`.

- [ ] **Step 2 — Contrôles.** Boutons, champs, selects, chips, onglets → `--radius-sm` (chips/badges → `--radius-pill`), focus visible (anneau canard `--teal` à faible opacité). Icônes carrées → `--radius-md`.

- [ ] **Step 3 — Transitions sobres.** Ajouter des transitions douces (`transition: … .2s cubic-bezier(.16,1,.3,1)`) sur hover/focus des éléments interactifs, et un bloc `@media (prefers-reduced-motion: reduce)` qui neutralise durées d'animation/transition. Aucune animation marketing.

- [ ] **Step 4 — Vérifier.** Run : `npm run build && npm run lint`. `npm run dev` : parcourir 3-4 pages (Synthèse, Suivi, À valider, Réglages), confirmer arrondis + ombres cohérents, focus visibles, et tester `prefers-reduced-motion` (devtools) → transitions coupées. Captures jointes.

- [ ] **Step 5 — Commit.**
```bash
git add frontend/src/styles.css
git commit -m "charte: reshape arrondis + ombres douces sur les composants partagés"
```

---

### Task 4 : Passe de finition page par page

**Files:**
- Modify: `frontend/src/styles.css`, et au besoin les composants/pages (`frontend/src/pages/*.jsx`, `frontend/src/components/*.jsx`) pour les styles en ligne ou couleurs codées en dur.

**Interfaces:**
- Consumes : tout ce qui précède.

- [ ] **Step 1 — Audit couleurs en dur.** Run : `grep -rniE "#[0-9a-f]{3,6}|rgba?\(" frontend/src/pages frontend/src/components | grep -v styles.css`. Pour chaque couleur en dur trouvée dans le JSX, la remplacer par le token de charte adéquat (garantit la parité clair/sombre).

- [ ] **Step 2 — Revue des 11 pages.** `npm run dev`, ouvrir chaque page (Agent, Alertes, À valider, Chat, Interlocuteurs, Liens, Miroir, Réglages, Suivi, Synthèse) en clair ET en sombre. Corriger dans `styles.css` les restes (espacements, contrastes, surfaces oubliées) pour une identité homogène.

- [ ] **Step 3 — Vérifier la parité.** Confirmer qu'aucune page n'a de zone illisible ou « froide » résiduelle dans l'un des deux thèmes. Capture clair + sombre d'une page dense (Suivi) jointe.

- [ ] **Step 4 — Build final.** Run : `npm run build && npm run lint`. Expected : verts.

- [ ] **Step 5 — Commit.**
```bash
git add -A frontend/src
git commit -m "charte: finition page par page + couleurs codées en dur migrées vers tokens"
```

---

## Self-Review

- **Couverture spec** : Décisions 2 (reshape complet → Tasks 1,3,4), 3 (sombre dérivé → Task 1 step 2), 6 (polices → Task 2), 5 (pas de fioriture → Task 3 step 3). Les simulateurs et la vitrine sont hors Lot 1 (lots 2 & 3, plans séparés). ✔
- **Granularité** : chaque tâche livre un état visible et commité ; pas de test unitaire fabriqué sur du CSS (vérif build+lint+visuel, honnête vu l'absence de runner). ✔
- **Cohérence des noms** : tokens de Task 1 réutilisés tels quels partout ; `--radius` aliasé pour compat. ✔
- **Review Focus** : parité clair/sombre (Task 4 step 1+3), contraste AA (Task 1 step 4), `--t-*` sur crème (Task 1 step 1), fallback polices (Task 2 step 5), reduced-motion (Task 3 step 3). ✔

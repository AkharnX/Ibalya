# Brief design — AgentOps (module Agent Ops & Delivery)

Document destiné à un designer produit ou à Figma AI / Figma Make.
L'application existe et fonctionne : il s'agit d'en refaire l'habillage, pas d'en
réinventer les fonctions. Les écrans, données et actions décrits ci-dessous sont
réels et déjà branchés à une API.

---

## 1. Prompt court (à coller dans Figma AI / Figma Make)

> Conçois l'interface d'**AgentOps**, un SaaS B2B français destiné aux dirigeants
> de PME (artisans, TPE de services, 5 à 50 salariés). L'application lit
> automatiquement les emails de l'entreprise et en extrait les **engagements**
> pris — qui a promis quoi, à qui, pour quand — puis alerte le dirigeant quand
> une promesse dérape et lui propose un message de relance à valider d'un clic.
>
> Cible : un dirigeant pressé, peu technophile, qui ouvre l'outil 3 minutes le
> matin sur ordinateur portable. Il doit comprendre en 5 secondes ce qui brûle.
>
> Style : **application métier moderne et sobre**, dans l'esprit de Linear,
> Height ou Vercel Dashboard — dense mais respirante, hiérarchie typographique
> forte, aucune illustration décorative, aucun emoji, pas de gradients criards.
> Thème sombre par défaut, thème clair en variante. Français uniquement.
>
> Écrans à produire : (1) **Synthèse** — tableau de bord de décision ;
> (2) **Suivi des engagements** — table filtrable ; (3) **Alertes** ;
> (4) **Agent** — configuration du contexte et des règles apprises ;
> (5) **Réglages** ; (6) **Panneau latéral de rédaction de message** ;
> (7) **Écran de connexion** ; (8) **États vides et premier lancement**.
>
> Livrer un design system complet : couleurs sémantiques, typographie, grille,
> composants (boutons, badges, tables, cartes, panneau latéral, champs,
> notifications), tous leurs états (défaut, survol, focus clavier, actif,
> désactivé, chargement, erreur).

---

## 2. Le produit en une page

**Problème.** Dans une PME, les engagements vivent dans les emails et dans la
tête du dirigeant. Rien n'est consigné : les devis promis s'oublient, les
fournisseurs en retard ne sont pas relancés, les échéances se télescopent.

**Solution.** Un agent se connecte en lecture seule à la boîte email, extrait
chaque engagement, les surveille dans le temps, et rend au dirigeant :
- une **synthèse** de ce qui demande une décision aujourd'hui,
- le **suivi** exhaustif de ses engagements,
- des **messages pré-rédigés** qu'il valide, modifie ou rejette.

**Promesse d'usage :** aucune saisie, aucune configuration, de la valeur dès le
premier jour.

**Deux principes non négociables à traduire visuellement :**
1. **Rien ne part sans validation humaine.** Chaque message généré est une
   proposition. Le design doit rendre l'action d'envoi consciente et
   réversible — jamais un bouton qu'on clique par réflexe.
2. **Un faux positif coûte dix fois un vrai positif.** L'interface ne doit
   jamais crier au loup. Le niveau de fiabilité de chaque information doit être
   lisible, et le calme visuel est une fonctionnalité.

---

## 3. Utilisateur et contexte d'usage

| | |
|---|---|
| **Persona** | Marc, 47 ans, dirigeant d'une menuiserie de 8 personnes. Gère devis, chantiers, fournisseurs et SAV par email. |
| **Compétence numérique** | Sait utiliser Gmail et Excel. N'a jamais utilisé de CRM. Se méfie de l'IA. |
| **Contexte** | Ordinateur portable 13–15", le matin ou le soir. Parfois sur tablette. Rarement sur mobile, mais l'app doit rester consultable. |
| **Durée de session** | 3 à 5 minutes, plusieurs fois par semaine. |
| **Question qu'il se pose en ouvrant** | « Qu'est-ce que j'ai oublié ? Qu'est-ce qui va me retomber dessus cette semaine ? » |

**Conséquences de design :** priorité absolue à la lisibilité au premier coup
d'œil ; les actions les plus fréquentes doivent être accessibles sans navigation ;
le vocabulaire doit être celui du métier, jamais celui de l'informatique
(« engagement », « relance », « échéance » — et non « ticket », « objet »,
« instance »).

---

## 4. Écrans à concevoir

### 4.1 Synthèse — page d'accueil

L'écran le plus important. Il répond à : *que dois-je décider maintenant ?*

**Bloc 1 — Indicateurs (5 tuiles cliquables en ligne)**
`Engagements suivis` · `Retards en cours` (accent ambre) · `Engagements à
risque` (accent rouge) · `Messages à valider` · `Messages lus (30 j)` (non
cliquable, informatif).
Chaque tuile : un nombre en grand, un libellé en petit. Le clic filtre l'écran
de suivi ou ouvre la file de messages.

**Bloc 2 — « Ce qui demande une décision maintenant »**
Liste de 0 à 8 lignes, triées par gravité. Chaque ligne comporte :
- une pastille de gravité : `À RISQUE` (rouge) ou `RETARD` (ambre) ;
- le titre de l'engagement (ex. « Pose de 12 fenêtres à l'école ») ;
- une ligne de contexte expliquant la cause
  (ex. « Échéance 06/08 — bloquée par : livraison des vitrages, attendue le 30/07 ») ;
- deux actions à droite : *marquer résolu* et *rédiger un message* (action principale).

> **Notion clé à faire ressentir :** la **consolidation par cause racine**. Un
> retard fournisseur qui menace trois chantiers ne produit pas trois alertes
> identiques, mais une cause et ses conséquences. Le design doit rendre ce lien
> visible (indentation, connecteur, ou regroupement) — c'est le différenciateur
> du produit.

**Bloc 3 — « Vue d'ensemble par catégorie »**
Trois cartes côte à côte : *Engagements en cours* (bleu), *Retards probables*
(ambre), *Engagements à risque* (rouge). Chacune : un compteur, deux lignes
d'aperçu, un lien « Voir les N engagements → ».

### 4.2 Suivi des engagements

Table dense et filtrable — l'écran de travail.

- **Filtres primaires** (pastilles) : Tous · En cours · Retards · À risque, avec compteur.
- **Filtres secondaires** (pastilles plus discrètes) : par type — Livraison,
  Devis, Facturation, Rendez-vous, Prise de contact.
- **Recherche** alignée à droite : par client.
- **Colonnes :** Type (badge coloré) · Engagement · Échéance · Fiabilité ·
  Statut · Actions.
- La cellule *Engagement* est riche : titre, sous-ligne
  `expéditeur → destinataire` en gris, marqueur « à confirmer » si la date a été
  déduite par l'agent, mention du blocage amont s'il existe, et **un lien
  d'action suggérée** (ex. « → Relancer le fournisseur »).
- **Fiabilité** : petite barre de progression (0–100 %), pas de chiffre brut.
- **Actions de ligne** : trois boutons icône — rédiger un message (principal),
  marquer livré, écarter.

### 4.3 Panneau latéral de rédaction (composant transverse)

Panneau qui glisse depuis la droite (largeur ~420 px), avec voile sombre sur le
fond. Trois états :
1. **Chargement** — « L'agent rédige le message… »
2. **Aperçu** — badge « Aperçu », champs *À* / *Objet* en lecture, corps du
   message en zone de texte verrouillée, teinte discrète.
3. **Modification** — badge « Modification » (ambre), zone de texte active et
   mise en évidence.

Pied de panneau : bouton secondaire *Modifier* / *Revenir à l'aperçu*, bouton
principal *Valider et envoyer*.
En-tête : l'intention (« Relancer le fournisseur ») et, en dessous, le motif
(« Livraison en retard depuis le 30/07, bloque 2 engagements clients »).

**Variante « file d'attente »** : même panneau, listant tous les messages en
attente, chaque ligne cliquable menant à l'aperçu.

### 4.4 Alertes

Table des détections de l'agent, avec cinq natures : échéance à risque, silence
anormal, contradiction, engagement orphelin, surcharge. Colonnes : nature,
détail, fiabilité, date, action *écarter*.
Un encart dépliable explique les cinq natures en langage courant.

### 4.5 Agent

Page de configuration lisible par un non-technicien, en deux parties :
- **« Ce que je comprends de votre activité »** — un texte structuré et éditable
  décrivant secteur, clients récurrents, fournisseurs critiques, rythme des
  affaires. *Aujourd'hui c'est un bloc JSON brut : c'est le point le plus faible
  de l'interface actuelle, à repenser en formulaire clair (champs, listes de
  personnes, étiquettes).*
- **« Vos intentions »** — trois questions ouvertes (priorités du moment,
  dossiers à surveiller, ce qui coûte le plus d'énergie).
- **« Ce que j'ai appris de vos corrections »** — liste des règles apprises,
  écrites en français, chacune désactivable.

### 4.6 Réglages

Deux panneaux : *Connexion* (canal email, relancer l'analyse initiale) et
*Comportement* (seuil de fiabilité, rythme du digest, envoi par email).
Puis une grille d'indicateurs de qualité, et le journal d'audit (table dense,
horodatée).

### 4.7 Connexion

Écran minimal centré : logo, un champ jeton d'accès, un bouton. Doit inspirer
confiance et sobriété.

### 4.8 États à ne pas oublier

- **Premier lancement** : aucune donnée, l'agent lit encore la boîte email.
  Prévoir un écran d'attente rassurant qui explique ce qui se passe.
- **Vide par filtrage** : « Aucun engagement ne correspond à ces filtres. »
- **Vide vertueux** : « Rien à arbitrer aujourd'hui. » — doit être valorisant,
  pas terne : c'est le but du produit.
- **Erreur** : service indisponible, jeton invalide, envoi échoué.
- **Chargement** : squelettes de table plutôt que roues qui tournent.

---

## 5. Direction artistique

**Registre :** outil de travail professionnel, calme, précis. Références :
Linear, Height, Vercel, Stripe Dashboard. À éviter absolument : esthétique
« startup IA » (dégradés violets, halos, particules), illustrations 3D,
emojis dans l'interface, ombres portées marquées, coins très arrondis.

**Thème sombre par défaut** (l'application s'utilise souvent le soir), thème
clair à livrer en variante complète.

**Couleurs — rôles sémantiques stricts.** Le point crucial : ne jamais réutiliser
une couleur d'état pour une couleur de catégorie.
- *Couleurs d'état* : rouge = à risque / en retard critique · ambre = retard,
  attention · vert = fait, sain · bleu = information.
- *Couleurs de catégorie* (types d'engagement, identité et non gravité) : une
  teinte distincte par type — Livraison, Devis, Facturation, Rendez-vous, Prise
  de contact — toutes lisibles côte à côte et distinguables en cas de daltonisme
  (vérifier deutéranopie et protanopie).
- Palette actuelle donnée à titre indicatif, à professionnaliser :
  fond `#0b0d16`, surfaces `#12141f` / `#171a28` / `#1d2032`, bordures `#262a3d`,
  texte `#eceef5` / `#9497ab` / `#6b6e82`, accent `#7c85f5`.

**Typographie.** Une seule famille sans-serif, très lisible en petites tailles
(Inter, Geist ou équivalent système). Échelle serrée : titre de page ~21 px,
titre de section ~15 px, corps 13,5–14 px, annotations 11,5–12 px. Chiffres
tabulaires pour toutes les colonnes numériques et les dates.

**Densité.** Dense mais aérée : hauteur de ligne de table ~44 px, gouttières de
12–14 px, largeur de contenu maximale ~1180 px centrée.

**Accessibilité.** Contraste AA minimum sur tout texte, y compris les libellés
gris. Focus clavier visible sur chaque élément interactif. Aucune information
portée par la couleur seule — toujours doublée d'un mot ou d'une icône.

---

## 6. Composants à livrer dans la bibliothèque

Boutons (principal, secondaire, fantôme, icône, destructeur) · Badges de type ·
Pastilles d'état · Tuile d'indicateur · Carte de catégorie · Ligne de priorité ·
Table (en-tête, ligne, ligne survolée, ligne sélectionnée, cellule riche) ·
Barre de fiabilité · Pastilles de filtre · Champ de recherche · Champs de
formulaire (texte, zone de texte, liste déroulante, nombre) · Panneau latéral ·
Voile · Notification éphémère (succès, erreur) · Encart dépliable · État vide ·
Squelette de chargement · Barre de navigation supérieure · Indicateur
d'environnement.

Chaque composant avec ses états : défaut, survol, focus clavier, actif,
désactivé, chargement, erreur.

---

## 7. Contraintes techniques

- **Implémentation** : React (Vite) + CSS natif avec variables. Pas de
  Tailwind ni de bibliothèque de composants imposée — mais un design qui se
  traduit en variables CSS et composants simples sera implémenté beaucoup plus
  vite.
- **Livrables attendus** : fichier Figma avec pages *Design system*, *Écrans*,
  *États* ; variables Figma pour couleurs, typographie et espacements
  (nommage sémantique, ex. `surface/1`, `text/secondary`, `state/risk`) ;
  composants avec variantes et auto-layout ; export des icônes en SVG.
- **Icônes** : jeu cohérent, style ligne, 16 et 20 px (Lucide ou Phosphor).
- **Écrans à cadrer** : 1440 px de large en priorité, 1280 px vérifié, tablette
  1024 px acceptable, mobile 390 px en consultation seule.
- **Langue** : français intégral, y compris les libellés système. Vouvoiement.
  Ton direct et factuel, jamais infantilisant, jamais commercial.

---

## 8. Trois questions à trancher avec le designer

1. Comment représenter visuellement la **chaîne de causalité** (un retard amont
   qui menace plusieurs engagements aval) sans alourdir la synthèse ?
2. Comment afficher la **fiabilité** d'une information issue de l'IA de manière
   honnête mais non anxiogène — barre, mention textuelle, ou traitement plus
   discret ?
3. Comment rendre la page **Agent** (le contexte que l'IA s'est construit)
   éditable par un dirigeant sans jamais exposer de structure technique ?

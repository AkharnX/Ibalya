# Recherche de pièces jointes : retrouver un fichier reçu par mail

Design, 19 novembre 2026. Rédigé avec Ibrahim à partir d'un retour de testeur.

## Intention

Un testeur essaie de retrouver ses factures, reçues en pièces jointes dans sa
boîte mail, pour faire sa comptabilité. Il les cherche une par une et il galère.
Les factures sont l'exemple déclencheur ; le besoin est général : **retrouver
n'importe quelle pièce jointe reçue** (devis, plan, photo de chantier,
contrat, relevé) par un mot de son nom, de son contenu, de l'expéditeur ou de
l'objet. Ibalya lit déjà la boîte, extrait le texte des pièces et sait relier
un message à son fil : l'utilisateur doit pouvoir taper « Qonto octobre »,
« plan terrasse » ou « devis Dupont » dans la barre de recherche et tomber sur
la pièce, avec un lien vers le mail d'origine pour la télécharger dans sa
messagerie.

## Ce qu'on a constaté dans le code et en production

- Les pièces jointes sont enregistrées (nom, type, texte extrait, jamais le
  fichier) **seulement pour les messages conservés** après le pré-filtre
  (`ingester.go`, branche `else` de `Run`). Les messages écartés comme
  automatiques ou newsletters n'en ont aucune.
- Or les factures viennent presque toujours de robots (facturation@, noreply@,
  opérateurs, banques, places de marché). En production : 516 messages
  analysés portent 35 pièces jointes ; 3 049 messages écartés n'en portent
  aucune ; aucune des 35 n'a un nom de facture. **Ibalya ne sait rien des
  factures aujourd'hui.** Ce n'est pas la recherche qui manque, c'est la
  matière.
- On ne garde jamais le fichier. Pour le récupérer, l'utilisateur retourne dans
  sa messagerie ; les connecteurs savent construire le lien vers un message
  (`LienWeb`) pour Gmail et Outlook, pas pour IMAP.
- La recherche transverse (`GET /api/recherche`, `Recherche.jsx`) couvre
  engagements, interlocuteurs et conversations en `LIKE`, six résultats par
  type. Pas de recherche plein texte en base aujourd'hui (PostgreSQL 16).
- La lecture complète couvre 30 jours à l'onboarding, puis 2 jours glissants à
  chaque cycle ; les messages déjà connus sont ignorés (`!inserted`), donc une
  simple relecture ne rattraperait pas les pièces jointes manquantes.
- **Seul le connecteur Gmail récupère les pièces jointes** (`piecesJointesGmail`).
  Les connecteurs Outlook (Microsoft Graph) et IMAP renvoient des messages sans
  pièces, même conservés : aucune pièce n'a jamais été enregistrée pour ces
  boîtes. En production, la boîte la plus fournie d'un testeur externe est en
  IMAP (697 messages) ; aucune boîte Outlook n'est raccordée.
- Les messages purgés par la commande `confidentialite` portent bien un motif
  `categorie_sensible_*` (`cmd/confidentialite/main.go`) : la règle « rien »
  par motif les couvre.
- Les motifs d'exclusion en production : `expediteur_automatique`,
  `newsletter`, `domaine_exclu`, `corps_vide`, `securite_suspect`,
  `categorie_sensible_sante`, `categorie_sensible_rh` ; dans le code aussi
  `fil_exclu`, `regle_apprise`, `publicitaire`.
- La page publique Données et sécurité promet que les emails automatiques et
  les hameçonnages « ne sont jamais transmis au modèle de langage ». L'OCR
  passe par le service Mistral : OCRiser une pièce d'un message automatique
  romprait cette promesse.

## Objectifs

- Depuis la barre de recherche, en mode « Pièces jointes », retrouver une pièce
  par un mot de son nom, de son contenu, de l'expéditeur ou de l'objet du
  message, et voir sa date.
- **Tout document est retrouvable au moins par son nom** : PDF, Word, Excel,
  PowerPoint, texte et CSV, même quand aucun texte n'en est extrait (PDF
  scanné sans OCR). Les photos ne sont retenues que si elles sont des documents
  photographiés, c'est-à-dire si l'OCR en tire du texte.
- Chaque résultat mène au mail d'origine dans la messagerie de l'utilisateur
  (Gmail, Outlook) ; faute de lien (IMAP), il donne de quoi le retrouver à la
  main.
- Les pièces jointes des messages automatiques et des newsletters sont
  indexées au fil de l'eau, et les 30 derniers jours des boîtes déjà raccordées
  sont rattrapés une fois au déploiement.
- Les boîtes **Gmail et IMAP** sont couvertes : le connecteur IMAP apprend à
  récupérer les pièces jointes, qu'il ignorait jusqu'ici.

## Non-objectifs

- Stocker ou servir le fichier lui-même : l'utilisateur le télécharge dans sa
  messagerie. Ce choix est celui d'Ibrahim ; il garde la promesse « jamais le
  fichier » de la page Données.
- Reconnaître qu'une pièce est une facture, en extraire montant, fournisseur ou
  numéro, produire un export comptable. Ce sera un autre chantier, s'il a lieu.
- **Outlook (Microsoft Graph)** : aucune boîte Outlook n'est raccordée en
  production. Le connecteur reste sans pièces jointes ; le texte public le dit,
  et la recherche d'une boîte Outlook renvoie une liste vide avec un libellé
  explicite. Ce sera fait au premier utilisateur Outlook.
- Remonter au-delà de 30 jours. Ibrahim prévoit des formules avec plus de
  profondeur plus tard ; la profondeur est un paramètre, pas une architecture.
- Modifier l'extraction d'engagements : les messages écartés restent écartés,
  rien de ce qu'on indexe ne part au modèle.

## Anti-objectifs (ce qui ferait de ce chantier un échec)

- Envoyer au service d'inférence le contenu d'un message automatique, d'une
  newsletter ou d'un hameçonnage, même pour de l'OCR.
- Ouvrir une pièce jointe d'un message classé `securite_suspect` : c'est
  précisément le vecteur d'attaque qu'on refuse d'ouvrir.
- Indexer quoi que ce soit d'un message d'une catégorie confidentielle (santé,
  RH, juridique), dont on promet de ne garder que l'expéditeur et la date.
- Faire grimper la facture d'OCR en passant les logos et pixels de suivi des
  newsletters à Mistral.
- Casser le cloisonnement : une recherche ne doit jamais voir une pièce d'une
  autre boîte (RLS, `app.user_id`, comme tout le reste).

## Contraintes

- Même table `attachments`, même cloisonnement RLS, mêmes règles de
  confidentialité que le corps des messages.
- Pré-filtre inchangé : les messages écartés gardent leur statut et leur motif,
  les compteurs du Miroir ne bougent pas, `TexteAttachments` (lu par
  l'extraction, par id de message analysé) n'est pas affecté.
- Le texte public (page Données et sécurité, catalogue des fonctionnalités) est
  mis à jour dans la même PR, sans tiret cadratin, sans chiffre inventé.
- Tout nouveau comportement est testé (test unitaire sur l'éligibilité, test de
  la requête de recherche, test du rattrapage sur le canal fixture), et le
  rattrapage est vérifié sur la pile jetable avant la prod (`env -i`, base de
  test, jamais le `.env` de prod).

## Approches envisagées

1. **Indexer chez nous (retenue).** À l'ingestion, extraire le texte des pièces
   jointes aussi pour les messages écartés (hors confidentiel et hameçonnage),
   dans la table existante, avec un index plein texte en français. Recherche
   instantanée, identique pour Gmail, Outlook et IMAP, et sur le contenu des
   PDF. C'est l'approche qu'Ibrahim a décrite. Coût : stockage du texte des
   pièces de robots (PDF générés, quelques Ko chacun), un rattrapage unique par
   boîte au déploiement.
2. **Interroger la messagerie en direct.** Au moment de la recherche, demander
   à Gmail « pièces jointes contenant X ». Rien à stocker, mais Gmail ne cherche
   pas dans le texte des PDF de la même façon, Microsoft Graph et IMAP encore
   moins (IMAP ne cherche pas dans les pièces jointes), et chaque recherche
   prend plusieurs secondes. On reproduirait la galère actuelle avec un autre
   habillage. Écartée.
3. **Garder les fichiers.** Stocker les octets des pièces jointes pour les
   servir directement. Supprimerait l'aller-retour vers la messagerie, mais
   rompt la promesse « jamais le fichier », multiplie le stockage et la surface
   d'attaque (fichiers de robots et de spammeurs sur notre serveur). Écartée
   par Ibrahim : l'utilisateur télécharge dans sa messagerie.

## Décisions prises

Par Ibrahim :

- Toutes les pièces jointes des 30 derniers jours sont indexées, robots compris.
- Un mode « Pièces jointes » dans la barre de recherche existante, pas une
  nouvelle page.
- Le résultat renvoie vers le mail dans la messagerie ; pas de téléchargement
  depuis Ibalya.
- Profondeur : 30 jours pour l'instant.

Par Ibrahim, sur ma recommandation, après relecture : **Gmail et IMAP
maintenant, Outlook plus tard**. Le testeur le plus actif est en IMAP ;
personne n'est en Outlook.

Par moi, à corriger si besoin (Ibrahim les a lues sans objection) :

- **Jamais pour `categorie_sensible_*` ni `securite_suspect`** : rien n'est
  extrait, rien n'est stocké, la pièce n'est même pas ouverte.
- **Pas d'OCR pour les messages écartés** : seulement la couche texte des PDF,
  DOCX et XLSX. Les images (`image/*`) des messages écartés sont ignorées (ce
  sont des logos). Les factures de robots sont des PDF générés avec texte. Pour
  les messages conservés, rien ne change (OCR comme aujourd'hui).
- **Les motifs `domaine_exclu`, `fil_exclu`, `regle_apprise` sont indexés** :
  ces règles coupent le bruit des engagements, elles ne cachent pas des
  documents. `corps_vide`, `newsletter`, `publicitaire`,
  `expediteur_automatique` aussi.
- **Rattrapage par une commande**, comme `-reappliquer-prefiltre`, lancée une
  fois en prod après le déploiement, puis inutile : l'ingestion courante prend
  le relais.
- **Recherche plein texte PostgreSQL** (`tsvector` en français, index GIN) sur
  nom + texte, complétée d'une correspondance sur l'expéditeur et l'objet du
  message. Pas de dépendance nouvelle.
- **Périmètre des pièces (décision d'Ibrahim) : les documents, pas les
  photos.** Un document (PDF, DOCX, XLSX, PPTX, TXT, CSV) est toujours
  enregistré avec son nom et son type, texte extrait ou pas : un PDF scanné
  sans OCR reste trouvable par son nom. Une image n'est enregistrée que si
  l'OCR en tire du texte (document photographié, reçu scanné) ; une photo sans
  texte n'est pas une pièce qu'on cherche. Les autres types (archives, plans,
  exécutables) sont ignorés. `TexteAttachments` ignore les lignes sans texte,
  donc l'extraction d'engagements ne change pas.

## Comportement attendu

### Connecteur IMAP : récupération des pièces jointes

Le connecteur IMAP remplit `Message.Attachments` comme Gmail le fait déjà :

- seules les parties qui portent un nom de fichier (disposition `attachment`,
  ou paramètre `name`/`filename`) sont des pièces jointes ; les parties
  `inline` sans nom (images de signature, HTML) ne le sont pas ; les types hors
  périmètre (archives, exécutables…) ne sont pas téléchargés ;
- une pièce plus grande que `channel.TailleMaxPJ` (12 Mo) n'est pas
  téléchargée ;
- le téléchargement des pièces ne doit pas faire échouer la lecture du message
  : une partie illisible est ignorée, le message est ingéré sans elle ;
- le format rendu à l'ingestion est strictement celui de Gmail (`Nom`, `Type`,
  `Donnees`), pour que tout le reste du pipeline soit commun.

Conséquence assumée : les messages **conservés** des boîtes IMAP auront
désormais leurs pièces (et l'OCR des images, comme Gmail), ce qui n'existait pas.
C'est le comportement attendu depuis le début, pas un effet de bord.

### À l'ingestion

Pour chaque message nouvellement inséré :

| Issue du pré-filtre | Pièces jointes |
|---|---|
| Conservé (analysé) | Mode **complet** : tout document (nom, type, texte s'il y en a) ; OCR des PDF scannés et des images si disponible ; une image n'est gardée que si l'OCR en tire du texte. |
| Écarté : `expediteur_automatique`, `newsletter`, `publicitaire`, `domaine_exclu`, `fil_exclu`, `regle_apprise`, `corps_vide` | Mode **texte seul** : tout document (nom, type, texte de la couche texte s'il y en a) ; pas d'OCR ; les images sont ignorées entièrement (sans OCR, impossible de distinguer un reçu photographié d'un logo). |
| Écarté : `categorie_sensible_*`, `securite_suspect` | Rien. La pièce n'est pas ouverte. |

Une fonction pure (`motif` → `aucune | texte_seul | complete`) porte cette
table et est testée unitairement. Tout motif commençant par `categorie_sensible_`
ou `securite_` donne `aucune` (règle de préfixe, pour qu'un motif ajouté plus
tard ne fasse pas sauter l'anti-objectif) ; un motif inconnu donne `texte_seul`,
jamais `complete`.

Un document dont le texte est vide est enregistré quand même, avec son nom et
son type, et reste trouvable par son nom. Une facture scannée sans couche
texte envoyée par un robot n'est donc trouvable que par son nom de fichier,
l'expéditeur ou l'objet. Accepté.

### Rattrapage des boîtes déjà raccordées

Commande `ibalya -indexer-pieces-jointes [-jours 30] [-appliquer]`, même
gabarit que le rattrapage du pré-filtre :

- pour chaque boîte active avec canal, dans son tenant, relit les messages des
  N derniers jours via le canal de la boîte (`FetchSince`), et pour chaque
  message **déjà connu** (même canal, même identifiant externe) **sans aucune
  pièce enregistrée** et dont la lecture rapporte au moins un document,
  applique la table ci-dessus selon son statut et son
  motif actuels, **plafonnée à `texte_seul`** : le rattrapage ne fait jamais
  d'OCR, même pour un message conservé. Sinon chaque lancement renverrait à
  Mistral les images de signature des messages conservés dont l'OCR n'avait
  rien donné, et la commande serait idempotente en écriture mais pas en coût.
  La matière cherchée (factures de robots) est de toute façon dans le texte ;
- sans `-appliquer`, affiche seulement ce qui serait indexé (messages
  concernés, pièces, par motif) ; avec, écrit et affiche le bilan ;
- idempotente : relancée, elle ne réécrit rien (les messages ayant déjà une
  pièce sont sautés) ;
- les messages inconnus de la base sont ignorés (ce n'est pas une ingestion).

Elle se lance une fois en prod après le déploiement, boîte par boîte, et son
bilan est consigné dans le journal d'audit (`agent`, `indexation_pieces_jointes`).

### Recherche

`GET /api/recherche/pieces-jointes?q=` (au moins trois caractères, sinon
liste vide), dans le tenant de la session. **Sans borne de date** : la
recherche porte sur tout ce qui est indexé. Les « 30 jours » sont la profondeur
du rattrapage initial et de la lecture à l'onboarding ; l'index grandit ensuite
au fil de l'eau et rien ne le purge (aucune rétention sur les messages
aujourd'hui). Une borne à 30 jours ferait disparaître la facture d'octobre fin
novembre, au moment précis où l'on fait sa comptabilité du mois. Correspondance :

- plein texte français (`websearch_to_tsquery('french', q)`) sur nom et texte
  de la pièce, via une colonne `tsvector` générée et un index GIN ;
- ou correspondance insensible à la casse sur l'expéditeur ou l'objet du
  message porteur, pour « Qonto » ou « OVH » même quand le PDF ne reprend pas
  le nom.

Résultats triés par date du message décroissante, 12 au plus, chacun avec :
identifiant de la pièce, nom, type MIME, identifiant du message et du fil,
expéditeur (nom ou adresse), objet, date, extrait du texte autour de la
correspondance (`ts_headline`, 160 caractères environ, vide si la
correspondance est sur l'expéditeur ou l'objet), et `url_web` construite par
`LienWeb` du canal de la boîte (vide pour IMAP).

### Interface

Dans la barre de recherche, deux chips sous le champ quand la liste est
ouverte : « Tout » (comportement actuel) et « Pièces jointes ». Le mode est
mémorisé pendant la session. Esquisse :

```
[🔍 facture qonto                                   ]
  ( Tout )  (• Pièces jointes )
  ┌──────────────────────────────────────────────────┐
  │ PDF  Facture-2026-10-QONTO.pdf           12/10   │
  │      Qonto · Votre facture d'octobre              │
  │      « …Total TTC 24,00 € · Facture n° F-2026-… » │
  │                               Ouvrir dans Gmail ↗ │
  ├──────────────────────────────────────────────────┤
  │ PDF  releve_octobre.pdf                  03/10   │
  │      Qonto · Relevé de compte                     │
  │                                      Voir le fil  │
  └──────────────────────────────────────────────────┘
```

- Le clic sur une ligne ouvre `url_web` dans un nouvel onglet ; sans `url_web`
  (IMAP), il ouvre le fil dans le Miroir (`/miroir?fil=`) si le fil s'y
  résout, sinon la ligne reste informative (expéditeur, objet, date suffisent
  à retrouver le mail à la main).
- Libellé du type : PDF, Word, Excel, PowerPoint, Texte, Image, déduit du
  type MIME puis de l'extension du nom.
- Vide : « Aucune pièce jointe pour « … ». »
- Le placeholder du champ devient « Rechercher un client, un engagement, une
  pièce jointe… ».

### Texte public et catalogue

- `landing/donnees.html`, section « Ce qu'Ibalya lit » : une phrase disant que
  les pièces jointes des emails automatiques (factures, relevés) sont lues en
  texte pour la recherche, sans OCR et sans passage par le modèle, et que les
  catégories confidentielles et les hameçonnages en restent exclus.
- `docs/FONCTIONNALITES.md`, section Livrables : ligne « Recherche de pièces
  jointes ».

## Mesure

- Après rattrapage en prod, par boîte : nombre de pièces indexées, par motif.
  Attendu : des dizaines de PDF de robots là où il y en avait zéro.
- Le testeur à l'origine du retour refait sa recherche de factures et dit s'il
  les trouve. C'est le seul critère qui compte.

## Laissé au constructeur

- La colonne `tsvector` générée est plafonnée par PostgreSQL à 1 Mo : le texte
  extrait d'un PDF de 12 Mo peut dépasser. L'expression générée tronque le
  texte (`left(texte, N)`), N à choisir ; l'INSERT et la migration au démarrage
  ne doivent jamais échouer pour une pièce trop longue.

- Le découpage exact des fonctions dans `ingest` (passer un mode à
  `enregistrerPiecesJointes` ou une fonction sœur) et le nom de la colonne
  `tsvector`.
- La taille des lots et la gestion du quota lors du rattrapage (30 jours de
  messages complets par boîte, une fois). En IMAP, préférer récupérer la
  structure du message (BODYSTRUCTURE) puis les seules parties utiles, plutôt
  que le message complet, si la bibliothèque le permet.
- L'ordre des résultats quand une pièce correspond à la fois par le texte et
  par l'expéditeur, et le libellé exact de l'extrait.
- Le style des chips et des lignes de résultat, dans la charte existante
  (plat, sans badge clignotant).
- La fixture de test : le canal fixture ne porte pas de pièces jointes
  aujourd'hui ; en ajouter (un PDF avec texte, un PDF scanné, une image, un
  zip) à un message automatique et à un message humain pour tester les lignes
  de la table et le périmètre des types.
- Le périmètre « document » dans `piecejointe` : PPTX, TXT et CSV ne sont pas
  extraits aujourd'hui ; décider s'ils sont extraits (PPTX via le même lecteur
  zip/XML que DOCX) ou seulement enregistrés par nom dans cette PR.

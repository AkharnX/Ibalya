# Réduire le bruit : réponse au retour BS ACCESS (volet « sous 30 jours »)

Source : retour d'utilisation de Stewe (BS ACCESS, grossiste en accessoires de téléphonie, 5 commerciaux), daté du 11 novembre 2026, sur une boîte réelle raccordée en IMAP après lecture de 30 jours. Mesures : 628 messages lus, 470 écartés sans IA, 62 engagements dont 29 actifs et 18 « en retard » pour la plupart faux, 84 alertes dont 80 « à vérifier », 31 brouillons dont jusqu'à 6 pour un même sujet, 12 dépendances proposées et 0 confirmée, 148 interlocuteurs tous « Autre ».

Verdict du testeur : « le bruit noie l'utile ». Son plan « sous 30 jours », repris ici tel quel : filtrage des automatiques et de l'hameçonnage, dédoublonnage des alertes et des brouillons, corrections des dates et libellés, détection des signaux de fin avec « c'est fait ? » en un clic.

## Décisions

### 1. Une alerte vivante par sujet

- Les clés de dédoublonnage des détecteurs « échéance à risque » et « promesse menacée » contiennent la date du jour (`detectors.go:149`, `:301`) : une alerte neuve par jour pour le même engagement. Les trois autres détecteurs sont déjà uniques par engagement ou par semaine.
- Décision : la clé ne contient plus la date. L'upsert existant de `CreateDetection` rafraîchit titre, détail, score et `created_at` tant que l'alerte est ouverte (`nouvelle`, `au_digest`), et ne ressuscite jamais une alerte écartée. Conséquence acceptée : une alerte traitée ou écartée ne revient pas pour le même engagement, même si l'échéance se rapproche encore.

### 2. Un brouillon par fil et par destinataire

- `maybeDraft` ne vérifie que « pas déjà un brouillon pour cette détection ». Une détection par jour (point 1) et plusieurs engagements par fil donnent six brouillons quasi identiques.
- Décision : avant d'appeler le modèle, l'agent vérifie qu'aucun brouillon `propose` n'existe déjà pour le même fil et le même destinataire (joint via la détection ou l'engagement). S'il en existe un, rien n'est créé. La commande manuelle « proposer une action » garde sa garde par engagement (clic explicite).
- Hors périmètre : fusionner trois engagements extraits pour une même commande (dédoublonnage sémantique). Noté pour le volet 90 jours.

### 3. Classer l'expéditeur avant l'extraction, et reconnaître l'hameçonnage

- Le pré-filtre actuel : une regex sur l'adresse (no-reply, notification, newsletter), `List-Unsubscribe`, 19 domaines. Les suivis de colis, commandes d'opérateurs, abonnements et notifications de logiciels passent.
- Décision : chaque message entrant reçoit une classe avant extraction : `humain`, `automatique`, `publicitaire`, `securite`. Seuls les `humain` vont à l'extraction. Signaux :
  - en-têtes `Auto-Submitted` (autre que `no`), `Precedence: bulk|list|junk`, `X-Auto-Response-Suppress`, `Feedback-ID` : capturés par les trois canaux dans un nouveau champ `AutoSubmitted` ;
  - adresse : regex élargie (noreply et variantes, notification(s), alert(e)(s), robot, automat, auto-confirm, shipment, shipping, tracking, suivi, order(s), commande(s)@, billing, invoice, abonnement, subscription, mailer, daemon, bounce, system) ;
  - domaines : liste actuelle plus transporteurs et places de marché (amazon, cdiscount, fnac, chronopost, colissimo, laposte.fr, ups, dhl, fedex, tnt, dpd, gls, mondialrelay) et hébergeurs/outils (ovh, ovhcloud, ionos, qonto, pennylane, sellsy, axonaut). Pas les opérateurs télécom par domaine : leurs commerciaux écrivent depuis ces domaines ;
  - corps : « message automatique », « ne pas répondre à ce », « ne répondez pas à ce », « envoyé automatiquement », « do not reply », « this is an automated » ;
  - `securite` (hameçonnage possible) sur objet + corps : connexion suspecte, activité inhabituelle/suspecte, compte suspendu/bloqué/désactivé/verrouillé, vérifiez votre identité/compte, confirmez vos informations/coordonnées, sous 24 h / dans les 24 heures, mot de passe expiré/expire, informations de paiement à mettre à jour, unusual sign-in, suspicious activity/login, verify your account.
- Motifs d'exclusion : `expediteur_automatique`, `newsletter`, `securite_suspect`. Les messages exclus gardent expéditeur, date et corps (comme aujourd'hui), ne sont jamais envoyés au modèle, et aucun brouillon ne peut les viser.
- Rattrapage des boîtes déjà lues : une commande `-reappliquer-prefiltre` reclasse les messages déjà en base avec les nouvelles règles ; un engagement dont le message source devient exclu et qui n'a reçu aucun autre événement (relance, confirmation, signal) passe en statut `annule` avec un événement `exclu_automatique`. Sans `--appliquer`, la commande compte et montre un échantillon.
- Affichage : le miroir d'activité indique le nombre de messages écartés par classe, avec la mention « hameçonnage possible, ne pas répondre » pour la classe `securite`.

### 4. Libellés et dates

- Jours : plus de « jour(s) ». Un libellé relatif en jours calendaires : « aujourd'hui », « demain », « hier », « dans N jours », « en retard de N jours », singulier au besoin. Le calcul se fait sur des dates (minuit local), pas sur des durées en heures : c'est ce qui produisait « dans 0 jour(s) » pour une échéance dépassée.
- Objets : les dates ISO (`2026-10-01`) que le modèle laisse dans les objets sont réécrites en `01/10` (année courante) ou `01/10/2027` (autre année) à l'insertion ; le prompt demande de ne pas répéter dans l'objet une date déjà portée par `echeance`.
- Interlocuteur inconnu : « expéditeur inconnu » et « destinataire inconnu » à la place de « ? ».

### 5. « C'est fait ? » en un clic

- Aucun engagement ne se ferme sans geste, et le geste existant (icône « Marquer livré ») n'est pas vu.
- Décision : un sixième détecteur, `a_clore`, non critique, une seule détection par engagement, pour tout engagement `en_retard` qui remplit l'une des deux conditions :
  - échéance dépassée depuis 7 jours ou plus et aucun événement depuis 7 jours ;
  - un message du fil postérieur à l'échéance contient un signal de fin : livré, réceptionné, bien reçu, reçu ce jour, payé, réglé, virement effectué/reçu, merci pour la livraison/l'intervention, facture jointe/ci-jointe/acquittée, c'est fait, terminé.
  Titre : « C'est fait ? ». Détail : la raison, avec l'extrait du message quand il y en a un. Jamais de brouillon pour ce type.
- Interface : dans Alertes, une détection `a_clore` propose « Oui, c'est fait » (l'engagement passe `livre`, l'alerte est écartée) et « Toujours en cours » (l'alerte est écartée, un événement `confirme` est ajouté). Dans la synthèse, les lignes en retard portent un bouton texte « C'est fait » en plus de l'icône.
- Hors périmètre : connecteurs vers facturation, paiement ou transporteurs.

## Contraintes générales

- Rien ne part sans validation : aucun changement ne touche à l'envoi.
- Copie : pas de tiret cadratin, pas de libellés en capitales espacées, pas de chiffres inventés.
- Les migrations de schéma suivent le style de `schema.go` (`ALTER TABLE … ADD COLUMN IF NOT EXISTS`) et sont testées avec le rôle `ibalya_app`, pas en super-utilisateur.
- Mesure : après déploiement, rejouer la boîte BS ACCESS et comparer aux chiffres du 11 novembre (alertes, brouillons, engagements en retard, exclusions).

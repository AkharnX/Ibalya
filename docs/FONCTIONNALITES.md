# Ibalya : catalogue des fonctionnalités

État au 15 novembre 2026. Ce fichier est la référence : la page Admin de l'application l'affiche tel quel.

## Ce que fait Ibalya en une phrase

Il lit la boîte mail d'un dirigeant de PME en lecture seule, repère les engagements pris ou reçus, surveille leurs échéances, et prépare les relances. Rien ne part sans un clic du dirigeant.

## 1. Raccordement et lecture

- **Trois canaux** : Gmail (OAuth Google), Outlook (OAuth Microsoft), toute boîte IMAP (identifiants chiffrés au repos). Connexion testable avant enregistrement.
- **Lecture seule** : l'agent ne modifie, ne déplace et ne supprime jamais un message.
- **Lecture initiale de 30 jours** à l'onboarding, puis lecture automatique toutes les 15 minutes.
- **Plusieurs boîtes par compte** : chaque boîte est un espace isolé (cloisonnement en base), on bascule de l'une à l'autre ; vue combinée « toutes les boîtes » pour la synthèse et les brouillons.
- **Mes autres adresses** : alias et adresses perso du dirigeant, pour ne pas le prendre pour un tiers.

## 2. Pré-filtre avant toute analyse

- **Classe d'expéditeur** décidée avant l'extraction : humain, automatique, publicitaire, sécurité. Seuls les humains partent au modèle.
- Signaux : en-têtes d'envoi automatique (Auto-Submitted, Precedence, Feedback-ID), adresses de robots, domaines de transporteurs, places de marché, hébergeurs et outils de gestion, phrases « message automatique », en-tête de désinscription.
- **Hameçonnage** : les gabarits d'urgence artificielle (connexion suspecte, compte suspendu, « sous 24 h ») sont écartés et jamais proposés à la relance.
- **Catégories confidentielles** (santé, ressources humaines, juridique) : le contenu n'est pas conservé, seuls l'expéditeur et la date restent.
- **Règles apprises** : ignorer un interlocuteur ou un domaine, fil exclu par le dirigeant.
- **Rattrapage** des boîtes déjà lues avec les règles actuelles (commande `-reappliquer-prefiltre`), en simulation puis en application.

## 3. Extraction des engagements

- **Qui promet quoi, à qui, pour quand** : émetteur, destinataire, objet, type (devis, livraison, relance, prise de contact, rendez-vous, facturation, autre), échéance explicite ou inférée.
- **Pièces jointes** : texte extrait, OCR au besoin, jamais le fichier conservé.
- **Un engagement par promesse dans un fil** : une reformulation du même engagement devient une mention de l'existant, qui hérite d'une échéance explicite.
- **Sens** : « je dois », « on me doit », « pour information » (deux tiers entre eux, le dirigeant en copie). Le « pour information » ne produit ni retard ni alerte.
- **Fiabilité** calculée à partir de signaux vérifiables (échéance explicite, formulation engageante, fil conversationnel, interlocuteur connu, historique de corrections), l'avis du modèle ne pesant qu'un dixième. Affichée en trois niveaux : élevée, à vérifier, incertaine. Rien n'est présenté proactivement sous le seuil de publication (0,6 par défaut, réglable).
- **Signaux sur un engagement ouvert** : avancement, livré, relance, confirmation, abandon, déduits des messages suivants.
- **Échéance inférée à confirmer** avant toute alerte.
- **Dates dans les objets** au format français ; libellés relatifs justes (« demain », « en retard de 6 jours »).

## 4. Détecteurs (toutes les 30 minutes)

1. **Échéance à risque** : échéance dans les 7 jours sans signal de progression.
2. **Silence anormal** : fil sans réponse au-delà de deux fois son rythme habituel (96 h par défaut), dans les deux sens (« on attend ma réponse », « je n'ai pas de retour »).
3. **Promesse menacée** : un retard amont confirmé met en danger une promesse aval dans les 7 jours (uniquement sur un lien de dépendance confirmé).
4. **Engagement orphelin** : rien ne s'est passé 5 jours après la promesse.
5. **Surcharge** : 5 échéances ou plus la même semaine.
6. **C'est fait ?** : retard de 7 jours sans activité, ou fil qui annonce la fin (livré, payé, facture jointe). Réponse en un clic : « Oui, c'est fait » ou « Toujours en cours ».

Une seule alerte vivante par engagement, rafraîchie à chaque cycle, jamais ressuscitée si elle a été écartée.

## 5. Dépendances

- **Candidats** par heuristique (même fil ou mêmes interlocuteurs, échéances rapprochées, types qui peuvent se bloquer), jugés par le modèle avec les décisions passées du dirigeant en exemples.
- **Seuil 0,7**, au plus cinq propositions par cycle, confiance affichée.
- **Le dirigeant tranche** : confirmer, rejeter, ou rejeter tous les candidats. Seuls les liens confirmés alimentent la détection des promesses menacées.

## 6. Livrables

- **Synthèse** : compteurs (actifs, en retard, retard probable, à valider, messages lus), « Vos 3 décisions du jour » classées par enjeu, le reste replié, alertes en cours, vue par catégorie.
- **Miroir d'activité** : ce que l'agent a compris des 30 derniers jours (engagements en cours, fils sans réponse, messages écartés par classe), à corriger d'un geste.
- **Capsule** : activité, rythme habituel, clients récurrents, fournisseurs critiques, interlocuteurs clés, déduits puis corrigés ; intentions du dirigeant (priorités, dossiers à surveiller).
- **Digest** quotidien ou hebdomadaire, dans l'application et par email si demandé, avec les brouillons d'action.
- **Brouillons de relance** rédigés à partir du fil d'origine, signés avec l'identité du dirigeant, relus par le modèle avant proposition ; un seul brouillon par fil et par destinataire.
- **À valider** : chaque brouillon se relit, se modifie, s'envoie ou se rejette. Aucun envoi automatique.
- **Assistant** : questions en langage naturel sur l'activité (« qui dois-je relancer cette semaine ? »), réponses sourcées sur les messages, historique effaçable.
- **Recherche** : clients, engagements, fils.

## 7. Apprentissage et corrections

- **Corrections d'un geste** sur chaque engagement : pas un engagement, mal résumé, abandonné, priorité haute pour cet interlocuteur, ne plus rien extraire de cet interlocuteur. Chaque geste devient une règle lisible et révocable.
- **Verdict d'extraction** (juste, faux, imprécis) distinct du statut, pour mesurer la qualité.
- **Règles métier** : délai d'alerte, silence par défaut, priorités du moment, modifiables et réversibles.
- **Interlocuteurs** : typage interne, client, fournisseur ; automatique par le domaine de l'entreprise et les listes de la capsule ; application à tout un domaine en un geste ; vigilance accrue par interlocuteur ; fiche par interlocuteur (où en est-on avec lui).
- **Indicateurs de réussite** : précision estimée, faux positifs, actions validées, corrections par semaine, exclusion pré-filtre, incidents critiques.

## 8. Sécurité et confidentialité

- **Comptes nominatifs**, sessions par cookie HttpOnly, changement de mot de passe dans l'application (ferme les autres sessions), limitation des tentatives de connexion.
- **Cloisonnement par boîte** en base (politiques de sécurité au niveau des lignes), rôle applicatif distinct du rôle d'administration.
- **Jetons OAuth et mots de passe IMAP chiffrés au repos**.
- **Journal d'audit** : chaque lecture, détection, brouillon, envoi, correction et connexion est horodaté et attribué.
- **Hébergement en Union européenne**, service d'inférence jamais exposé, fournisseur d'IA européen qui n'entraîne pas ses modèles sur les données.
- **En-têtes de sécurité** sur le site et l'application (politique de contenu, HSTS, pas d'inclusion dans un cadre).

## 9. Exploitation

- **Déploiement continu** : chaque merge sur la branche principale reconstruit, redémarre et vérifie la santé du service, avec retour arrière automatique en cas d'échec.
- **Sauvegardes** de la base chaque nuit, vérification hebdomadaire.
- **Évaluation** de l'extraction sur un corpus étiqueté (commande `evaluer`), recalibrage de la fiabilité (`recalibrer`), purge des catégories confidentielles sur l'existant (`confidentialite`).
- **Mode démonstration** sur un scénario fourni (Menuiserie Dupont), sans boîte réelle.
- **Site vitrine** avec six simulateurs de retour sur investissement, hypothèses modifiables, aucun chiffre inventé.

## Architecture

Pas de microservices : un monolithe modulaire en Go (ingestion, extraction, détecteurs, livrables, API) qui sert aussi l'interface React et le site, un service d'inférence Python (FastAPI) séparé et jamais exposé, une base PostgreSQL. Nginx devant, un déploiement par serveur.

## Hors périmètre à ce jour

Connecteurs vers les outils de gestion (facturation, paiement, transporteurs), résumé du matin, vue par dossier, travail en équipe, WhatsApp et SMS, application Google en production (validation en cours).

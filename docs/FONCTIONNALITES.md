# Ibalya : catalogue des fonctionnalités

État au 15 novembre 2026. Ce fichier est la référence : la page Admin de l'application l'affiche tel quel. Colonne « Où » : l'écran ou la commande qui porte la fonctionnalité.

## Raccordement et lecture

| Fonctionnalité | Ce que ça fait | Où |
|---|---|---|
| Gmail | Raccordement par autorisation Google, sans mot de passe | Réglages, Connexion |
| Outlook | Raccordement par autorisation Microsoft | Réglages, Connexion |
| IMAP | Toute boîte avec identifiants, chiffrés au repos, connexion testable avant enregistrement | Réglages, Connexion |
| Lecture seule | L'agent ne modifie, ne déplace et ne supprime jamais un message | Tous les canaux |
| Lecture initiale | 30 jours d'historique à l'onboarding, puis lecture toutes les 15 minutes | Onboarding, planificateur |
| Plusieurs boîtes | Une boîte par espace, cloisonnée en base, bascule de l'une à l'autre, vue combinée pour la synthèse et les brouillons | Réglages, Mes boîtes |
| Mes autres adresses | Alias et adresses perso du dirigeant, pour ne pas le prendre pour un tiers | Réglages, Votre identité |

## Pré-filtre avant toute analyse

| Fonctionnalité | Ce que ça fait | Où |
|---|---|---|
| Classe d'expéditeur | Humain, automatique, publicitaire ou sécurité, décidée avant l'extraction ; seuls les humains partent au modèle | Ingestion |
| Signaux d'automate | En-têtes d'envoi automatique, adresses de robots, domaines de transporteurs, places de marché, hébergeurs et outils de gestion, phrases « message automatique », en-tête de désinscription | Ingestion |
| Hameçonnage | Gabarits d'urgence artificielle écartés et jamais proposés à la relance | Ingestion, Miroir |
| Catégories confidentielles | Santé, ressources humaines, juridique : contenu non conservé, seuls l'expéditeur et la date restent | Réglages, Confidentialité |
| Règles apprises | Ignorer un interlocuteur ou un domaine, exclure un fil | Règles métier |
| Rattrapage | Reclasser une boîte déjà lue avec les règles actuelles, en simulation puis en application | Commande `-reappliquer-prefiltre` |

## Extraction des engagements

| Fonctionnalité | Ce que ça fait | Où |
|---|---|---|
| Extraction | Qui promet quoi, à qui, pour quand ; type (devis, livraison, relance, prise de contact, rendez-vous, facturation, autre) ; échéance explicite ou inférée | Engagements |
| Pièces jointes | Texte extrait, OCR au besoin, jamais le fichier conservé | Ingestion |
| Un engagement par promesse | Une reformulation du même engagement dans le fil devient une mention de l'existant, qui hérite d'une échéance explicite | Ingestion |
| Sens | « Je dois », « on me doit », « pour information » ; le « pour information » ne produit ni retard ni alerte | Engagements, Synthèse |
| Fiabilité | Score calculé sur des signaux vérifiables, l'avis du modèle pesant un dixième ; trois niveaux : élevée, à vérifier, incertaine ; rien de proactif sous le seuil de publication (0,6 par défaut) | Engagements, Réglages |
| Signaux de suivi | Avancement, livré, relance, confirmation, abandon, déduits des messages suivants | Ingestion |
| Échéance inférée | Toujours confirmée par le dirigeant avant une alerte | Engagements |
| Libellés | Dates au format français dans les objets, libellés relatifs justes (« demain », « en retard de 6 jours ») | Partout |

## Détecteurs (toutes les 30 minutes)

| Détecteur | Déclenchement | Où |
|---|---|---|
| Échéance à risque | Échéance dans les 7 jours sans signal de progression | Alertes |
| Silence anormal | Fil sans réponse au-delà de deux fois son rythme habituel (96 h par défaut), dans les deux sens | Alertes |
| Promesse menacée | Un retard amont confirmé met en danger une promesse aval dans les 7 jours, sur lien de dépendance confirmé seulement | Alertes |
| Engagement orphelin | Rien ne s'est passé 5 jours après la promesse | Alertes |
| Surcharge | 5 échéances ou plus la même semaine | Alertes |
| C'est fait ? | Retard de 7 jours sans activité, ou fil qui annonce la fin (livré, payé, facture jointe) ; réponse en un clic | Alertes, Synthèse |
| Une alerte par engagement | Rafraîchie à chaque cycle, jamais ressuscitée si elle a été écartée | Alertes |

## Dépendances

| Fonctionnalité | Ce que ça fait | Où |
|---|---|---|
| Candidats | Même fil ou mêmes interlocuteurs, échéances rapprochées, types qui peuvent se bloquer, jugés par le modèle avec les décisions passées en exemples | Cycle |
| Rétention | Seuil 0,7, cinq propositions par cycle au plus, confiance affichée | Dépendances |
| Décision | Confirmer, rejeter, ou rejeter tous les candidats ; seuls les liens confirmés alimentent « promesse menacée » | Dépendances |

## Livrables

| Fonctionnalité | Ce que ça fait | Où |
|---|---|---|
| Synthèse | Compteurs, « vos 3 décisions du jour » classées par enjeu, le reste replié, alertes en cours, vue par catégorie | Synthèse |
| Miroir d'activité | Ce que l'agent a compris des 30 derniers jours, messages écartés par classe, corrections d'un geste | Miroir d'activité |
| Capsule | Activité, rythme, clients récurrents, fournisseurs critiques, interlocuteurs clés, déduits puis corrigés ; intentions du dirigeant | Règles métier |
| Digest | Quotidien ou hebdomadaire, dans l'application et par email si demandé, avec les brouillons d'action | Réglages, Comportement |
| Brouillons de relance | Rédigés à partir du fil d'origine, signés avec l'identité du dirigeant, relus par le modèle ; un seul par fil et par destinataire | À valider |
| Validation | Chaque brouillon se relit, se modifie, s'envoie ou se rejette ; aucun envoi automatique | À valider |
| Assistant | Questions en langage naturel sur l'activité, réponses sourcées sur les messages, historique effaçable | Assistant |
| Recherche | Clients, engagements, fils | Barre de recherche |

## Apprentissage et corrections

| Fonctionnalité | Ce que ça fait | Où |
|---|---|---|
| Corrections d'un geste | Pas un engagement, mal résumé, abandonné, priorité haute, ne plus rien extraire de cet interlocuteur ; chaque geste devient une règle lisible et révocable | Engagements, Règles métier |
| Verdict d'extraction | Juste, faux, imprécis, distinct du statut, pour mesurer la qualité | Engagements |
| Règles métier | Délai d'alerte, silence par défaut, priorités du moment, modifiables et réversibles | Règles métier |
| Interlocuteurs | Types interne, client, fournisseur ; typage automatique par domaine et listes de la capsule ; application à tout un domaine ; vigilance accrue ; fiche par interlocuteur | Interlocuteurs |
| Indicateurs de réussite | Précision estimée, faux positifs, actions validées, corrections par semaine, exclusion pré-filtre, incidents critiques | Réglages |

## Sécurité et confidentialité

| Fonctionnalité | Ce que ça fait | Où |
|---|---|---|
| Comptes | Nominatifs, session par cookie HttpOnly, changement de mot de passe qui ferme les autres sessions, tentatives de connexion limitées | Connexion, Réglages |
| Cloisonnement | Par boîte, en base (sécurité au niveau des lignes), rôle applicatif distinct du rôle d'administration | Base de données |
| Secrets | Jetons OAuth et mots de passe IMAP chiffrés au repos | Base de données |
| Journal d'audit | Chaque lecture, détection, brouillon, envoi, correction et connexion, horodaté et attribué | Réglages, Journal |
| Hébergement | Union européenne, service d'inférence jamais exposé, fournisseur d'IA européen qui n'entraîne pas ses modèles sur les données | Infrastructure |
| En-têtes de sécurité | Politique de contenu, HSTS, pas d'inclusion dans un cadre | Site et application |
| Page Admin | Catalogue des fonctionnalités et liste des comptes avec dernière connexion et dernière activité, réservée aux adresses d'administration | Admin |

## Exploitation

| Fonctionnalité | Ce que ça fait | Où |
|---|---|---|
| Déploiement continu | Chaque merge reconstruit, redémarre et vérifie la santé, avec retour arrière automatique en cas d'échec | GitHub Actions |
| Sauvegardes | Base sauvegardée chaque nuit, vérifiée chaque semaine | Serveur |
| Évaluation | Extraction mesurée sur un corpus étiqueté, recalibrage de la fiabilité | Commandes `evaluer`, `recalibrer` |
| Purge confidentielle | Application des catégories confidentielles à l'existant | Commande `confidentialite` |
| Démonstration | Scénario fourni (Menuiserie Dupont), sans boîte réelle | Mode fixture |
| Site vitrine | Six simulateurs de retour sur investissement, hypothèses modifiables, aucun chiffre inventé | ibalya.com |

## Architecture

Pas de microservices : un monolithe modulaire en Go (ingestion, extraction, détecteurs, livrables, API) qui sert aussi l'interface React et le site, un service d'inférence Python (FastAPI) séparé et jamais exposé, une base PostgreSQL. Nginx devant, un déploiement par serveur.

## Hors périmètre à ce jour

| Chantier | Pourquoi pas encore |
|---|---|
| Connecteurs vers les outils de gestion (facturation, paiement, transporteurs) | Clôture automatique de ce qui est livré ou payé, à concevoir |
| Résumé du matin en cinq lignes | Non commencé |
| Vue par dossier ou par interlocuteur | Non commencé |
| Travail en équipe (partager un engagement) | Non commencé |
| WhatsApp et SMS | Non commencé |
| Application Google en production | Dossier de validation côté Google à monter |

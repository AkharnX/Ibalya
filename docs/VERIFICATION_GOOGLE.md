# Vérification Google OAuth — dossier Ibalya

Ibalya demande deux *restricted scopes* Gmail. La vérification a donc deux
niveaux : la **vérification de marque** (formulaire + vidéo, gratuit, quelques
semaines) et, pour servir des utilisateurs externes, l'**évaluation de sécurité
CASA** (tiers agréé, annuel, payant, plusieurs semaines à mois).

## Déjà en place (vérifié)

- Domaine `ibalya.com` vérifié dans Search Console.
- Page d'accueil publique : https://ibalya.com/ (décrit le produit).
- Règles de confidentialité : https://ibalya.com/confidentialite.html
  — avec la clause **Limited Use** et la déclaration du partage à Mistral.
- App publiée en **production** dans Google Cloud (plus l'expiration à 7 jours).

## Scopes à justifier (copier-coller dans la console)

### `.../auth/gmail.readonly` — lecture seule

> Ibalya lit les messages de la boîte du dirigeant pour en extraire
> automatiquement les engagements (promesses faites et reçues), suivre leurs
> échéances et détecter les risques : échéance à risque, silence anormal,
> engagement oublié, contradiction entre un retard amont et une promesse aval.
> Un engagement peut apparaître dans n'importe quel fil, donc l'accès en lecture
> au corps des messages est nécessaire. Le scope `gmail.metadata` ne suffit pas :
> il ne donne pas le texte du message, d'où l'engagement est extrait.
> `gmail.readonly` est le scope minimal permettant de lire sans jamais modifier,
> classer ni supprimer.

### `.../auth/gmail.send` — envoi

> Pour chaque alerte, Ibalya pré-rédige un message de relance à partir du
> contexte réel de la conversation. Le dirigeant le relit, l'ajuste, puis
> l'envoie d'un clic. `gmail.send` est le scope minimal pour envoyer au nom de
> l'utilisateur : il ne permet ni de lire, ni de modifier, ni de supprimer.
> Aucun envoi n'a lieu sans action explicite du dirigeant.

## Conformité Limited Use (à cocher / rappeler)

Les données Gmail :
- servent uniquement aux fonctions ci-dessus ;
- ne sont transférées qu'à Mistral AI (UE) pour l'analyse strictement
  nécessaire, jamais à des fins publicitaires ;
- ne sont pas utilisées pour entraîner des modèles généralisés ;
- ne sont pas lues par un humain sauf accord explicite, sécurité, ou obligation
  légale.

C'est exactement le texte de la page de confidentialité — cohérence assurée.

## Vidéo de démonstration (exigée pour les restricted scopes)

Non listée, ~2-3 min, en montrant l'URL à l'écran. Plan :

1. **L'écran de consentement OAuth**, avec l'URL visible : on doit y voir le
   `client_id` de l'app et les deux scopes demandés (readonly + send).
2. **Le raccordement** : l'utilisateur accepte, retour sur l'app.
3. **L'usage des données** : montrer que l'agent lit les mails et en extrait des
   engagements (page Suivi), et qu'une relance est pré-rédigée mais **envoyée
   seulement au clic** (page À valider). C'est ce que Google veut voir : à quoi
   servent concrètement les deux scopes.
4. **Dire à voix haute ou en légende** : « L'utilisation des données Google par
   Ibalya respecte la Google API Services User Data Policy, y compris les
   exigences Limited Use. »

Tourne la vidéo avec le compte de démo, sur une boîte qui a de vrais échanges
(celle de Yacouba, 18 engagements, est idéale) — pas une adresse de
notifications.

## CASA (le vrai pôle long)

Pour des utilisateurs **externes** avec des restricted scopes, Google impose une
évaluation CASA (Tier 2) par un laboratoire agréé, à renouveler chaque année.
C'est ce qui prend le plus de temps et peut coûter. À lancer en parallèle de la
vérification de marque, pas après.

Tant que ce n'est pas fait : l'app fonctionne, mais les utilisateurs voient
l'écran « application non vérifiée » (ils passent par « Paramètres avancés →
continuer »), et le nombre d'utilisateurs est plafonné.

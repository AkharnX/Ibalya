# Cloisonnement des données et délégation

Conception arrêtée le 23 août 2026, **non implémentée**. Hors périmètre MVP :
le CDC est écrit au singulier, « l'agent restitue au dirigeant ». Ce document
existe pour que le raisonnement ne soit pas refait, et pour qu'on parte de là
le jour où un deuxième utilisateur devient nécessaire.

## Ce qui a déclenché la réflexion

Yacouba s'est connecté et a vu le récapitulatif d'Ibrahim : 603 messages de sa
boîte Gmail personnelle, dont 15 sur ses candidatures et 28 bancaires. Aucune
des onze tables métier n'a de propriétaire ; l'authentification dit qui entre,
jamais quelles données il peut voir.

Mesure prise dans l'immédiat, à la place du cloisonnement : la session n'est
ouverte qu'au titulaire du canal connecté (`autoriserSession`, internal/api).
La règle se lit dans les données elles-mêmes, `oauth_tokens.account_email`, et
ne dépend pas de l'état actif ou non des autres comptes.

## Principe : le canal a un propriétaire

Plutôt qu'un réglage « commun ou personnel » à l'échelle du client — deux jeux
de règles de visibilité à écrire, et un sens de panne qui expose des données
privées — la nature partagée ou non se déduit de **qui possède le canal**.

| Ce qui est connecté | Propriétaire | Qui voit les engagements |
|---|---|---|
| le Gmail d'une personne | cette personne | elle seule |
| une boîte d'accueil, `contact@` | l'organisation | toute l'équipe |

Le client exprime son choix par ce qu'il raccorde et au nom de qui, pas par un
interrupteur qu'on peut mettre à l'envers. Un seul chemin de code.

Règle de visibilité, en une phrase : **on voit les engagements des canaux qu'on
possède, plus ceux de l'organisation ; un dirigeant voit en plus ceux de ses
membres.**

## Émetteur et responsable

Le CDC 8.2 fait regrouper le détecteur de surcharge « par responsable », mais
l'entité Engagement (CDC 5.1) n'a pas cet attribut. L'implémentation regroupe
donc par émetteur en l'appelant responsable. C'est exact tant que celui qui
promet est celui qui fait, donc tant qu'il n'y a qu'une personne.

Deux attributs distincts sont nécessaires :

- **émetteur** — qui a promis. Extrait de l'email, jamais modifié : c'est un fait.
- **responsable** — qui doit le faire. Modifiable : c'est la délégation.

Le détecteur de surcharge doit alors regrouper sur le responsable, sans quoi il
mesure la charge de la mauvaise personne.

## Délégation

Trois gestes : partager un engagement à l'organisation, se l'attribuer, demander
à quelqu'un de s'en charger. Ils se traduisent en événements du journal
(CDC 5.2) déjà en place : `partage`, `attribue`, `prise_en_charge_demandee`,
`prise_en_charge_acceptee`, `prise_en_charge_refusee`. L'historique « qui a
confié quoi à qui, et quand » et sa trace d'audit viennent sans travail
supplémentaire, et s'affichent dans le panneau de traçabilité existant.

Deux contraintes à respecter :

**Partager un engagement n'est pas partager la conversation.** L'engagement est
un résumé, mais il pointe vers son message d'origine et le panneau affiche le
fil entier. Par défaut on partage l'engagement seul ; joindre le fil doit être
un geste explicite du propriétaire.

**Une réponse part de la boîte qui possède le fil.** Un responsable désigné sur
un engagement issu du Gmail d'un autre peut suivre et relancer, mais l'envoi
passe par la boîte d'origine, sans quoi la conversation se scinde. Cela compose
bien avec la marche 3 : le responsable prépare, le propriétaire du canal valide
et le message part de chez lui. Sur un canal d'organisation, le responsable
valide lui-même.

## Découpage des entités partagées

- **Capsule** — les faits inférés (secteur, clients récurrents, fournisseurs
  critiques) appartiennent à l'organisation ; les intentions du temps 2
  (priorités du moment, ce qui coûte le plus d'énergie) sont personnelles.
- **Règles apprises** — personnelles par défaut ; « ne plus m'alerter sur ce
  fil » ne vaut que pour son auteur. Le dirigeant peut en promouvoir une au
  niveau de l'organisation.
- **Réglages** — le seuil de publication est de l'organisation, la fréquence de
  digest est personnelle.

## Mise en œuvre

Le filtrage doit être appliqué par PostgreSQL, en sécurité au niveau des lignes,
et non par un `WHERE` ajouté à la main dans chaque requête. Avec une
cinquantaine de requêtes, un seul oubli rouvre la fuite, silencieusement. Une
requête qui omet le filtre doit ne rien renvoyer, pas tout renvoyer.

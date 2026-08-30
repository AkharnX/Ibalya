# La fiabilité d'un engagement

Chaque engagement porte un score entre 0 et 1, affiché dans l'interface sous le
nom « fiabilité » et stocké dans `engagements.confiance`.

## Il n'y a pas de formule

C'est le point important, et il vaut mieux le dire franchement : **le score
n'est pas calculé**. Le modèle le déclare lui-même, en même temps qu'il extrait
l'engagement.

L'instruction qui le lui demande tient en une ligne
(`llm-service/app/prompts.py`) :

> `confiance : entre 0 et 1 — ta certitude qu'il s'agit d'un vrai engagement`

Le back-end ne fait ensuite que deux choses :

| Étape | Où | Effet |
|---|---|---|
| Bornage | `llm-service/app/main.py` | ramène la valeur dans `[0, 1]` |
| Bornage | `engine/extraction.go` — `clamp01` | même garde côté Go, au cas où |

Aucun signal externe n'entre dans le score : ni l'ancienneté du fil, ni le
nombre d'échanges avec l'interlocuteur, ni la présence d'une échéance explicite,
ni l'historique des corrections du dirigeant.

## À quoi il sert

Un seul usage, mais il est structurant : le **seuil de publication**, réglable
dans Réglages, par défaut `0.6`. Sous ce seuil, un engagement n'est jamais
présenté de lui-même — c'est le premier rempart anti-churn du CDC (7.2).

Quatre endroits appliquent ce seuil :

- `engine/suivi.go` — le suivi n'affiche pas l'engagement
- `engine/deliver.go` (deux fois) — pas de détection ni de brouillon
- `api/server.go` — la synthèse l'ignore

L'engagement reste en base et consultable ; il ne remonte simplement pas tout
seul.

## Ce que valent ces scores aujourd'hui

Distribution observée sur les 28 engagements de la base de recette :

| Score | Nombre |
|---|---|
| 0,98 | 1 |
| 0,95 | 12 |
| 0,90 | 4 |
| 0,85 | 3 |
| 0,80 | 3 |
| 0,70 | 4 |
| 0,30 | 1 |

Toutes les valeurs sont des multiples de 0,05, et 0,95 revient douze fois. Ce
n'est pas la signature d'une estimation calibrée, c'est celle d'un modèle qui
choisit une valeur ronde plausible. Vingt engagements sur vingt-huit sont à 0,90
ou plus, ce qui rend le seuil de 0,6 presque inopérant : il n'écarte
pratiquement rien.

## Le repli sans modèle

Quand le service d'inférence tourne sans fournisseur (`provider.py`), une
extraction dégradée par mots-clés attribue `0,85` si une échéance a été trouvée,
`0,70` sinon. C'est un repli de développement, pas une mesure.

## Ce qu'il faudrait pour que le score veuille dire quelque chose

Une vraie formule combinerait la déclaration du modèle avec des signaux
vérifiables, indépendants de lui :

- une échéance explicite dans le texte, plutôt qu'inférée
- un verbe d'engagement à la première personne, plutôt qu'une tournure vague
- l'ancienneté de la relation avec l'interlocuteur
- le taux de corrections passées du dirigeant sur cet interlocuteur ou ce type

Le dernier signal est le plus intéressant : Ibalya enregistre déjà les
corrections (`POST /api/engagements/{id}/correct`) et les règles apprises. Un
score qui apprend des corrections serait calibré par l'usage réel plutôt que par
l'aplomb du modèle.

Tant que ce travail n'est pas fait, la fiabilité affichée doit être lue pour ce
qu'elle est : l'avis du modèle sur son propre travail.

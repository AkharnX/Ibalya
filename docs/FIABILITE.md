# La fiabilité d'un engagement

Chaque engagement porte un score entre 0 et 1, affiché dans l'interface sous
forme de niveau et stocké dans `engagements.confiance`.

## Ce qui n'allait pas

Le score était celui que le modèle déclarait sur lui-même, en une ligne
d'instruction : « confiance : entre 0 et 1 — ta certitude qu'il s'agit d'un
vrai engagement ». Le back-end ne faisait que le borner.

Deux problèmes, tous deux mesurés sur la base de recette.

**La distribution ne séparait rien.** Toutes les valeurs étaient des multiples
de 0,05, 0,95 revenait douze fois, et vingt engagements sur vingt-huit
dépassaient 0,90. Le seuil de publication de 0,6 n'écartait donc presque rien.

**Le score était inversé.** Les engagements notés au-dessus de 0,90 étaient
corrigés ou abandonnés par le dirigeant dans 43 % des cas, contre 0 % pour les
autres. C'est le comportement attendu : demander à un modèle sa propre
certitude mesure son aisance à formuler, pas son exactitude.

## La formule

Le score part d'une base et ajoute des signaux vérifiables **sans le modèle**,
dont l'avis ne pèse plus qu'un dixième.

| Signal | Poids | Ce qu'il vaut |
|---|---|---|
| Base | 0,35 | point de départ sans aucune preuve |
| Échéance explicite | +0,20 | une date écrite, pas déduite |
| Formulation engageante | +0,15 | « je vous livre jeudi », pas « il faudrait avancer » |
| Fil conversationnel | +0,10 | au moins un aller-retour, pas une diffusion |
| Interlocuteur connu | +0,10 | au moins trois échanges antérieurs |
| Avis du modèle | +0,10 max | plafonné : il ne peut plus emporter la décision |
| Corrections passées | −0,20 max | part des engagements de cet interlocuteur déjà corrigés |

La pénalité ne s'applique qu'à partir de quatre engagements avec le même
interlocuteur : en dessous, l'historique ne veut rien dire.

Un engagement sans échéance, sans formulation engageante, venant d'un inconnu,
plafonne à 0,45 même si le modèle annonce 1,00. C'est le comportement qu'on
cherchait.

Le détail des signaux retenus est enregistré avec l'engagement dans
l'événement « cree » : un score qu'on ne peut pas expliquer ne vaut pas mieux
que celui qu'il remplace.

## L'affichage

Trois niveaux, plus de pourcentage :

| Niveau | Score |
|---|---|
| Élevée | ≥ 0,75 |
| À vérifier | ≥ 0,50 |
| Incertaine | < 0,50 |

« 92 % » annonce une mesure calibrée. Ce score n'en est pas une : il combine
cinq signaux pondérés à la main, et rien ne garantit encore que 0,92 se trompe
deux fois moins que 0,84. Le niveau dit ce qu'on sait sans promettre ce qu'on
ignore.

Le seuil de publication par défaut passe de 0,6 à 0,5, pour coïncider avec la
frontière entre « incertaine » et « à vérifier » : ce qu'on publie s'aligne
enfin sur ce qu'on affiche.

## Ce que ça a donné, sans embellir

Distribution avant et après recalcul des vingt-huit engagements existants :

| | Avant | Après |
|---|---|---|
| Élevée | 23 | 15 |
| À vérifier / Incertaine | 5 | 13 |

Le pouvoir prédictif, lui, s'est amélioré sans devenir bon :

| Part corrigée ou abandonnée | Ancien score | Nouveau score |
|---|---|---|
| Fiabilité haute | 43 % | 40 % |
| Fiabilité basse | 0 % | 31 % |

L'inversion est en grande partie levée — l'écart passe de 43 points à l'envers
à 9 — mais le score ne prédit toujours pas correctement : une fiabilité haute
devrait être corrigée **moins** souvent qu'une basse, pas légèrement plus.

Deux raisons, et aucune ne se règle par une formule plus astucieuse. Vingt-huit
engagements ne suffisent pas à conclure quoi que ce soit. Et « corrigé »
englobe des ajustements qui ne sont pas des erreurs, comme préciser une
échéance.

## Ce qu'il reste à faire

Les poids sont posés à la main. Pour qu'ils soient ajustés sur les données il
faut plusieurs centaines d'engagements et une distinction nette entre une
correction qui signale une erreur et une qui complète une information. Les deux
manquent aujourd'hui.

En attendant, la formule a au moins trois vertus que l'ancienne n'avait pas :
elle repose sur des signaux qu'on peut vérifier, elle s'explique, et l'avis du
modèle ne peut plus la dominer.

## Reprise de l'existant

```
go run ./cmd/recalibrer              # simule et montre l'effet sur chaque engagement
go run ./cmd/recalibrer --appliquer  # réécrit les scores
```

L'avis d'origine du modèle est relu dans l'événement « cree », qui le
conservait : la reprise ne perd rien.

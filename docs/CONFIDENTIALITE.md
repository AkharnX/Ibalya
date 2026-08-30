# Filtres de catégories sensibles

Réponse à l'exigence du CDC : « filtres de catégories (RH, juridique, santé),
exclusion configurable ».

## Ce que ça fait

Un message reconnu dans une catégorie active est écarté **avant l'extraction**.
Il n'est jamais transmis au modèle, et son sujet comme son corps ne sont pas
conservés : seuls l'expéditeur, les destinataires et la date restent en base,
pour garder trace de l'échange. La messagerie d'origine n'est pas touchée et
reste la source de vérité.

## Où ça agit

Dans le pré-filtre EF-11 (`backend/internal/ingest`), avant toute inférence.
La détection est purement lexicale : envoyer un arrêt maladie à un modèle tiers
pour savoir s'il faut le lui cacher n'aurait aucun sens.

La décision est prise **avant** l'écriture en base, dans `Ingester.Run` : le
contenu sensible n'est pas stocké puis effacé, il n'est pas stocké du tout.

## Les trois catégories

| Catégorie | Par défaut | Exemples reconnus |
|---|---|---|
| Santé | active | arrêt de travail, certificat médical, hospitalisation |
| Ressources humaines | active | candidature, CV, bulletin de paie, rupture conventionnelle |
| Juridique | inactive | mise en demeure, huissier, prud'hommes |

Le juridique est délibérément inactif au départ. Santé et RH portent des
données sur des personnes — un candidat, un salarié — que le dirigeant n'a pas
à suivre. Le juridique est le plus souvent une affaire entre entreprises,
chargée d'échéances réelles : c'est justement ce qu'Ibalya doit voir. Il reste
activable d'une case dans Réglages.

## Précision

Le risque n'est pas de rater un message sensible, c'est d'écarter du courrier
d'affaires ordinaire : le dirigeant croit alors l'agent en panne sans savoir
pourquoi. Les motifs sont donc des expressions de plusieurs mots, jamais des
mots isolés comme « contrat » ou « dossier ».

Mesuré sur 766 messages réels : 9 messages atteignaient le modèle et relevaient
d'une catégorie sensible, tous correctement identifiés, aucun faux positif.

Un sigle a dû être retiré en cours de route : « CPAM » désigne aussi un
employeur, et une offre d'emploi « Data Scientist – CPAM des Yvelines » passait
pour un dossier médical.

## Rattrapage de l'existant

Les filtres agissent à l'ingestion, donc sur ce qui arrive après leur
activation. Pour le courrier déjà stocké :

```
go run ./cmd/confidentialite              # compte et montre un échantillon
go run ./cmd/confidentialite --appliquer  # exclut et purge le contenu
```

La commande suit le réglage en vigueur, pas toutes les catégories. La purge est
irréversible côté Ibalya, d'où la simulation par défaut.

## Réglage

`GET`/`PUT /api/settings`, clé `categories_sensibles`, valeur JSON :

```json
{"sante": true, "rh": true, "juridique": false}
```

Une clé inconnue est ignorée, et un réglage illisible retombe sur les valeurs
par défaut : une erreur de configuration ne doit pas désactiver les filtres en
silence.

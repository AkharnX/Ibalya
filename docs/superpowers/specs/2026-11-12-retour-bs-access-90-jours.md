# Retour BS ACCESS, volet « sous 90 jours »

Suite de `2026-11-12-retour-bs-access-30-jours.md`. Points 5, 4 (reste), 9, 12, 8 et 7 du retour de Stewe (11 novembre 2026), dans l'ordre décidé avec Ibrahim le 12 novembre : sens des engagements, dédoublonnage des engagements, typage des interlocuteurs, dépendances resserrées, « 3 décisions du jour ». Le point 11 (application Google en production) est un dossier à monter côté console Google, pas du code.

## Décisions

### 1. Trois listes nettes : je dois, on me doit, pour information

- Un engagement a un émetteur (qui promet) et un destinataire. Le sens se calcule, il ne se stocke pas : `je_dois` si l'émetteur est une adresse du dirigeant (compte raccordé ou « mes autres adresses »), `on_me_doit` si c'est le destinataire, `pour_info` sinon (deux tiers qui se parlent dans un fil où le dirigeant est en copie).
- Les alertes de retard, d'échéance à risque, d'oubli et « C'est fait ? » ne portent que sur `je_dois` et `on_me_doit`. Le silence anormal reste par fil (il ne dépend pas d'un engagement). Les compteurs de retard de la synthèse et les priorités excluent `pour_info`.
- Suivi : trois onglets « Je dois », « On me doit », « Pour information », en plus des filtres existants. Chaque ligne porte son sens.

### 2. Un engagement par promesse dans un fil

- Aujourd'hui, un fil ne dédoublonne que sur l'objet exact. Le modèle reformule, donc « Livraison des 12 vitrages doubles » et « Livrer 12 vitrages doubles à l'atelier » deviennent deux engagements, puis trois.
- Décision : à la création, si un engagement ouvert du même fil a un objet proche (mêmes mots significatifs, après retrait des mots vides et des accents, recouvrement de 60 % ou plus, ou l'un contenu dans l'autre) et un type compatible (identique, ou l'un des deux « autre »), on ne crée rien : on ajoute un événement `mention` à l'engagement existant, avec le message source, et on garde l'échéance la plus précise (une échéance explicite remplace une échéance inférée).

### 3. Interlocuteurs typés sans saisie ligne à ligne

- Règles automatiques, dans l'ordre : adresse du dirigeant ou même domaine que le compte raccordé (hors messageries grand public : gmail, outlook, hotmail, live, yahoo, orange, free, sfr, laposte, wanadoo, icloud, proton, gmx) → `interne` ; adresse ou domaine présent dans les « clients récurrents » de la capsule → `client` ; dans les « fournisseurs critiques » → `fournisseur` ; sinon on ne touche pas (`autre`).
- Appliquées à la création d'un interlocuteur et, pour l'existant, par un bouton « Typer automatiquement » sur la page Interlocuteurs qui ne modifie que les « autre » et dit combien ont changé.
- Typage par domaine en un geste : changer le type d'un interlocuteur propose d'appliquer le même type à tout son domaine (hors messageries grand public). Filtre « Non typés ».

### 4. Dépendances : moins, mais crédibles

- Candidats : même fil, ou mêmes interlocuteurs, et des types qui peuvent se bloquer : l'amont est une livraison, un devis, une facture ou « autre » ; l'aval une livraison, une facture ou « autre », et un rendez-vous seulement derrière une livraison (la pose après la réception). Plus de lien entre un rendez-vous et un devis sans rapport.
- Le modèle garde le dernier mot, mais le seuil de rétention passe de 0,5 à 0,7, et au plus 5 candidats nouveaux par cycle, les mieux notés d'abord.
- Écran Liens : le niveau de confiance s'affiche (élevée à partir de 0,85, sinon « à vérifier »), et un bouton « Rejeter tous les candidats ».

### 5. La synthèse s'ouvre sur trois décisions

- Chaque priorité reçoit un enjeu : en retard et je dois, 3 ; en retard et on me doit, 2 ; retard probable (bloqué par un amont), 2 ; interlocuteur typé client, +1 ; priorité haute, +1 ; échéance dans les deux jours, +1. Les trois premières s'affichent sous « Vos 3 décisions du jour », les autres repliées derrière « Voir les N autres ».
- Le seuil de publication par défaut est le même partout : 0,6 (le moteur disait 0,5, l'écran 0,6).

## Contraintes générales

- Rien ne part sans validation. Pas de tiret cadratin, pas de capitales espacées, pas de chiffres inventés.
- Les règles se testent sans base (fonctions pures) ; l'intégration se vérifie sur la pile jetable avec les fixtures.
- Pas de migration de schéma dans ce volet.

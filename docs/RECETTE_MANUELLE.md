# Recette manuelle

Les cas à vérifier à la main dans l'application, que les tests automatiques ne
couvrent pas : ce qui dépend de l'affichage, du parcours réel et du jugement.

Les tests automatiques (Go, pytest, corpus d'évaluation) tournent en
intégration continue à chaque poussée. Cette liste est pour toi, dans
l'interface, après un déploiement ou avant de montrer le produit.

Convention : `[ ]` à cocher, **Attendu** décrit le comportement correct, et la
ligne _Pourquoi_ rappelle le bug ou la règle que le cas protège.

---

## 1. Correction d'un engagement

Ces gestes sont dans le menu « ⋯ » d'un engagement, dans **Opérations → Suivi**.

- [ ] **« Ne plus rien extraire de cet interlocuteur »** sur un engagement que
      *tu* as pris (ex. « confirmer ma présence au rendez-vous »).
      **Attendu** : la règle vise l'autre partie (le correspondant), jamais ta
      propre adresse.
      _Pourquoi : la règle visait ibkebe et bloquait 27 engagements sur 28._

- [ ] **« Ne plus rien extraire de cet interlocuteur »** quand l'engagement
      n'implique que toi.
      **Attendu** : refus explicite, « aucun interlocuteur distinct de vous ».

- [ ] **« Ce n'est pas un engagement »**.
      **Attendu** : l'engagement passe en « Écarté », et sa fiabilité future
      pour cet interlocuteur baisse.

- [ ] **« Engagement réel, mais mal résumé »** et **« Abandonné — mais l'agent
      avait raison »**.
      **Attendu** : les deux sont distincts de « ce n'est pas un engagement »
      et ne comptent pas comme des erreurs d'extraction.
      _Pourquoi : un abandon commercial reste une extraction réussie._

- [ ] **« Priorité haute pour cet interlocuteur »**.
      **Attendu** : vise le correspondant, pas toi, comme pour l'ignorer.

## 2. Fiabilité

- [ ] Les engagements affichent un **niveau** (Élevée / À vérifier /
      Incertaine), **pas un pourcentage**.
      _Pourquoi : « 92 % » annonce une précision que le score n'a pas._

- [ ] Un engagement sans échéance, venant d'un inconnu, avec une tournure
      vague, sort en **Incertaine** même si l'objet paraît clair.

- [ ] Le filtre par niveau de fiabilité (Suivi) montre le bon compte pour
      chaque niveau.

## 3. Confidentialité (Réglages → Confidentialité)

- [ ] **Santé** et **Ressources humaines** sont cochées par défaut, **Juridique**
      non.

- [ ] Décocher RH, enregistrer, recharger : le réglage tient.

- [ ] Après un cycle, une candidature ou un CV reçu **n'apparaît pas** dans les
      engagements, et son contenu n'est pas lisible dans les fils.
      _Pourquoi : le contenu sensible ne doit jamais atteindre le modèle ni
      être conservé._

- [ ] Un mail d'affaires ordinaire (devis, livraison, facture client) **passe**
      normalement.
      _Pourquoi : un faux positif fait croire l'agent en panne._

## 4. Digest

- [ ] Une alerte de silence dont **tu** es le dernier à devoir répondre dit
      « En attente de votre réponse », pas « sans réponse de X ».
      _Pourquoi : le libellé inversait les rôles une fois sur deux._

- [ ] Le digest reçu par mail vient bien de l'adresse d'expédition réglée, et
      n'est pas lui-même ré-ingéré comme un engagement au cycle suivant.

- [ ] Les engagements sous le seuil de publication n'apparaissent pas seuls
      dans le digest.

## 5. Signature et identité (Réglages → Votre identité)

- [ ] Modifier la signature libre, enregistrer : un brouillon généré ensuite
      utilise **cette** signature, pas un nom inventé.
      _Pourquoi : le modèle signait cinq noms différents, dont un mauvais
      prénom._

- [ ] Le nom, prénom, fonction et société saisis se retrouvent dans la
      signature composée quand la signature libre est vide.

## 6. Connexion et raccordement (Réglages → Connexion)

- [ ] Tester une configuration IMAP invalide : message d'erreur clair, la
      configuration n'est **pas** enregistrée tant qu'elle n'est pas prouvée.

- [ ] Tester une configuration IMAP pointant vers une adresse interne
      (127.0.0.1, une IP privée) : refus explicite.
      _Pourquoi : protection SSRF._

- [ ] Le mot de passe IMAP enregistré ne réapparaît jamais en clair à la
      relecture (champ pré-rempli de points).

## 7. Authentification et accès

- [ ] Se tromper de mot de passe plusieurs fois d'affilée : après quelques
      essais, le login est temporairement bloqué.
      _Pourquoi : limiteur anti-force-brute._

- [ ] Se déconnecter, puis tenter d'ouvrir une page interne directement par son
      URL : renvoi vers la connexion.

## 8. Cycle et état

- [ ] Lancer un cycle : pendant qu'il tourne, l'interface indique clairement
      « en cours ».

- [ ] Après le cycle, le nombre d'engagements a un sens (ni zéro inexpliqué, ni
      explosion de doublons).
      _Pourquoi : une règle « ignorer » mal ciblée peut vider l'extraction en
      silence — surveiller une chute brutale._

## 9. Sauvegarde

- [ ] `scripts/sauvegarde.sh --verifier` : la restauration dans une base jetable
      compare les tables sans écart.
      _Ce cas est en ligne de commande, pas dans l'interface._

---

## Après un changement de l'extraction ou de la fiabilité

Avant de considérer une modification comme bonne, lancer le banc de mesure :

```
cd backend && go run ./cmd/evaluer              # pré-filtre, sans coût
cd backend && go run ./cmd/evaluer --avec-modele # pipeline complet, consomme des jetons
```

**Attendu** : précision et rappel ne baissent pas par rapport à la mesure
précédente. Une régression fait sortir la commande en erreur.

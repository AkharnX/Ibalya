# Démo équipe — Ibalya MVP

## Avant la réunion (5 min)

```bash
cd ~/agnet-ia
./scripts/demo.sh          # remet le scénario à zéro, dates recalculées au jour J
```

Ouvrir `http://<serveur>:9999` (jeton dans `.env`). Les deux services et la base
tournent déjà ; si besoin : `make db`, `LLM_PROVIDER=mock make run-llm`, `make demo`.

## Le scénario « Menuiserie Dupont » (celui que vivra chaque client)

Marc Dupont, menuisier. 14 emails sur 30 jours, zéro saisie, zéro configuration :

1. Il a **promis à la mairie** de poser 12 fenêtres dans 4 jours (échéance à risque)
2. Son **fournisseur de vitrage** avait promis de livrer il y a 3 jours — rien, et
   silence depuis 6 jours (retard + silence anormal)
3. La pose **dépend** du vitrage → l'agent croise les deux : **contradiction critique**,
   avec brouillon de relance du fournisseur prêt à partir (recommandation croisée)
4. Il a promis un **devis pergola** il y a 12 jours, aucune suite → **orphelin**
   (le « trou noir » des PME)
5. Il a calé **5 interventions la même semaine** → **surcharge** détectée en amont

## Déroulé de démo (10 min)

| Étape | Onglet | Message |
|---|---|---|
| 1. Miroir | Miroir | « Voici ce que l'agent comprend à J+1, sans aucune saisie » |
| 2. Alertes | Aperçu | La contradiction critique en bandeau rouge — personne d'autre ne détecte ça |
| 3. Détections | Détections | Les 5 détecteurs, chacun avec score et explication |
| 4. Liens | Liens | L'agent propose, le dirigeant confirme — jamais de lien décidé par le LLM seul |
| 5. Brouillon | Brouillons | Valider d'un clic la relance fournisseur → envoyée + tracée |
| 6. Correction | Engagements | « Pas un engagement » d'un geste → règle apprise lisible |
| 7. Règles | Règles | La mémoire de l'agent : explicite, auditable, réversible |
| 8. KPI | KPI | Les critères de réussite du CDC, mesurés en continu |
| 9. Audit | Audit | Chaque lecture, détection, action : horodatée (RGPD + confiance) |

## Messages clés pour Yacouba (produit) et Aliou (marché)

- **Zéro discipline de saisie** : OAuth et c'est tout — la règle d'or d'adoption du CDC
- **Anti-churn intégré** : seuil de confiance, rien sous le seuil n'est poussé
- **Human-in-the-loop** : aucun message ne part sans clic (marche 3), argument de vente
- **Aucune règle sectorielle codée** : le même moteur pour un menuisier ou un cabinet
  de conseil — le scénario démo est une menuiserie, mais c'est un paramètre
- **Économie unitaire** : pré-filtre avant LLM (7 % d'exclusion sur la démo, montera
  avec le volume réel de newsletters/notifications)

## État technique honnête (si on nous pose la question)

- Extraction en mode **mock** (règles) pour la démo — la clé Mistral la remplace sans
  toucher au code (`LLM_PROVIDER=mistral` + `MISTRAL_API_KEY`)
- Connecteur Gmail **codé et prêt** : il manque les identifiants OAuth Google
  (console cloud, ~15 min) pour la première connexion réelle
- Reste avant pilote client : clé Mistral, OAuth Google, test de charge inférence (R3/R7)

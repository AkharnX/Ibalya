# AgentOps — Module Agent Ops & Delivery (MVP)

Implémentation du cahier des charges **AgentOS PME — Agent Ops & Delivery V2** (juin 2026).

Un agent qui lit les canaux de communication d'une PME (Gmail, lecture seule),
en extrait les **engagements** (qui a promis quoi, à qui, pour quand), les
surveille dans le temps (5 détecteurs) et restitue au dirigeant un miroir
d'activité, des digests et des brouillons d'action validables d'un clic.

## Architecture (CDC section 4)

| Couche | Composant | Rôle |
|---|---|---|
| 1. Ingestion | `backend/internal/channel` + `ingest` | Connecteur Gmail derrière l'interface de canal (EF-10), normalisation, pré-filtre avant LLM (EF-11) |
| 2. Extraction | `llm-service/` (Python, FastAPI) | Extraction d'engagements + score de confiance, via Mistral (interchangeable) |
| 3. Graphe + horloge | `backend/internal/engine` | Liens de dépendance par heuristique + confirmation, 5 détecteurs |
| 4. Livrables | `backend/internal/engine` + `api` | Miroir J+1, capsule hybride, digest, alertes, brouillons (marche 3) |

- **Backend Go** : ingestion, persistance (PostgreSQL), orchestration, détecteurs, API + tableau de bord sur le port **9999**.
- **Service LLM Python** (127.0.0.1:8092, jamais exposé) : extraction, capsule, brouillons. Fournisseur derrière une abstraction (`LLM_PROVIDER=mistral|mock`).
- MVP **mono-client** : un déploiement = une PME.

## Démarrage

```bash
cp .env.example .env        # renseigner ADMIN_TOKEN, MISTRAL_API_KEY, GOOGLE_CLIENT_ID/SECRET
make db                     # PostgreSQL (Docker, 127.0.0.1:5435)
make run-llm                # service LLM (127.0.0.1:8092)
make run-backend            # API + tableau de bord (:9999)
```

Tableau de bord : `http://<serveur>:9999` — se connecter avec `ADMIN_TOKEN`.

### Connexion Gmail (OAuth)

1. Google Cloud Console → Credentials → OAuth Client ID (type Web).
2. URI de redirection : `{PUBLIC_BASE_URL}/api/oauth/google/callback`.
3. Renseigner `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` dans `.env`.
4. Tableau de bord → Réglages → « Connecter Gmail ».

À la connexion, l'onboarding démarre automatiquement : lecture des 30 derniers
jours → **Miroir d'activité** → capsule temps 1 (le miroir est généré AVANT
les questions de setup — CDC 9.1).

### Mode démonstration (sans Gmail ni clé LLM)

```bash
LLM_PROVIDER=mock make run-llm     # extraction factice par mots-clés
make demo                          # connecteur fixture (fixtures/messages.json)
```

## Garde-fous (CDC sections 11 et 13)

- Aucun message ne part sans **validation explicite** (marche 3).
- Seules les détections/extractions **au-dessus du seuil** (`seuil_publication`,
  défaut 0,6) sont présentées proactivement — règle anti-churn.
- Échéances **inférées** signalées et inertes tant que non confirmées.
- Chaque correction du dirigeant devient une **règle lisible** (pas de boîte noire).
- **Audit trail** immuable de chaque lecture, extraction, détection et action.
- LLM interchangeable (`llm-service/app/provider.py`).

## Séquencement MVP (CDC 15.2)

- [x] Lot 1 — Socle : connecteur derrière interface, normalisation, pré-filtre, persistance
- [x] Lot 2 — Extraction + scoring + journal d'événements
- [x] Lot 3 — Graphe heuristique + 5 détecteurs
- [x] Lot 4 — Miroir + capsule hybride (2 temps)
- [x] Lot 5 — Digest + alertes + brouillons + marche 3 + audit trail + règles

Reste avant production : identifiants OAuth Google, clé Mistral, hébergement UE
définitif, test de charge du coût d'inférence (R3/R7).

# Ibalya — Module Agent Ops & Delivery (MVP)

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

- **Backend Go** : ingestion, persistance (PostgreSQL), orchestration, détecteurs, API sur le port **9999**.
- **Frontend React** (`frontend/`, Vite + react-router) : SPA 6 pages (Pilotage, Engagements, Alertes, Miroir, Agent, Réglages), buildée dans `frontend/dist` et servie par le backend. `make front` pour builder, `make front-dev` pour le hot-reload.
- **Service LLM Python** (127.0.0.1:8092, jamais exposé) : extraction, capsule, brouillons. Fournisseur derrière une abstraction (`LLM_PROVIDER=mistral|mock`).
- MVP **mono-client** : un déploiement = une PME.

## Démarrage

```bash
cp .env.example .env        # renseigner ADMIN_TOKEN, MISTRAL_API_KEY, GOOGLE_CLIENT_ID/SECRET
make db                     # PostgreSQL (Docker, 127.0.0.1:5435)
make restart-all            # build front + back, démarre les deux services
make status                 # vérifie que tout répond
```

Au quotidien : `make restart` après toute modification du backend (recompile et
relance — évite le décalage binaire/processus), `make front` après une
modification du frontend, `make demo` pour rejouer le scénario de démonstration,
`make logs` pour suivre le journal, `make help` pour la liste complète.

`make status` ne se contente pas du code HTTP : il vérifie que `/api/synthese`
renvoie bien du JSON et non la page du tableau de bord — signe que le binaire en
cours d'exécution connaît les routes actuelles.

Tableau de bord : `https://ibalya.com` — connexion par email et mot de passe.

### Comptes

```bash
make utilisateur EMAIL=prenom@exemple.fr NOM="Prénom Nom"
```

Le mot de passe est saisi sans écho : il n'apparaît ni à l'écran, ni dans
l'historique du shell. Minimum 10 caractères, haché en bcrypt.

Les sessions durent 30 jours (cookie `HttpOnly`, `Secure`, `SameSite=Lax`) ;
seul le haché du jeton de session est conservé en base.

`ADMIN_TOKEN` n'est plus un accès administrateur mais un **jeton de service**,
accepté uniquement depuis la boucle locale (`make status`, `scripts/demo.sh`) :
inutilisable depuis Internet, y compris avec un en-tête `X-Real-IP` forgé.

### Mise en ligne (une seule fois)

Faire pointer les enregistrements DNS `A` de `ibalya.com` et `www.ibalya.com`
vers l'IP du serveur, puis :

```bash
./scripts/setup_https.sh
```

Le script vérifie le DNS, installe le vhost nginx (`deploy/nginx-ibalya.com.conf`),
obtient le certificat Let's Encrypt, bascule `PUBLIC_BASE_URL` sur le domaine,
redémarre le backend, vérifie l'accès depuis l'extérieur — et seulement alors
ferme le port 9999 au public.

### Connexion Gmail (OAuth)

1. Google Cloud Console → Credentials → OAuth Client ID (type Web).
2. URI de redirection : `https://ibalya.com/api/oauth/google/callback`.
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

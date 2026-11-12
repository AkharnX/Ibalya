# Réduire le bruit (retour BS ACCESS, volet 30 jours) : plan d'implémentation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Une alerte et un brouillon par sujet, aucun engagement extrait d'un email automatique ou d'un hameçonnage, des libellés de dates justes, et un « C'est fait ? » en un clic pour les retards.

**Architecture:** Tout se joue dans le backend Go : clés de dédoublonnage des détecteurs, garde des brouillons par fil, une classification de l'expéditeur avant extraction (nouveau fichier `ingest/classe.go`), un sixième détecteur `a_clore`, et des fonctions pures de libellés. Les canaux capturent un en-tête de plus. Le frontend ajoute deux boutons et trois libellés.

**Tech Stack:** Go 1.22 (net/http mux, pgx), Postgres 16, React + Vite, tests `go test` sans base (fonctions pures), vérifications d'intégration sur la pile jetable (Postgres docker 5437, backend `CHANNEL=fixture`, scénario `fixtures/messages.json`).

**Spec:** `docs/superpowers/specs/2026-11-12-retour-bs-access-30-jours.md`

## Global Constraints

- Rien ne part sans validation : aucun changement sur l'envoi.
- Copie française sans tiret cadratin ni libellés en capitales espacées ; pas de chiffres inventés.
- Migrations : `ALTER TABLE … ADD COLUMN IF NOT EXISTS` dans `backend/internal/store/schema.go`, vérifiées avec le rôle `ibalya_app`.
- Messages exclus : jamais envoyés au modèle, jamais visés par un brouillon.
- Chaque tâche se termine par `cd backend && go vet ./... && go test ./...` vert et un commit.

## Review Focus

1. Une alerte « échéance à risque » écartée par le dirigeant ne doit pas réapparaître le lendemain (test dans la tâche 1 : la clé est identique d'un jour à l'autre).
2. Un expéditeur humain avec un prénom et un domaine d'opérateur (`marc.dupont@orange.com`) doit rester `humain` (test dans la tâche 3).
3. Un vrai message de fournisseur « votre commande a été expédiée » signé d'une personne ne doit pas être classé automatique sur le seul corps (test dans la tâche 3 : seules les phrases « message automatique / ne pas répondre » déclenchent).
4. Une échéance dépassée de 6 jours ne doit jamais s'afficher « dans 0 jour » (test dans la tâche 6, calcul en jours calendaires).
5. « C'est fait ? » ne doit pas se déclencher sur un engagement fraîchement en retard (moins de 7 jours) sans signal de fin (test dans la tâche 7).

---

### Task 1: Une alerte vivante par engagement

**Files:**
- Modify: `backend/internal/engine/detectors.go:149` et `:301`
- Create: `backend/internal/engine/detectors_test.go`

**Interfaces:**
- Produces: `func cleAlerte(typ string, ids ...int64) string` dans `detectors.go`, retourne `"<typ>:<id1>[:<id2>]"`, sans date.

- [ ] **Step 1: Écrire le test**

```go
func TestCleAlerteSansDate(t *testing.T) {
	if got := cleAlerte("echeance_a_risque", 42); got != "echeance_a_risque:42" { t.Fatal(got) }
	if got := cleAlerte("contradiction", 7, 9); got != "contradiction:7:9" { t.Fatal(got) }
}
```

- [ ] **Step 2: `go test ./internal/engine/ -run TestCleAlerteSansDate` échoue (fonction absente)**
- [ ] **Step 3: Implémenter `cleAlerte` et l'utiliser aux deux endroits ; `today` reste utilisé par `surcharge` (clé par semaine, inchangée)**
- [ ] **Step 4: `go vet ./... && go test ./...` vert**
- [ ] **Step 5: Commit** `alertes: une seule alerte vivante par engagement (clé sans date pour échéance à risque et promesse menacée)`

### Task 2: Un brouillon par fil et par destinataire

**Files:**
- Modify: `backend/internal/store/queries.go` (après `HasActiveDraftForDetection`)
- Modify: `backend/internal/engine/deliver.go:293` (dans `maybeDraft`, après la garde par détection et une fois `toEmail` connu)

**Interfaces:**
- Produces: `func (s *Store) HasProposedDraftForThread(ctx context.Context, threadID int64, toEmail string) (bool, error)` : vrai s'il existe un brouillon `statut='propose'` dont `lower(to_email)=lower($2)` et dont la détection (`detections.thread_id`) ou l'engagement (`engagements.thread_id`) est sur `$1`.

- [ ] **Step 1: Implémenter la requête (LEFT JOIN detections, LEFT JOIN engagements, `count(*) > 0`)**
- [ ] **Step 2: Dans `maybeDraft`, après le calcul de `toEmail` et avant `capsuleLLM`, si `d.ThreadID != nil` et la garde est vraie : `return nil`**
- [ ] **Step 3: Vérification d'intégration sur la pile jetable** : démarrer la pile, lancer `POST /api/onboarding/run` puis deux fois `POST /api/cycle/run`, puis `SELECT to_email, count(*) FROM drafts WHERE statut='propose' GROUP BY 1` : aucune ligne au-dessus de 1 par fil (vérifier avec la jointure).
- [ ] **Step 4: `go vet ./... && go test ./...` vert, commit** `brouillons: un seul brouillon proposé par fil et par destinataire`

### Task 3: Classer l'expéditeur avant l'extraction

**Files:**
- Create: `backend/internal/ingest/classe.go`
- Create: `backend/internal/ingest/classe_test.go`
- Modify: `backend/internal/ingest/prefilter.go` (remplacer `noReplyPattern` et `excludedDomains` par l'appel à la classe)
- Modify: `backend/internal/ingest/prefilter_test.go` (cas existants inchangés, nouveaux motifs)

**Interfaces:**
- Produces:
  - `type Classe string` avec `ClasseHumain = "humain"`, `ClasseAutomatique = "automatique"`, `ClassePublicitaire = "publicitaire"`, `ClasseSecurite = "securite"`.
  - `func ClasserExpediteur(m store.Message) Classe` : ordre d'évaluation `securite` (objet + corps), puis `automatique` (en-tête `m.AutoSubmitted`, adresse, domaine, phrases du corps), puis `publicitaire` (`m.ListUnsubscribe`), sinon `humain`.
  - `ExclusionReason` retourne `"securite_suspect"` pour `ClasseSecurite`, `"expediteur_automatique"` pour `ClasseAutomatique`, `"newsletter"` pour `ClassePublicitaire` ; le reste de la fonction (fil exclu, règles apprises, corps vide) inchangé.
- Consumes: `store.Message.AutoSubmitted` (tâche 4 ; en attendant, le champ est ajouté au modèle dans cette tâche avec la valeur `false` partout).

- [ ] **Step 1: Écrire `classe_test.go` en table** : `shipment-tracking@amazon.fr` → automatique ; `no-reply@notification.orange.fr` → automatique ; `marc.dupont@orange.com` corps humain → humain ; corps « Ceci est un message automatique, merci de ne pas y répondre » → automatique ; corps « Bonjour, votre commande a été expédiée ce matin. Cordialement, Julie » → humain ; objet « Connexion suspecte sur votre compte » → securite ; corps « votre compte sera suspendu sous 24h, vérifiez votre identité » → securite ; `AutoSubmitted: true` → automatique ; `ListUnsubscribe: true` seul → publicitaire ; `commandes@vitrages-pro.fr` corps humain → humain (une adresse `commandes@` d'un fournisseur réel reste humaine : le motif adresse ne porte que sur `commande(s)@` suivi d'un domaine de la liste, ou sur `order` / `commande` combiné à `noreply|auto|notification`).
- [ ] **Step 2: `go test ./internal/ingest/ -run TestClasserExpediteur` échoue**
- [ ] **Step 3: Implémenter `classe.go`** : regex adresse, liste de domaines (celle de la spec), regex phrases automatiques, regex hameçonnage (celle de la spec), toutes insensibles à la casse et aux accents via `remplaceAccents` de `categories.go`.
- [ ] **Step 4: Brancher `ExclusionReason`, lancer `go test ./internal/ingest/` : les tests existants passent encore**
- [ ] **Step 5: Commit** `pré-filtre: classe d'expéditeur (humain, automatique, publicitaire, sécurité) avant extraction ; hameçonnage écarté`

### Task 4: Capturer l'en-tête d'envoi automatique dans les trois canaux

**Files:**
- Create: `backend/internal/channel/automatique.go` + `automatique_test.go`
- Modify: `backend/internal/channel/channel.go` (`Message.AutoSubmitted bool`), `imap.go:391`, `gmail.go:291`, `outlook.go` (demander `internetMessageHeaders` dans le `$select` des messages)
- Modify: `backend/internal/store/models.go` (`Message.AutoSubmitted`), `schema.go` (`ALTER TABLE messages ADD COLUMN IF NOT EXISTS auto_submitted BOOLEAN NOT NULL DEFAULT false;`), `queries.go` (`InsertMessage` et les SELECT de messages)
- Modify: `backend/internal/ingest/ingester.go` (recopie du champ canal → store)

**Interfaces:**
- Produces: `func EstEnvoiAutomatique(entete func(nom string) string) bool` : vrai si `Auto-Submitted` existe et vaut autre chose que `no`, ou `Precedence` ∈ {bulk, list, junk}, ou `X-Auto-Response-Suppress` non vide, ou `Feedback-ID` non vide. Comparaisons insensibles à la casse.

- [ ] **Step 1: Test en table de `EstEnvoiAutomatique`** (sept cas : auto-generated, `no`, bulk, absent, suppress, feedback-id, casse mélangée)
- [ ] **Step 2: Test rouge, implémenter, test vert**
- [ ] **Step 3: Brancher les trois canaux et la colonne ; vérifier la migration avec le rôle `ibalya_app` sur la pile jetable**
- [ ] **Step 4: `go vet ./... && go test ./...` vert, commit** `canaux: en-tête d'envoi automatique capturé (Auto-Submitted, Precedence, Feedback-ID) et stocké`

### Task 5: Rattraper les boîtes déjà lues

**Files:**
- Create: `backend/internal/ingest/rattrapage.go`
- Modify: `backend/cmd/server/main.go` (flag `-reappliquer-prefiltre` et `-appliquer`)

**Interfaces:**
- Produces: `func Reappliquer(ctx context.Context, st *store.Store, appliquer bool) (Bilan, error)` avec `type Bilan struct { Examines, Exclus int; ParMotif map[string]int; EngagementsAnnules int; Echantillon []string }`. Parcourt les messages `status IN ('pending','analyzed')`, applique `ExclusionReason` ; si non vide et `appliquer` : `MarkMessage(id, "excluded", &motif)` ; pour chaque engagement créé depuis ce message (`engagement_events` type `cree` avec `message_id`) sans autre événement : `UpdateEngagement(statut='annule')` + `AddEvent(…, "exclu_automatique", …)`.
- Le flag sans `-appliquer` imprime le bilan et dix exemples (expéditeur, objet, motif) sans écrire.

- [ ] **Step 1: Implémenter, puis vérifier sur la pile jetable** : charger les fixtures, injecter via SQL un message `shipment-tracking@amazon.fr` déjà `analyzed` avec un engagement `cree`, lancer la commande sans puis avec `-appliquer`, vérifier `status='excluded'` et l'engagement `annule`.
- [ ] **Step 2: Commit** `pré-filtre: commande de rattrapage pour reclasser les messages déjà lus`

### Task 6: Libellés et dates

**Files:**
- Create: `backend/internal/engine/libelles.go` + `libelles_test.go`
- Modify: `backend/internal/engine/detectors.go:146,219-224`, `extraction.go` (`createEngagement`, objet normalisé), `llm-service/app/prompts.py` (une phrase : « n'inclus pas dans objet une date déjà portée par echeance »)
- Modify: `frontend/src/pages/Suivi.jsx:207`, `Miroir.jsx:42`, `App.jsx:220`, `Agent.jsx:143`

**Interfaces:**
- Produces:
  - `func Jours(n int) string` → `"1 jour"`, `"3 jours"`.
  - `func LibelleEcheance(echeance, now time.Time) string` sur jours calendaires (dates tronquées à minuit dans le fuseau de `now`) : `"aujourd'hui"`, `"demain"`, `"hier"`, `"dans 3 jours"`, `"en retard de 6 jours"`.
  - `func NormaliserDatesObjet(objet string, now time.Time) string` : `2026-10-01` → `01/10` si année courante, sinon `01/10/2027`.

- [ ] **Step 1: Tests** : `Jours(1)`, `Jours(2)` ; `LibelleEcheance` pour J, J+1, J-1, J+3, J-6 (y compris échéance à 23 h 59 et `now` à 00 h 05 : toujours « aujourd'hui ») ; `NormaliserDatesObjet("Livraison 2026-10-01 vitrages", nov. 2026)` → `"Livraison 01/10 vitrages"`, année 2027 → `01/10/2027`, objet sans date inchangé.
- [ ] **Step 2: Rouge, implémenter, vert**
- [ ] **Step 3: Remplacer les titres des détecteurs** : échéance à risque → `"Échéance " + LibelleEcheance(...) + " sans signal de progression"` ; silence → `"Sans retour depuis " + Jours(j)` et `"En attente de votre réponse depuis " + Jours(j)` ; `createEngagement` applique `NormaliserDatesObjet`.
- [ ] **Step 4: Frontend** : `'?'` → `'expéditeur inconnu'` / `'destinataire inconnu'` ; `jour(s)` → pluriel calculé. `npm run lint && npm run build` vert.
- [ ] **Step 5: Commit** `libellés: jours relatifs en calendaire, dates dans les objets au format français, plus de « jour(s) » ni de « ? »`

### Task 7: Détecteur « C'est fait ? »

**Files:**
- Modify: `backend/internal/engine/detectors.go` (map ligne 36, nouvelle fonction), `deliver.go` (rien : `maybeDraft` ignore déjà les types inconnus)
- Create: `backend/internal/engine/cloture.go` + `cloture_test.go`
- Modify: `backend/internal/api/server.go` (route `POST /api/detections/{id}/clore`), `frontend/src/components/ui.jsx` (`DET_LABELS.a_clore = 'C\'est fait ?'`), `frontend/src/pages/Alertes.jsx` (deux boutons sur ce type), `frontend/src/pages/Synthese.jsx:160` (bouton texte « C'est fait » à côté de l'icône)

**Interfaces:**
- Produces:
  - `func SignalFin(texte string) (extrait string, ok bool)` : vrai si le texte contient l'un des motifs de la spec (insensible casse/accents) ; `extrait` = la phrase (jusqu'à 120 caractères) qui contient le motif.
  - `func (e *Engine) detectAClore(ctx, p capsuleParams, today string) (int, error)` : pour chaque engagement `en_retard` avec échéance ; condition A : échéance ≤ now − 7 j et `LastEventOfTypes(tous types)` ≤ now − 7 j ; condition B : un message du fil (`messages.thread_id`, `sent_at > echeance`) dont `SignalFin(subject+body)` est vrai. Détection `Type: "a_clore"`, `Score: 0.6`, `Critique: false`, `Titre: "C'est fait ?"`, `Detail` : « « objet » : échéance dépassée depuis N jours, sans activité. » ou « « objet » : un message du JJ/MM semble dire que c'est fait : « extrait » ». Clé `cleAlerte("a_clore", eng.ID)`.
  - Route `POST /api/detections/{id}/clore` corps `{"fait": bool}` : si `fait`, `UpdateEngagement(statut='livre')` + `AddEvent("livre")` ; sinon `AddEvent("confirme")` ; dans les deux cas la détection passe `ecartee` ; audit `cloture_confirmee` / `cloture_refusee`.

- [ ] **Step 1: Tests `SignalFin`** : « Colis bien reçu ce matin, merci » → vrai ; « Virement effectué ce jour » → vrai ; « Pouvez-vous me confirmer la date de livraison ? » → faux ; « Facture ci-jointe » → vrai ; extrait tronqué à 120 caractères.
- [ ] **Step 2: Rouge, implémenter `SignalFin`, vert**
- [ ] **Step 3: Implémenter `detectAClore` et l'enregistrer en sixième position de la map ; test pur de la condition A** via une fonction `aClorePourDelai(echeance, dernierEvenement, now time.Time) bool` (J-6 sans événement → faux ; J-7 → vrai ; J-20 avec événement J-3 → faux).
- [ ] **Step 4: Route et frontend** ; `npm run lint && npm run build`.
- [ ] **Step 5: Vérification d'intégration** : sur la pile jetable, forcer via SQL un engagement en retard de 10 jours sans événement, lancer un cycle, vérifier une détection `a_clore`, appeler la route avec `fait: true`, vérifier `statut='livre'` et la détection `ecartee`.
- [ ] **Step 6: Commit** `détecteurs: « C'est fait ? » pour les retards anciens ou les fils qui annoncent la fin, clôture en un clic`

### Task 8: Compter les exclusions par classe dans le miroir

**Files:**
- Modify: `backend/internal/api/server.go:419-430` (`messages_exclus_par_motif` : map motif → compte, `GROUP BY exclude_reason`)
- Modify: `frontend/src/pages/Miroir.jsx` (sous les compteurs : « N messages écartés : X automatiques, Y publicitaires, Z suspects (hameçonnage possible, ne pas répondre) », affiché seulement si N > 0)

- [ ] **Step 1: Implémenter, `npm run lint && npm run build`, vérifier sur la pile jetable que la ligne apparaît**
- [ ] **Step 2: Commit** `miroir: messages écartés par classe, mention hameçonnage`

### Task 9: Livraison et mesure

- [ ] **Step 1: Revue finale du diff de la branche** (`git diff origin/main --stat`), `go vet`, `go test`, lint, build.
- [ ] **Step 2: PR vers `main`** avec le résumé et le lien vers la spec. Merge par Ibrahim, déploiement par le CD.
- [ ] **Step 3: Après déploiement, lancer `-reappliquer-prefiltre` sans `-appliquer` sur la prod pour voir le bilan, puis avec `-appliquer` sur accord d'Ibrahim.**
- [ ] **Step 4: Demander à Stewe de relever les mêmes indicateurs qu'au 11 novembre.**

# Recherche de pièces jointes : plan d'implémentation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Retrouver n'importe quel document reçu en pièce jointe (robots compris) depuis la barre de recherche, avec un lien vers le mail d'origine, pour les boîtes Gmail et IMAP.

**Architecture:** Le pipeline d'ingestion enregistre désormais les documents joints aussi pour les messages écartés (mode « texte seul », sans OCR), selon une table `statut/motif → mode` pure et testée ; le connecteur IMAP apprend à extraire les pièces du message brut qu'il télécharge déjà. Une colonne `tsvector` générée sur `attachments` porte la recherche plein texte française ; une route `GET /api/recherche/pieces-jointes` la sert, la barre de recherche gagne un mode « Pièces jointes ». Une commande `-indexer-pieces-jointes` rattrape une fois les 30 derniers jours des boîtes déjà raccordées.

**Tech Stack:** Go 1.2x (monolithe `backend/`), PostgreSQL 16 (`tsvector`, GIN, `websearch_to_tsquery`, `ts_headline`), `github.com/emersion/go-message` (IMAP MIME), React 18 + Vite (`frontend/`), fixtures JSON (`fixtures/messages.json` via `scripts/gen_fixtures.py`).

**Spec:** `docs/superpowers/specs/2026-11-19-recherche-pieces-jointes-design.md`

## Global Constraints

- Jamais les octets d'une pièce en base : nom, type, texte seulement (`attachments`).
- Rien (pas même l'ouverture de la pièce) pour un motif commençant par `categorie_sensible_` ou `securite_` ; un motif inconnu donne `texte_seul`, jamais `complete`.
- Aucun OCR (donc aucun appel au service Mistral) pour un message écarté, ni pendant le rattrapage, quel que soit le statut.
- Périmètre « document » : PDF, DOCX, XLSX, PPTX, TXT, CSV, toujours enregistrés même sans texte ; image enregistrée seulement si l'OCR en tire du texte ; tout autre type ignoré.
- Cloisonnement RLS inchangé : toute requête passe par `s.q(ctx)` / `Store.Q(ctx)` dans le tenant ; `attachments` reste dans la boucle RLS de `schema.go`.
- Recherche sans borne de date ; au moins trois caractères, sinon liste vide ; 12 résultats au plus, triés par date du message décroissante.
- Outlook hors périmètre : connecteur inchangé, texte public explicite.
- Copie publique et interface : pas de tiret cadratin, pas de libellé en majuscules espacées, pas de chiffre inventé ; libellés en français.
- Tests base de données : conditionnés par `TEST_ADMIN_URL` / `TEST_APP_URL` (gabarit `store/isolation_test.go`), jamais le `.env` de prod ; vérification finale sur une pile jetable (`env -i`, base jetable Docker).
- Pas de nouvelle dépendance Go ni npm.
- Commits en français, un par tâche, avec les lignes d'attribution de la session.

## Review Focus

Entrées que la spec implique sans qu'une tâche ne les teste d'elle-même ; chaque ligne a reçu son test dans la tâche indiquée.

1. **Pièce de 12 Mo + 1 octet en IMAP** : elle ne doit pas être retenue, et le message doit quand même être converti (Tâche 4, `TestPieceJointeIMAPTropGrosse`).
2. **Texte extrait de plusieurs Mo** : la colonne `tsvector` générée plafonne à 1 Mo ; l'INSERT ne doit jamais échouer (Tâche 3, `TestInsertAttachmentTexteEnorme`).
3. **Requête avec guillemets, opérateurs ou accents** (`"facture" -relevé`, `Qonto&`) : `websearch_to_tsquery` ne lève pas d'erreur et la recherche renvoie une liste, pas un 500 (Tâche 3, `TestRechercherPiecesRequeteBizarre`).
4. **Document nommé mais sans texte** (zip renommé `.pdf`, PDF scanné sans OCR) : enregistré avec `texte=''` et trouvable par son nom ; `TexteAttachments` l'ignore (Tâche 2, `TestPlanPieceDocumentSansTexte` ; Tâche 3, `TestRechercherPiecesParNomSeul`).
5. **Rattrapage relancé deux fois** : la seconde passe ne concerne aucun message et n'écrit rien (Tâche 5, `TestIndexerPiecesIdempotent`).

---

### Task 1 : périmètre « document » dans `piecejointe`

**Files:**
- Modify: `backend/internal/piecejointe/extract.go`
- Test: `backend/internal/piecejointe/extract_test.go`

**Interfaces:**
- Produces: `func Document(nom, mime string) bool` (PDF, DOCX, XLSX, PPTX, TXT, CSV) ; `func Image(nom, mime string) bool` ; `func Libelle(nom, mime string) string` → `"PDF" | "Word" | "Excel" | "PowerPoint" | "Texte" | "Image" | ""` ; `Extraire` gère en plus `pptx` (balises `<a:t>` des `ppt/slides/*.xml`, `Methode: "pptx"`) et `txt`/`csv` (`Methode: "texte"`). `Traitable` reste et devient `Document || Image`.

- [ ] **Step 1 : écrire les tests qui échouent**

```go
func TestDocumentImageEtLibelle(t *testing.T) {
	cas := []struct{ nom, mime string; doc, img bool; lib string }{
		{"facture.pdf", "application/pdf", true, false, "PDF"},
		{"devis.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", true, false, "Word"},
		{"releve.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true, false, "Excel"},
		{"pres.pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", true, false, "PowerPoint"},
		{"notes.txt", "text/plain", true, false, "Texte"},
		{"export.csv", "text/csv", true, false, "Texte"},
		{"scan.jpg", "image/jpeg", false, true, "Image"},
		{"plan.dwg", "application/octet-stream", false, false, ""},
		{"archive.zip", "application/zip", false, false, ""},
		{"sans-extension", "application/pdf", true, false, "PDF"},
	}
	for _, c := range cas {
		if Document(c.nom, c.mime) != c.doc || Image(c.nom, c.mime) != c.img || Libelle(c.nom, c.mime) != c.lib {
			t.Errorf("%s : doc=%v img=%v lib=%q", c.nom, Document(c.nom, c.mime), Image(c.nom, c.mime), Libelle(c.nom, c.mime))
		}
	}
}

func TestExtrairePptxEtTexte(t *testing.T) {
	// archive zip en mémoire avec ppt/slides/slide1.xml contenant <a:t>Budget 2026</a:t>
	r, err := Extraire("pres.pptx", "", zipAvec(t, "ppt/slides/slide1.xml", `<p:sld><a:t>Budget 2026</a:t></p:sld>`))
	if err != nil || !strings.Contains(r.Texte, "Budget 2026") || r.Methode != "pptx" { t.Fatalf("pptx : %+v %v", r, err) }
	r, err = Extraire("notes.txt", "text/plain", []byte("Rendez-vous lundi\r\n\r\n\r\n\r\nà 9 h"))
	if err != nil || r.Texte != "Rendez-vous lundi\n\nà 9 h" || r.Methode != "texte" { t.Fatalf("txt : %+v %v", r, err) }
}
```

`zipAvec` : helper de test qui construit une archive zip en mémoire avec un fichier ; les tests docx/xlsx existants ont sûrement l'équivalent, le réutiliser.

- [ ] **Step 2 : lancer, vérifier l'échec**

Run: `cd backend && go test ./internal/piecejointe/ -run 'TestDocumentImageEtLibelle|TestExtrairePptxEtTexte' -v`
Expected: FAIL, `undefined: Document`

- [ ] **Step 3 : implémenter**

Dans `extract.go` : `famille` reconnaît `pptx` (ext `pptx` ou mime contenant `presentationml`) et `texte` (ext `txt`, `csv`, ou mime `text/plain`, `text/csv`). `Document` vrai pour `pdf, docx, xlsx, pptx, texte`. `Image` vrai pour `image`. `Libelle` par famille. `Extraire` : `pptx` via `extraireOOXML` avec une regexp `(?s)<a:t(?:\s[^>]*)?>(.*?)</a:t>` et la liste des fichiers pertinents étendue à `ppt/slides/*.xml` ; `texte` : `nettoyer(string(data))` (CSV compris, les séparateurs restent). `Traitable` renvoie `Document(nom, mime) || Image(nom, mime)`.

- [ ] **Step 4 : lancer tout le paquet**

Run: `cd backend && go test ./internal/piecejointe/ -v`
Expected: PASS, tests existants inclus

- [ ] **Step 5 : commit**

```bash
git add backend/internal/piecejointe
git commit -m "piecejointe : périmètre document (PDF, Word, Excel, PowerPoint, texte), libellés, extraction pptx et texte brut"
```

---

### Task 2 : mode d'enregistrement des pièces par statut et motif

**Files:**
- Create: `backend/internal/ingest/pieces.go`
- Create: `backend/internal/ingest/pieces_test.go`
- Modify: `backend/internal/ingest/ingester.go` (`Run` lignes 108-124, `enregistrerPiecesJointes` lignes 133-168)

**Interfaces:**
- Consumes: `piecejointe.Document`, `piecejointe.Image`, `piecejointe.Extraire` (Tâche 1) ; `store.InsertAttachment(ctx, messageID, store.Attachment)`.
- Produces: `type ModePieces string` avec `ModeAucune = "aucune"`, `ModeTexteSeul = "texte_seul"`, `ModeComplete = "complete"` ; `func ModePour(statut string, motif *string) ModePieces` ; `func planPiece(mode ModePieces, nom, mime string) (traiter, ocr bool)` ; `func (ing *Ingester) enregistrerPiecesJointes(ctx, messageID int64, pjs []channel.PieceJointe, mode ModePieces) int` (retourne le nombre de lignes insérées).

- [ ] **Step 1 : tests qui échouent**

```go
func TestModePour(t *testing.T) {
	m := func(s string) *string { return &s }
	cas := []struct{ statut string; motif *string; attendu ModePieces }{
		{"pending", nil, ModeComplete},
		{"analyzed", nil, ModeComplete},
		{"excluded", m("expediteur_automatique"), ModeTexteSeul},
		{"excluded", m("newsletter"), ModeTexteSeul},
		{"excluded", m("publicitaire"), ModeTexteSeul},
		{"excluded", m("domaine_exclu"), ModeTexteSeul},
		{"excluded", m("fil_exclu"), ModeTexteSeul},
		{"excluded", m("regle_apprise"), ModeTexteSeul},
		{"excluded", m("corps_vide"), ModeTexteSeul},
		{"excluded", m("motif_inconnu_futur"), ModeTexteSeul},
		{"excluded", nil, ModeTexteSeul},
		{"excluded", m("securite_suspect"), ModeAucune},
		{"excluded", m("securite_nouveau_motif"), ModeAucune},
		{"excluded", m("categorie_sensible_sante"), ModeAucune},
		{"excluded", m("categorie_sensible_rh"), ModeAucune},
		{"excluded", m("categorie_sensible_juridique"), ModeAucune},
	}
	for _, c := range cas {
		if got := ModePour(c.statut, c.motif); got != c.attendu { t.Errorf("%s/%v : %s, attendu %s", c.statut, c.motif, got, c.attendu) }
	}
}

func TestPlanPiece(t *testing.T) {
	cas := []struct{ mode ModePieces; nom, mime string; traiter, ocr bool }{
		{ModeAucune, "facture.pdf", "application/pdf", false, false},
		{ModeTexteSeul, "facture.pdf", "application/pdf", true, false},
		{ModeTexteSeul, "logo.png", "image/png", false, false},
		{ModeTexteSeul, "archive.zip", "application/zip", false, false},
		{ModeComplete, "facture.pdf", "application/pdf", true, true},
		{ModeComplete, "recu.jpg", "image/jpeg", true, true},
		{ModeComplete, "archive.zip", "application/zip", false, false},
	}
	for _, c := range cas {
		tr, ocr := planPiece(c.mode, c.nom, c.mime)
		if tr != c.traiter || ocr != c.ocr { t.Errorf("%s %s : traiter=%v ocr=%v", c.mode, c.nom, tr, ocr) }
	}
}

// Review Focus 4 : un document sans texte est quand même une ligne, une image sans texte non.
func TestPlanPieceDocumentSansTexte(t *testing.T) {
	if !garderSansTexte("scan.pdf", "application/pdf") { t.Fatal("un document sans texte doit être enregistré (nom et type)") }
	if garderSansTexte("photo.jpg", "image/jpeg") { t.Fatal("une image sans texte OCR ne doit pas être enregistrée") }
}
```

- [ ] **Step 2 : lancer, vérifier l'échec**

Run: `cd backend && go test ./internal/ingest/ -run 'TestModePour|TestPlanPiece' -v`
Expected: FAIL, `undefined: ModePour`

- [ ] **Step 3 : implémenter `pieces.go`**

`ModePour` : statut différent de `excluded` → `ModeComplete` ; motif nul → `ModeTexteSeul` ; préfixes `categorie_sensible_`, `securite_` → `ModeAucune` ; sinon `ModeTexteSeul`. `planPiece` : `ModeAucune` → `(false,false)` ; `ModeTexteSeul` → `(Document, false)` ; `ModeComplete` → `(Document || Image, true)`. `garderSansTexte(nom, mime) bool` = `piecejointe.Document(nom, mime)`.

- [ ] **Step 4 : brancher dans `ingester.go`**

`enregistrerPiecesJointes(ctx, messageID, pjs, mode) int` : pour chaque pièce, `traiter, ocr := planPiece(mode, pj.Nom, pj.Type)` ; si `!traiter` → suivante ; `Extraire` ; si `r.BesoinOCR && ocr && ing.OCR != nil` → OCR ; si `texte == "" && !garderSansTexte(...)` → suivante ; sinon `InsertAttachment` avec `BesoinOCR: false`. Une erreur d'`Extraire` sur un document (archive Office illisible) enregistre quand même nom et type, texte vide. Garde-fou : au plus 8 pièces par message (comme Gmail).

Dans `Run`, les trois branches appellent la même fonction : catégorie sensible → `ModeAucune` (donc rien, mais le code le dit) ; `ExclusionReason` non vide → `ModePour("excluded", &reason)` ; conservé → `ModeComplete`. Retirer le commentaire « Pièces jointes seulement sur les messages conservés » et le remplacer par le pourquoi (factures de robots, voir spec).

- [ ] **Step 5 : lancer le paquet et construire**

Run: `cd backend && go test ./internal/ingest/ -v && go build ./...`
Expected: PASS, build OK

- [ ] **Step 6 : commit**

```bash
git add backend/internal/ingest
git commit -m "ingestion : les documents joints des messages écartés sont enregistrés en texte seul, jamais pour le confidentiel ni l'hameçonnage"
```

---

### Task 3 : index plein texte et requêtes de recherche (store)

**Files:**
- Modify: `backend/internal/store/schema.go` (après `CREATE INDEX IF NOT EXISTS idx_attachments_message`, ligne 95)
- Modify: `backend/internal/store/models.go` (nouveau type `PieceTrouvee`)
- Create: `backend/internal/store/pieces.go`
- Create: `backend/internal/store/pieces_test.go`

**Interfaces:**
- Produces:
  - colonne `attachments.recherche tsvector GENERATED ALWAYS AS (to_tsvector('french', left(nom, 200) || ' ' || left(texte, 200000))) STORED` + `CREATE INDEX IF NOT EXISTS idx_attachments_recherche ON attachments USING GIN (recherche)`.
  - `type PieceTrouvee struct { ID int64; Nom, TypeMime string; MessageID, ThreadID int64; ExternalID, Channel string; Expediteur, Objet string; Date time.Time; Extrait string }` (tags json en snake_case : `id, nom, type_mime, message_id, fil_id, external_id, channel, expediteur, objet, date, extrait`).
  - `func (s *Store) RechercherPieces(ctx context.Context, q string, limite int) ([]PieceTrouvee, error)`.
  - `func (s *Store) MessagesConnusSansPiece(ctx context.Context, channel string, externalIDs []string) ([]Message, error)` : messages du tenant dont `(channel, external_id)` est dans la liste et qui n'ont aucune ligne `attachments` ; renvoie `ID, ExternalID, Status, ExcludeReason, Subject, Sender`.

- [ ] **Step 1 : tests qui échouent** (gabarit `isolation_test.go` : `t.Skip` sans `TEST_ADMIN_URL`/`TEST_APP_URL`, `New`, `CreateUser` avec une adresse unique par exécution, `EnTenant`)

```go
func TestRechercherPieces(t *testing.T) {
	st, ctx, uid := baseDeTest(t) // helper local : New + CreateUser + retourne l'id
	_ = st.EnTenant(ctx, uid, func(ctx context.Context) error {
		th, _ := st.UpsertThread(ctx, "fixture", "th-1", "Votre facture d'octobre", time.Now())
		id, _, _ := st.InsertMessage(ctx, Message{ThreadID: th, ExternalID: "m-1", Channel: "fixture", Sender: "noreply@qonto.com", Recipients: []string{}, SentAt: time.Now(), Subject: "Votre facture d'octobre"})
		_ = st.MarkMessage(ctx, id, "excluded", ptr("expediteur_automatique")) // ptr : helper local *string
		_ = st.InsertAttachment(ctx, id, Attachment{Nom: "Facture-2026-10.pdf", TypeMime: "application/pdf", Texte: "Facture n° F-2026-10 Total TTC 24,00 EUR"})
		_ = st.InsertAttachment(ctx, id, Attachment{Nom: "releve.pdf", TypeMime: "application/pdf", Texte: ""})

		res, err := st.RechercherPieces(ctx, "factures", 12) // pluriel : racinisation française
		if err != nil || len(res) != 1 || res[0].Nom != "Facture-2026-10.pdf" { t.Fatalf("factures : %v %+v", err, res) }
		if res[0].Expediteur != "noreply@qonto.com" || res[0].Objet != "Votre facture d'octobre" || res[0].ThreadID != th { t.Fatalf("champs : %+v", res[0]) }
		if !strings.Contains(res[0].Extrait, "Total TTC") || strings.Contains(res[0].Extrait, "<b>") { t.Fatalf("extrait : %q", res[0].Extrait) }

		res, _ = st.RechercherPieces(ctx, "qonto", 12) // par expéditeur : les deux pièces
		if len(res) != 2 || res[0].Extrait != "" && res[1].Extrait != "" { t.Fatalf("expéditeur : %+v", res) }

		res, _ = st.RechercherPieces(ctx, "zzzz", 12)
		if len(res) != 0 { t.Fatalf("aucun résultat attendu : %+v", res) }
		return nil
	})
}

// Review Focus 4
func TestRechercherPiecesParNomSeul(t *testing.T) {
	// pièce avec texte vide, nom "plan-terrasse-v2.pdf" : trouvée par "terrasse", et TexteAttachments(message) == ""
}

// Review Focus 3
func TestRechercherPiecesRequeteBizarre(t *testing.T) {
	for _, q := range []string{`"facture" -relevé`, `Qonto&|!`, `(((`, `l'été`} {
		if _, err := st.RechercherPieces(ctx, q, 12); err != nil { t.Fatalf("%q : %v", q, err) }
	}
}

// Review Focus 2
func TestInsertAttachmentTexteEnorme(t *testing.T) {
	// texte de 3 000 000 de caractères : InsertAttachment réussit, la pièce est trouvée par son nom
}

func TestMessagesConnusSansPiece(t *testing.T) {
	// deux messages insérés, une pièce sur le premier : la fonction renvoie le second seul, avec Status et ExcludeReason ; un external_id inconnu est ignoré
}
```

- [ ] **Step 2 : lancer contre une base jetable, vérifier l'échec**

```bash
docker run -d --name ibalya-db-plan -e POSTGRES_USER=ibalya -e POSTGRES_PASSWORD=plan -e POSTGRES_DB=ibalya -p 127.0.0.1:5438:5432 postgres:16
cd backend && TEST_ADMIN_URL='postgres://ibalya:plan@127.0.0.1:5438/ibalya?sslmode=disable' TEST_APP_URL='postgres://ibalya_app:plan@127.0.0.1:5438/ibalya?sslmode=disable' go test ./internal/store/ -run 'Pieces|Attachment' -v
```
Expected: FAIL, `undefined: RechercherPieces` (le rôle `ibalya_app` est provisionné par `store.New` à partir de `TEST_APP_URL`).

- [ ] **Step 3 : implémenter**

`schema.go` : les deux instructions `ALTER TABLE attachments ADD COLUMN IF NOT EXISTS recherche …` et `CREATE INDEX IF NOT EXISTS …`, avec un commentaire sur le plafond de 1 Mo du `tsvector` (d'où `left(texte, 200000)`).

`pieces.go`, `RechercherPieces` : `motif := "%" + lower(q) + "%"` ; requête

```sql
WITH req AS (SELECT websearch_to_tsquery('french', $1) AS q)
SELECT a.id, a.nom, a.type_mime, m.id, m.thread_id, m.external_id, m.channel,
       COALESCE(NULLIF(p.name,''), m.sender), m.subject, m.sent_at,
       CASE WHEN a.recherche @@ req.q
            THEN ts_headline('french', left(a.texte, 20000), req.q, 'MaxWords=24, MinWords=10, MaxFragments=1')
            ELSE '' END
FROM attachments a JOIN messages m ON m.id = a.message_id
LEFT JOIN persons p ON p.email = m.sender, req
WHERE a.recherche @@ req.q OR lower(a.nom) LIKE $2 OR lower(m.sender) LIKE $2 OR lower(m.subject) LIKE $2
ORDER BY m.sent_at DESC, a.id LIMIT $3
```

Retirer `<b>` et `</b>` de l'extrait en Go. `websearch_to_tsquery` ne lève jamais d'erreur de syntaxe ; si `q` ne produit aucun lexème (`l'été` peut donner un tsquery vide), la branche `LIKE` suffit.

`MessagesConnusSansPiece` : `SELECT m.id, m.external_id, m.status, m.exclude_reason, m.subject, m.sender FROM messages m WHERE m.channel=$1 AND m.external_id = ANY($2) AND NOT EXISTS (SELECT 1 FROM attachments a WHERE a.message_id=m.id)`.

- [ ] **Step 4 : lancer les tests**

Même commande qu'au Step 2. Expected: PASS. Puis `go test ./internal/store/` complet (les autres tests sautent sans variables, c'est attendu).

- [ ] **Step 5 : commit**

```bash
git add backend/internal/store
git commit -m "store : index plein texte français sur les pièces jointes, recherche par nom, contenu, expéditeur et objet"
```

---

### Task 4 : pièces jointes en IMAP

**Files:**
- Modify: `backend/internal/channel/imap.go` (`convertirBrut` lignes 359-400, `corpsTexte` lignes 417-442)
- Test: `backend/internal/channel/imap_test.go`

**Interfaces:**
- Produces: `corpsTexte(lecteur *gomail.Reader) (texte string, pieces []PieceJointe)` ; `convertirBrut` remplit `Message.Attachments`. Format identique à Gmail : `PieceJointe{Nom, Type, Donnees}`.

- [ ] **Step 1 : tests qui échouent**

```go
func TestPieceJointeIMAP(t *testing.T) {
	i := connecteur()
	pdf := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 faux contenu"))
	msg := "From: noreply@qonto.com\r\nTo: dirigeant@pme.fr\r\nSubject: Facture\r\nMessage-ID: <pj@ex.fr>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"mx\"\r\n\r\n" +
		"--mx\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nVotre facture est jointe.\r\n" +
		"--mx\r\nContent-Type: image/png\r\nContent-Disposition: inline\r\nContent-Transfer-Encoding: base64\r\n\r\naGVsbG8=\r\n" +
		"--mx\r\nContent-Type: application/pdf; name=\"Facture-2026-10.pdf\"\r\nContent-Disposition: attachment; filename=\"Facture-2026-10.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\n" + pdf + "\r\n--mx--\r\n"
	m, err := i.convertirBrut([]byte(msg), time.Now())
	if err != nil { t.Fatal(err) }
	if !strings.Contains(m.Body, "Votre facture est jointe") { t.Fatalf("corps : %q", m.Body) }
	if len(m.Attachments) != 1 { t.Fatalf("1 pièce attendue (l'image inline sans nom n'en est pas une), %d", len(m.Attachments)) }
	pj := m.Attachments[0]
	if pj.Nom != "Facture-2026-10.pdf" || !strings.HasPrefix(pj.Type, "application/pdf") || string(pj.Donnees) != "%PDF-1.4 faux contenu" { t.Fatalf("%+v", pj) }
}

func TestPieceJointeIMAPInlineNommee(t *testing.T) {
	// une partie Content-Disposition: inline avec filename="signature.png" EST une pièce (comme Gmail : Filename non vide)
}

// Review Focus 1
func TestPieceJointeIMAPTropGrosse(t *testing.T) {
	// partie attachment de TailleMaxPJ+1 octets (Content-Transfer-Encoding: binary, pas de base64) : 0 pièce, message converti, corps intact
}
```

- [ ] **Step 2 : lancer, vérifier l'échec**

Run: `cd backend && go test ./internal/channel/ -run 'PieceJointeIMAP' -v`
Expected: FAIL, `m.Attachments` vide

- [ ] **Step 3 : implémenter**

Dans `corpsTexte`, la boucle `NextPart` gère aussi `*gomail.AttachmentHeader` : `nom, _ := h.Filename()` ; `ct, _, _ := h.ContentType()` ; lecture `io.ReadAll(io.LimitReader(partie.Body, TailleMaxPJ+1))` ; si `len > TailleMaxPJ` ou `nom == ""` → ignorée ; sinon ajoutée (au plus 8). Pour `*gomail.InlineHeader`, si `params["name"]` du `ContentType()` est non vide, même traitement (pièce nommée en inline). Le corps texte/HTML garde sa logique. `convertirBrut` : `m.Body, m.Attachments = corpsTexte(lecteur)`. Le message brut est déjà téléchargé en entier (`BodySection {}`), aucune requête IMAP supplémentaire : la mention « ne pas télécharger les types hors périmètre » de la spec est sans objet ici, le filtrage par type est fait à l'ingestion (Tâche 2), exactement comme pour Gmail.

- [ ] **Step 4 : lancer le paquet**

Run: `cd backend && go test ./internal/channel/ -v`
Expected: PASS

- [ ] **Step 5 : commit**

```bash
git add backend/internal/channel
git commit -m "imap : les pièces jointes nommées du message sont rapportées comme en Gmail (plafond 12 Mo, 8 pièces)"
```

---

### Task 5 : rattrapage `-indexer-pieces-jointes`

**Files:**
- Create: `backend/internal/ingest/indexation.go`
- Create: `backend/internal/ingest/indexation_test.go`
- Modify: `backend/internal/channel/fixture.go` (champ `attachments`)
- Modify: `scripts/gen_fixtures.py` et régénérer `fixtures/messages.json`
- Modify: `backend/cmd/server/main.go` (flags lignes 30-34 ; branchement après la construction de `ing`, ligne 157 et suivantes)

**Interfaces:**
- Consumes: `ModePour`, `planPiece`, `enregistrerPiecesJointes` (Tâche 2) ; `Store.MessagesConnusSansPiece` (Tâche 3) ; `Store.UtilisateursActifsAvecCanal`, `Store.EnTenant`, `Store.Audit`.
- Produces: `type BilanPieces struct { Lus, Concernes, Pieces int; ParMotif map[string]int; Echantillon []string }` ; `func (ing *Ingester) IndexerPieces(ctx context.Context, since time.Time, max int, appliquer bool) (BilanPieces, error)` ; flags `-indexer-pieces-jointes`, `-jours int` (défaut 30), `-appliquer` réutilisé ; fixture : `"attachments": [{"nom": "...", "type": "...", "contenu_base64": "..."}]` sur un message.

- [ ] **Step 1 : fixture avec pièces**

Dans `scripts/gen_fixtures.py`, ajouter :
- sur `fx-109` (no-reply@quincaillerie-plus.fr, newsletter) : `facture-QP-2026-10.pdf`, `application/pdf`, PDF minimal généré par le script (une page, Helvetica, texte `Facture QP-2026-10 Total TTC 148,00 EUR`), encodé base64 ;
- sur `fx-103` (commandes@vitrages-pro.fr, conservé) : `bon-de-commande-12-vitrages.pdf` même procédé, texte `Bon de commande 12 vitrages doubles` ; et `logo.png` (`image/png`, PNG 1×1 en base64) pour vérifier qu'une image sans OCR n'est pas enregistrée.

Le PDF minimal : écrire un fichier PDF à la main (objets catalog, pages, page, font Helvetica, contenu `BT /F1 12 Tf 72 720 Td (texte) Tj ET`, table xref correcte) ; `ledongthuc/pdf` doit en lire le texte, vérifié par le test de l'étape 3. Régénérer : `python3 scripts/gen_fixtures.py` (venv : voir mémoire `venv-python-ibalya`).

`fixture.go` : champ `Attachments []struct{ Nom string `json:"nom"`; Type string `json:"type"`; ContenuBase64 string `json:"contenu_base64"` } `json:"attachments"`` ; `FetchSince` décode en `[]PieceJointe` ; `FetchMetaSince` les vide comme le corps.

- [ ] **Step 2 : tests qui échouent** (base jetable comme Tâche 3, canal fixture `fixtures/messages.json`, `Ingester{Store: st, Channel: fx}`, sans OCR)

```go
func TestIndexerPieces(t *testing.T) {
	st, ctx, uid := baseDeTest(t); fx := channel.NewFixture("../../../fixtures/messages.json")
	ing := &Ingester{Store: st, Channel: fx}
	_ = st.EnTenant(ctx, uid, func(ctx context.Context) error {
		if _, err := ing.Run(ctx, time.Now().AddDate(0, 0, -30), 1000); err != nil { t.Fatal(err) }
		// l'ingestion a déjà enregistré les pièces : on simule l'état d'avant en les effaçant
		st.Q(ctx).Exec(ctx, `DELETE FROM attachments`)
		b, err := ing.IndexerPieces(ctx, time.Now().AddDate(0, 0, -30), 1000, false)
		if err != nil || b.Concernes != 2 || b.Pieces != 2 || b.ParMotif["newsletter"] != 1 || b.ParMotif["conserve"] != 1 { t.Fatalf("simulation : %+v %v", b, err) }
		var n int; st.Q(ctx).QueryRow(ctx, `SELECT count(*) FROM attachments`).Scan(&n)
		if n != 0 { t.Fatal("la simulation n'écrit rien") }
		b, _ = ing.IndexerPieces(ctx, time.Now().AddDate(0, 0, -30), 1000, true)
		st.Q(ctx).QueryRow(ctx, `SELECT count(*) FROM attachments`).Scan(&n)
		if b.Pieces != 2 || n != 2 { t.Fatalf("application : %+v, %d lignes (logo.png ne doit pas y être)", b, n) }
		return nil
	})
}

// Review Focus 5
func TestIndexerPiecesIdempotent(t *testing.T) {
	// après une application, une seconde passe (appliquer=true) donne Concernes == 0 et le nombre de lignes ne bouge pas
}

func TestIndexerPiecesJamaisOCR(t *testing.T) {
	// Ingester avec OCR qui t.Fatal si appelé ; le rattrapage sur un message conservé portant logo.png ne l'appelle pas et n'enregistre pas l'image
}
```

- [ ] **Step 3 : lancer, vérifier l'échec**

Run (variables `TEST_*` comme Tâche 3) : `cd backend && go test ./internal/ingest/ -run 'IndexerPieces' -v`
Expected: FAIL, `undefined: IndexerPieces`

- [ ] **Step 4 : implémenter `indexation.go`**

`IndexerPieces` : `msgs := ing.canal(ctx).FetchSince(ctx, since, max)` ; `Lus = len(msgs)` ; garder ceux avec `len(Attachments) > 0`, indexer par `ExternalID` ; `connus := ing.Store.MessagesConnusSansPiece(ctx, canal.Name(), ids)` ; pour chaque connu : `mode := ModePour(m.Status, m.ExcludeReason)` plafonné à `ModeTexteSeul` si `ModeComplete` ; compter les pièces que `planPiece` retiendrait (`Pieces`), `Concernes++`, `ParMotif[motif ou "conserve"]++`, échantillon « expéditeur | objet | n pièces » (dix au plus) ; si `appliquer` → `enregistrerPiecesJointes(ctx, m.ID, pjs, mode)` et `Pieces` compte les lignes réellement insérées. L'OCR n'est jamais invoqué parce que le mode plafonné le fixe à `ocr=false`.

`main.go` : flags `indexerPJ := flag.Bool("indexer-pieces-jointes", false, "…")`, `jours := flag.Int("jours", 30, "…")`. Après la construction de `ing` (ligne 157 et suivantes) : si `*indexerPJ`, `ids := st.UtilisateursActifsAvecCanal(ctx)` ; pour chaque id `st.EnTenant(ctx, id, …)` → `ing.IndexerPieces(ctx, time.Now().AddDate(0,0,-*jours), 5000, *appliquer)` ; afficher `email : N messages lus, M concernés, P pièces` puis `ParMotif` et l'échantillon ; si `appliquer`, `st.Audit(ctx, "agent", "indexation_pieces_jointes", bilan)` ; `return` avant le lancement du serveur. L'adresse de la boîte : `ing.canal(ctx).AccountEmail(ctx)`.

- [ ] **Step 5 : lancer les tests et construire**

Run: `cd backend && go test ./internal/ingest/ ./internal/channel/ -v && go build ./... && go vet ./...`
Expected: PASS, build OK

- [ ] **Step 6 : commit**

```bash
git add backend/internal/ingest backend/internal/channel/fixture.go backend/cmd/server/main.go scripts/gen_fixtures.py fixtures/messages.json
git commit -m "rattrapage : -indexer-pieces-jointes relit les N derniers jours et enregistre les documents des messages déjà connus, sans OCR ; fixture avec pièces"
```

---

### Task 6 : route `GET /api/recherche/pieces-jointes`

**Files:**
- Create: `backend/internal/api/recherche_pieces.go`
- Create: `backend/internal/api/recherche_pieces_test.go`
- Modify: `backend/internal/api/server.go` (ligne 155, après `GET /api/recherche`)

**Interfaces:**
- Consumes: `Store.RechercherPieces` (Tâche 3) ; `piecejointe.Libelle` (Tâche 1) ; `s.Engine.Canal(ctx)` (`Name()`, `AccountEmail(ctx)`, `LienWeb(compte, externalID)`).
- Produces: JSON `[]PieceResultat` : `{"id", "nom", "type_mime", "type_libelle", "message_id", "fil_id", "expediteur", "objet", "date" (RFC3339), "extrait", "url_web"}` ; `func (s *Server) recherchePieces(w, r)`.

- [ ] **Step 1 : tests qui échouent**

```go
func TestRecherchePiecesRequeteCourte(t *testing.T) {
	s := &Server{} // aucun store : la route doit répondre avant d'y toucher
	for _, q := range []string{"", "ab", "  a "} {
		r := httptest.NewRequest(http.MethodGet, "/api/recherche/pieces-jointes?q="+url.QueryEscape(q), nil)
		w := httptest.NewRecorder()
		s.recherchePieces(w, r)
		if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" { t.Fatalf("%q : %d %s", q, w.Code, w.Body.String()) }
	}
}

func TestPieceResultatDepuis(t *testing.T) {
	p := pieceResultatDepuis(store.PieceTrouvee{ID: 3, Nom: "f.pdf", TypeMime: "application/pdf", MessageID: 7, ThreadID: 2, ExternalID: "x", Channel: "gmail", Date: date}, "https://mail.google.com/…")
	if p.TypeLibelle != "PDF" || p.FilID != 2 || p.URLWeb == "" || p.Date != date.Format(time.RFC3339) { t.Fatalf("%+v", p) }
}
```

- [ ] **Step 2 : lancer, vérifier l'échec**

Run: `cd backend && go test ./internal/api/ -run 'RecherchePieces|PieceResultat' -v`
Expected: FAIL, `undefined`

- [ ] **Step 3 : implémenter**

`recherchePieces` : `q := TrimSpace(Query("q"))` ; `len([]rune(q)) < 3` → `writeJSON(w, []PieceResultat{})` ; sinon `RechercherPieces(ctx, q, 12)` ; `canal := s.Engine.Canal(ctx)` ; `compte, _ := canal.AccountEmail(ctx)` ; pour chaque pièce, `url := ""` et si `p.Channel == canal.Name()` → `canal.LienWeb(compte, p.ExternalID)` ; `pieceResultatDepuis(p, url)`. Erreur store → 500 avec `http.Error`. Route : `mux.HandleFunc("GET /api/recherche/pieces-jointes", s.auth(s.recherchePieces))`.

- [ ] **Step 4 : lancer, construire**

Run: `cd backend && go test ./internal/api/ -v && go build ./...`
Expected: PASS

- [ ] **Step 5 : commit**

```bash
git add backend/internal/api
git commit -m "api : GET /api/recherche/pieces-jointes, recherche plein texte des pièces avec lien vers le mail d'origine"
```

---

### Task 7 : mode « Pièces jointes » dans la barre de recherche

**Files:**
- Modify: `frontend/src/components/Recherche.jsx`
- Modify: `frontend/src/styles.css` (bloc `.recherche-*`, lignes 927-965)

**Interfaces:**
- Consumes: `api('/recherche/pieces-jointes?q=…')` → tableau de `PieceResultat` (Tâche 6) ; `api('/recherche?q=…')` existant.

- [ ] **Step 1 : implémenter**

Dans `Recherche.jsx` :
- état `mode` (`'tout' | 'pieces'`), initialisé depuis `sessionStorage['recherche-mode']` dans un `try/catch`, réécrit au changement ;
- l'effet de frappe appelle `/recherche` ou `/recherche/pieces-jointes` selon le mode ;
- sous le champ, quand la liste est ouverte, deux boutons `.recherche-chip` « Tout » / « Pièces jointes » (`aria-pressed`), avant les résultats ;
- rendu d'une pièce (`.recherche-item.recherche-pj`) : `recherche-type` = `type_libelle` ; titre = `nom` ; `recherche-detail` = `expediteur · objet` ; une colonne date (`dd/MM`) ; sous le détail, `extrait` s'il est non vide (`.recherche-extrait`) ; au bout de la ligne, texte « Ouvrir dans Gmail » quand `url_web` contient `mail.google.com`, « Ouvrir dans Outlook » sinon si `url_web` est non vide, « Voir le fil » sinon ;
- `aller(r)` pour une pièce : `url_web` → `window.open(url_web, '_blank', 'noopener')` ; sinon `navigate('/miroir?fil=' + fil_id)` ;
- vide : « Aucune pièce jointe pour « … ». » en mode pièces, texte actuel sinon ; si `GET /api/canal` (chargé une fois à l'ouverture du mode) indique une boîte Outlook, le vide dit « Les pièces jointes des boîtes Outlook ne sont pas encore lues. » ;
- placeholder : « Rechercher un client, un engagement, une pièce jointe… » ;
- clavier : les flèches et Entrée marchent dans les deux modes (même tableau `res`).

CSS : `.recherche-modes{display:flex;gap:6px;padding:8px 10px 4px;}` ; `.recherche-chip` plat, bordure `var(--border)`, `[aria-pressed=true]` fond `var(--secondary)` ; `.recherche-pj{grid-template-columns:96px 1fr auto;}` ; `.recherche-extrait{grid-column:2 / span 2;font-size:11.5px;color:var(--muted-foreground);white-space:normal;}` ; `.recherche-ouvrir{grid-column:2 / span 2;font-size:11.5px;color:var(--primary);}`. Rester dans la charte (voir mémoire `design-anti-ia`).

- [ ] **Step 2 : construire**

Run: `cd frontend && npm run build --silent`
Expected: `✓ built`, aucun avertissement sur `Recherche.jsx`

- [ ] **Step 3 : vérifier à la main sur la pile jetable**

Démarrer un backend jetable (binaire fraîchement construit) avec `env -i`, variables explicites, base jetable de la Tâche 3 (`DATABASE_URL`/`APP_DATABASE_URL` vers `127.0.0.1:5438`), `CHANNEL=fixture`, `FIXTURE_PATH=$PWD/fixtures/messages.json`, `FRONTEND_DIR=$PWD/frontend/dist`, `API_ADDR=127.0.0.1:9996`, `LLM_SERVICE_URL=http://127.0.0.1:1`, `ENCRYPTION_KEY` et `ADMIN_TOKEN` quelconques (voir `config.Verifier`) ; créer un compte (`-creer-utilisateur`, mot de passe sur stdin), se connecter, déclencher une lecture (bouton « Analyser » ou `POST /api/ingest` selon la route existante, `since_days` 30), puis dans la barre : mode « Pièces jointes », taper « facture » → `facture-QP-2026-10.pdf` apparaît avec `no-reply@quincaillerie-plus.fr`, l'extrait « Total TTC 148,00 EUR », « Voir le fil » (fixture sans lien web) ; taper « vitrages » → `bon-de-commande-12-vitrages.pdf` ; `logo.png` n'apparaît jamais. Capture d'écran dans le scratchpad. Arrêter le backend jetable.

- [ ] **Step 4 : commit**

```bash
git add frontend/src/components/Recherche.jsx frontend/src/styles.css
git commit -m "recherche : mode « Pièces jointes » dans la barre, résultats datés avec extrait et lien vers le mail d'origine"
```

---

### Task 8 : texte public et catalogue

**Files:**
- Modify: `landing/donnees.html` (lignes 48 et 51)
- Modify: `docs/FONCTIONNALITES.md` (sections Extraction, Livrables, Hors périmètre)

- [ ] **Step 1 : `donnees.html`**

Ligne 48, après « jamais transmis au modèle de langage. » ajouter : « Leurs documents joints (factures, relevés, bons de commande) sont lus en texte, sans reconnaissance optique et sans passage par le modèle, pour que vous puissiez les retrouver depuis la recherche. Les catégories confidentielles et les hameçonnages en restent exclus, pièces jointes comprises. »

Ligne 51 devient : « <strong>Les fichiers joints</strong> : Ibalya en extrait le texte (PDF, Word, Excel, PowerPoint, texte ; une image seulement si elle contient un document lisible) et ne conserve jamais le fichier lui-même. Pour le récupérer, vous retournez dans votre messagerie. Les boîtes Outlook n'ont pas encore cette lecture des pièces jointes. »

Mettre la date du chapeau (ligne 40) au jour de la mise en ligne.

- [ ] **Step 2 : `FONCTIONNALITES.md`**

- Section « Extraction des engagements », ligne « Pièces jointes » : « Texte extrait des documents (PDF, Word, Excel, PowerPoint, texte), OCR au besoin pour les messages conservés, jamais le fichier conservé ; les documents des messages écartés sont indexés en texte seul, sans OCR | Ingestion ».
- Section « Livrables », nouvelle ligne : « Recherche de pièces jointes | Mode « Pièces jointes » de la barre : par nom, contenu, expéditeur ou objet, résultats datés avec extrait et lien vers le mail d'origine (Gmail, IMAP) | Barre de recherche ».
- Section « Exploitation », nouvelle ligne : « Indexation des pièces | Rattrapage des documents joints des N derniers jours d'une boîte, simulation puis application, jamais d'OCR | Commande `-indexer-pieces-jointes` ».
- Section « Hors périmètre », nouvelle ligne : « Pièces jointes des boîtes Outlook | Connecteur Microsoft Graph sans lecture des pièces, au premier utilisateur Outlook ».
- Date de l'en-tête : jour de la mise en ligne.

- [ ] **Step 3 : relire**

Run: `grep -n '—' landing/donnees.html docs/FONCTIONNALITES.md`
Expected: aucune ligne (pas de tiret cadratin).

- [ ] **Step 4 : commit**

```bash
git add landing/donnees.html docs/FONCTIONNALITES.md
git commit -m "vitrine et catalogue : lecture des documents joints des emails automatiques, recherche de pièces jointes, Outlook hors périmètre"
```

---

### Task 9 : PR, mise en prod et rattrapage

**Files:** aucun nouveau. Branche `recherche-pieces-jointes`.

- [ ] **Step 1 : suite complète**

```bash
cd backend && go vet ./... && TEST_ADMIN_URL=… TEST_APP_URL=… go test ./... 2>&1 | tail -20
cd ../frontend && npm run build --silent
```
Expected: tout PASS ; `frontend/dist` reconstruit depuis `main` ensuite (le dossier est servi par la prod : `git show main:frontend/src/components/Recherche.jsx > …`, rebuild, restaurer le fichier de la branche), comme fait pour la PR #120.

- [ ] **Step 2 : nettoyer la base jetable**

Run: `docker rm -f ibalya-db-plan`

- [ ] **Step 3 : pousser et ouvrir la PR**

`git push https://github.com/AkharnX/Ibalya.git recherche-pieces-jointes` puis `gh pr create --repo AkharnX/Ibalya --base main --head recherche-pieces-jointes` ; corps : pourquoi (retour testeur, 0 facture indexée), quoi (tâches 1 à 8), vérification (tests, pile jetable, captures), **marche à suivre après déploiement** (Step 5). Ibrahim merge avec `! gh pr merge <n> --squash --repo AkharnX/Ibalya`.

- [ ] **Step 4 : vérifier le déploiement**

Attendre que `.deploy-stamp` égale le commit de merge, `systemctl is-active ibalya-backend`, `curl -s http://127.0.0.1:9999/api/health`. Vérifier en base que la colonne `recherche` et l'index existent : `docker exec ibalya-db psql -U ibalya -d ibalya -Atc "\d attachments"`.

- [ ] **Step 5 : rattrapage en prod, simulation puis application**

```bash
cd /home/akharn/agnet-ia && set -a && . ./.env && set +a
./backend/bin/ibalya -indexer-pieces-jointes -jours 30            # simulation : rien n'est écrit
./backend/bin/ibalya -indexer-pieces-jointes -jours 30 -appliquer
docker exec ibalya-db psql -U ibalya -d ibalya -Atc "select u.email, count(*) from attachments a join messages m on m.id=a.message_id join users u on u.id=m.user_id where m.status='excluded' group by 1"
```
Expected : la simulation liste par boîte les messages concernés et les pièces par motif ; après application, des pièces existent sur des messages `excluded` (zéro avant). Noter les chiffres dans la réponse à Ibrahim, et dans la mémoire du projet (retour Stewe) que le testeur doit refaire sa recherche de factures.

package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// baseDeTest ouvre la base de test (TEST_ADMIN_URL + TEST_APP_URL, sinon le
// test est ignoré) et crée un tenant neuf, unique par exécution.
func baseDeTest(t *testing.T) (*Store, context.Context, int64) {
	t.Helper()
	adminURL, appURL := os.Getenv("TEST_ADMIN_URL"), os.Getenv("TEST_APP_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("TEST_ADMIN_URL / TEST_APP_URL non définis")
	}
	ctx := context.Background()
	st, err := New(ctx, adminURL, appURL)
	if err != nil {
		t.Fatalf("ouverture base : %v", err)
	}
	email := fmt.Sprintf("pj-%d-%s@test.fr", time.Now().UnixNano(), strings.ToLower(t.Name()))
	uid, err := st.CreateUser(ctx, email, "Test pièces", "MotDePasseTest123!")
	if err != nil {
		t.Fatalf("créer le tenant : %v", err)
	}
	return st, ctx, uid
}

func ptr(s string) *string { return &s }

// messageAvec insère un fil et un message écarté comme automatique, et renvoie
// l'id du message et celui du fil.
func messageAvec(t *testing.T, st *Store, ctx context.Context, externalID, sender, subject string) (int64, int64) {
	t.Helper()
	th, err := st.UpsertThread(ctx, "fixture", "th-"+externalID, subject, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := st.InsertMessage(ctx, Message{ThreadID: th, ExternalID: externalID, Channel: "fixture",
		Sender: sender, Recipients: []string{}, SentAt: time.Now(), Subject: subject, Body: "corps"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.MarkMessage(ctx, id, "excluded", ptr("expediteur_automatique")); err != nil {
		t.Fatal(err)
	}
	return id, th
}

func TestRechercherPieces(t *testing.T) {
	st, ctx, uid := baseDeTest(t)
	err := st.EnTenant(ctx, uid, func(ctx context.Context) error {
		id, th := messageAvec(t, st, ctx, "m-1", "noreply@qonto.com", "Votre facture d'octobre")
		if err := st.InsertAttachment(ctx, id, Attachment{Nom: "Facture-2026-10.pdf", TypeMime: "application/pdf",
			Texte: "Facture n° F-2026-10 Total TTC 24,00 EUR"}); err != nil {
			t.Fatal(err)
		}
		if err := st.InsertAttachment(ctx, id, Attachment{Nom: "releve.pdf", TypeMime: "application/pdf"}); err != nil {
			t.Fatal(err)
		}

		res, err := st.RechercherPieces(ctx, "factures", 12) // pluriel : racinisation française
		if err != nil || len(res) != 1 || res[0].Nom != "Facture-2026-10.pdf" {
			t.Fatalf("factures : %v %+v", err, res)
		}
		r := res[0]
		if r.Expediteur != "noreply@qonto.com" || r.Objet != "Votre facture d'octobre" || r.ThreadID != th ||
			r.MessageID != id || r.Channel != "fixture" || r.ExternalID != "m-1" || r.TypeMime != "application/pdf" {
			t.Fatalf("champs : %+v", r)
		}
		if !strings.Contains(r.Extrait, "Total TTC") || strings.Contains(r.Extrait, "<b>") {
			t.Fatalf("extrait : %q", r.Extrait)
		}

		res, err = st.RechercherPieces(ctx, "qonto", 12) // par expéditeur : les deux pièces
		if err != nil || len(res) != 2 || res[0].Extrait != "" || res[1].Extrait != "" {
			t.Fatalf("expéditeur : %v %+v", err, res)
		}

		res, err = st.RechercherPieces(ctx, "zzzz", 12)
		if err != nil || len(res) != 0 {
			t.Fatalf("aucun résultat attendu : %v %+v", err, res)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Une pièce sans texte reste trouvable par son nom, et l'extraction
// d'engagements ne la voit pas.
func TestRechercherPiecesParNomSeul(t *testing.T) {
	st, ctx, uid := baseDeTest(t)
	_ = st.EnTenant(ctx, uid, func(ctx context.Context) error {
		id, _ := messageAvec(t, st, ctx, "m-2", "archi@cabinet.fr", "Plans")
		if err := st.InsertAttachment(ctx, id, Attachment{Nom: "plan-terrasse-v2.pdf", TypeMime: "application/pdf"}); err != nil {
			t.Fatal(err)
		}
		res, err := st.RechercherPieces(ctx, "terrasse", 12)
		if err != nil || len(res) != 1 || res[0].Nom != "plan-terrasse-v2.pdf" || res[0].Extrait != "" {
			t.Fatalf("par nom : %v %+v", err, res)
		}
		if txt, _ := st.TexteAttachments(ctx, id); txt != "" {
			t.Fatalf("l'extraction ne doit pas voir une pièce sans texte : %q", txt)
		}
		return nil
	})
}

// Guillemets, opérateurs, parenthèses, apostrophes : jamais une erreur SQL.
func TestRechercherPiecesRequeteBizarre(t *testing.T) {
	st, ctx, uid := baseDeTest(t)
	_ = st.EnTenant(ctx, uid, func(ctx context.Context) error {
		for _, q := range []string{`"facture" -relevé`, `Qonto&|!`, `(((`, `l'été`, `a:b`, `!!!`} {
			if _, err := st.RechercherPieces(ctx, q, 12); err != nil {
				t.Fatalf("%q : %v", q, err)
			}
		}
		return nil
	})
}

// Le tsvector généré plafonne à 1 Mo : un texte énorme ne doit pas faire
// échouer l'insertion, et la pièce reste trouvable par son nom.
func TestInsertAttachmentTexteEnorme(t *testing.T) {
	st, ctx, uid := baseDeTest(t)
	_ = st.EnTenant(ctx, uid, func(ctx context.Context) error {
		id, _ := messageAvec(t, st, ctx, "m-3", "noreply@grosdoc.fr", "Catalogue")
		enorme := strings.Repeat("catalogue produit référence ", 110000) // ~3 Mo
		if err := st.InsertAttachment(ctx, id, Attachment{Nom: "catalogue-hiver.pdf", TypeMime: "application/pdf", Texte: enorme}); err != nil {
			t.Fatalf("insertion : %v", err)
		}
		res, err := st.RechercherPieces(ctx, "hiver", 12)
		if err != nil || len(res) != 1 {
			t.Fatalf("par nom : %v %+v", err, res)
		}
		return nil
	})
}

func TestMessagesConnusSansPiece(t *testing.T) {
	st, ctx, uid := baseDeTest(t)
	_ = st.EnTenant(ctx, uid, func(ctx context.Context) error {
		id1, _ := messageAvec(t, st, ctx, "m-4", "a@ex.fr", "Avec pièce")
		id2, _ := messageAvec(t, st, ctx, "m-5", "b@ex.fr", "Sans pièce")
		if err := st.InsertAttachment(ctx, id1, Attachment{Nom: "x.pdf", TypeMime: "application/pdf"}); err != nil {
			t.Fatal(err)
		}
		res, err := st.MessagesConnusSansPiece(ctx, "fixture", []string{"m-4", "m-5", "inconnu"})
		if err != nil || len(res) != 1 || res[0].ID != id2 || res[0].ExternalID != "m-5" {
			t.Fatalf("%v %+v", err, res)
		}
		if res[0].Status != "excluded" || res[0].ExcludeReason == nil || *res[0].ExcludeReason != "expediteur_automatique" ||
			res[0].Sender != "b@ex.fr" || res[0].Subject != "Sans pièce" {
			t.Fatalf("champs : %+v", res[0])
		}
		return nil
	})
}

// Un message reclassé après coup en confidentiel ou hameçonnage (commande
// confidentialite, rattrapage du pré-filtre) peut encore porter des pièces de
// l'époque où il était conservé : la recherche ne doit jamais les montrer, et
// DeleteAttachments doit permettre de les retirer.
func TestRechercherPiecesIgnoreMotifsInterdits(t *testing.T) {
	st, ctx, uid := baseDeTest(t)
	_ = st.EnTenant(ctx, uid, func(ctx context.Context) error {
		for i, motif := range []string{"categorie_sensible_rh", "securite_suspect"} {
			id, _ := messageAvec(t, st, ctx, fmt.Sprintf("m-i%d", i), "x@ex.fr", "")
			if err := st.MarkMessage(ctx, id, "excluded", ptr(motif)); err != nil {
				t.Fatal(err)
			}
			if err := st.InsertAttachment(ctx, id, Attachment{Nom: "bulletin-de-paie-cache.pdf", TypeMime: "application/pdf", Texte: "bulletin de paie confidentiel"}); err != nil {
				t.Fatal(err)
			}
		}
		res, err := st.RechercherPieces(ctx, "bulletin", 12)
		if err != nil || len(res) != 0 {
			t.Fatalf("les pièces de motifs interdits ne doivent jamais sortir : %v %+v", err, res)
		}
		var n int
		_ = st.Q(ctx).QueryRow(ctx, `SELECT count(*) FROM attachments`).Scan(&n)
		if n != 2 {
			t.Fatalf("2 lignes attendues avant nettoyage, %d", n)
		}
		id, _ := messageAvec(t, st, ctx, "m-i9", "y@ex.fr", "Nettoyage")
		_ = st.InsertAttachment(ctx, id, Attachment{Nom: "a.pdf", TypeMime: "application/pdf"})
		if err := st.DeleteAttachments(ctx, id); err != nil {
			t.Fatal(err)
		}
		_ = st.Q(ctx).QueryRow(ctx, `SELECT count(*) FROM attachments WHERE message_id=$1`, id).Scan(&n)
		if n != 0 {
			t.Fatalf("DeleteAttachments doit vider le message, %d restantes", n)
		}
		return nil
	})
}

// Un octet nul est de l'UTF-8 valide mais PostgreSQL le refuse : il doit
// disparaître avant l'insertion, sinon la pièce est perdue, nom compris.
func TestValideUTF8SupprimeNul(t *testing.T) {
	if got := valideUTF8("Rel\x00ev\xe9 \x00de compte"); strings.ContainsRune(got, 0) {
		t.Fatalf("octet nul conservé : %q", got)
	}
	if got := valideUTF8("sain"); got != "sain" {
		t.Fatalf("%q", got)
	}
}

package ingest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"ibalya/backend/internal/channel"
	"ibalya/backend/internal/store"
)

// baseDeTest : base de test (TEST_ADMIN_URL + TEST_APP_URL, sinon ignoré) et un
// tenant neuf par exécution.
func baseDeTest(t *testing.T) (*store.Store, context.Context, int64) {
	t.Helper()
	adminURL, appURL := os.Getenv("TEST_ADMIN_URL"), os.Getenv("TEST_APP_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("TEST_ADMIN_URL / TEST_APP_URL non définis")
	}
	ctx := context.Background()
	st, err := store.New(ctx, adminURL, appURL)
	if err != nil {
		t.Fatalf("ouverture base : %v", err)
	}
	email := fmt.Sprintf("idx-%d-%s@test.fr", time.Now().UnixNano(), strings.ToLower(t.Name()))
	uid, err := st.CreateUser(ctx, email, "Test indexation", "MotDePasseTest123!")
	if err != nil {
		t.Fatalf("créer le tenant : %v", err)
	}
	return st, ctx, uid
}

const fixtureTest = "../../../fixtures/messages.json"

// ingereSansPieces lit la fixture puis efface les pièces : l'état d'une boîte
// raccordée avant cette fonctionnalité.
func ingereSansPieces(t *testing.T, ing *Ingester, st *store.Store, ctx context.Context) {
	t.Helper()
	if _, err := ing.Run(ctx, time.Now().AddDate(0, 0, -30), 1000); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Q(ctx).Exec(ctx, `DELETE FROM attachments`); err != nil {
		t.Fatal(err)
	}
}

func nbPieces(t *testing.T, st *store.Store, ctx context.Context) int {
	t.Helper()
	var n int
	if err := st.Q(ctx).QueryRow(ctx, `SELECT count(*) FROM attachments`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIndexerPieces(t *testing.T) {
	st, ctx, uid := baseDeTest(t)
	ing := &Ingester{Store: st, Channel: channel.NewFixture(fixtureTest)}
	_ = st.EnTenant(ctx, uid, func(ctx context.Context) error {
		ingereSansPieces(t, ing, st, ctx)
		depuis := time.Now().AddDate(0, 0, -30)

		b, err := ing.IndexerPieces(ctx, depuis, 1000, false)
		if err != nil || b.Concernes != 2 || b.Pieces != 2 || b.ParMotif["expediteur_automatique"] != 1 || b.ParMotif["conserve"] != 1 {
			t.Fatalf("simulation : %+v %v", b, err)
		}
		if n := nbPieces(t, st, ctx); n != 0 {
			t.Fatalf("la simulation n'écrit rien, %d lignes", n)
		}

		b, err = ing.IndexerPieces(ctx, depuis, 1000, true)
		if err != nil || b.Pieces != 2 {
			t.Fatalf("application : %+v %v", b, err)
		}
		if n := nbPieces(t, st, ctx); n != 2 {
			t.Fatalf("%d lignes, 2 attendues (logo.png ne doit pas y être)", n)
		}
		var noms []string
		rows, _ := st.Q(ctx).Query(ctx, `SELECT nom FROM attachments ORDER BY nom`)
		for rows.Next() {
			var n string
			_ = rows.Scan(&n)
			noms = append(noms, n)
		}
		rows.Close()
		if strings.Join(noms, ",") != "bon-de-commande-12-vitrages.pdf,facture-QP-2026-10.pdf" {
			t.Fatalf("noms : %v", noms)
		}
		res, err := st.RechercherPieces(ctx, "facture", 12)
		if err != nil || len(res) != 1 || !strings.Contains(res[0].Extrait, "148,00") {
			t.Fatalf("le texte du PDF généré doit être indexé : %v %+v", err, res)
		}
		return nil
	})
}

// Relancé, le rattrapage ne concerne plus rien et n'écrit rien.
func TestIndexerPiecesIdempotent(t *testing.T) {
	st, ctx, uid := baseDeTest(t)
	ing := &Ingester{Store: st, Channel: channel.NewFixture(fixtureTest)}
	_ = st.EnTenant(ctx, uid, func(ctx context.Context) error {
		ingereSansPieces(t, ing, st, ctx)
		depuis := time.Now().AddDate(0, 0, -30)
		if _, err := ing.IndexerPieces(ctx, depuis, 1000, true); err != nil {
			t.Fatal(err)
		}
		avant := nbPieces(t, st, ctx)
		b, err := ing.IndexerPieces(ctx, depuis, 1000, true)
		if err != nil || b.Concernes != 0 || b.Pieces != 0 || nbPieces(t, st, ctx) != avant {
			t.Fatalf("seconde passe : %+v %v (%d lignes avant)", b, err, avant)
		}
		return nil
	})
}

// Le rattrapage n'appelle jamais l'OCR, même pour un message conservé qui
// porte une image.
func TestIndexerPiecesJamaisOCR(t *testing.T) {
	st, ctx, uid := baseDeTest(t)
	ing := &Ingester{Store: st, Channel: channel.NewFixture(fixtureTest),
		OCR: func(ctx context.Context, nom, typeMime string, data []byte) (string, error) {
			t.Fatalf("OCR appelé pendant le rattrapage pour %q", nom)
			return "", nil
		}}
	_ = st.EnTenant(ctx, uid, func(ctx context.Context) error {
		// L'ingestion normale, elle, aurait le droit : on la fait sans OCR.
		ingSans := &Ingester{Store: st, Channel: ing.Channel}
		ingereSansPieces(t, ingSans, st, ctx)
		b, err := ing.IndexerPieces(ctx, time.Now().AddDate(0, 0, -30), 1000, true)
		if err != nil || b.Pieces != 2 {
			t.Fatalf("%+v %v", b, err)
		}
		var n int
		_ = st.Q(ctx).QueryRow(ctx, `SELECT count(*) FROM attachments WHERE nom='logo.png'`).Scan(&n)
		if n != 0 {
			t.Fatal("logo.png ne doit pas être enregistré")
		}
		return nil
	})
}

package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"ibalya/backend/internal/store"
)

// Moins de trois caractères : liste vide, sans toucher à la base (le serveur
// de test n'en a pas).
func TestRecherchePiecesRequeteCourte(t *testing.T) {
	s := &Server{}
	for _, q := range []string{"", "ab", "  a ", "é "} {
		r := httptest.NewRequest(http.MethodGet, "/api/recherche/pieces-jointes?q="+url.QueryEscape(q), nil)
		w := httptest.NewRecorder()
		s.recherchePieces(w, r)
		if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
			t.Fatalf("%q : %d %s", q, w.Code, w.Body.String())
		}
	}
}

func TestPieceResultatDepuis(t *testing.T) {
	date := time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC)
	p := pieceResultatDepuis(store.PieceTrouvee{ID: 3, Nom: "f.pdf", TypeMime: "application/pdf", MessageID: 7, ThreadID: 2,
		ExternalID: "x", Channel: "gmail", Expediteur: "Qonto", Objet: "Facture", Date: date, Extrait: "Total"},
		"https://mail.google.com/mail/u/0/#all/x")
	if p.ID != 3 || p.TypeLibelle != "PDF" || p.FilID != 2 || p.MessageID != 7 || p.URLWeb == "" ||
		p.Date != "2026-10-12T09:30:00Z" || p.Expediteur != "Qonto" || p.Objet != "Facture" || p.Extrait != "Total" {
		t.Fatalf("%+v", p)
	}
	if q := pieceResultatDepuis(store.PieceTrouvee{Nom: "scan.jpg", TypeMime: "image/jpeg"}, ""); q.TypeLibelle != "Image" || q.URLWeb != "" {
		t.Fatalf("%+v", q)
	}
}

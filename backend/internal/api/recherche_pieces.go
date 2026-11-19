package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"ibalya/backend/internal/piecejointe"
	"ibalya/backend/internal/store"
)

// PieceResultat : un résultat du mode « Pièces jointes » de la barre de
// recherche. On ne sert jamais le fichier : url_web mène au mail dans la
// messagerie de l'utilisateur (vide en IMAP, qui n'a pas d'interface web ;
// l'interface renvoie alors vers le fil).
type PieceResultat struct {
	ID          int64  `json:"id"`
	Nom         string `json:"nom"`
	TypeMime    string `json:"type_mime"`
	TypeLibelle string `json:"type_libelle"`
	MessageID   int64  `json:"message_id"`
	FilID       int64  `json:"fil_id"`
	Expediteur  string `json:"expediteur"`
	Objet       string `json:"objet"`
	Date        string `json:"date"`
	Extrait     string `json:"extrait"`
	URLWeb      string `json:"url_web"`
}

func pieceResultatDepuis(p store.PieceTrouvee, urlWeb string) PieceResultat {
	return PieceResultat{
		ID: p.ID, Nom: p.Nom, TypeMime: p.TypeMime, TypeLibelle: piecejointe.Libelle(p.Nom, p.TypeMime),
		MessageID: p.MessageID, FilID: p.ThreadID, Expediteur: p.Expediteur, Objet: p.Objet,
		Date: p.Date.UTC().Format(time.RFC3339), Extrait: p.Extrait, URLWeb: urlWeb,
	}
}

// limiteResultatsPieces : douze lignes suffisent dans une liste déroulante ;
// au-delà, l'utilisateur précise sa recherche.
const limiteResultatsPieces = 12

// GET /api/recherche/pieces-jointes?q=
func (s *Server) recherchePieces(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	out := []PieceResultat{}
	// Deux caractères ne discriminent rien et feraient balayer toute la base.
	if len([]rune(q)) < 3 {
		writeJSON(w, out)
		return
	}
	ctx := r.Context()
	pieces, err := s.Store.RechercherPieces(ctx, q, limiteResultatsPieces)
	if err != nil {
		log.Printf("api: recherche de pièces jointes : %v", err)
		http.Error(w, "recherche indisponible", http.StatusInternalServerError)
		return
	}
	canal := s.Engine.Canal(ctx)
	compte, _ := canal.AccountEmail(ctx)
	for _, p := range pieces {
		url := ""
		if p.Channel == canal.Name() {
			url = canal.LienWeb(compte, p.ExternalID)
		}
		out = append(out, pieceResultatDepuis(p, url))
	}
	writeJSON(w, out)
}

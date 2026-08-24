package api

import (
	"net/http"
	"strings"
)

// Recherche transverse : engagements, interlocuteurs, conversations.
//
// Avec cent cinquante interlocuteurs et six cents messages, retrouver un
// dossier supposait de savoir dans quelle page chercher. Les filtres de chaque
// écran ne portent que sur ce que l'écran affiche déjà.
type Resultat struct {
	Type   string `json:"type"` // engagement | interlocuteur | conversation
	ID     int64  `json:"id"`
	Titre  string `json:"titre"`
	Detail string `json:"detail,omitempty"`
}

// GET /api/recherche?q=
func (s *Server) recherche(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	out := []Resultat{}
	// Deux caractères ne discriminent rien et feraient balayer toute la base.
	if len(q) < 3 {
		writeJSON(w, out)
		return
	}
	motif := "%" + strings.ToLower(q) + "%"
	ctx := r.Context()

	ajouter := func(requete, typ string) {
		rows, err := s.Store.Pool.Query(ctx, requete, motif)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var res Resultat
			if err := rows.Scan(&res.ID, &res.Titre, &res.Detail); err != nil {
				return
			}
			res.Type = typ
			out = append(out, res)
		}
	}

	ajouter(`SELECT e.id, e.objet,
		    coalesce(pe.email,'') || ' → ' || coalesce(pd.email,'')
		 FROM engagements e
		 LEFT JOIN persons pe ON pe.id = e.emetteur_id
		 LEFT JOIN persons pd ON pd.id = e.destinataire_id
		 WHERE lower(e.objet) LIKE $1 AND e.statut <> 'abandonne'
		 ORDER BY e.maj_le DESC LIMIT 6`, "engagement")

	ajouter(`SELECT id, coalesce(nullif(name,''), email), email
		 FROM persons WHERE lower(email) LIKE $1 OR lower(name) LIKE $1
		 ORDER BY id DESC LIMIT 6`, "interlocuteur")

	ajouter(`SELECT id, coalesce(nullif(subject,''),'(sans objet)'),
		    coalesce(to_char(last_message_at,'DD/MM/YYYY'),'')
		 FROM threads WHERE lower(subject) LIKE $1
		 ORDER BY last_message_at DESC NULLS LAST LIMIT 6`, "conversation")

	writeJSON(w, out)
}

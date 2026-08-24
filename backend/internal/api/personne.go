package api

import (
	"net/http"
	"time"

	"ibalya/backend/internal/store"
)

// Fiche d'un interlocuteur : ce qu'on sait de lui, et surtout où on en est.
//
// La page listait les personnes déduites des échanges sans permettre d'en
// ouvrir une. Or c'est là que se trouve la question utile du CDC — « où en
// est-on avec ce client » — dont la réponse était éparpillée entre le suivi
// des engagements et les conversations.
type FichePersonne struct {
	store.Person
	MessagesEchanges int                `json:"messages_echanges"`
	DernierEchange   *time.Time         `json:"dernier_echange,omitempty"`
	EnCours          int                `json:"en_cours"`
	EnRetard         int                `json:"en_retard"`
	Livres           int                `json:"livres"`
	Engagements      []store.Engagement `json:"engagements"`
	Fils             []store.Thread     `json:"fils"`
}

// GET /api/persons/{id}
func (s *Server) fichePersonne(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := s.Store.GetPerson(ctx, pathID(r))
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	if p == nil {
		httpError(w, 404, "interlocuteur introuvable")
		return
	}
	f := FichePersonne{Person: *p, Engagements: []store.Engagement{}, Fils: []store.Thread{}}

	if last, n, err := s.Store.LastExchangeWithContact(ctx, p.Email); err == nil {
		f.MessagesEchanges, f.DernierEchange = n, last
	}
	// scanEngagements rend nil sur un résultat vide : sans ce garde-fou, le JSON
	// porte « null » là où le front attend une liste.
	if engs, err := s.Store.EngagementsDeContact(ctx, p.Email); err == nil && engs != nil {
		f.Engagements = engs
		for _, e := range engs {
			switch e.Statut {
			case "en_retard":
				f.EnRetard++
			case "livre":
				f.Livres++
			case "ouvert", "confirme":
				f.EnCours++
			}
		}
	}
	if fils, err := s.Store.FilsDeContact(ctx, p.Email, 15); err == nil && fils != nil {
		f.Fils = fils
	}
	writeJSON(w, f)
}

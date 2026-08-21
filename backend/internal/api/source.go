package api

import (
	"fmt"
	"net/http"
	"net/url"

	"ibalya/backend/internal/store"
)

// La traçabilité est une exigence du CDC (attribut `source` de l'entité
// Engagement) : le dirigeant doit pouvoir remonter de l'engagement extrait au
// message qui l'a produit, sans quitter l'application ni chercher dans Gmail.

type MessageSource struct {
	store.Message
	EstSource bool   `json:"est_source"` // le message d'où l'engagement a été extrait
	URLGmail  string `json:"url_gmail,omitempty"`
}

type SourceReponse struct {
	EngagementID  int64           `json:"engagement_id"`
	Objet         string          `json:"objet"`
	Sujet         string          `json:"sujet"`
	Canal         string          `json:"canal"`
	URLGmailFil   string          `json:"url_gmail_fil,omitempty"`
	Messages      []MessageSource `json:"messages"`
}

// urlGmail construit un lien direct vers un message ou un fil dans Gmail.
// Le paramètre authuser évite d'atterrir sur le mauvais compte quand
// l'utilisateur est connecté à plusieurs boîtes Google dans son navigateur.
func urlGmail(compte, externalID string) string {
	if externalID == "" {
		return ""
	}
	base := "https://mail.google.com/mail/"
	if compte != "" {
		base += "?authuser=" + url.QueryEscape(compte)
	}
	return base + "#all/" + externalID
}

// GET /api/engagements/{id}/source — le fil complet, message source signalé.
func (s *Server) engagementSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := pathID(r)

	eng, err := s.Store.GetEngagement(ctx, id)
	if err != nil || eng == nil {
		httpError(w, 404, "engagement introuvable")
		return
	}
	rep := SourceReponse{EngagementID: eng.ID, Objet: eng.Objet}

	if eng.ThreadID == nil {
		httpError(w, 404, "aucune conversation rattachée à cet engagement")
		return
	}
	fil, err := s.Store.GetThread(ctx, *eng.ThreadID)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	rep.Sujet = fil.Subject
	rep.Canal = fil.Channel

	msgs, err := s.Store.ThreadMessages(ctx, *eng.ThreadID)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	_, compte, _ := s.Store.GetOAuthToken(ctx, "google")
	if fil.Channel == "gmail" {
		rep.URLGmailFil = urlGmail(compte, fil.ExternalID)
	}
	for _, m := range msgs {
		ms := MessageSource{Message: m}
		ms.EstSource = eng.SourceMessageID != nil && *eng.SourceMessageID == m.ID
		if m.Channel == "gmail" {
			ms.URLGmail = urlGmail(compte, m.ExternalID)
		}
		rep.Messages = append(rep.Messages, ms)
	}
	if len(rep.Messages) == 0 {
		httpError(w, 404, fmt.Sprintf("aucun message conservé pour le fil %d", *eng.ThreadID))
		return
	}
	writeJSON(w, rep)
}

package api

import (
	"context"
	"errors"
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
	EngagementID int64           `json:"engagement_id,omitempty"`
	Objet        string          `json:"objet"`
	Sujet        string          `json:"sujet"`
	Canal        string          `json:"canal"`
	URLGmailFil  string          `json:"url_gmail_fil,omitempty"`
	Messages     []MessageSource `json:"messages"`
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

// filSource assemble la conversation d'un fil. eng est facultatif : fourni, il
// met en évidence le message d'origine et rappelle l'objet extrait ; absent, le
// fil est simplement restitué — c'est le cas des fils sans réponse du miroir,
// qui n'ont par définition produit aucun engagement.
func (s *Server) filSource(ctx context.Context, threadID int64, eng *store.Engagement) (*SourceReponse, error) {
	fil, err := s.Store.GetThread(ctx, threadID)
	if err != nil {
		return nil, err
	}
	if fil == nil {
		return nil, errIntrouvable
	}
	rep := &SourceReponse{Sujet: fil.Subject, Canal: fil.Channel}
	if eng != nil {
		rep.EngagementID = eng.ID
		rep.Objet = eng.Objet
	}

	msgs, err := s.Store.ThreadMessages(ctx, threadID)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, errIntrouvable
	}
	_, compte, _ := s.Store.GetOAuthToken(ctx, "google")
	if fil.Channel == "gmail" {
		rep.URLGmailFil = urlGmail(compte, fil.ExternalID)
	}
	for _, m := range msgs {
		ms := MessageSource{Message: m}
		if eng != nil {
			ms.EstSource = eng.SourceMessageID != nil && *eng.SourceMessageID == m.ID
		}
		if m.Channel == "gmail" {
			ms.URLGmail = urlGmail(compte, m.ExternalID)
		}
		rep.Messages = append(rep.Messages, ms)
	}
	return rep, nil
}

var errIntrouvable = errors.New("aucun message conservé pour cette conversation")

// GET /api/engagements/{id}/source — le fil complet, message d'origine signalé.
func (s *Server) engagementSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eng, err := s.Store.GetEngagement(ctx, pathID(r))
	if err != nil || eng == nil {
		httpError(w, 404, "engagement introuvable")
		return
	}
	if eng.ThreadID == nil {
		httpError(w, 404, "aucune conversation rattachée à cet engagement")
		return
	}
	rep, err := s.filSource(ctx, *eng.ThreadID, eng)
	if err != nil {
		s.repondreSource(w, err)
		return
	}
	writeJSON(w, rep)
}

// GET /api/threads/{id}/source — la conversation seule.
// Le miroir liste des fils sans réponse : ils n'ont produit aucun engagement,
// et n'étaient donc atteignables par aucune route.
func (s *Server) threadSource(w http.ResponseWriter, r *http.Request) {
	rep, err := s.filSource(r.Context(), pathID(r), nil)
	if err != nil {
		s.repondreSource(w, err)
		return
	}
	writeJSON(w, rep)
}

func (s *Server) repondreSource(w http.ResponseWriter, err error) {
	if errors.Is(err, errIntrouvable) {
		httpError(w, 404, err.Error())
		return
	}
	httpError(w, 500, err.Error())
}

// Package engine orchestre les couches 2 à 4 : extraction, graphe,
// détecteurs et livrables.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"agentops/backend/internal/channel"
	"agentops/backend/internal/llm"
	"agentops/backend/internal/store"
)

type Engine struct {
	Store   *store.Store
	LLM     *llm.Client
	Channel channel.Reader
}

// capsuleLLM assemble la capsule complète transmise au modèle : les faits
// inférés ET les intentions déclarées par le dirigeant. Sans les intentions,
// le modèle ignore ce qui compte pour lui en ce moment (CDC 9.2 : c'est
// l'intention qui détermine la priorisation).
func (e *Engine) capsuleLLM(ctx context.Context) json.RawMessage {
	c, err := e.Store.GetCapsule(ctx)
	if err != nil || c == nil {
		return json.RawMessage(`{}`)
	}
	faits := c.Facts
	if len(faits) == 0 {
		faits = json.RawMessage(`{}`)
	}
	intentions := c.Intentions
	if len(intentions) == 0 {
		intentions = json.RawMessage(`{}`)
	}
	b, err := json.Marshal(map[string]json.RawMessage{
		"faits":      faits,
		"intentions": intentions,
	})
	if err != nil {
		return faits
	}
	return b
}

// SeuilPublication : sous ce score de confiance, un engagement n'est jamais
// présenté proactivement (CDC 7.2 — premier rempart anti-churn).
func (e *Engine) SeuilPublication(ctx context.Context) float64 {
	s := e.Store.GetSetting(ctx, "seuil_publication", "0.6")
	if v, err := parseFloat(s); err == nil {
		return v
	}
	return 0.6
}

type ExtractStats struct {
	MessagesAnalyses  int `json:"messages_analyses"`
	EngagementsCrees  int `json:"engagements_crees"`
	EvenementsAjoutes int `json:"evenements_ajoutes"`
}

// RunExtraction traite les messages en attente par lots : appel au service
// LLM, création des engagements + événements, application des règles apprises.
func (e *Engine) RunExtraction(ctx context.Context, batchSize int) (ExtractStats, error) {
	var st ExtractStats
	capsule := e.capsuleLLM(ctx)
	rules, _ := e.Store.ListRules(ctx, true)
	accountEmail, _ := e.Channel.AccountEmail(ctx)

	for {
		msgs, err := e.Store.ListPendingMessages(ctx, batchSize)
		if err != nil {
			return st, err
		}
		if len(msgs) == 0 {
			break
		}
		req := llm.ExtractRequest{
			Capsule:         capsule,
			AccountEmail:    accountEmail,
			OpenEngagements: []llm.OpenEngagement{},
		}
		seenEng := map[int64]bool{}
		for _, m := range msgs {
			req.Messages = append(req.Messages, llm.ExtractMessage{
				ID:      m.ID,
				Sender:  m.Sender,
				To:      strings.Join(m.Recipients, ", "),
				Subject: m.Subject,
				Body:    m.Body,
				SentAt:  m.SentAt.Format(time.RFC3339),
			})
			open, _ := e.Store.OpenEngagementsByThread(ctx, m.ThreadID)
			for _, o := range open {
				if !seenEng[o.ID] {
					seenEng[o.ID] = true
					req.OpenEngagements = append(req.OpenEngagements, llm.OpenEngagement{ID: o.ID, Objet: o.Objet})
				}
			}
		}
		resp, err := e.LLM.Extract(ctx, req)
		if err != nil {
			return st, err
		}
		byMsg := map[int64]llm.ExtractResult{}
		for _, r := range resp.Results {
			byMsg[r.MessageID] = r
		}
		for _, m := range msgs {
			r := byMsg[m.ID]
			for _, ex := range r.Engagements {
				if created := e.createEngagement(ctx, m, ex, rules); created {
					st.EngagementsCrees++
				}
			}
			for _, up := range r.Updates {
				if e.applyUpdate(ctx, m, up) {
					st.EvenementsAjoutes++
				}
			}
			// si le marquage échoue, on sort : sinon le même lot serait
			// refetché indéfiniment (boucle infinie sous cycleMu)
			if err := e.Store.MarkMessage(ctx, m.ID, "analyzed", nil); err != nil {
				return st, fmt.Errorf("marquage message %d: %w", m.ID, err)
			}
			st.MessagesAnalyses++
		}
	}
	if st.MessagesAnalyses > 0 {
		e.Store.Audit(ctx, "agent", "extraction_cycle", st)
	}
	return st, nil
}

func (e *Engine) createEngagement(ctx context.Context, m store.Message, ex llm.ExtractedEngagement, rules []store.LearnedRule) bool {
	// règles apprises : ignorer cet interlocuteur → pas d'engagement
	for _, r := range rules {
		if r.Action == "ignorer" && r.PorteeType == "interlocuteur" &&
			(strings.EqualFold(r.PorteeCible, ex.EmetteurEmail) || strings.EqualFold(r.PorteeCible, ex.DestinataireEmail)) {
			return false
		}
	}
	eng := store.Engagement{
		Objet:           strings.TrimSpace(ex.Objet),
		Type:            normalizeType(ex.Type),
		Statut:          "ouvert",
		Confiance:       clamp01(ex.Confiance),
		Priorite:        "normale",
		SourceMessageID: &m.ID,
		ThreadID:        &m.ThreadID,
		CreeLe:          m.SentAt, // l'engagement date du message, pas de l'analyse
	}
	if eng.Objet == "" {
		return false
	}
	if ex.EmetteurEmail != "" {
		if id, err := e.Store.UpsertPerson(ctx, strings.ToLower(ex.EmetteurEmail), ""); err == nil {
			eng.EmetteurID = &id
		}
	}
	if ex.DestinataireEmail != "" {
		if id, err := e.Store.UpsertPerson(ctx, strings.ToLower(ex.DestinataireEmail), ""); err == nil {
			eng.DestinataireID = &id
		}
	}
	if ex.Echeance != "" {
		if t, err := time.Parse("2006-01-02", ex.Echeance); err == nil {
			eng.Echeance = &t
			eng.EcheanceInferee = ex.EcheanceInferee
			// une échéance explicite est considérée confirmée ; une échéance
			// inférée doit être confirmée par le dirigeant (CDC 7.3)
			eng.EcheanceConfirmee = !ex.EcheanceInferee
		}
	}
	// priorité haute si l'interlocuteur est marqué sensible ou règle apprise
	for _, r := range rules {
		if r.Action == "priorite_haute" && r.PorteeType == "interlocuteur" &&
			(strings.EqualFold(r.PorteeCible, ex.EmetteurEmail) || strings.EqualFold(r.PorteeCible, ex.DestinataireEmail)) {
			eng.Priorite = "haute"
		}
	}
	id, err := e.Store.CreateEngagement(ctx, eng)
	if err != nil {
		log.Printf("extraction: création engagement: %v", err)
		return false
	}
	if id == 0 {
		return false // doublon : déjà extrait de ce message
	}
	_ = e.Store.AddEvent(ctx, id, "cree", &m.ID, map[string]any{"confiance": eng.Confiance})
	return true
}

func (e *Engine) applyUpdate(ctx context.Context, m store.Message, up llm.EngagementUpdate) bool {
	eng, err := e.Store.GetEngagement(ctx, up.EngagementID)
	if err != nil || eng == nil {
		return false
	}
	switch up.Type {
	case "signal_progression":
		_ = e.Store.AddEvent(ctx, eng.ID, "signal_progression", &m.ID, nil)
	case "livre":
		_ = e.Store.UpdateEngagement(ctx, eng.ID, map[string]any{"statut": "livre"})
		_ = e.Store.AddEvent(ctx, eng.ID, "livre", &m.ID, nil)
	case "relance":
		_ = e.Store.AddEvent(ctx, eng.ID, "relance", &m.ID, nil)
	case "abandonne":
		_ = e.Store.UpdateEngagement(ctx, eng.ID, map[string]any{"statut": "abandonne"})
		_ = e.Store.AddEvent(ctx, eng.ID, "abandonne", &m.ID, nil)
	case "confirme":
		if eng.Statut == "ouvert" {
			_ = e.Store.UpdateEngagement(ctx, eng.ID, map[string]any{"statut": "confirme"})
		}
		_ = e.Store.AddEvent(ctx, eng.ID, "confirme", &m.ID, nil)
	default:
		return false
	}
	return true
}

// normalizeType borne le type d'engagement au vocabulaire connu du produit.
func normalizeType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "devis", "livraison", "relance", "prise_de_contact", "rendez_vous", "facturation":
		return strings.ToLower(strings.TrimSpace(t))
	default:
		return "autre"
	}
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

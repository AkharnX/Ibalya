package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// L'onboarding lit trente jours de messagerie : plusieurs minutes sur une vraie
// boîte. Sans retour visible, le dirigeant ne sait pas si quelque chose se
// passe. On expose donc l'avancement réel plutôt qu'une animation.

const cleEtatOnboarding = "onboarding_etat"

type EtatOnboarding struct {
	Phase     string     `json:"phase"` // lecture / analyse / miroir / capsule / termine / erreur
	DemarreLe time.Time  `json:"demarre_le"`
	TermineLe *time.Time `json:"termine_le,omitempty"`
	Erreur    string     `json:"erreur,omitempty"`
	// compteurs relus en base à chaque appel : ils avancent pendant le travail
	MessagesLus      int `json:"messages_lus"`
	MessagesFiltres  int `json:"messages_filtres"`
	MessagesAnalyses int `json:"messages_analyses"`
	Engagements      int `json:"engagements"`
}

var libellesPhase = map[string]string{
	"lecture": "Lecture de votre messagerie",
	"analyse": "Analyse des messages retenus",
	"miroir":  "Composition du miroir d'activité",
	"capsule": "Compréhension de votre activité",
	"termine": "Terminé",
	"erreur":  "Interrompu",
}

func (s *Server) marquerPhase(ctx context.Context, phase, erreur string) {
	etat := EtatOnboarding{Phase: phase, DemarreLe: time.Now(), Erreur: erreur}
	// conserve l'heure de départ si l'onboarding est déjà en cours
	if brut := s.Store.GetSetting(ctx, cleEtatOnboarding, ""); brut != "" {
		var precedent EtatOnboarding
		if json.Unmarshal([]byte(brut), &precedent) == nil && !precedent.DemarreLe.IsZero() {
			etat.DemarreLe = precedent.DemarreLe
		}
	}
	if phase == "termine" || phase == "erreur" {
		maintenant := time.Now()
		etat.TermineLe = &maintenant
	}
	b, _ := json.Marshal(etat)
	_ = s.Store.SetSetting(ctx, cleEtatOnboarding, string(b))
}

// GET /api/onboarding/status
func (s *Server) onboardingStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var etat EtatOnboarding
	if brut := s.Store.GetSetting(ctx, cleEtatOnboarding, ""); brut != "" {
		_ = json.Unmarshal([]byte(brut), &etat)
	}
	if etat.Phase == "" {
		writeJSON(w, map[string]any{"phase": "", "en_cours": false})
		return
	}
	// compteurs lus en direct : ils progressent pendant le traitement
	q := func(sql string) int {
		var n int
		_ = s.Store.Q(ctx).QueryRow(ctx, sql).Scan(&n)
		return n
	}
	etat.MessagesLus = q(`SELECT count(*) FROM messages`)
	etat.MessagesFiltres = q(`SELECT count(*) FROM messages WHERE status='excluded'`)
	etat.MessagesAnalyses = q(`SELECT count(*) FROM messages WHERE status='analyzed'`)
	etat.Engagements = q(`SELECT count(*) FROM engagements`)

	writeJSON(w, map[string]any{
		"phase":             etat.Phase,
		"libelle":           libellesPhase[etat.Phase],
		"en_cours":          etat.TermineLe == nil,
		"demarre_le":        etat.DemarreLe,
		"termine_le":        etat.TermineLe,
		"erreur":            etat.Erreur,
		"messages_lus":      etat.MessagesLus,
		"messages_filtres":  etat.MessagesFiltres,
		"messages_analyses": etat.MessagesAnalyses,
		"engagements":       etat.Engagements,
	})
}

// POST /api/onboarding/ack — le dirigeant a vu le résultat, on masque le bandeau
func (s *Server) onboardingAck(w http.ResponseWriter, r *http.Request) {
	_ = s.Store.SetSetting(r.Context(), cleEtatOnboarding, "")
	writeJSON(w, map[string]string{"status": "ok"})
}

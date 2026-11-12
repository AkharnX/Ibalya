// Package ingest porte la couche 1 : normalisation et pré-filtrage avant LLM (EF-11).
// Le filtre n'utilise JAMAIS le LLM — règles pures, exécutées avant toute inférence.
package ingest

import (
	"strings"

	"ibalya/backend/internal/store"
)

// ExclusionReason rend le motif pour lequel un message n'est pas analysé, ou
// une chaîne vide s'il doit l'être. La classe de l'expéditeur (classe.go) passe
// en premier, puis les choix du dirigeant (fil exclu, règles apprises).
func ExclusionReason(m store.Message, threadExcluded bool, rules []store.LearnedRule) string {
	sender := strings.ToLower(m.Sender)

	// La classe de l'expéditeur (automate, publicité, hameçonnage) se décide
	// avant tout le reste : voir classe.go.
	if classe, motif := ClasserExpediteur(m); classe != ClasseHumain {
		return motif
	}
	at := strings.LastIndex(sender, "@")
	if threadExcluded {
		return "fil_exclu"
	}
	// 5. Règles apprises : ignorer un interlocuteur ou un domaine
	for _, r := range rules {
		if !r.Active || r.Action != "ignorer" {
			continue
		}
		cible := strings.ToLower(r.PorteeCible)
		switch r.PorteeType {
		case "interlocuteur":
			if sender == cible {
				return "regle_apprise"
			}
		case "domaine":
			if at >= 0 && strings.HasSuffix(sender[at:], "@"+strings.TrimPrefix(cible, "@")) {
				return "regle_apprise"
			}
		}
	}
	// 6. Corps vide → rien à extraire
	if strings.TrimSpace(m.Body) == "" {
		return "corps_vide"
	}
	return ""
}

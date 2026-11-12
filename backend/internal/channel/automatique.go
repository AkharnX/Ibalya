package channel

import "strings"

// EstEnvoiAutomatique lit les en-têtes qui trahissent un envoi par automate :
// Auto-Submitted (RFC 3834, toute valeur sauf « no »), Precedence bulk/list/
// junk, X-Auto-Response-Suppress (Exchange) et Feedback-ID (plateformes
// d'envoi en masse). `entete` rend la valeur d'un en-tête, ou "" s'il manque ;
// la casse du nom ne compte pas selon le canal, on essaie donc telle quelle.
func EstEnvoiAutomatique(entete func(nom string) string) bool {
	if v := strings.ToLower(strings.TrimSpace(entete("Auto-Submitted"))); v != "" && v != "no" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(entete("Precedence"))) {
	case "bulk", "list", "junk":
		return true
	}
	if strings.TrimSpace(entete("X-Auto-Response-Suppress")) != "" {
		return true
	}
	if strings.TrimSpace(entete("Feedback-ID")) != "" {
		return true
	}
	return false
}

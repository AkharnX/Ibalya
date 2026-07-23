// Package ingest porte la couche 1 : normalisation et pré-filtrage avant LLM (EF-11).
// Le filtre n'utilise JAMAIS le LLM — règles pures, exécutées avant toute inférence.
package ingest

import (
	"regexp"
	"strings"

	"agentops/backend/internal/store"
)

var noReplyPattern = regexp.MustCompile(`(?i)(no-?reply|do-?not-?reply|notification|mailer-daemon|postmaster|newsletter|marketing@|info@.*\.(mailchimp|sendgrid|brevo))`)

// domaines de services tiers exclus d'office
var excludedDomains = []string{
	"amazonses.com", "sendgrid.net", "mailchimp.com", "mailgun.org", "brevo.com",
	"notifications.google.com", "docs.google.com", "calendar-notification.google.com",
	"github.com", "gitlab.com", "atlassian.com", "slack.com", "linkedin.com",
	"facebookmail.com", "twitter.com", "x.com", "stripe.com", "paypal.com",
}

// ExclusionReason retourne la raison d'exclusion d'un message, ou "" s'il doit
// passer à l'extraction. rules = règles apprises actives.
func ExclusionReason(m store.Message, threadExcluded bool, rules []store.LearnedRule) string {
	sender := strings.ToLower(m.Sender)

	// 1. Expéditeur automatique / no-reply
	if noReplyPattern.MatchString(sender) {
		return "expediteur_automatique"
	}
	// 2. En-tête de désinscription (newsletter)
	if m.ListUnsubscribe {
		return "newsletter"
	}
	// 3. Domaines de notification tiers
	at := strings.LastIndex(sender, "@")
	if at >= 0 {
		domain := sender[at+1:]
		for _, d := range excludedDomains {
			if domain == d || strings.HasSuffix(domain, "."+d) {
				return "domaine_exclu"
			}
		}
	}
	// 4. Fil exclu par le dirigeant (règle apprise)
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

package ingest

import (
	"testing"

	"ibalya/backend/internal/store"
)

// Le pré-filtre est le garde-fou économique (EF-11) : chaque message qu'il
// laisse passer coûte une inférence. Un faux négatif coûte de l'argent, un
// faux positif fait rater un engagement.
func TestExclusionReason(t *testing.T) {
	msg := func(sender, body string, unsub bool) store.Message {
		return store.Message{Sender: sender, Body: body, ListUnsubscribe: unsub}
	}

	cas := []struct {
		nom      string
		message  store.Message
		filExclu bool
		regles   []store.LearnedRule
		attendu  string
	}{
		{"message ordinaire", msg("client@exemple.fr", "Bonjour, où en est le devis ?", false), false, nil, ""},
		{"expéditeur no-reply", msg("no-reply@service.fr", "Votre commande", false), false, nil, "expediteur_automatique"},
		{"variante noreply sans tiret", msg("noreply@service.fr", "Votre commande", false), false, nil, "expediteur_automatique"},
		{"en-tête de désinscription", msg("contact@boutique.fr", "Nos promotions", true), false, nil, "newsletter"},
		{"domaine de notification tiers", msg("notify@github.com", "PR ouverte", false), false, nil, "domaine_exclu"},
		{"sous-domaine d'un domaine exclu", msg("bounce@em.sendgrid.net", "Alerte", false), false, nil, "domaine_exclu"},
		{"fil exclu par le dirigeant", msg("client@exemple.fr", "Bonjour", false), true, nil, "fil_exclu"},
		{"corps vide", msg("client@exemple.fr", "   \n  ", false), false, nil, "corps_vide"},
		{
			"règle apprise sur un interlocuteur",
			msg("spam@exemple.fr", "Bonjour", false), false,
			[]store.LearnedRule{{Active: true, Action: "ignorer", PorteeType: "interlocuteur", PorteeCible: "spam@exemple.fr"}},
			"regle_apprise",
		},
		{
			"règle inactive : sans effet",
			msg("spam@exemple.fr", "Bonjour", false), false,
			[]store.LearnedRule{{Active: false, Action: "ignorer", PorteeType: "interlocuteur", PorteeCible: "spam@exemple.fr"}},
			"",
		},
		{
			"règle d'un autre type : sans effet sur le filtre",
			msg("client@exemple.fr", "Bonjour", false), false,
			[]store.LearnedRule{{Active: true, Action: "priorite_haute", PorteeType: "interlocuteur", PorteeCible: "client@exemple.fr"}},
			"",
		},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := ExclusionReason(c.message, c.filExclu, c.regles); got != c.attendu {
				t.Errorf("attendu %q, obtenu %q", c.attendu, got)
			}
		})
	}
}

// La casse de l'adresse ne doit jamais faire échapper un expéditeur au filtre.
func TestExclusionInsensibleALaCasse(t *testing.T) {
	m := store.Message{Sender: "NO-REPLY@Service.FR", Body: "Bonjour"}
	if got := ExclusionReason(m, false, nil); got != "expediteur_automatique" {
		t.Errorf("une adresse en majuscules doit être filtrée, obtenu %q", got)
	}
}

// Le motif ne reconnaissait pas le tiret bas : « no_reply@ », la forme d'Apple,
// passait le pré-filtre. Quatre reçus ont ainsi atteint le modèle et produit
// deux engagements sur des renouvellements d'abonnement.
func TestExpediteursAutomatiquesToutesFormes(t *testing.T) {
	automatiques := []string{
		"no_reply@email.apple.com",
		"noreply@service.fr",
		"no-reply@service.fr",
		"no.reply@service.fr",
		"donotreply@service.fr",
		"do-not-reply@service.fr",
		"ne-pas-repondre@service.fr",
		"nepasrepondre@service.fr",
		"mailer-daemon@service.fr",
		"postmaster@service.fr",
	}
	for _, a := range automatiques {
		m := store.Message{Sender: a}
		if r := ExclusionReason(m, false, nil); r != "expediteur_automatique" {
			t.Errorf("%s aurait dû être écarté, obtenu %q", a, r)
		}
	}
	// Une adresse humaine qui contient « reply » ne doit pas être écartée.
	for _, a := range []string{"marie.replyat@client.fr", "paul@replay-studio.fr"} {
		if r := ExclusionReason(store.Message{Sender: a}, false, nil); r == "expediteur_automatique" {
			t.Errorf("%s est une adresse humaine, elle ne doit pas être écartée", a)
		}
	}
}

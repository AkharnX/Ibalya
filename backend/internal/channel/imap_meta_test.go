package channel

import (
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
)

// Vérifie la conversion enveloppe -> message SANS corps (mode métadonnées,
// cas de Stewe / bsaccess). Le corps doit rester vide, l'expéditeur et les
// destinataires (To + Cc) bien remplis, le sens sortant/entrant correct.
func TestDepuisEnveloppe(t *testing.T) {
	i := &IMAP{cfg: IMAPConfig{Utilisateur: "moi@bsaccess.fr"}}
	env := &imap.Envelope{
		Date:      time.Date(2026, 9, 10, 8, 30, 0, 0, time.UTC),
		Subject:   "Devis chantier",
		MessageID: "<abc@bsaccess.fr>",
		From:      []imap.Address{{Name: "Durand", Mailbox: "durand", Host: "immo.fr"}},
		To:        []imap.Address{{Mailbox: "moi", Host: "bsaccess.fr"}},
		Cc:        []imap.Address{{Mailbox: "compta", Host: "immo.fr"}},
	}
	m := i.depuisEnveloppe(env, time.Now())

	if m.Body != "" {
		t.Fatalf("le corps doit être vide en mode métadonnées, obtenu %q", m.Body)
	}
	if m.Sender != "durand@immo.fr" {
		t.Errorf("expéditeur: %q", m.Sender)
	}
	if len(m.Recipients) != 2 || m.Recipients[0] != "moi@bsaccess.fr" || m.Recipients[1] != "compta@immo.fr" {
		t.Errorf("destinataires (To+Cc) mal reconstruits: %v", m.Recipients)
	}
	if m.Subject != "Devis chantier" {
		t.Errorf("objet: %q", m.Subject)
	}
	if m.Outbound {
		t.Errorf("message entrant (de Durand) marqué sortant")
	}
	if !m.SentAt.Equal(env.Date) {
		t.Errorf("date: %v, attendu %v", m.SentAt, env.Date)
	}

	// Un message envoyé par le dirigeant lui-même est sortant.
	env.From = []imap.Address{{Mailbox: "moi", Host: "bsaccess.fr"}}
	if !i.depuisEnveloppe(env, time.Now()).Outbound {
		t.Errorf("message du dirigeant doit être sortant")
	}
}

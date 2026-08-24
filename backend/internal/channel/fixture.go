package channel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Fixture est un connecteur de démonstration : il lit des messages depuis un
// fichier JSON. Il permet de valider toute la chaîne (pré-filtre, extraction,
// détecteurs, livrables) sans identifiants Gmail.
type Fixture struct {
	Path string
	sent []map[string]string
}

type fixtureFile struct {
	AccountEmail string `json:"account_email"`
	Messages     []struct {
		ExternalID string   `json:"external_id"`
		ThreadID   string   `json:"thread_id"`
		Subject    string   `json:"subject"`
		Sender     string   `json:"sender"`
		SenderName string   `json:"sender_name"`
		Recipients []string `json:"recipients"`
		SentAt     string   `json:"sent_at"` // RFC3339, ou "-Nd" = il y a N jours
		Body       string   `json:"body"`
		Unsub      bool     `json:"list_unsubscribe"`
	} `json:"messages"`
}

func NewFixture(path string) *Fixture { return &Fixture{Path: path} }

func (f *Fixture) Name() string { return "fixture" }

func (f *Fixture) load() (*fixtureFile, error) {
	b, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, err
	}
	var ff fixtureFile
	if err := json.Unmarshal(b, &ff); err != nil {
		return nil, fmt.Errorf("fixture invalide: %w", err)
	}
	return &ff, nil
}

func (f *Fixture) AccountEmail(ctx context.Context) (string, error) {
	ff, err := f.load()
	if err != nil {
		return "", err
	}
	return ff.AccountEmail, nil
}

func parseFixtureDate(s string) time.Time {
	if len(s) > 2 && s[0] == '-' && s[len(s)-1] == 'd' {
		var days int
		fmt.Sscanf(s, "-%dd", &days)
		return time.Now().AddDate(0, 0, -days)
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Now()
}

func (f *Fixture) FetchSince(ctx context.Context, since time.Time, max int) ([]Message, error) {
	ff, err := f.load()
	if err != nil {
		return nil, err
	}
	var out []Message
	for _, m := range ff.Messages {
		sentAt := parseFixtureDate(m.SentAt)
		if sentAt.Before(since) {
			continue
		}
		out = append(out, Message{
			ExternalID:       m.ExternalID,
			ThreadExternalID: m.ThreadID,
			Subject:          m.Subject,
			Sender:           m.Sender,
			SenderName:       m.SenderName,
			Recipients:       m.Recipients,
			SentAt:           sentAt,
			Body:             m.Body,
			Outbound:         m.Sender == ff.AccountEmail,
			ListUnsubscribe:  m.Unsub,
		})
		if len(out) >= max {
			break
		}
	}
	return out, nil
}

func (f *Fixture) Send(ctx context.Context, to, subject, body string) error {
	f.sent = append(f.sent, map[string]string{"to": to, "subject": subject, "body": body})
	fmt.Printf("[fixture] message envoyé à %s : %s\n", to, subject)
	return nil
}

// LienWeb : les fixtures ne renvoient vers aucune interface.
func (f *Fixture) LienWeb(compte, externalID string) string { return "" }

// SendFrom : le connecteur de démonstration n'envoie rien, l'expéditeur n'a
// donc pas d'effet. Présent pour satisfaire l'interface de canal (EF-10).
func (f *Fixture) SendFrom(ctx context.Context, from, fromNom, to, subject, body string) error {
	return f.Send(ctx, to, subject, body)
}

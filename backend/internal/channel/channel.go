// Package channel définit l'interface de lecture de canal commune (EF-10).
// Tout connecteur (Gmail au MVP, WhatsApp en fast-follow) l'implémente :
// coder en dur un fournisseur est un défaut bloquant d'après le CDC.
package channel

import (
	"context"
	"time"
)

// Message est la forme normalisée (CDC 6.3) : seule cette forme est vue par les couches suivantes.
type Message struct {
	ExternalID       string
	ThreadExternalID string
	Subject          string
	Sender           string // adresse email
	SenderName       string
	Recipients       []string
	SentAt           time.Time
	Body             string
	Outbound         bool // envoyé par le compte connecté
	ListUnsubscribe  bool // en-tête de désinscription présent (pré-filtre EF-11)
}

// Reader est l'interface de lecture/action d'un canal.
type Reader interface {
	Name() string
	// AccountEmail retourne l'adresse du compte connecté.
	AccountEmail(ctx context.Context) (string, error)
	// FetchSince récupère les messages depuis une date (fenêtre d'historique).
	FetchSince(ctx context.Context, since time.Time, max int) ([]Message, error)
	// Send envoie un message sortant (marche 3 — uniquement après validation explicite).
	Send(ctx context.Context, to, subject, body string) error
}

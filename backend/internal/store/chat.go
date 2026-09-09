package store

import (
	"context"
	"encoding/json"
)

// Historique de l'assistant conversationnel. Chaque tour est une ligne
// cloisonnée par tenant (RLS) : un utilisateur ne relit que ses conversations.

type TourChat struct {
	Role    string   `json:"role"` // "user" | "assistant"
	Content string   `json:"content"`
	Sources []string `json:"sources"`
}

// AjouterTourChat enregistre un tour de conversation pour le tenant courant.
func (s *Store) AjouterTourChat(ctx context.Context, role, content string, sources []string) error {
	if sources == nil {
		sources = []string{}
	}
	b, err := json.Marshal(sources)
	if err != nil {
		return err
	}
	_, err = s.q(ctx).Exec(ctx,
		`INSERT INTO chat_messages (role, content, sources) VALUES ($1,$2,$3)`,
		role, content, b)
	return err
}

// HistoriqueChat renvoie les derniers tours du tenant, du plus ancien au plus
// récent (ordre de lecture naturel). La limite borne le volume relu.
func (s *Store) HistoriqueChat(ctx context.Context, limit int) ([]TourChat, error) {
	out := []TourChat{}
	rows, err := s.q(ctx).Query(ctx, `
		SELECT role, content, sources FROM (
			SELECT role, content, sources, created_at, id
			  FROM chat_messages
			 ORDER BY created_at DESC, id DESC
			 LIMIT $1
		) t ORDER BY created_at ASC, id ASC`, limit)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var t TourChat
		var brut []byte
		if err := rows.Scan(&t.Role, &t.Content, &brut); err != nil {
			return out, err
		}
		if len(brut) > 0 {
			_ = json.Unmarshal(brut, &t.Sources)
		}
		if t.Sources == nil {
			t.Sources = []string{}
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// EffacerHistoriqueChat vide l'historique du tenant courant (« nouvelle
// conversation »).
func (s *Store) EffacerHistoriqueChat(ctx context.Context) error {
	_, err := s.q(ctx).Exec(ctx, `DELETE FROM chat_messages`)
	return err
}

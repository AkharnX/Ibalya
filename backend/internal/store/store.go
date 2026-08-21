package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connexion base: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping base: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		return nil, fmt.Errorf("migration schéma: %w", err)
	}
	return &Store{Pool: pool}, nil
}

// --- Settings ---

func (s *Store) GetSetting(ctx context.Context, key, def string) string {
	var v string
	err := s.Pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO settings (key, value) VALUES ($1,$2)
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, key, value)
	return err
}

// --- OAuth tokens ---

func (s *Store) SaveOAuthToken(ctx context.Context, provider string, token []byte, email string) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO oauth_tokens (provider, token, account_email, updated_at)
		VALUES ($1,$2,$3,now())
		ON CONFLICT (provider) DO UPDATE SET token=EXCLUDED.token, account_email=EXCLUDED.account_email, updated_at=now()`,
		provider, token, email)
	return err
}

func (s *Store) GetOAuthToken(ctx context.Context, provider string) ([]byte, string, error) {
	var token []byte
	var email string
	err := s.Pool.QueryRow(ctx, `SELECT token, account_email FROM oauth_tokens WHERE provider=$1`, provider).
		Scan(&token, &email)
	if err == pgx.ErrNoRows {
		return nil, "", nil
	}
	return token, email, err
}

// --- Audit ---

func (s *Store) Audit(ctx context.Context, actor, eventType string, payload any) {
	b, _ := json.Marshal(payload)
	if b == nil {
		b = []byte(`{}`)
	}
	_, _ = s.Pool.Exec(ctx, `INSERT INTO audit_log (actor, event_type, payload) VALUES ($1,$2,$3)`,
		actor, eventType, b)
}

func (s *Store) ListAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, ts, actor, event_type, payload FROM audit_log ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Ts, &e.Actor, &e.EventType, &e.Payload); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpdateOAuthTokenOnly met à jour le jeton SANS toucher à l'adresse du compte.
// À utiliser lors d'un rafraîchissement : SaveOAuthToken écraserait l'email.
func (s *Store) UpdateOAuthTokenOnly(ctx context.Context, provider string, token []byte) error {
	_, err := s.Pool.Exec(ctx, `UPDATE oauth_tokens SET token=$2, updated_at=now() WHERE provider=$1`,
		provider, token)
	return err
}

// SetOAuthAccountEmail enregistre l'adresse du compte connecté.
func (s *Store) SetOAuthAccountEmail(ctx context.Context, provider, email string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE oauth_tokens SET account_email=$2 WHERE provider=$1`,
		provider, email)
	return err
}

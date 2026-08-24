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
	// Coffre chiffre les jetons OAuth au repos. Nil tant qu'aucune clé n'est
	// configurée : les jetons restent alors lisibles, ce que Verifier interdit
	// sur une installation publique.
	Coffre *Coffre
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

// Les jetons OAuth donnent accès à toute la correspondance : une sauvegarde
// qui fuite ne doit pas les livrer. Ils sont chiffrés au repos avec la clé de
// l'environnement, qui n'est donc pas dans la même sauvegarde qu'eux.
func (s *Store) chiffrerJeton(token []byte) ([]byte, error) {
	if s.Coffre == nil {
		return token, nil
	}
	c, err := s.Coffre.Chiffrer(string(token))
	if err != nil {
		return nil, err
	}
	// Enveloppé en JSON : la colonne est de type jsonb.
	return json.Marshal(map[string]string{"chiffre": c})
}

// dechiffrerJeton accepte aussi un jeton en clair : les installations
// antérieures au chiffrement en contiennent, et il serait absurde de perdre un
// raccordement fonctionnel au moment de la mise à jour.
func (s *Store) dechiffrerJeton(brut []byte) ([]byte, error) {
	if len(brut) == 0 {
		return brut, nil
	}
	var enveloppe struct {
		Chiffre string `json:"chiffre"`
	}
	if err := json.Unmarshal(brut, &enveloppe); err != nil || enveloppe.Chiffre == "" {
		return brut, nil // ancien format, en clair
	}
	if s.Coffre == nil {
		return nil, fmt.Errorf("jeton chiffré mais aucune clé de chiffrement configurée")
	}
	clair, err := s.Coffre.Dechiffrer(enveloppe.Chiffre)
	if err != nil {
		return nil, err
	}
	return []byte(clair), nil
}

func (s *Store) SaveOAuthToken(ctx context.Context, provider string, token []byte, email string) error {
	token, err := s.chiffrerJeton(token)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO oauth_tokens (provider, token, account_email, updated_at)
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
	if err != nil {
		return nil, "", err
	}
	clair, err := s.dechiffrerJeton(token)
	return clair, email, err
}

// ChiffrerJetonsExistants rechiffre les jetons restés en clair.
//
// Sans cette reprise, un jeton n'est chiffré qu'au premier rafraîchissement :
// celui d'un canal inactif resterait lisible indéfiniment dans la base et dans
// toutes les sauvegardes.
func (s *Store) ChiffrerJetonsExistants(ctx context.Context) (int, error) {
	if s.Coffre == nil {
		return 0, nil
	}
	rows, err := s.Pool.Query(ctx, `SELECT provider, token FROM oauth_tokens`)
	if err != nil {
		return 0, err
	}
	type entree struct {
		provider string
		token    []byte
	}
	var aReprendre []entree
	for rows.Next() {
		var e entree
		if err := rows.Scan(&e.provider, &e.token); err != nil {
			rows.Close()
			return 0, err
		}
		var enveloppe struct {
			Chiffre string `json:"chiffre"`
		}
		// Déjà chiffré : rien à faire.
		if err := json.Unmarshal(e.token, &enveloppe); err == nil && enveloppe.Chiffre != "" {
			continue
		}
		aReprendre = append(aReprendre, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	n := 0
	for _, e := range aReprendre {
		if err := s.UpdateOAuthTokenOnly(ctx, e.provider, e.token); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
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
	token, err := s.chiffrerJeton(token)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `UPDATE oauth_tokens SET token=$2, updated_at=now() WHERE provider=$1`,
		provider, token)
	return err
}

// SetOAuthAccountEmail enregistre l'adresse du compte connecté.
func (s *Store) SetOAuthAccountEmail(ctx context.Context, provider, email string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE oauth_tokens SET account_email=$2 WHERE provider=$1`,
		provider, email)
	return err
}

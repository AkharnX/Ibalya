package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// DureeSession : au-delà, il faut se reconnecter.
const DureeSession = 30 * 24 * time.Hour

type User struct {
	ID                int64      `json:"id"`
	Email             string     `json:"email"`
	Nom               string     `json:"nom"`
	Actif             bool       `json:"actif"`
	CreeLe            time.Time  `json:"cree_le"`
	DerniereConnexion *time.Time `json:"derniere_connexion"`
}

// hashSession : le jeton de session n'est jamais stocké en clair. Une fuite de
// la base ne permet donc pas d'usurper une session en cours.
func hashSession(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// CreateUser crée un compte. Le mot de passe est haché avec bcrypt.
func (s *Store) CreateUser(ctx context.Context, email, nom, motDePasse string) (int64, error) {
	if len(motDePasse) < 10 {
		return 0, fmt.Errorf("le mot de passe doit faire au moins 10 caractères")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(motDePasse), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.Pool.QueryRow(ctx, `INSERT INTO users (email, nom, password_hash)
		VALUES ($1,$2,$3) RETURNING id`, normEmail(email), strings.TrimSpace(nom), string(hash)).Scan(&id)
	return id, err
}

func (s *Store) SetPassword(ctx context.Context, userID int64, motDePasse string) error {
	if len(motDePasse) < 10 {
		return fmt.Errorf("le mot de passe doit faire au moins 10 caractères")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(motDePasse), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, userID, string(hash))
	return err
}

// Authenticate vérifie le couple email/mot de passe en temps constant vis-à-vis
// de l'existence du compte : un email inconnu coûte le même temps qu'un mot de
// passe faux, ce qui évite d'énumérer les comptes.
func (s *Store) Authenticate(ctx context.Context, email, motDePasse string) (*User, error) {
	var u User
	var hash string
	err := s.Pool.QueryRow(ctx, `SELECT id, email, nom, actif, cree_le, derniere_connexion, password_hash
		FROM users WHERE email=$1`, normEmail(email)).
		Scan(&u.ID, &u.Email, &u.Nom, &u.Actif, &u.CreeLe, &u.DerniereConnexion, &hash)
	if err == pgx.ErrNoRows {
		// hachage factice pour égaliser le temps de réponse
		bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinvalidinv"), []byte(motDePasse))
		return nil, fmt.Errorf("identifiants incorrects")
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(motDePasse)) != nil {
		return nil, fmt.Errorf("identifiants incorrects")
	}
	if !u.Actif {
		return nil, fmt.Errorf("ce compte est désactivé")
	}
	return &u, nil
}

// CreateSession retourne le jeton EN CLAIR (à poser en cookie) ; la base n'en
// garde que le haché.
func (s *Store) CreateSession(ctx context.Context, userID int64) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	expire := time.Now().Add(DureeSession)
	if _, err := s.Pool.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expire_le)
		VALUES ($1,$2,$3)`, hashSession(token), userID, expire); err != nil {
		return "", time.Time{}, err
	}
	_, _ = s.Pool.Exec(ctx, `UPDATE users SET derniere_connexion=now() WHERE id=$1`, userID)
	// purge opportuniste des sessions expirées
	_, _ = s.Pool.Exec(ctx, `DELETE FROM sessions WHERE expire_le < now()`)
	return token, expire, nil
}

func (s *Store) UserBySession(ctx context.Context, token string) (*User, error) {
	var u User
	err := s.Pool.QueryRow(ctx, `SELECT u.id, u.email, u.nom, u.actif, u.cree_le, u.derniere_connexion
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash=$1 AND s.expire_le > now() AND u.actif`, hashSession(token)).
		Scan(&u.ID, &u.Email, &u.Nom, &u.Actif, &u.CreeLe, &u.DerniereConnexion)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, hashSession(token))
	return err
}

// DeleteSessionsOf ferme toutes les sessions d'un compte. Appelé au changement
// de mot de passe : sans cela, une session ouverte par un tiers avec l'ancien
// mot de passe survit précisément à la mesure censée la révoquer.
func (s *Store) DeleteSessionsOf(ctx context.Context, userID int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID)
	return err
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE actif`).Scan(&n)
	return n, err
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, email, nom, actif, cree_le, derniere_connexion
		FROM users ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Nom, &u.Actif, &u.CreeLe, &u.DerniereConnexion); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UserByEmail sert à la connexion Google : Google prouve l'identité, mais
// l'autorisation vient de notre propre table.
func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := s.Pool.QueryRow(ctx, `SELECT id, email, nom, actif, cree_le, derniere_connexion
		FROM users WHERE email=$1`, normEmail(email)).
		Scan(&u.ID, &u.Email, &u.Nom, &u.Actif, &u.CreeLe, &u.DerniereConnexion)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

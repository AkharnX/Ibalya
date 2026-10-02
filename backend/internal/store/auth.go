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

// User est un ESPACE de données (une boîte). Son id sert d'app.user_id partout :
// c'est l'unité de cloisonnement RLS. Un espace appartient à un compte (login).
type User struct {
	ID                int64      `json:"id"`
	Email             string     `json:"email"`
	Nom               string     `json:"nom"`
	Libelle           string     `json:"libelle"`
	Actif             bool       `json:"actif"`
	CreeLe            time.Time  `json:"cree_le"`
	DerniereConnexion *time.Time `json:"derniere_connexion"`
}

// Compte est l'identité de CONNEXION (login). Un compte possède un ou plusieurs
// espaces (User) entre lesquels il bascule ; le mot de passe vit ici.
type Compte struct {
	ID                int64      `json:"id"`
	Email             string     `json:"email"`
	Nom               string     `json:"nom"`
	Actif             bool       `json:"actif"`
	CreeLe            time.Time  `json:"cree_le"`
	DerniereConnexion *time.Time `json:"derniere_connexion"`
}

// SessionInfo résout une session : quel compte est connecté, et quel espace il
// regarde en ce moment (l'espace actif, dont l'id devient app.user_id).
type SessionInfo struct {
	Compte *Compte
	Espace *User
}

// Espace résume une boîte pour l'écran de gestion (liste des boîtes du compte).
type Espace struct {
	ID       int64  `json:"id"`
	Libelle  string `json:"libelle"`
	Email    string `json:"email"`
	Connecte bool   `json:"connecte"`
	Actif    bool   `json:"actif"`
}

// hashSession : le jeton de session n'est jamais stocké en clair. Une fuite de
// la base ne permet donc pas d'usurper une session en cours.
func hashSession(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// CreateCompte crée une identité de connexion (login). Le mot de passe est
// haché avec bcrypt. Un compte n'a pas de données propres : il possède des
// espaces (voir CreateEspace).
func (s *Store) CreateCompte(ctx context.Context, email, nom, motDePasse string) (int64, error) {
	if len(motDePasse) < 10 {
		return 0, fmt.Errorf("le mot de passe doit faire au moins 10 caractères")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(motDePasse), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.Pool.QueryRow(ctx, `INSERT INTO comptes (email, nom, password_hash)
		VALUES ($1,$2,$3) RETURNING id`, normEmail(email), strings.TrimSpace(nom), string(hash)).Scan(&id)
	return id, err
}

// CreateEspace crée un espace (boîte) rattaché à un compte. L'email (adresse de
// la boîte) peut être vide au départ : il sera renseigné au raccordement. L'id
// retourné est le tenant (app.user_id) de cet espace.
func (s *Store) CreateEspace(ctx context.Context, compteID int64, libelle, email string) (int64, error) {
	var emailArg any // NULL tant qu'aucune boîte n'est raccordée
	if e := normEmail(email); e != "" {
		emailArg = e
	}
	libelle = strings.TrimSpace(libelle)
	if libelle == "" {
		libelle = "Ma boîte"
	}
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO users (compte_id, libelle, email)
		VALUES ($1,$2,$3) RETURNING id`, compteID, libelle, emailArg).Scan(&id)
	return id, err
}

// CreateUser crée un compte ET son premier espace, et renvoie l'id de l'espace
// (le tenant). Conservé pour le CLI de provisionnement et les tests : créer un
// « utilisateur » revient à créer un login avec une première boîte.
func (s *Store) CreateUser(ctx context.Context, email, nom, motDePasse string) (int64, error) {
	compteID, err := s.CreateCompte(ctx, email, nom, motDePasse)
	if err != nil {
		return 0, err
	}
	libelle := strings.TrimSpace(nom)
	if libelle == "" {
		libelle = normEmail(email)
	}
	return s.CreateEspace(ctx, compteID, libelle, email)
}

// SetPassword change le mot de passe d'un COMPTE (login).
func (s *Store) SetPassword(ctx context.Context, compteID int64, motDePasse string) error {
	if len(motDePasse) < 10 {
		return fmt.Errorf("le mot de passe doit faire au moins 10 caractères")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(motDePasse), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `UPDATE comptes SET password_hash=$2 WHERE id=$1`, compteID, string(hash))
	return err
}

// Authenticate vérifie le couple email/mot de passe en temps constant vis-à-vis
// de l'existence du compte : un email inconnu coûte le même temps qu'un mot de
// passe faux, ce qui évite d'énumérer les comptes.
func (s *Store) Authenticate(ctx context.Context, email, motDePasse string) (*Compte, error) {
	var c Compte
	var hash string
	err := s.Pool.QueryRow(ctx, `SELECT id, email, nom, actif, cree_le, derniere_connexion, password_hash
		FROM comptes WHERE email=$1`, normEmail(email)).
		Scan(&c.ID, &c.Email, &c.Nom, &c.Actif, &c.CreeLe, &c.DerniereConnexion, &hash)
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
	if !c.Actif {
		return nil, fmt.Errorf("ce compte est désactivé")
	}
	return &c, nil
}

// CompteByEmail sert à la connexion Google : Google prouve l'identité, mais
// l'autorisation vient de notre propre table de comptes.
func (s *Store) CompteByEmail(ctx context.Context, email string) (*Compte, error) {
	var c Compte
	err := s.Pool.QueryRow(ctx, `SELECT id, email, nom, actif, cree_le, derniere_connexion
		FROM comptes WHERE email=$1`, normEmail(email)).
		Scan(&c.ID, &c.Email, &c.Nom, &c.Actif, &c.CreeLe, &c.DerniereConnexion)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateSession retourne le jeton EN CLAIR (à poser en cookie) ; la base n'en
// garde que le haché.
func (s *Store) CreateSession(ctx context.Context, compteID, espaceActifID int64) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	expire := time.Now().Add(DureeSession)
	if _, err := s.Pool.Exec(ctx, `INSERT INTO sessions (token_hash, compte_id, user_id, expire_le)
		VALUES ($1,$2,$3,$4)`, hashSession(token), compteID, espaceActifID, expire); err != nil {
		return "", time.Time{}, err
	}
	_, _ = s.Pool.Exec(ctx, `UPDATE comptes SET derniere_connexion=now() WHERE id=$1`, compteID)
	// purge opportuniste des sessions expirées
	_, _ = s.Pool.Exec(ctx, `DELETE FROM sessions WHERE expire_le < now()`)
	return token, expire, nil
}

// UserBySession résout le compte connecté ET l'espace actif de la session. C'est
// l'id de l'espace (SessionInfo.Espace.ID) qui devient app.user_id : tout le
// cloisonnement RLS découle de ce seul point.
func (s *Store) UserBySession(ctx context.Context, token string) (*SessionInfo, error) {
	var u User
	var c Compte
	err := s.Pool.QueryRow(ctx, `SELECT
			u.id, COALESCE(u.email,''), u.nom, u.libelle, u.actif, u.cree_le, u.derniere_connexion,
			c.id, c.email, c.nom, c.actif, c.cree_le, c.derniere_connexion
		FROM sessions s
		JOIN comptes c ON c.id = s.compte_id
		JOIN users   u ON u.id = s.user_id
		WHERE s.token_hash=$1 AND s.expire_le > now() AND u.actif AND c.actif`, hashSession(token)).
		Scan(&u.ID, &u.Email, &u.Nom, &u.Libelle, &u.Actif, &u.CreeLe, &u.DerniereConnexion,
			&c.ID, &c.Email, &c.Nom, &c.Actif, &c.CreeLe, &c.DerniereConnexion)
	if err != nil {
		return nil, err
	}
	return &SessionInfo{Compte: &c, Espace: &u}, nil
}

// SetEspaceActif bascule la session sur un autre espace. La sous-requête garantit
// qu'on ne bascule QUE vers un espace du même compte que la session.
func (s *Store) SetEspaceActif(ctx context.Context, token string, espaceID int64) error {
	ct, err := s.Pool.Exec(ctx, `UPDATE sessions SET user_id=$2
		WHERE token_hash=$1 AND compte_id=(SELECT compte_id FROM users WHERE id=$2)`,
		hashSession(token), espaceID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("espace introuvable pour cette session")
	}
	return nil
}

// EspaceAppartientA vérifie qu'un espace est bien rattaché à ce compte. À
// appeler AVANT toute action ciblant un espace par id (jamais faire confiance à
// l'id envoyé par le client).
func (s *Store) EspaceAppartientA(ctx context.Context, compteID, espaceID int64) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND compte_id=$2 AND actif)`,
		espaceID, compteID).Scan(&ok)
	return ok, err
}

// ListEspaces liste les boîtes d'un compte, en marquant l'espace actif de la
// session. Requête ADMIN (bypass RLS) : elle lit oauth_tokens/settings de tous
// les espaces du compte pour savoir lesquels ont une boîte raccordée.
func (s *Store) ListEspaces(ctx context.Context, compteID, espaceActifID int64) ([]Espace, error) {
	rows, err := s.admin.Query(ctx, `SELECT u.id, u.libelle, COALESCE(u.email,''),
			(EXISTS(SELECT 1 FROM oauth_tokens o WHERE o.user_id=u.id)
			 OR EXISTS(SELECT 1 FROM settings st WHERE st.user_id=u.id AND st.key='imap_hote' AND st.value<>'')) AS connecte
		FROM users u WHERE u.compte_id=$1 AND u.actif ORDER BY u.id`, compteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Espace
	for rows.Next() {
		var e Espace
		if err := rows.Scan(&e.ID, &e.Libelle, &e.Email, &e.Connecte); err != nil {
			return nil, err
		}
		e.Actif = e.ID == espaceActifID
		out = append(out, e)
	}
	return out, rows.Err()
}

// EspaceParDefaut choisit l'espace actif à l'ouverture d'une session : de
// préférence une boîte déjà raccordée, sinon le plus ancien. Requête ADMIN.
func (s *Store) EspaceParDefaut(ctx context.Context, compteID int64) (int64, error) {
	var id int64
	err := s.admin.QueryRow(ctx, `SELECT u.id FROM users u
		WHERE u.compte_id=$1 AND u.actif
		ORDER BY (EXISTS(SELECT 1 FROM oauth_tokens o WHERE o.user_id=u.id)
		 OR EXISTS(SELECT 1 FROM settings st WHERE st.user_id=u.id AND st.key='imap_hote' AND st.value<>'')) DESC,
		 u.id
		LIMIT 1`, compteID).Scan(&id)
	return id, err
}

// CompteDUnEspace renvoie le compte propriétaire d'un espace (pour rebasculer la
// session après suppression de l'espace actif).
func (s *Store) CompteDUnEspace(ctx context.Context, espaceID int64) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `SELECT compte_id FROM users WHERE id=$1`, espaceID).Scan(&id)
	return id, err
}

// SetEspaceEmail renseigne l'adresse de la boîte d'un espace (au raccordement).
// Best-effort : un conflit d'unicité (compte_id,email) ne doit pas faire échouer
// le raccordement lui-même.
func (s *Store) SetEspaceEmail(ctx context.Context, espaceID int64, email string) {
	if e := normEmail(email); e != "" {
		_, _ = s.q(ctx).Exec(ctx, `UPDATE users SET email=$2 WHERE id=$1
			AND NOT EXISTS (SELECT 1 FROM users u2 WHERE u2.compte_id=users.compte_id AND u2.email=$2 AND u2.id<>users.id)`,
			espaceID, e)
	}
}

// AdresseRaccordeeAilleurs indique qu'une AUTRE boîte du même compte est déjà
// raccordée à cette adresse. On refuse alors le raccordement : deux boîtes ne
// doivent jamais lire la même messagerie. Requête ADMIN (elle traverse les
// espaces du compte, hors RLS). Couvre OAuth (oauth_tokens.account_email) et
// IMAP (users.email, renseigné au raccordement).
func (s *Store) AdresseRaccordeeAilleurs(ctx context.Context, compteID, espaceID int64, email string) (bool, error) {
	e := normEmail(email)
	if e == "" {
		return false, nil
	}
	var ok bool
	err := s.admin.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM users u
		 WHERE u.compte_id=$1 AND u.id<>$2 AND lower(u.email)=$3
		UNION
		SELECT 1 FROM oauth_tokens o JOIN users u ON u.id=o.user_id
		 WHERE u.compte_id=$1 AND u.id<>$2 AND lower(o.account_email)=$3)`,
		compteID, espaceID, e).Scan(&ok)
	return ok, err
}

// AnnulerRaccordement retire le raccordement de la boîte COURANTE (jeton OAuth +
// type de canal). Sert à défaire un raccordement refusé parce que l'adresse est
// déjà prise par une autre boîte. Opère sous RLS (boîte courante uniquement).
func (s *Store) AnnulerRaccordement(ctx context.Context) {
	q := s.q(ctx)
	_, _ = q.Exec(ctx, `DELETE FROM oauth_tokens`)
	_, _ = q.Exec(ctx, `DELETE FROM settings WHERE key='canal_type'`)
}

// ordreSuppressionEspace : ordre topologique de purge des tables de données d'un
// espace (filles d'abord), pour ne pas violer les clés étrangères internes.
var ordreSuppressionEspace = []string{
	"drafts", "detections", "engagement_events", "dependency_links",
	"engagements", "messages", "threads", "persons", "capsule",
	"learned_rules", "reports", "settings", "oauth_tokens", "chat_messages", "audit_log",
}

// SupprimerEspace efface un espace et TOUTES ses données. Opération ADMIN (bypass
// RLS pour atteindre un espace qui n'est pas le tenant courant), transactionnelle,
// avec vérification d'appartenance. Ne touche jamais la messagerie d'origine.
func (s *Store) SupprimerEspace(ctx context.Context, compteID, espaceID int64) error {
	tx, err := s.admin.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND compte_id=$2)`,
		espaceID, compteID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("espace introuvable pour ce compte")
	}
	for _, table := range ordreSuppressionEspace {
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE user_id=$1", table), espaceID); err != nil {
			return fmt.Errorf("purge %s: %w", table, err)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, espaceID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id=$1 AND compte_id=$2`, espaceID, compteID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, hashSession(token))
	return err
}

// DeleteSessionsOf ferme toutes les sessions d'un compte. Appelé au changement
// de mot de passe : sans cela, une session ouverte par un tiers avec l'ancien
// mot de passe survit précisément à la mesure censée la révoquer.
func (s *Store) DeleteSessionsOf(ctx context.Context, compteID int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM sessions WHERE compte_id=$1`, compteID)
	return err
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE actif`).Scan(&n)
	return n, err
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, COALESCE(email,''), nom, actif, cree_le, derniere_connexion
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
	err := s.Pool.QueryRow(ctx, `SELECT id, COALESCE(email,''), nom, actif, cree_le, derniere_connexion
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

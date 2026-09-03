package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// roleApp est le rôle base non privilégié sous lequel tourne l'application. Il
// n'est ni super-utilisateur ni bypassrls : c'est indispensable pour que les
// politiques d'isolation par utilisateur (RLS) s'appliquent réellement — un
// super-utilisateur les contournerait sans bruit.
const roleApp = "ibalya_app"

type Store struct {
	// Pool applicatif : rôle non privilégié, soumis à RLS. Toutes les requêtes
	// runtime passent par là. Sans tenant positionné, RLS ne renvoie rien.
	Pool *pgxpool.Pool
	// admin : rôle privilégié, uniquement pour la migration et le provisioning.
	// Jamais utilisé pour servir des données — sinon RLS serait contourné.
	admin *pgxpool.Pool
	// Coffre chiffre les jetons OAuth au repos. Nil tant qu'aucune clé n'est
	// configurée : les jetons restent alors lisibles, ce que Verifier interdit
	// sur une installation publique.
	Coffre *Coffre
}

// New ouvre la base. adminURL a les privilèges (migration, provisioning du rôle
// applicatif). appURL se connecte sous le rôle non privilégié soumis à RLS ;
// vide, on retombe sur adminURL (mode dev sans cloisonnement fort).
func New(ctx context.Context, adminURL, appURL string) (*Store, error) {
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		return nil, fmt.Errorf("connexion base (admin): %w", err)
	}
	if err := admin.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping base: %w", err)
	}
	// Le rôle applicatif doit exister AVANT la migration : le schéma lui accorde
	// des droits. On le provisionne à partir des identifiants de appURL.
	if appURL != "" {
		if err := provisionnerRole(ctx, admin, appURL); err != nil {
			return nil, fmt.Errorf("provisioning rôle applicatif: %w", err)
		}
	}
	if _, err := admin.Exec(ctx, schema); err != nil {
		return nil, fmt.Errorf("migration schéma: %w", err)
	}
	app := admin
	if appURL != "" {
		app, err = pgxpool.New(ctx, appURL)
		if err != nil {
			return nil, fmt.Errorf("connexion base (app): %w", err)
		}
		if err := app.Ping(ctx); err != nil {
			return nil, fmt.Errorf("ping base (app): %w", err)
		}
	}
	return &Store{Pool: app, admin: admin}, nil
}

// provisionnerRole crée (ou met à jour) le rôle applicatif à partir de appURL.
// Idempotent : rejoué à chaque démarrage sans effet de bord.
func provisionnerRole(ctx context.Context, admin *pgxpool.Pool, appURL string) error {
	u, err := url.Parse(appURL)
	if err != nil {
		return err
	}
	nom := u.User.Username()
	pass, _ := u.User.Password()
	if nom == "" {
		return fmt.Errorf("appURL sans utilisateur")
	}
	// Guillemets doublés : la valeur vient de notre environnement, mais on
	// échappe par principe. Identifiant et mot de passe sont injectés en
	// littéraux car CREATE/ALTER ROLE ne se paramètrent pas.
	litNom := `"` + strings.ReplaceAll(nom, `"`, `""`) + `"`
	litPass := `'` + strings.ReplaceAll(pass, `'`, `''`) + `'`
	sql := fmt.Sprintf(`DO $$ BEGIN
		IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname=%s) THEN
			CREATE ROLE %s LOGIN NOSUPERUSER NOBYPASSRLS;
		END IF;
	END $$;
	ALTER ROLE %s WITH LOGIN NOSUPERUSER NOBYPASSRLS PASSWORD %s;`,
		`'`+strings.ReplaceAll(nom, `'`, `''`)+`'`, litNom, litNom, litPass)
	_, err = admin.Exec(ctx, sql)
	return err
}

// clé de contexte portant la connexion liée au tenant courant.
type connTenant struct{}

// EnTenant exécute fn avec le tenant positionné sur une connexion dédiée : les
// politiques RLS ne laissent alors voir et écrire QUE les données de cet
// utilisateur. La variable est remise à zéro avant que la connexion retourne
// au pool, pour ne jamais fuiter le tenant au suivant.
//
// On utilise une variable de session (pas une transaction) pour ne pas garder
// une transaction ouverte pendant les appels lents au modèle de langage.
func (s *Store) EnTenant(ctx context.Context, userID int64, fn func(ctx context.Context) error) error {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT set_config('app.user_id', $1, false)",
		strconv.FormatInt(userID, 10)); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT set_config('app.user_id', '', false)")
	return fn(context.WithValue(ctx, connTenant{}, conn))
}

// Querier : dénominateur commun d'un pool, d'une connexion et d'une transaction.
type Querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// Q est la version exportée de q : elle permet aux couches moteur et API
// d'exécuter leurs requêtes brutes sur la connexion du tenant courant, donc
// sous RLS. Toute requête hors store DOIT passer par là, jamais par .Pool.
func (s *Store) Q(ctx context.Context) Querier { return s.q(ctx) }

// q renvoie l'exécuteur à utiliser : la connexion liée au tenant si on est dans
// un EnTenant, sinon le pool applicatif. Hors tenant, RLS ne renvoie rien sur
// les tables cloisonnées — c'est le comportement voulu (fermé par défaut).
func (s *Store) q(ctx context.Context) Querier {
	if c, ok := ctx.Value(connTenant{}).(*pgxpool.Conn); ok {
		return c
	}
	return s.Pool
}

// --- Settings ---

func (s *Store) GetSetting(ctx context.Context, key, def string) string {
	var v string
	err := s.q(ctx).QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&v)
	if err != nil {
		return def
	}
	return v
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.q(ctx).Exec(ctx, `INSERT INTO settings (key, value) VALUES ($1,$2)
		ON CONFLICT (user_id, key) DO UPDATE SET value=EXCLUDED.value`, key, value)
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
	// connecte_le=now() : c'est un consentement, le compte à rebours des sept
	// jours repart de zéro. UpdateOAuthTokenOnly, lui, ne touche pas ce champ.
	_, err = s.q(ctx).Exec(ctx, `INSERT INTO oauth_tokens (provider, token, account_email, updated_at, connecte_le)
		VALUES ($1,$2,$3,now(),now())
		ON CONFLICT (user_id, provider) DO UPDATE SET token=EXCLUDED.token, account_email=EXCLUDED.account_email,
		  updated_at=now(), connecte_le=now()`,
		provider, token, email)
	return err
}

func (s *Store) GetOAuthToken(ctx context.Context, provider string) ([]byte, string, error) {
	var token []byte
	var email string
	err := s.q(ctx).QueryRow(ctx, `SELECT token, account_email FROM oauth_tokens WHERE provider=$1`, provider).
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
	rows, err := s.admin.Query(ctx, `SELECT user_id, provider, token FROM oauth_tokens`)
	if err != nil {
		return 0, err
	}
	type entree struct {
		userID   int64
		provider string
		token    []byte
	}
	var aReprendre []entree
	for rows.Next() {
		var e entree
		if err := rows.Scan(&e.userID, &e.provider, &e.token); err != nil {
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
	// Réécriture en admin : cette migration de démarrage est cross-tenant et
	// n'a pas de tenant positionné.
	n := 0
	for _, e := range aReprendre {
		chiffre, err := s.chiffrerJeton(e.token)
		if err != nil {
			return n, err
		}
		if _, err := s.admin.Exec(ctx,
			`UPDATE oauth_tokens SET token=$3, updated_at=now() WHERE user_id=$1 AND provider=$2`,
			e.userID, e.provider, chiffre); err != nil {
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
	rows, err := s.q(ctx).Query(ctx,
		`SELECT id, ts, actor, event_type, payload FROM audit_log
		 WHERE user_id = nullif(current_setting('app.user_id', true),'')::bigint
		 ORDER BY id DESC LIMIT $1`, limit)
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
	_, err = s.q(ctx).Exec(ctx, `UPDATE oauth_tokens SET token=$2, updated_at=now() WHERE provider=$1`,
		provider, token)
	return err
}

// SetOAuthAccountEmail enregistre l'adresse du compte connecté.
func (s *Store) SetOAuthAccountEmail(ctx context.Context, provider, email string) error {
	_, err := s.q(ctx).Exec(ctx, `UPDATE oauth_tokens SET account_email=$2 WHERE provider=$1`,
		provider, email)
	return err
}

// EtatConnexionOAuth décrit l'ancienneté d'une connexion OAuth, pour prévenir
// avant qu'elle n'expire. provider vide ou jamais connecté → connecte==false.
func (s *Store) EtatConnexionOAuth(ctx context.Context, provider string) (connecte bool, depuisJours int) {
	var connecteLe *time.Time
	err := s.q(ctx).QueryRow(ctx,
		`SELECT connecte_le FROM oauth_tokens WHERE provider=$1`, provider).Scan(&connecteLe)
	if err != nil || connecteLe == nil {
		return false, 0
	}
	return true, int(time.Since(*connecteLe).Hours() / 24)
}

// ProprietaireParDefaut renvoie l'utilisateur titulaire de la boîte connectée.
// Sert au jeton d'administration local, qui agit comme le propriétaire, et au
// scheduler mono-boîte. Requête admin : elle traverse les tenants.
func (s *Store) ProprietaireParDefaut(ctx context.Context) (int64, error) {
	var id int64
	err := s.admin.QueryRow(ctx, `
		SELECT u.id FROM users u
		  JOIN oauth_tokens o ON lower(o.account_email) = lower(u.email)
		 WHERE o.provider='google'
		 ORDER BY o.updated_at DESC LIMIT 1`).Scan(&id)
	return id, err
}

// UtilisateursActifsAvecCanal liste les utilisateurs actifs ayant une boîte
// connectée. Le scheduler boucle dessus pour lancer un cycle par tenant.
// Requête admin : énumération cross-tenant.
func (s *Store) UtilisateursActifsAvecCanal(ctx context.Context) ([]int64, error) {
	rows, err := s.admin.Query(ctx, `
		SELECT DISTINCT u.id FROM users u
		  JOIN oauth_tokens o ON o.user_id = u.id
		 WHERE u.actif ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

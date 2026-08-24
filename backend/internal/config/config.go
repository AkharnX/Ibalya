package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL           string
	APIAddr               string
	AdminToken            string
	LLMServiceURL         string
	PublicBaseURL         string
	GoogleClientID        string
	GoogleClientSecret    string
	Channel               string // gmail | imap | fixture
	IMAPHote              string
	IMAPPort              int
	IMAPUtilisateur       string
	IMAPMotDePasse        string
	IMAPDossier           string
	SMTPHote              string
	SMTPPort              int
	IMAPTLSSansVerif      bool
	CleChiffrement        string
	MicrosoftClientID     string
	MicrosoftClientSecret string
	MicrosoftTenant       string
	// Expéditeur de service : le digest part de là, pas de la boîte du dirigeant.
	ServiceSMTPHote  string
	ServiceSMTPPort  int
	ServiceSMTPLogin string
	ServiceSMTPCle   string
	ServiceMailDe    string
	ServiceMailNom   string
	FixturePath      string
	FrontendDir      string
	LandingDir       string
	IngestInterval   int // minutes
	DetectInterval   int // minutes
	DigestHour       int
}

// env lit une variable d'environnement en retirant les espaces parasites :
// un espace en fin de ligne dans .env rendait le jeton d'accès inutilisable
// (la valeur stockée différait de celle saisie par le dirigeant).
func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// motsDePasseParDefaut liste les valeurs de remplissage publiées dans le dépôt.
// Elles conviennent à un poste de développement, jamais à une installation
// exposée : le dépôt étant public, elles sont connues de tous.
var motsDePasseParDefaut = []string{"ibalya_dev", "changez-moi", "motdepasse"}

// Verifier refuse une configuration dangereuse sur une installation publique.
//
// La base de production a tourné avec « ibalya_dev », la valeur d'exemple du
// dépôt. Un avertissement dans les journaux ne suffit pas, personne ne les lit :
// le service refuse de démarrer plutôt que de servir avec un identifiant connu.
// Le développement local n'est pas concerné, la règle ne s'applique qu'à une
// adresse publique en https.
func (c Config) Verifier() error {
	publique := strings.HasPrefix(c.PublicBaseURL, "https://")
	if !publique {
		return nil
	}
	for _, faible := range motsDePasseParDefaut {
		if strings.Contains(c.DatabaseURL, ":"+faible+"@") {
			return fmt.Errorf("la base utilise le mot de passe d'exemple %q, publié dans le dépôt : changez DB_PASSWORD et DATABASE_URL", faible)
		}
	}
	if c.IMAPTLSSansVerif {
		return fmt.Errorf("IMAP_TLS_SKIP_VERIFY accepte n'importe quel certificat : interdit sur une installation publique")
	}
	if c.AdminToken != "" && len(c.AdminToken) < 32 {
		return fmt.Errorf("ADMIN_TOKEN fait %d caractères, 32 au minimum sur une installation publique", len(c.AdminToken))
	}
	return nil
}

func Load() Config {
	return Config{
		DatabaseURL:           env("DATABASE_URL", "postgres://ibalya:ibalya_dev@127.0.0.1:5435/ibalya?sslmode=disable"),
		APIAddr:               env("API_ADDR", "127.0.0.1:9999"),
		AdminToken:            env("ADMIN_TOKEN", ""),
		LLMServiceURL:         env("LLM_SERVICE_URL", "http://127.0.0.1:8092"),
		PublicBaseURL:         env("PUBLIC_BASE_URL", "http://localhost:9999"),
		GoogleClientID:        env("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:    env("GOOGLE_CLIENT_SECRET", ""),
		Channel:               env("CHANNEL", "gmail"),
		IMAPHote:              env("IMAP_HOST", ""),
		IMAPPort:              envInt("IMAP_PORT", 993),
		IMAPUtilisateur:       env("IMAP_USER", ""),
		IMAPMotDePasse:        env("IMAP_PASSWORD", ""),
		IMAPDossier:           env("IMAP_FOLDER", "INBOX"),
		SMTPHote:              env("SMTP_HOST", ""),
		SMTPPort:              envInt("SMTP_PORT", 587),
		IMAPTLSSansVerif:      env("IMAP_TLS_SKIP_VERIFY", "") == "1",
		CleChiffrement:        env("ENCRYPTION_KEY", ""),
		MicrosoftClientID:     env("MICROSOFT_CLIENT_ID", ""),
		MicrosoftClientSecret: env("MICROSOFT_CLIENT_SECRET", ""),
		// « common » accepte comptes professionnels et personnels ; un
		// identifiant de locataire restreint à une seule organisation.
		MicrosoftTenant:  env("MICROSOFT_TENANT", "common"),
		ServiceSMTPHote:  env("SERVICE_SMTP_HOST", "smtp-relay.brevo.com"),
		ServiceSMTPPort:  envInt("SERVICE_SMTP_PORT", 587),
		ServiceSMTPLogin: env("SERVICE_SMTP_LOGIN", ""),
		ServiceSMTPCle:   env("SERVICE_SMTP_KEY", ""),
		ServiceMailDe:    env("SERVICE_MAIL_FROM", ""),
		ServiceMailNom:   env("SERVICE_MAIL_NAME", "Ibalya"),
		FixturePath:      env("FIXTURE_PATH", ""),
		FrontendDir:      env("FRONTEND_DIR", "frontend/dist"),
		LandingDir:       env("LANDING_DIR", "landing"),
		IngestInterval:   envInt("INGEST_INTERVAL_MINUTES", 15),
		DetectInterval:   envInt("DETECT_INTERVAL_MINUTES", 30),
		DigestHour:       envInt("DIGEST_HOUR", 7),
	}
}

package config

import "strings"
import "testing"

func TestVerifierRefuseLesIdentifiantsPublies(t *testing.T) {
	cas := []struct {
		nom       string
		cfg       Config
		refuse    bool
		attenduIn string
	}{
		{
			nom: "production avec le mot de passe d'exemple",
			cfg: Config{PublicBaseURL: "https://ibalya.com",
				DatabaseURL: "postgres://ibalya:ibalya_dev@127.0.0.1:5435/ibalya"},
			refuse: true, attenduIn: "ibalya_dev",
		},
		{
			nom: "développement local avec le même mot de passe",
			cfg: Config{PublicBaseURL: "http://localhost:9999",
				DatabaseURL: "postgres://ibalya:ibalya_dev@127.0.0.1:5435/ibalya"},
			refuse: false,
		},
		{
			nom: "production avec un vrai mot de passe",
			cfg: Config{PublicBaseURL: "https://ibalya.com",
				DatabaseURL: "postgres://ibalya:kQ8vT2mXp9LzR4wN@127.0.0.1:5435/ibalya",
				AdminToken:  strings.Repeat("a", 40)},
			refuse: false,
		},
		{
			nom: "production avec un jeton d'admin trop court",
			cfg: Config{PublicBaseURL: "https://ibalya.com",
				DatabaseURL: "postgres://ibalya:kQ8vT2mXp9LzR4wN@127.0.0.1:5435/ibalya",
				AdminToken:  "court"},
			refuse: true, attenduIn: "ADMIN_TOKEN",
		},
		{
			nom: "production sans jeton d'admin : autorisé, il est facultatif",
			cfg: Config{PublicBaseURL: "https://ibalya.com",
				DatabaseURL: "postgres://ibalya:kQ8vT2mXp9LzR4wN@127.0.0.1:5435/ibalya"},
			refuse: false,
		},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			err := c.cfg.Verifier()
			if c.refuse && err == nil {
				t.Fatal("cette configuration aurait dû être refusée")
			}
			if !c.refuse && err != nil {
				t.Fatalf("configuration valide refusée : %v", err)
			}
			if c.refuse && !strings.Contains(err.Error(), c.attenduIn) {
				t.Fatalf("le message doit dire quoi corriger, obtenu : %v", err)
			}
		})
	}
}

// env retire les espaces parasites : une valeur avec un espace en fin de ligne
// dans .env avait rendu le jeton d'accès inutilisable.
func TestEnvRetireLesEspaces(t *testing.T) {
	t.Setenv("IBALYA_TEST_CLE", "  valeur  ")
	if got := env("IBALYA_TEST_CLE", "defaut"); got != "valeur" {
		t.Fatalf("attendu \"valeur\", obtenu %q", got)
	}
	t.Setenv("IBALYA_TEST_CLE", "   ")
	if got := env("IBALYA_TEST_CLE", "defaut"); got != "defaut" {
		t.Fatalf("une valeur vide doit retomber sur le défaut, obtenu %q", got)
	}
}

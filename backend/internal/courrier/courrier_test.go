package courrier

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Une configuration incomplète ne doit pas faire échouer l'agent : l'appelant
// se rabat sur la boîte du dirigeant.
func TestConfigurationIncompleteRendNil(t *testing.T) {
	cas := []Config{
		{},
		{Hote: "smtp.ex.fr"},
		{Hote: "smtp.ex.fr", Cle: "k"},
		{Cle: "k", De: "a@b.fr"},
	}
	for i, c := range cas {
		if s := Nouveau(c); s != nil {
			t.Errorf("cas %d : configuration incomplète acceptée", i)
		}
	}
	if s := Nouveau(Config{Hote: "smtp.ex.fr", Cle: "k", De: "digest@ex.fr"}); s == nil {
		t.Fatal("une configuration complète doit être acceptée")
	}
}

// Les valeurs implicites évitent une configuration bavarde.
func TestValeursParDefaut(t *testing.T) {
	s := Nouveau(Config{Hote: "smtp.ex.fr", Cle: "k", De: "digest@ex.fr"})
	if s.cfg.Port != 587 {
		t.Errorf("port %d, attendu 587", s.cfg.Port)
	}
	if s.cfg.Login != "digest@ex.fr" {
		t.Errorf("sans login explicite, l'adresse fait foi : %q", s.cfg.Login)
	}
	if !strings.Contains(s.Expediteur(), "digest@ex.fr") {
		t.Errorf("expéditeur %q", s.Expediteur())
	}
}

// Un service nil ne doit jamais provoquer de panique : c'est le cas normal
// quand aucun fournisseur n'est configuré.
func TestServiceNilNePaniquePas(t *testing.T) {
	var s *Service
	if s.Configure() {
		t.Error("un service nil n'est pas configuré")
	}
	if s.Expediteur() != "" {
		t.Error("un service nil n'a pas d'expéditeur")
	}
	if err := s.Envoyer(context.Background(), "a@b.fr", "x", "y"); err == nil {
		t.Error("un service nil doit refuser d'envoyer")
	}
}

// Les accents ne passent pas bruts dans un en-tête : ils doivent être encodés.
func TestEnteteAccentueeEncodee(t *testing.T) {
	got := encoder("Échéance à risque")
	if strings.ContainsAny(got, "Éàé") {
		t.Fatalf("les accents doivent être encodés : %q", got)
	}
	if !strings.HasPrefix(got, "=?UTF-8?") {
		t.Fatalf("encodage RFC 2047 attendu : %q", got)
	}
}

// Envoi réel, ignoré sans configuration : la CI n'a pas d'identifiants.
func TestEnvoiReel(t *testing.T) {
	dest := os.Getenv("COURRIER_TEST_DEST")
	if dest == "" {
		t.Skip("COURRIER_TEST_DEST non défini")
	}
	port, _ := strconv.Atoi(os.Getenv("SERVICE_SMTP_PORT"))
	s := Nouveau(Config{
		Hote: os.Getenv("SERVICE_SMTP_HOST"), Port: port,
		Login: os.Getenv("SERVICE_SMTP_LOGIN"), Cle: os.Getenv("SERVICE_SMTP_KEY"),
		De: os.Getenv("SERVICE_MAIL_FROM"), DeNom: os.Getenv("SERVICE_MAIL_NAME"),
	})
	if s == nil {
		t.Fatal("configuration incomplète")
	}
	t.Logf("expéditeur : %s", s.Expediteur())
	if err := s.Envoyer(context.Background(), dest,
		"Ibalya — vérification de l'expéditeur de service",
		"Ceci confirme que le digest partira bien de cette adresse,\net non de votre boîte personnelle.\n\n— Ibalya\n"); err != nil {
		t.Fatalf("envoi : %v", err)
	}
	t.Logf("message envoyé à %s", dest)
}

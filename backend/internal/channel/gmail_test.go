package channel

import (
	"errors"
	"strings"
	"testing"
)

// Le lien doit ouvrir le BON compte : un dirigeant connecté à plusieurs
// sessions Google atterrirait sinon dans la mauvaise boîte.
func TestGmailLienWeb(t *testing.T) {
	g := &Gmail{}
	u := g.LienWeb("marc@exemple.fr", "18f2a")
	if !strings.Contains(u, "authuser=marc%40exemple.fr") {
		t.Errorf("le compte doit être encodé dans l'URL, obtenu %q", u)
	}
	if !strings.HasSuffix(u, "#all/18f2a") {
		t.Errorf("l'identifiant du message doit terminer l'URL, obtenu %q", u)
	}
	if g.LienWeb("marc@exemple.fr", "") != "" {
		t.Error("sans identifiant de message, aucun lien ne doit être produit")
	}
	if u := g.LienWeb("", "18f2a"); strings.Contains(u, "authuser") {
		t.Errorf("sans compte connu, pas de paramètre authuser : %q", u)
	}
}

// IMAP ne désigne aucune interface web : il ne doit pas fabriquer de faux lien.
func TestIMAPNaPasDeLienWeb(t *testing.T) {
	i := NewIMAP(IMAPConfig{Hote: "imap.exemple.fr", Utilisateur: "a@exemple.fr"})
	if l := i.LienWeb("a@exemple.fr", "18f2a"); l != "" {
		t.Errorf("attendu une chaîne vide, obtenu %q", l)
	}
}

// Les valeurs par défaut évitent au dirigeant de tout saisir : le port IMAP
// sécurisé, le dossier de réception, et un hôte SMTP déduit de l'hôte IMAP,
// convention respectée par la quasi-totalité des hébergeurs.
func TestIMAPValeursParDefaut(t *testing.T) {
	i := NewIMAP(IMAPConfig{Hote: "imap.ovh.net", Utilisateur: "a@b.fr"})
	if i.cfg.Port != 993 {
		t.Errorf("port %d, attendu 993", i.cfg.Port)
	}
	if i.cfg.Dossier != "INBOX" {
		t.Errorf("dossier %q, attendu INBOX", i.cfg.Dossier)
	}
	if i.cfg.SMTPHote != "smtp.ovh.net" {
		t.Errorf("hôte SMTP %q, attendu smtp.ovh.net", i.cfg.SMTPHote)
	}
	if i.cfg.SMTPPort != 587 {
		t.Errorf("port SMTP %d, attendu 587", i.cfg.SMTPPort)
	}
}

// Tout connecteur doit satisfaire l'interface : c'est l'exigence EF-10, et
// c'est ce qui permet d'en ajouter un sans toucher au reste.
func TestConnecteursRespectentLInterface(t *testing.T) {
	var _ Reader = (*Gmail)(nil)
	var _ Reader = (*IMAP)(nil)
	var _ Reader = (*Fixture)(nil)
}

func TestTraduireErreurJeton(t *testing.T) {
	src := errors.New(`auth: cannot fetch token: 400 Response: {"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)

	g := traduireErreurJeton("google", src)
	if !strings.Contains(g.Error(), "Gmail") || !strings.Contains(g.Error(), "reconnecté") {
		t.Fatalf("message peu clair pour Google : %v", g)
	}
	if !errors.Is(g, src) {
		t.Fatal("l'erreur d'origine doit rester consultable (technique conservée)")
	}

	o := traduireErreurJeton("microsoft", src)
	if !strings.Contains(o.Error(), "Outlook") {
		t.Fatalf("message peu clair pour Microsoft : %v", o)
	}

	// Une erreur qui n'est pas un invalid_grant passe telle quelle.
	autre := errors.New("connexion réseau interrompue")
	if traduireErreurJeton("google", autre).Error() != autre.Error() {
		t.Fatal("une erreur ordinaire ne doit pas être réécrite")
	}
}

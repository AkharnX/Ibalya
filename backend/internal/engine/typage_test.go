package engine

import "testing"

// 148 interlocuteurs tous « Autre » chez BS ACCESS : le typage doit se déduire
// du domaine et de la capsule, et ne jamais prendre une messagerie grand public
// pour le domaine de l'entreprise.
func TestTyperInterlocuteur(t *testing.T) {
	soi := map[string]bool{"dirigeant@menuiserie-dupont.fr": true, "perso@gmail.com": true}
	clients := []string{"paul.rossi@gmail.com", "client.fr"}
	fournisseurs := []string{"vitrages-pro.fr"}
	cas := []struct {
		nom, email, compte, attendu string
	}{
		{"même domaine que le compte", "marc@menuiserie-dupont.fr", "dirigeant@menuiserie-dupont.fr", "interne"},
		{"compte gmail, contact gmail : rien", "inconnu@gmail.com", "dirigeant@gmail.com", ""},
		{"autre adresse du dirigeant", "perso@gmail.com", "dirigeant@menuiserie-dupont.fr", "interne"},
		{"client par domaine", "sophie@client.fr", "dirigeant@menuiserie-dupont.fr", "client"},
		{"client par adresse", "Paul.Rossi@gmail.com", "dirigeant@menuiserie-dupont.fr", "client"},
		{"fournisseur par domaine", "commandes@vitrages-pro.fr", "dirigeant@menuiserie-dupont.fr", "fournisseur"},
		{"inconnu", "qqn@ailleurs.fr", "dirigeant@menuiserie-dupont.fr", ""},
	}
	for _, c := range cas {
		if got := TyperInterlocuteur(c.email, c.compte, soi, clients, fournisseurs); got != c.attendu {
			t.Errorf("%s : attendu %q, obtenu %q", c.nom, c.attendu, got)
		}
	}
}

func TestDomaine(t *testing.T) {
	if Domaine(" Marc@Menuiserie-Dupont.FR ") != "menuiserie-dupont.fr" || Domaine("sans-arobase") != "" {
		t.Fatal("extraction du domaine")
	}
	if !MessageriesGrandPublic["gmail.com"] || MessageriesGrandPublic["menuiserie-dupont.fr"] {
		t.Fatal("liste des messageries grand public")
	}
}

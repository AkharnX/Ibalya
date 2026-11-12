package engine

import "testing"

// Le sens d'un engagement décide qui doit quoi à qui. Un échange entre deux
// tiers où le dirigeant est en copie n'est ni une dette ni une créance : il
// ne doit produire ni retard ni relance (constat BS ACCESS, point 5).
func TestSens(t *testing.T) {
	soi := map[string]bool{"dirigeant@menuiserie-dupont.fr": true, "perso@gmail.com": true}
	cas := []struct {
		nom, emetteur, destinataire, attendu string
	}{
		{"je promets à un client", "dirigeant@menuiserie-dupont.fr", "paul.rossi@gmail.com", SensJeDois},
		{"un fournisseur me promet", "commandes@vitrages-pro.fr", "dirigeant@menuiserie-dupont.fr", SensOnMeDoit},
		{"deux tiers entre eux", "a@tiers.fr", "b@autre.fr", SensPourInfo},
		{"mon alias perso promet", "perso@gmail.com", "paul.rossi@gmail.com", SensJeDois},
		{"casse et espaces", "  Dirigeant@Menuiserie-Dupont.fr ", "x@y.fr", SensJeDois},
		{"émetteur inconnu, destinataire moi", "", "dirigeant@menuiserie-dupont.fr", SensOnMeDoit},
		{"les deux vides", "", "", SensPourInfo},
	}
	for _, c := range cas {
		if got := Sens(c.emetteur, c.destinataire, soi); got != c.attendu {
			t.Errorf("%s : attendu %s, obtenu %s", c.nom, c.attendu, got)
		}
	}
}

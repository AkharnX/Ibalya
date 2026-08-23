package api

import "testing"

func TestAutoriserSession(t *testing.T) {
	const prop = "ibkebe2002@gmail.com"
	cas := []struct {
		nom, proprietaire, email string
		attendu                  bool
	}{
		{"le titulaire du canal entre", prop, prop, true},
		{"casse différente", prop, "IbKebe2002@Gmail.COM", true},
		{"espaces parasites", prop, "  ibkebe2002@gmail.com ", true},
		{"un autre compte est refusé", prop, "kebe.yacouba06@gmail.com", false},
		{"aucun canal raccordé : on laisse entrer", "", "premier@compte.fr", true},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			ok, motif := autoriserSession(c.proprietaire, c.email)
			if ok != c.attendu {
				t.Fatalf("propriétaire %q, compte %q : attendu %v, obtenu %v", c.proprietaire, c.email, c.attendu, ok)
			}
			if !ok && motif == "" {
				t.Fatal("un refus doit être motivé, l'utilisateur voit ce message")
			}
		})
	}
}

// Le cas qui motive la garde : Yacouba a un compte, et sans elle il voit la
// boîte personnelle d'Ibrahim.
func TestUnCoequipierNObtientPasDeSession(t *testing.T) {
	ok, motif := autoriserSession("ibkebe2002@gmail.com", "kebe.yacouba06@gmail.com")
	if ok {
		t.Fatal("un compte qui n'est pas le titulaire du canal a obtenu une session")
	}
	if motif == "" {
		t.Fatal("le refus doit expliquer pourquoi")
	}
}

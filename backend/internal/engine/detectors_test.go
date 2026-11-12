package engine

import "testing"

// Une alerte par engagement, vivante d'un cycle à l'autre : la clé de
// dédoublonnage ne doit pas contenir la date, sinon une alerte neuve naît
// chaque jour pour le même sujet (constat BS ACCESS, 11 nov. 2026).
func TestCleAlerteSansDate(t *testing.T) {
	if got := cleAlerte("echeance_a_risque", 42); got != "echeance_a_risque:42" {
		t.Fatalf("clé inattendue : %q", got)
	}
	if got := cleAlerte("contradiction", 7, 9); got != "contradiction:7:9" {
		t.Fatalf("clé inattendue : %q", got)
	}
}

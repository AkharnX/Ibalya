package engine

import "testing"

// 12 dépendances proposées, 0 crédible chez BS ACCESS : on ne relie que des
// types qui peuvent se bloquer, et on ne garde que les candidats sûrs.
func TestDependanceCompatible(t *testing.T) {
	cas := []struct {
		amont, aval string
		attendu     bool
	}{
		{"livraison", "livraison", true},
		{"devis", "livraison", true},
		{"autre", "facturation", true},
		{"devis", "rendez_vous", false},
		{"rendez_vous", "livraison", false},
		{"prise_de_contact", "devis", false},
		{"livraison", "rendez_vous", true},
	}
	for _, c := range cas {
		if got := dependanceCompatible(c.amont, c.aval); got != c.attendu {
			t.Errorf("%s → %s : attendu %v", c.amont, c.aval, c.attendu)
		}
	}
}

func TestRetenirCandidats(t *testing.T) {
	in := []candidatJuge{{score: 0.95}, {score: 0.4}, {score: 0.72}, {score: 0.9}, {score: 0.71}, {score: 0.88}, {score: 0.75}}
	out := retenirCandidats(in, 0.7, 5)
	if len(out) != 5 {
		t.Fatalf("attendu 5 candidats, obtenu %d", len(out))
	}
	for i := 1; i < len(out); i++ {
		if out[i].score > out[i-1].score {
			t.Fatal("les candidats doivent être triés par score décroissant")
		}
	}
	for _, c := range out {
		if c.score < 0.7 {
			t.Fatalf("candidat sous le seuil retenu : %.2f", c.score)
		}
	}
	if len(retenirCandidats([]candidatJuge{{score: 0.3}}, 0.7, 5)) != 0 {
		t.Fatal("rien ne doit passer sous le seuil")
	}
}

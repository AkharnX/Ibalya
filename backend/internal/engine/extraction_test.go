package engine

import "testing"

// Le type d'engagement pilote l'action suggérée et la couleur affichée : une
// valeur inventée par le modèle ne doit jamais atteindre la base.
func TestNormalizeType(t *testing.T) {
	connus := []string{"devis", "livraison", "relance", "prise_de_contact", "rendez_vous", "facturation"}
	for _, v := range connus {
		if got := normalizeType(v); got != v {
			t.Errorf("%q doit être conservé, obtenu %q", v, got)
		}
	}
	for _, v := range []string{"DEVIS", "  Livraison  ", "Rendez_Vous"} {
		if got := normalizeType(v); got == "autre" {
			t.Errorf("%q devrait être reconnu malgré casse et espaces, obtenu %q", v, got)
		}
	}
	for _, v := range []string{"", "chantier", "n'importe quoi", "devis; DROP TABLE"} {
		if got := normalizeType(v); got != "autre" {
			t.Errorf("%q doit retomber sur autre, obtenu %q", v, got)
		}
	}
}

// Un score de confiance hors bornes fausserait le seuil de publication, donc
// la règle anti-churn du CDC.
func TestClamp01(t *testing.T) {
	cas := map[float64]float64{-1: 0, 0: 0, 0.5: 0.5, 1: 1, 1.7: 1, 42: 1}
	for entree, attendu := range cas {
		if got := clamp01(entree); got != attendu {
			t.Errorf("clamp01(%v) = %v, attendu %v", entree, got, attendu)
		}
	}
}

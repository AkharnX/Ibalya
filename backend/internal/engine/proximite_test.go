package engine

import "testing"

// Le modèle reformule : « Livraison des 12 vitrages doubles » et « Livrer 12
// vitrages doubles à l'atelier » sont la même promesse. Sans ce rapprochement,
// une commande donne trois engagements et six brouillons (constat BS ACCESS).
func TestObjetsProches(t *testing.T) {
	cas := []struct {
		a, b    string
		attendu bool
	}{
		{"Livraison des 12 vitrages doubles", "Livrer 12 vitrages doubles à l'atelier", true},
		{"Envoi du devis pour la pergola", "Devis pergola bioclimatique", true},
		{"Pose des volets roulants", "Vernissage de l'escalier", false},
		{"Pose des volets roulants chez M. Bianchi", "Pose des volets roulants", true},
		{"Relance", "Relancer le client", true},
		{"", "Devis", false},
		{"Facture définitive chantier Leroy", "Rendez-vous pour prise de mesures", false},
	}
	for _, c := range cas {
		if got := ObjetsProches(c.a, c.b); got != c.attendu {
			t.Errorf("%q / %q : attendu %v", c.a, c.b, c.attendu)
		}
	}
}

func TestTypesCompatibles(t *testing.T) {
	cas := []struct {
		a, b    string
		attendu bool
	}{
		{"devis", "devis", true},
		{"devis", "autre", true},
		{"autre", "livraison", true},
		{"devis", "rendez_vous", false},
		{"", "livraison", true},
	}
	for _, c := range cas {
		if got := typesCompatibles(c.a, c.b); got != c.attendu {
			t.Errorf("%q / %q : attendu %v", c.a, c.b, c.attendu)
		}
	}
}

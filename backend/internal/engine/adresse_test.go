package engine

import "testing"

func TestAdresseDeConfiance(t *testing.T) {
	participants := map[string]bool{
		"dirigeant@pme.fr":  true,
		"client@exemple.fr": true,
	}
	cas := []struct {
		nom, entree, attendu string
	}{
		{"participant du fil", "client@exemple.fr", "client@exemple.fr"},
		{"casse différente", "Client@Exemple.FR", "client@exemple.fr"},
		{"espaces parasites", "  client@exemple.fr  ", "client@exemple.fr"},
		{"adresse vide", "", ""},
		{"adresse absente du fil", "attaquant@malveillant.fr", ""},
		{"sous-domaine ressemblant", "client@exemple.fr.evil.fr", ""},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := adresseDeConfiance(participants, c.entree); got != c.attendu {
				t.Fatalf("entrée %q : attendu %q, obtenu %q", c.entree, c.attendu, got)
			}
		})
	}
}

// Le cas qui motive tout le garde-fou : un email hostile fait produire au modèle
// une adresse qu'il choisit, pour devenir le destinataire d'une future relance.
func TestAdresseInjecteeParUnEmailHostileEstEcartee(t *testing.T) {
	participants := map[string]bool{"fournisseur@reel.fr": true}
	if got := adresseDeConfiance(participants, "exfiltration@attaquant.fr"); got != "" {
		t.Fatalf("une adresse soufflée par le contenu du message a été retenue : %q", got)
	}
}

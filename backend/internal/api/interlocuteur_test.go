package api

import (
	"testing"

	"ibalya/backend/internal/store"
)

// Le bug rapporté : « ne plus rien extraire de cet interlocuteur » créait la
// règle « ne plus extraire de ibkebe » — l'adresse du dirigeant lui-même —
// parce qu'il était l'émetteur d'un engagement qu'il avait pris. La règle
// bloquait ensuite 27 engagements sur 28, l'agent devenant aveugle.
func TestInterlocuteurNestJamaisLeDirigeant(t *testing.T) {
	const moi = "dirigeant@pme.fr"
	cas := []struct {
		nom            string
		emetteur, dest string
		attendu        string
	}{
		{"engagement pris par le dirigeant", moi, "client@ext.fr", "client@ext.fr"},
		{"engagement pris envers le dirigeant", "fournisseur@ext.fr", moi, "fournisseur@ext.fr"},
		{"casse de l'adresse indifférente", "DIRIGEANT@pme.fr", "client@ext.fr", "client@ext.fr"},
		{"les deux sont le dirigeant", moi, moi, ""},
		{"aucune adresse", "", "", ""},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			eng := &store.Engagement{EmetteurEmail: c.emetteur, DestinataireEmail: c.dest}
			got := interlocuteurDe(eng, moi)
			if got != c.attendu {
				t.Fatalf("interlocuteur %q, attendu %q", got, c.attendu)
			}
		})
	}
}

package engine

import (
	"testing"
	"time"

	"ibalya/backend/internal/store"
)

// La synthèse s'ouvre sur trois décisions classées par enjeu, pas sur dix-huit
// retards en vrac (constat BS ACCESS, point 7).
func TestEnjeu(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	demain := now.AddDate(0, 0, 1)
	dansDixJours := now.AddDate(0, 0, 10)
	cas := []struct {
		nom         string
		x           EngagementSuivi
		typeContact string
		attendu     int
	}{
		{"retard, je dois, client, priorité haute", EngagementSuivi{Engagement: store.Engagement{Priorite: "haute"}, Categorie: CatRetard, Sens: SensJeDois}, "client", 5},
		{"retard, on me doit", EngagementSuivi{Categorie: CatRetard, Sens: SensOnMeDoit}, "", 2},
		{"retard probable", EngagementSuivi{Categorie: CatRisque, Sens: SensJeDois}, "", 2},
		{"en cours, échéance demain", EngagementSuivi{Engagement: store.Engagement{Echeance: &demain}, Categorie: CatEnCours, Sens: SensJeDois}, "", 1},
		{"en cours, échéance lointaine", EngagementSuivi{Engagement: store.Engagement{Echeance: &dansDixJours}, Categorie: CatEnCours, Sens: SensJeDois}, "", 0},
		{"pour information", EngagementSuivi{Categorie: CatRetard, Sens: SensPourInfo}, "client", 0},
	}
	for _, c := range cas {
		if got := Enjeu(c.x, c.typeContact, now); got != c.attendu {
			t.Errorf("%s : attendu %d, obtenu %d", c.nom, c.attendu, got)
		}
	}
}

func TestTrierParEnjeu(t *testing.T) {
	in := []PrioriteItem{{EngagementID: 1, Enjeu: 2}, {EngagementID: 2, Enjeu: 5}, {EngagementID: 3, Enjeu: 2}, {EngagementID: 4, Enjeu: 3}}
	out := trierParEnjeu(in)
	ids := []int64{out[0].EngagementID, out[1].EngagementID, out[2].EngagementID, out[3].EngagementID}
	if !(ids[0] == 2 && ids[1] == 4 && ids[2] == 1 && ids[3] == 3) {
		t.Fatalf("ordre inattendu : %v", ids)
	}
}

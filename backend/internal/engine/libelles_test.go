package engine

import (
	"testing"
	"time"
)

func TestJours(t *testing.T) {
	if got := Jours(1); got != "1 jour" {
		t.Fatal(got)
	}
	if got := Jours(3); got != "3 jours" {
		t.Fatal(got)
	}
	if got := Jours(0); got != "0 jour" {
		t.Fatal(got)
	}
}

// Les libellés se calculent en jours calendaires : une échéance « ce soir »
// est « aujourd'hui » à 00 h 05 comme à 23 h, et une échéance dépassée de six
// jours ne s'affiche jamais « dans 0 jour » (constat BS ACCESS, 11 nov. 2026).
func TestLibelleEcheance(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	now := time.Date(2026, 11, 12, 0, 5, 0, 0, paris)
	jour := func(d int, h int) time.Time { return time.Date(2026, 11, 12+d, h, 59, 0, 0, paris) }
	cas := []struct {
		nom      string
		echeance time.Time
		attendu  string
	}{
		{"même jour, tard le soir", jour(0, 23), "aujourd'hui"},
		{"demain", jour(1, 8), "demain"},
		{"hier", jour(-1, 8), "hier"},
		{"dans trois jours", jour(3, 8), "dans 3 jours"},
		{"en retard de six jours", jour(-6, 8), "en retard de 6 jours"},
		{"échéance stockée en UTC à minuit", time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), "dans 3 jours"},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := LibelleEcheance(c.echeance, now); got != c.attendu {
				t.Fatalf("attendu %q, obtenu %q", c.attendu, got)
			}
		})
	}
}

func TestNormaliserDatesObjet(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	cas := map[string]string{
		"Livraison 2026-10-01 vitrages":        "Livraison 01/10 vitrages",
		"Échéance 2027-01-27 : pose":           "Échéance 27/01/2027 : pose",
		"Devis pergola, relance le 2026-11-20": "Devis pergola, relance le 20/11",
		"Sans date":                            "Sans date",
	}
	for in, attendu := range cas {
		if got := NormaliserDatesObjet(in, now); got != attendu {
			t.Errorf("%q : attendu %q, obtenu %q", in, attendu, got)
		}
	}
}

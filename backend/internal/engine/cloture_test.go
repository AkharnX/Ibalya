package engine

import (
	"strings"
	"testing"
	"time"
)

// Un fil qui annonce la fin (« bien reçu », « virement effectué », « facture
// ci-jointe ») doit faire poser la question « C'est fait ? » plutôt que
// laisser l'engagement « en retard de 41 jours » (constat BS ACCESS).
func TestSignalFin(t *testing.T) {
	cas := []struct {
		texte   string
		attendu bool
	}{
		{"Colis bien reçu ce matin, merci pour la réactivité.", true},
		{"Virement effectué ce jour pour la facture 2026-118.", true},
		{"Vous trouverez la facture ci-jointe.", true},
		{"Les fenêtres ont été livrées hier, tout est conforme.", true},
		{"Pouvez-vous me confirmer la date de livraison ?", false},
		{"Nous attendons toujours le règlement.", false},
	}
	for _, c := range cas {
		if _, ok := SignalFin(c.texte); ok != c.attendu {
			t.Errorf("%q : attendu %v", c.texte, c.attendu)
		}
	}
}

func TestSignalFinExtrait(t *testing.T) {
	long := "Bonjour. " + strings.Repeat("Un long préambule sans intérêt, ", 6) + "le colis est bien reçu et tout est en ordre, nous reviendrons vers vous pour la suite du chantier dès la semaine prochaine. Merci."
	extrait, ok := SignalFin(long)
	if !ok {
		t.Fatal("signal attendu")
	}
	if n := len([]rune(extrait)); n > 120 {
		t.Fatalf("extrait trop long : %d caractères", n)
	}
	if !strings.Contains(extrait, "bien reçu") {
		t.Fatalf("l'extrait doit contenir le signal : %q", extrait)
	}
}

// « C'est fait ? » par ancienneté : échéance dépassée depuis 7 jours ou plus,
// et rien ne s'est passé depuis 7 jours. Un retard frais n'est pas concerné.
func TestAClorePourDelai(t *testing.T) {
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	j := func(d int) time.Time { return now.AddDate(0, 0, d) }
	cas := []struct {
		nom                 string
		echeance, dernierEv time.Time
		attendu             bool
	}{
		{"retard de 6 jours, aucun événement", j(-6), j(-30), false},
		{"retard de 7 jours, aucun événement", j(-7), j(-30), true},
		{"retard ancien mais relancé il y a 3 jours", j(-20), j(-3), false},
		{"retard ancien, dernier événement il y a 7 jours", j(-20), j(-7), true},
	}
	for _, c := range cas {
		if got := aClorePourDelai(c.echeance, c.dernierEv, now); got != c.attendu {
			t.Errorf("%s : attendu %v", c.nom, c.attendu)
		}
	}
}

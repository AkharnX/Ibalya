package engine

import "testing"

// Le défaut qu'on corrige : le modèle pouvait annoncer 0,95 sur un engagement
// sans échéance, sans formulation engageante et venant d'un inconnu. Un tel
// engagement doit rester sous le seuil de publication (0,6 par défaut) même
// quand le modèle est catégorique.
func TestUnModeleCategoriqueNeSuffitPlus(t *testing.T) {
	score, _ := CalculerFiabilite(SignauxFiabilite{
		AvisModele:                  1.0,
		TauxCorrectionInterlocuteur: -1,
	})
	if score >= 0.6 {
		t.Fatalf("score %.2f : l'avis du modèle emporte encore la décision", score)
	}
	if NiveauFiabilite(score) != "incertaine" {
		t.Fatalf("niveau %q, attendu incertaine", NiveauFiabilite(score))
	}
}

// À l'inverse, un engagement portant toutes les marques du vrai doit sortir en
// haut même si le modèle est tiède.
func TestLesSignauxVerifiablesPriment(t *testing.T) {
	score, detail := CalculerFiabilite(SignauxFiabilite{
		AvisModele:                  0.2,
		EcheanceExplicite:           true,
		FormulationEngageante:       true,
		FilConversationnel:          true,
		InterlocuteurConnu:          true,
		TauxCorrectionInterlocuteur: -1,
	})
	if score < 0.75 {
		t.Fatalf("score %.2f, attendu au moins 0,75 — détail %v", score, detail)
	}
	if NiveauFiabilite(score) != "elevee" {
		t.Fatalf("niveau %q, attendu elevee", NiveauFiabilite(score))
	}
}

// Un interlocuteur dont les engagements finissent systématiquement corrigés
// doit tirer les suivants vers le bas. C'est le seul signal qui apprend.
func TestHistoriqueDeCorrectionsPenalise(t *testing.T) {
	base := SignauxFiabilite{
		AvisModele: 0.9, EcheanceExplicite: true, FormulationEngageante: true,
		TauxCorrectionInterlocuteur: -1,
	}
	sans, _ := CalculerFiabilite(base)
	base.TauxCorrectionInterlocuteur = 1.0
	avec, _ := CalculerFiabilite(base)
	if avec >= sans {
		t.Fatalf("la pénalité n'a pas joué : %.2f puis %.2f", sans, avec)
	}
	if d := sans - avec; d < 0.19 || d > 0.21 {
		t.Fatalf("écart %.2f, attendu la pénalité maximale de 0,20", d)
	}
}

// Un historique trop mince ne doit pas pénaliser : deux engagements corrigés
// ne disent rien d'un correspondant.
func TestHistoriqueMinceNePenalisePas(t *testing.T) {
	base := SignauxFiabilite{AvisModele: 0.9, EcheanceExplicite: true, TauxCorrectionInterlocuteur: -1}
	score, detail := CalculerFiabilite(base)
	if _, present := detail["penalite_corrections"]; present {
		t.Fatal("une pénalité a été appliquée sans historique suffisant")
	}
	if score <= 0 {
		t.Fatal("score nul")
	}
}

func TestScoreToujoursBorne(t *testing.T) {
	cas := []SignauxFiabilite{
		{AvisModele: 5, EcheanceExplicite: true, FormulationEngageante: true,
			FilConversationnel: true, InterlocuteurConnu: true, TauxCorrectionInterlocuteur: -1},
		{AvisModele: -3, TauxCorrectionInterlocuteur: 5},
	}
	for _, s := range cas {
		if score, _ := CalculerFiabilite(s); score < 0 || score > 1 {
			t.Fatalf("score hors bornes : %.2f", score)
		}
	}
}

func TestFormulationEngageante(t *testing.T) {
	engageantes := []string{
		"Je vous livre le vitrage jeudi.",
		"Nous confirmons la commande.",
		"Merci de valider avant vendredi.",
		"Le dossier doit être rendu avant le 15 septembre.",
		"C'est noté, je reviens vers vous.",
	}
	for _, p := range engageantes {
		if !FormulationEstEngageante(p) {
			t.Errorf("non détectée : %s", p)
		}
	}
	vagues := []string{
		"Il faudrait qu'on avance sur le sujet un de ces jours.",
		"Bonne réception, cordialement.",
		"Le chantier progresse bien.",
		"",
	}
	for _, p := range vagues {
		if FormulationEstEngageante(p) {
			t.Errorf("faux positif sur une tournure vague : %s", p)
		}
	}
}

func TestNiveaux(t *testing.T) {
	cas := []struct {
		score  float64
		niveau string
	}{
		{1.0, "elevee"}, {0.75, "elevee"},
		{0.74, "a_verifier"}, {0.50, "a_verifier"},
		{0.49, "incertaine"}, {0.0, "incertaine"},
	}
	for _, c := range cas {
		if got := NiveauFiabilite(c.score); got != c.niveau {
			t.Errorf("%.2f → %q, attendu %q", c.score, got, c.niveau)
		}
	}
}

package engine

import "testing"

// Le modèle signe malgré la consigne, et se trompe de nom. On coupe ce qu'il a
// écrit après la formule de politesse, en gardant la formule elle-même.
func TestRetirerSignatureModele(t *testing.T) {
	cas := []struct{ nom, entree, attendu string }{
		{
			"nom inventé après la formule",
			"Je reste à votre disposition.\n\nCordialement,\nIbrahima B. Kebe",
			"Je reste à votre disposition.\n\nCordialement,",
		},
		{
			"autre prénom inventé",
			"Merci de votre retour.\n\nBien cordialement,\nIsmaël Kebe",
			"Merci de votre retour.\n\nBien cordialement,",
		},
		{
			"signature sur deux lignes",
			"À bientôt.\n\nCordialement,\nIbrahim Kebe\nCTO — Kebe Agency",
			"À bientôt.\n\nCordialement,",
		},
		{
			"formule seule, rien à couper",
			"Dans l'attente de votre retour,\nCordialement.",
			"Dans l'attente de votre retour,\nCordialement.",
		},
		{
			"aucune formule : on ne touche à rien",
			"Peux-tu me confirmer la date ?",
			"Peux-tu me confirmer la date ?",
		},
		{
			"le mot cordialement au milieu du texte ne coupe rien",
			"Il m'a répondu cordialement mais sans date.\nMerci de relancer.",
			"Il m'a répondu cordialement mais sans date.\nMerci de relancer.",
		},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := retirerSignatureModele(c.entree); got != c.attendu {
				t.Fatalf("obtenu :\n%q\nattendu :\n%q", got, c.attendu)
			}
		})
	}
}

// La signature libre prime sur les quatre champs : ils ne couvrent pas une
// mention légale, un téléphone ou une seconde ligne d'adresse.
func TestSignatureComposee(t *testing.T) {
	cas := []struct{ nom, prenom, patronyme, fonction, societe, attendu string }{
		{"tout renseigné", "Ibrahim", "Kebe", "CTO", "Kebe Agency", "Ibrahim Kebe\nCTO — Kebe Agency"},
		{"sans fonction", "Ibrahim", "Kebe", "", "Kebe Agency", "Ibrahim Kebe\nKebe Agency"},
		{"sans société", "Ibrahim", "Kebe", "CTO", "", "Ibrahim Kebe\nCTO"},
		{"nom seul", "Ibrahim", "", "", "", "Ibrahim"},
		{"rien : pas de signature", "", "", "CTO", "Kebe Agency", ""},
		{"espaces parasites", "  Ibrahim ", " Kebe ", "", "", "Ibrahim Kebe"},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := SignatureComposee(c.prenom, c.patronyme, c.fonction, c.societe); got != c.attendu {
				t.Fatalf("obtenu %q, attendu %q", got, c.attendu)
			}
		})
	}
}

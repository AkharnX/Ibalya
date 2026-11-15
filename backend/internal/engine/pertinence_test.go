package engine

import "testing"

// Un interlocuteur compte s'il a un engagement avec le dirigeant, ou si les
// deux se sont écrit. Un automate, une liste de diffusion ou une adresse en
// copie qui n'a jamais répondu n'ont rien à faire dans le carnet.
func TestPersonnePertinente(t *testing.T) {
	cas := []struct {
		nom                         string
		recus, envoyes, engagements int
		automatique, attendu        bool
	}{
		{"échange dans les deux sens", 3, 2, 0, false, true},
		{"un engagement, même sans réponse", 1, 0, 1, false, true},
		{"il m'écrit, je ne réponds jamais", 5, 0, 0, false, false},
		{"en copie de mes envois, jamais répondu", 0, 4, 0, false, false},
		{"automate, même avec un engagement", 3, 1, 1, true, false},
		{"aucune activité", 0, 0, 0, false, false},
	}
	for _, c := range cas {
		if got := PersonnePertinente(c.recus, c.envoyes, c.engagements, c.automatique); got != c.attendu {
			t.Errorf("%s : attendu %v", c.nom, c.attendu)
		}
	}
}

package channel

import "testing"

// Les en-têtes d'envoi automatique sont le signal le plus sûr qu'aucune
// personne n'a écrit le message : les robots les posent, les humains jamais.
func TestEstEnvoiAutomatique(t *testing.T) {
	cas := []struct {
		nom     string
		entetes map[string]string
		attendu bool
	}{
		{"Auto-Submitted auto-generated", map[string]string{"Auto-Submitted": "auto-generated"}, true},
		{"Auto-Submitted auto-replied", map[string]string{"auto-submitted": "auto-replied"}, true},
		{"Auto-Submitted no : un humain", map[string]string{"Auto-Submitted": "no"}, false},
		{"Precedence bulk", map[string]string{"Precedence": "bulk"}, true},
		{"Precedence list, casse mélangée", map[string]string{"precedence": "List"}, true},
		{"X-Auto-Response-Suppress", map[string]string{"X-Auto-Response-Suppress": "All"}, true},
		{"Feedback-ID", map[string]string{"Feedback-ID": "abc:campagne:sendgrid"}, true},
		{"aucun en-tête", map[string]string{}, false},
		{"Precedence normal", map[string]string{"Precedence": "normal"}, false},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			lire := func(nom string) string {
				for k, v := range c.entetes {
					if equalFoldASCII(k, nom) {
						return v
					}
				}
				return ""
			}
			if got := EstEnvoiAutomatique(lire); got != c.attendu {
				t.Fatalf("attendu %v, obtenu %v", c.attendu, got)
			}
		})
	}
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 'a' - 'A'
		}
		if y >= 'A' && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

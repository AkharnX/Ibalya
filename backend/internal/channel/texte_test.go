package channel

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Le bug exact vu en production : 432 insertions rejetées sur « invalid byte
// sequence for encoding UTF8: 0xc3 ». Une coupe à l'octet près sectionnait un
// caractère accentué, et le message entier était perdu en silence.
func TestTroncatureNeBrisePasUnCaractere(t *testing.T) {
	// Le 4000e octet tombe au milieu du « é ».
	corps := strings.Repeat("a", 3999) + "é" + strings.Repeat("b", 100)

	brut := corps[:4000]
	if utf8.ValidString(brut) {
		t.Fatal("le cas de test ne reproduit pas le bug : la coupe brute est valide")
	}
	if brut[len(brut)-1] != 0xc3 {
		t.Fatalf("dernier octet 0x%x, attendu 0xc3 comme en production", brut[len(brut)-1])
	}

	sur := Tronquer(corps, 4000)
	if !utf8.ValidString(sur) {
		t.Fatal("la troncature sûre a produit de l'UTF-8 invalide")
	}
	if len(sur) != 3999 {
		t.Fatalf("longueur %d : on doit reculer d'un octet, pas davantage", len(sur))
	}
}

func TestTroncature(t *testing.T) {
	cas := []struct {
		nom, entree  string
		max, attendu int
	}{
		{"plus court que la limite", "bonjour", 100, 7},
		{"exactement la limite", "bonjour", 7, 7},
		{"coupe entre deux caractères simples", "abcdefgh", 4, 4},
		{"coupe au milieu d'un accent", "abcé", 4, 3},
		{"coupe au milieu d'un emoji", "ab🔒", 4, 2},
		{"limite nulle", "abc", 0, 3},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			got := Tronquer(c.entree, c.max)
			if len(got) != c.attendu {
				t.Fatalf("longueur %d, attendu %d (%q)", len(got), c.attendu, got)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("résultat invalide : %q", got)
			}
		})
	}
}

// Un message mal encodé à la source ne doit pas faire perdre la conversation
// entière : mieux vaut un caractère manquant qu'un message absent.
func TestNettoyageUTF8(t *testing.T) {
	sale := "Bonjour " + string([]byte{0xc3}) + " Marc"
	propre := NettoyerUTF8(sale)
	if !utf8.ValidString(propre) {
		t.Fatal("le nettoyage a laissé une séquence invalide")
	}
	if !strings.Contains(propre, "Bonjour") || !strings.Contains(propre, "Marc") {
		t.Fatalf("le texte lisible doit être conservé : %q", propre)
	}
	// Un texte déjà valide ne doit pas être altéré.
	if NettoyerUTF8("déjà valide 🔒") != "déjà valide 🔒" {
		t.Fatal("un texte valide a été modifié")
	}
}

package store

import "strings"
import "testing"

const phrase = "une-phrase-de-chiffrement-suffisamment-longue-2026"

func TestCoffreAllerRetour(t *testing.T) {
	c, err := NouveauCoffre(phrase)
	if err != nil || c == nil {
		t.Fatalf("création : %v", err)
	}
	for _, clair := range []string{"mot-de-passe", "", "é@#{}[]•unicode", strings.Repeat("x", 500)} {
		chiffre, err := c.Chiffrer(clair)
		if err != nil {
			t.Fatalf("chiffrement : %v", err)
		}
		if clair != "" && strings.Contains(chiffre, clair) {
			t.Fatal("le clair apparaît dans le chiffré")
		}
		out, err := c.Dechiffrer(chiffre)
		if err != nil {
			t.Fatalf("déchiffrement : %v", err)
		}
		if out != clair {
			t.Fatalf("aller-retour : %q devient %q", clair, out)
		}
	}
}

// Deux chiffrements du même secret doivent différer : un nonce réutilisé
// laisserait deviner que deux comptes partagent le même mot de passe.
func TestNonceDifferentACehaqueFois(t *testing.T) {
	c, _ := NouveauCoffre(phrase)
	a, _ := c.Chiffrer("identique")
	b, _ := c.Chiffrer("identique")
	if a == b {
		t.Fatal("deux chiffrements identiques : le nonce n'est pas renouvelé")
	}
}

// GCM authentifie : une valeur modifiée en base doit être rejetée, pas
// interprétée.
func TestValeurAltereeRejetee(t *testing.T) {
	c, _ := NouveauCoffre(phrase)
	chiffre, _ := c.Chiffrer("secret")
	altere := chiffre[:len(chiffre)-2] + "AA"
	if _, err := c.Dechiffrer(altere); err == nil {
		t.Fatal("une valeur altérée a été acceptée")
	}
}

func TestMauvaiseCleRejetee(t *testing.T) {
	a, _ := NouveauCoffre(phrase)
	b, _ := NouveauCoffre("une-autre-phrase-tout-aussi-longue-mais-differente")
	chiffre, _ := a.Chiffrer("secret")
	if _, err := b.Dechiffrer(chiffre); err == nil {
		t.Fatal("une clé incorrecte a déchiffré")
	}
}

// Sans clé, on refuse plutôt que d'écrire en clair.
func TestSansCleOnRefuse(t *testing.T) {
	c, err := NouveauCoffre("")
	if err != nil || c != nil {
		t.Fatalf("attendu nil sans erreur, obtenu %v %v", c, err)
	}
	if _, err := c.Chiffrer("x"); err == nil {
		t.Fatal("un coffre absent doit refuser de chiffrer")
	}
}

func TestCleTropCourteRefusee(t *testing.T) {
	if _, err := NouveauCoffre("trop-courte"); err == nil {
		t.Fatal("une clé de moins de 32 caractères doit être refusée")
	}
}

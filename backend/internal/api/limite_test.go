package api

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiteurBloqueApresLeQuota(t *testing.T) {
	l := nouveauLimiteur(3, time.Minute)
	for i := 0; i < 3; i++ {
		if l.Bloque("ip:1.2.3.4") {
			t.Fatalf("bloqué dès la tentative %d, avant d'atteindre le quota", i+1)
		}
		l.Echec("ip:1.2.3.4")
	}
	if !l.Bloque("ip:1.2.3.4") {
		t.Fatal("le quota est atteint mais rien ne bloque")
	}
}

func TestLimiteurIsoleLesCles(t *testing.T) {
	l := nouveauLimiteur(2, time.Minute)
	l.Echec("ip:1.1.1.1")
	l.Echec("ip:1.1.1.1")
	if !l.Bloque("ip:1.1.1.1") {
		t.Fatal("l'adresse fautive devrait être bloquée")
	}
	if l.Bloque("ip:2.2.2.2") {
		t.Fatal("une autre adresse ne doit pas hériter du blocage")
	}
}

// La clé par compte existe pour arrêter une attaque répartie sur plusieurs
// adresses : c'est le cas que la seule clé IP ne couvre pas.
func TestLimiteurArreteUneAttaqueRepartie(t *testing.T) {
	l := nouveauLimiteur(3, time.Minute)
	for i, ip := range []string{"ip:1.1.1.1", "ip:2.2.2.2", "ip:3.3.3.3"} {
		if l.Bloque(ip, "compte:cible@ex.fr") {
			t.Fatalf("bloqué trop tôt, à la tentative %d", i+1)
		}
		l.Echec(ip, "compte:cible@ex.fr")
	}
	if !l.Bloque("ip:4.4.4.4", "compte:cible@ex.fr") {
		t.Fatal("trois adresses différentes ont visé le même compte sans être arrêtées")
	}
}

func TestLimiteurOublieApresLaFenetre(t *testing.T) {
	l := nouveauLimiteur(2, 40*time.Millisecond)
	l.Echec("ip:9.9.9.9")
	l.Echec("ip:9.9.9.9")
	if !l.Bloque("ip:9.9.9.9") {
		t.Fatal("devrait être bloqué juste après les échecs")
	}
	time.Sleep(60 * time.Millisecond)
	if l.Bloque("ip:9.9.9.9") {
		t.Fatal("la fenêtre est passée, le blocage devrait être levé")
	}
}

func TestLimiteurSuccesEffaceLeCompteur(t *testing.T) {
	l := nouveauLimiteur(3, time.Minute)
	l.Echec("ip:5.5.5.5")
	l.Echec("ip:5.5.5.5")
	l.Succes("ip:5.5.5.5")
	l.Echec("ip:5.5.5.5")
	if l.Bloque("ip:5.5.5.5") {
		t.Fatal("une connexion réussie doit remettre le compteur à zéro")
	}
}

// Derrière nginx l'adresse de pair vaut toujours 127.0.0.1 : sans lecture de
// l'en-tête, tout Internet partagerait un seul compteur.
func TestClientIPLitLEnTeteDuRelais(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	if got := clientIP(r); got != "127.0.0.1" {
		t.Fatalf("sans en-tête, attendu 127.0.0.1, obtenu %s", got)
	}
	r.Header.Set("X-Real-IP", "203.0.113.7")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Fatalf("attendu 203.0.113.7, obtenu %s", got)
	}
	r.Header.Del("X-Real-IP")
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if got := clientIP(r); got != "203.0.113.9" {
		t.Fatalf("attendu la première adresse de la chaîne, obtenu %s", got)
	}
}

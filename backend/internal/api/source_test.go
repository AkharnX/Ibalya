package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Le lien doit ouvrir le BON compte : un dirigeant connecté à plusieurs
// sessions Google atterrirait sinon dans la mauvaise boîte.
func TestUrlGmail(t *testing.T) {
	u := urlGmail("marc@exemple.fr", "18f2a")
	if !strings.Contains(u, "authuser=marc%40exemple.fr") {
		t.Errorf("le compte doit être encodé dans l'URL, obtenu %q", u)
	}
	if !strings.HasSuffix(u, "#all/18f2a") {
		t.Errorf("l'identifiant du message doit terminer l'URL, obtenu %q", u)
	}
	if urlGmail("marc@exemple.fr", "") != "" {
		t.Error("sans identifiant de message, aucun lien ne doit être produit")
	}
	if u := urlGmail("", "18f2a"); strings.Contains(u, "authuser") {
		t.Errorf("sans compte connu, pas de paramètre authuser : %q", u)
	}
}

// Le jeton de service ne doit jamais être exploitable depuis Internet, y
// compris avec un en-tête X-Forwarded-For ou X-Real-IP forgé.
func TestEstLocal(t *testing.T) {
	cas := []struct {
		adresse string
		entetes map[string]string
		local   bool
	}{
		{"127.0.0.1:5555", nil, true},
		{"[::1]:5555", nil, true},
		{"192.168.1.10:5555", nil, false},
		{"51.75.12.4:443", nil, false},
		{"51.75.12.4:443", map[string]string{"X-Real-IP": "127.0.0.1"}, false},
		{"51.75.12.4:443", map[string]string{"X-Forwarded-For": "127.0.0.1"}, false},
	}
	for _, c := range cas {
		r := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		r.RemoteAddr = c.adresse
		for k, v := range c.entetes {
			r.Header.Set(k, v)
		}
		if got := estLocal(r); got != c.local {
			t.Errorf("estLocal(%s, %v) = %v, attendu %v", c.adresse, c.entetes, got, c.local)
		}
	}
}

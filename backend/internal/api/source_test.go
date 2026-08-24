package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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

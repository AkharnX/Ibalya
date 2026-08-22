package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Limiteur de tentatives de connexion.
//
// Sans lui, rien n'empêche d'essayer les mots de passe en boucle : bcrypt rend
// chaque essai coûteux pour le serveur, pas pour l'attaquant qui parallélise.
// On compte les échecs par adresse IP et par compte visé, les deux étant
// nécessaires : la clé IP arrête le bourrinage d'un seul compte, la clé compte
// arrête la même attaque répartie sur plusieurs adresses.
type limiteur struct {
	mu      sync.Mutex
	echecs  map[string][]time.Time
	fenetre time.Duration
	max     int
}

func nouveauLimiteur(max int, fenetre time.Duration) *limiteur {
	return &limiteur{echecs: map[string][]time.Time{}, fenetre: fenetre, max: max}
}

// recentes purge les tentatives sorties de la fenêtre et retourne le reste.
// Appelée sous verrou.
func (l *limiteur) recentes(cle string, maintenant time.Time) []time.Time {
	var garde []time.Time
	for _, t := range l.echecs[cle] {
		if maintenant.Sub(t) < l.fenetre {
			garde = append(garde, t)
		}
	}
	if len(garde) == 0 {
		delete(l.echecs, cle)
	} else {
		l.echecs[cle] = garde
	}
	return garde
}

// Bloque indique si l'une des clés a dépassé le quota.
func (l *limiteur) Bloque(cles ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	maintenant := time.Now()
	for _, c := range cles {
		if len(l.recentes(c, maintenant)) >= l.max {
			return true
		}
	}
	return false
}

// Echec enregistre une tentative ratée sur chaque clé.
func (l *limiteur) Echec(cles ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	maintenant := time.Now()
	for _, c := range cles {
		l.echecs[c] = append(l.recentes(c, maintenant), maintenant)
	}
}

// Succes efface le compteur : une connexion réussie ne doit pas laisser
// l'utilisateur légitime enfermé par ses propres fautes de frappe.
func (l *limiteur) Succes(cles ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range cles {
		delete(l.echecs, c)
	}
}

// clientIP retourne l'adresse réelle du client. Derrière nginx, l'adresse de
// pair est toujours 127.0.0.1 : sans cette lecture d'en-tête, tout Internet
// partagerait le même compteur et un seul attaquant bloquerait tout le monde.
// L'en-tête n'est digne de confiance que parce que nginx le réécrit et que le
// port du backend n'est pas joignable directement depuis l'extérieur.
func clientIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Real-IP")); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if premier := strings.TrimSpace(strings.Split(v, ",")[0]); premier != "" {
			return premier
		}
	}
	hote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return hote
}

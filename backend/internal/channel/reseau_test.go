package channel

import (
	"net"
	"testing"
	"time"
)

// Le test de connexion accepte un hôte arbitraire : sans garde-fou il devient
// un scanner du réseau interne, et atteint les métadonnées du fournisseur cloud.
func TestAdressesInternesRefusees(t *testing.T) {
	internes := []string{
		"127.0.0.1",       // boucle locale
		"::1",             // boucle locale IPv6
		"10.0.0.5",        // privée
		"172.17.0.2",      // privée, réseau Docker
		"192.168.1.10",    // privée
		"169.254.169.254", // métadonnées cloud
		"100.64.0.1",      // plage partagée opérateur
		"0.0.0.0",         // non spécifiée
	}
	for _, a := range internes {
		if !estInterne(net.ParseIP(a)) {
			t.Errorf("%s doit être refusée", a)
		}
	}
	publiques := []string{
		"142.250.75.238",     // Google
		"52.97.128.1",        // Microsoft
		"2a00:1450:4007::68", // IPv6 publique
		"51.75.12.4",         // OVH
	}
	for _, a := range publiques {
		if estInterne(net.ParseIP(a)) {
			t.Errorf("%s est publique et doit être acceptée", a)
		}
	}
}

// Le contrôle porte sur l'adresse résolue au moment de la connexion, pas sur
// le nom saisi : un nom peut pointer vers une adresse publique à la
// vérification puis vers une adresse interne à la connexion.
func TestControleAuMomentDeLaConnexion(t *testing.T) {
	d := dialerRestreint(false, time.Second)
	if d.Control == nil {
		t.Fatal("le dialer doit contrôler l'adresse au moment de la connexion")
	}
	if err := d.Control("tcp", "127.0.0.1:993", nil); err == nil {
		t.Error("la boucle locale doit être refusée")
	}
	if err := d.Control("tcp", "142.250.75.238:993", nil); err != nil {
		t.Errorf("une adresse publique doit passer : %v", err)
	}
}

// En développement on veut joindre un serveur local : le contournement est le
// même que celui du certificat, et la garde de configuration l'interdit en
// production.
func TestDeveloppementAutoriseLInterne(t *testing.T) {
	if d := dialerRestreint(true, time.Second); d.Control != nil {
		t.Fatal("en développement, aucune restriction ne doit s'appliquer")
	}
}

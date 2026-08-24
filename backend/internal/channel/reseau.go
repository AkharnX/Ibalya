package channel

import (
	"fmt"
	"net"
	"syscall"
	"time"
)

// Refus des destinations internes.
//
// Le test de connexion accepte l'hôte et le port saisis par le dirigeant :
// c'est nécessaire, chaque hébergeur a les siens. Mais sans garde-fou, il
// devient un scanner du réseau interne — les messages d'erreur distinguent un
// port ouvert d'un port fermé, et l'adresse de métadonnées du fournisseur
// cloud est joignable.
//
// Le contrôle porte sur l'adresse résolue au moment d'ouvrir la connexion, et
// non sur le nom saisi : un nom peut se résoudre en adresse publique lors de
// la vérification puis en adresse interne à la connexion, technique dite de
// rebinding DNS.
func dialerRestreint(autoriserInterne bool, delai time.Duration) *net.Dialer {
	d := &net.Dialer{Timeout: delai}
	if autoriserInterne {
		return d
	}
	d.Control = func(reseau, adresse string, _ syscall.RawConn) error {
		hote, _, err := net.SplitHostPort(adresse)
		if err != nil {
			return err
		}
		ip := net.ParseIP(hote)
		if ip == nil {
			return fmt.Errorf("adresse illisible : %s", hote)
		}
		if estInterne(ip) {
			return fmt.Errorf("destination interne refusée (%s) : "+
				"un serveur de messagerie doit être joignable publiquement", ip)
		}
		return nil
	}
	return d
}

// estInterne couvre la boucle locale, les plages privées, le lien-local — qui
// porte les métadonnées des fournisseurs cloud — et les adresses non
// routables.
func estInterne(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	// 100.64.0.0/10, plage partagée des opérateurs, souvent utilisée en interne.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}

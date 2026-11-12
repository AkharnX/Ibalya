package engine

import (
	"context"
	"strings"
)

// Sens d'un engagement vu du dirigeant. Il se calcule à partir des adresses,
// jamais stocké : les « autres adresses » du dirigeant changent, le sens suit.
const (
	SensJeDois   = "je_dois"    // le dirigeant a promis
	SensOnMeDoit = "on_me_doit" // on a promis au dirigeant
	SensPourInfo = "pour_info"  // deux tiers entre eux, le dirigeant en copie
)

// Sens classe l'engagement selon que l'émetteur ou le destinataire est une
// adresse du dirigeant (`soi` : compte raccordé, identifiant, autres adresses).
func Sens(emetteur, destinataire string, soi map[string]bool) string {
	e := strings.ToLower(strings.TrimSpace(emetteur))
	d := strings.ToLower(strings.TrimSpace(destinataire))
	switch {
	case e != "" && soi[e]:
		return SensJeDois
	case d != "" && soi[d]:
		return SensOnMeDoit
	default:
		return SensPourInfo
	}
}

// soi rend l'ensemble des adresses du dirigeant pour le tenant courant.
func (e *Engine) soi(ctx context.Context) map[string]bool {
	compte, _ := e.Canal(ctx).AccountEmail(ctx)
	return e.Store.AdressesSoi(ctx, compte)
}

// concerneLeDirigeant dit si un engagement mérite alertes et relances : oui
// s'il engage le dirigeant ou lui est dû, non s'il ne fait que passer en copie.
func concerneLeDirigeant(emetteur, destinataire string, soi map[string]bool) bool {
	return Sens(emetteur, destinataire, soi) != SensPourInfo
}

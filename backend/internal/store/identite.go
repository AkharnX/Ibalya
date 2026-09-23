package store

import (
	"context"
	"regexp"
)

var sepAdresses = regexp.MustCompile(`[,;\s]+`)

// AdressesSoi retourne l'ensemble (normalisé) des adresses considérées comme
// « le dirigeant » pour le tenant courant : la boîte connectée, l'email de
// login, et les alias qu'il a déclarés (réglage « adresses_soi »).
//
// Sans cela, l'agent ne connaît qu'UNE adresse « soi » (la boîte connectée) :
// un dirigeant qui lit sa boîte pro mais apparaît aussi via son adresse perso
// (login, ou en copie) se retrouve traité comme un interlocuteur externe —
// d'où les non-sens « X ne vous a pas répondu » où X est le dirigeant lui-même.
func (s *Store) AdressesSoi(ctx context.Context, compteCanal string) map[string]bool {
	soi := map[string]bool{}
	ajouter := func(e string) {
		if n := normEmail(e); n != "" {
			soi[n] = true
		}
	}
	ajouter(compteCanal)

	var login string
	_ = s.q(ctx).QueryRow(ctx,
		`SELECT email FROM users WHERE id = nullif(current_setting('app.user_id', true),'')::bigint`).
		Scan(&login)
	ajouter(login)

	for _, a := range sepAdresses.Split(s.GetSetting(ctx, "adresses_soi", ""), -1) {
		ajouter(a)
	}
	return soi
}

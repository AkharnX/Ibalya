package engine

import (
	"context"
	"strings"
)

// Signature construite depuis les réglages, jamais par le modèle.
//
// Le prompt lui demandait pourtant de ne pas signer. Sur douze brouillons il a
// signé quand même, de cinq noms différents — « Ibrahima B. Kebe »,
// « Ismaël Kebe » — dont deux qui n'étaient pas le prénom du dirigeant. Un
// message signé d'un faux nom part chez un client : la consigne ne suffit pas,
// il faut retirer au modèle la possibilité de se tromper.
func (e *Engine) signature(ctx context.Context) string {
	get := func(c string) string { return strings.TrimSpace(e.Store.GetSetting(ctx, c, "")) }
	nom := strings.TrimSpace(get("identite_prenom") + " " + get("identite_nom"))
	if nom == "" {
		return ""
	}
	lignes := []string{nom}
	if r := strings.TrimSpace(strings.Trim(get("identite_fonction")+" — "+get("identite_societe"), " —")); r != "" {
		lignes = append(lignes, r)
	}
	return strings.Join(lignes, "\n")
}

// apposerSignature retire ce que le modèle aurait signé malgré la consigne,
// puis ajoute la signature configurée.
func (e *Engine) apposerSignature(ctx context.Context, corps string) string {
	corps = retirerSignatureModele(strings.TrimRight(corps, " \n\t"))
	sig := e.signature(ctx)
	if sig == "" {
		return corps
	}
	return corps + "\n\n" + sig
}

var formulesPolitesse = []string{
	"cordialement", "bien cordialement", "bien à vous", "sincèrement",
	"respectueusement", "salutations", "merci d'avance",
}

// retirerSignatureModele coupe tout ce qui suit la formule de politesse finale.
// On garde la formule, qui fait partie du message, et on jette le nom éventuel.
func retirerSignatureModele(corps string) string {
	lignes := strings.Split(corps, "\n")
	for i := len(lignes) - 1; i >= 0 && i >= len(lignes)-6; i-- {
		l := strings.ToLower(strings.TrimRight(strings.TrimSpace(lignes[i]), ",.!"))
		if l == "" {
			continue
		}
		for _, f := range formulesPolitesse {
			if l == f {
				return strings.TrimRight(strings.Join(lignes[:i+1], "\n"), " \n\t")
			}
		}
	}
	return corps
}

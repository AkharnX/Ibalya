package engine

import (
	"regexp"
	"strings"
)

// Rapprochement de deux objets d'engagement. Le modèle reformule d'un message
// à l'autre (« Livraison des 12 vitrages doubles », « Livrer 12 vitrages
// doubles à l'atelier ») : on compare des racines de mots, sans les mots
// vides ni les verbes d'action génériques qui se retrouvent partout.

var motsVides = map[string]bool{
	"le": true, "la": true, "les": true, "un": true, "une": true, "des": true, "du": true, "de": true,
	"au": true, "aux": true, "et": true, "ou": true, "pour": true, "par": true, "chez": true, "dans": true,
	"sur": true, "sous": true, "avec": true, "sans": true, "vers": true, "en": true, "a": true, "d": true,
	"l": true, "ce": true, "cet": true, "cette": true, "ces": true, "son": true, "sa": true, "ses": true,
	"mon": true, "ma": true, "mes": true, "votre": true, "vos": true, "notre": true, "nos": true, "leur": true,
	"the": true, "of": true, "to": true, "for": true, "m": true, "mme": true, "mr": true,
}

// Racines (quatre lettres) des verbes et noms d'action trop génériques pour
// rapprocher deux objets à eux seuls : « envoyer devis pergola » et « envoyer
// devis véranda » ne sont pas la même promesse.
var racinesGeneriques = map[string]bool{
	"envo": true, "rela": true, "livr": true, "pose": true, "conf": true, "rapp": true, "appe": true,
	"suiv": true, "poin": true, "fair": true, "mett": true, "donn": true, "tran": true, "prop": true,
	"fini": true, "term": true, "prep": true, "lanc": true, "fixe": true, "pren": true,
}

var nonLettres = regexp.MustCompile(`[^a-z0-9]+`)

// racines découpe un objet en racines de mots (quatre lettres, ou le nombre
// entier), sans accents ni mots vides. Renvoie aussi les racines dites
// significatives (hors verbes génériques).
func racines(objet string) (toutes, significatives map[string]bool) {
	toutes, significatives = map[string]bool{}, map[string]bool{}
	texte := nonLettres.ReplaceAllString(sansAccents.Replace(strings.ToLower(objet)), " ")
	for _, mot := range strings.Fields(texte) {
		if motsVides[mot] || (len(mot) < 3 && !estNombre(mot)) {
			continue
		}
		r := mot
		if !estNombre(mot) && len(r) > 4 {
			r = r[:4]
		}
		toutes[r] = true
		if !racinesGeneriques[r] {
			significatives[r] = true
		}
	}
	return
}

func estNombre(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ObjetsProches dit si deux objets désignent vraisemblablement la même
// promesse : recouvrement de Jaccard d'au moins 0,6, ou le plus petit ensemble
// de racines inclus à 80 % dans l'autre. Le calcul se fait sur les racines
// significatives ; si l'un des objets n'en a aucune (« Relance »), sur toutes.
func ObjetsProches(a, b string) bool {
	ta, sa := racines(a)
	tb, sb := racines(b)
	if len(ta) == 0 || len(tb) == 0 {
		return false
	}
	xa, xb := sa, sb
	if len(sa) == 0 || len(sb) == 0 {
		xa, xb = ta, tb
	}
	inter := 0
	for r := range xa {
		if xb[r] {
			inter++
		}
	}
	if inter == 0 {
		return false
	}
	union := len(xa) + len(xb) - inter
	petit := len(xa)
	if len(xb) < petit {
		petit = len(xb)
	}
	return float64(inter)/float64(union) >= 0.6 || float64(inter)/float64(petit) >= 0.8
}

// typesCompatibles : deux engagements du même type, ou dont l'un n'a pas de
// type précis, peuvent être la même promesse.
func typesCompatibles(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	return a == b || a == "" || b == "" || a == "autre" || b == "autre"
}

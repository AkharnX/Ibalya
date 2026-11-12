package engine

import (
	"context"
	"encoding/json"
	"strings"
)

// Typage des interlocuteurs sans saisie ligne à ligne : le domaine de
// l'entreprise donne les internes, la capsule donne clients récurrents et
// fournisseurs critiques. Le reste garde « autre », on ne devine pas.

// MessageriesGrandPublic : un domaine partagé par tout le monde ne dit rien
// de l'entreprise. Jamais « interne », jamais typé d'un bloc par domaine.
var MessageriesGrandPublic = map[string]bool{
	"gmail.com": true, "googlemail.com": true, "outlook.com": true, "outlook.fr": true,
	"hotmail.com": true, "hotmail.fr": true, "live.com": true, "live.fr": true, "msn.com": true,
	"yahoo.com": true, "yahoo.fr": true, "orange.fr": true, "wanadoo.fr": true, "free.fr": true,
	"sfr.fr": true, "neuf.fr": true, "laposte.net": true, "bbox.fr": true, "icloud.com": true,
	"me.com": true, "mac.com": true, "protonmail.com": true, "proton.me": true, "pm.me": true,
	"gmx.com": true, "gmx.fr": true, "aol.com": true, "numericable.fr": true,
}

// Domaine rend la partie après l'arobase, en minuscules, ou "" sans arobase.
func Domaine(email string) string {
	e := strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(e, "@")
	if at < 0 || at == len(e)-1 {
		return ""
	}
	return e[at+1:]
}

// dansListe dit si l'adresse, ou son domaine, figure dans une liste de la
// capsule (adresses ou domaines, en casse libre).
func dansListe(email, domaine string, liste []string) bool {
	for _, v := range liste {
		v = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(v, "@")))
		if v == "" {
			continue
		}
		if v == email || (domaine != "" && v == domaine) {
			return true
		}
	}
	return false
}

// TyperInterlocuteur applique les règles, dans l'ordre : adresse du dirigeant
// ou domaine du compte (hors messageries grand public) → interne ; clients
// récurrents → client ; fournisseurs critiques → fournisseur ; sinon "".
func TyperInterlocuteur(email, compte string, soi map[string]bool, clients, fournisseurs []string) string {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" {
		return ""
	}
	d := Domaine(e)
	if soi[e] {
		return "interne"
	}
	if dc := Domaine(compte); dc != "" && d == dc && !MessageriesGrandPublic[d] {
		return "interne"
	}
	if dansListe(e, d, clients) {
		return "client"
	}
	if dansListe(e, d, fournisseurs) {
		return "fournisseur"
	}
	return ""
}

// listesCapsule lit clients récurrents et fournisseurs critiques dans les
// faits de la capsule. Les listes y sont des tableaux de chaînes, ou une
// chaîne par ligne selon l'écran qui les a saisies.
func (e *Engine) listesCapsule(ctx context.Context) (clients, fournisseurs []string) {
	c, err := e.Store.GetCapsule(ctx)
	if err != nil || c == nil || len(c.Facts) == 0 {
		return nil, nil
	}
	var facts map[string]json.RawMessage
	if json.Unmarshal(c.Facts, &facts) != nil {
		return nil, nil
	}
	return listeDe(facts["clients_recurrents"]), listeDe(facts["fournisseurs_critiques"])
}

func listeDe(brut json.RawMessage) []string {
	if len(brut) == 0 {
		return nil
	}
	var liste []string
	if json.Unmarshal(brut, &liste) == nil {
		return liste
	}
	var texte string
	if json.Unmarshal(brut, &texte) == nil {
		return strings.FieldsFunc(texte, func(r rune) bool { return r == '\n' || r == ',' || r == ';' })
	}
	return nil
}

// TypeAutomatique rend le type déduit pour une adresse du tenant courant.
func (e *Engine) TypeAutomatique(ctx context.Context, email string) string {
	compte, _ := e.Canal(ctx).AccountEmail(ctx)
	clients, fournisseurs := e.listesCapsule(ctx)
	return TyperInterlocuteur(email, compte, e.Store.AdressesSoi(ctx, compte), clients, fournisseurs)
}

// TyperAutomatiquement passe tous les interlocuteurs encore « autre » au
// crible des règles et rend le nombre de types posés.
func (e *Engine) TyperAutomatiquement(ctx context.Context) (int, error) {
	personnes, err := e.Store.ListPersons(ctx)
	if err != nil {
		return 0, err
	}
	compte, _ := e.Canal(ctx).AccountEmail(ctx)
	soi := e.Store.AdressesSoi(ctx, compte)
	clients, fournisseurs := e.listesCapsule(ctx)
	n := 0
	for _, p := range personnes {
		if p.Type != "" && p.Type != "autre" {
			continue
		}
		t := TyperInterlocuteur(p.Email, compte, soi, clients, fournisseurs)
		if t == "" {
			continue
		}
		if err := e.Store.SetPersonTypeIfAutre(ctx, p.ID, t); err == nil {
			n++
		}
	}
	if n > 0 {
		e.Store.Audit(ctx, "agent", "interlocuteurs_types_auto", map[string]int{"modifies": n})
	}
	return n, nil
}

// typerSiNouveau pose le type déduit sur un interlocuteur encore « autre ».
func (e *Engine) typerSiNouveau(ctx context.Context, id int64, email string) {
	if t := e.TypeAutomatique(ctx, email); t != "" {
		_ = e.Store.SetPersonTypeIfAutre(ctx, id, t)
	}
}

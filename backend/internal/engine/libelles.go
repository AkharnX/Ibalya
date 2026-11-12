package engine

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Jours écrit un nombre de jours avec l'accord : « 1 jour », « 3 jours ».
// Plus de « jour(s) » dans ce qu'on montre au dirigeant.
func Jours(n int) string {
	if n == 1 || n == -1 || n == 0 {
		return fmt.Sprintf("%d jour", n)
	}
	return fmt.Sprintf("%d jours", n)
}

// joursCalendaires compte les jours de calendrier entre now et t, dans le
// fuseau de now. Une durée en heures divisée par 24 tronque vers zéro : une
// échéance ce soir et une échéance dépassée de quelques heures donnaient
// toutes deux « 0 jour ». Ici on compare des dates à minuit.
func joursCalendaires(t, now time.Time) int {
	loc := now.Location()
	a := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	tt := t.In(loc)
	b := time.Date(tt.Year(), tt.Month(), tt.Day(), 0, 0, 0, 0, loc)
	return int(b.Sub(a).Hours() / 24)
}

// LibelleEcheance situe une échéance par rapport à maintenant, en français
// courant : « aujourd'hui », « demain », « hier », « dans 3 jours »,
// « en retard de 6 jours ».
func LibelleEcheance(echeance, now time.Time) string {
	switch d := joursCalendaires(echeance, now); {
	case d == 0:
		return "aujourd'hui"
	case d == 1:
		return "demain"
	case d == -1:
		return "hier"
	case d > 1:
		return "dans " + Jours(d)
	default:
		return "en retard de " + Jours(-d)
	}
}

var dateISO = regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)

// NormaliserDatesObjet réécrit les dates ISO que le modèle laisse parfois
// dans un objet (« Livraison 2026-10-01 ») au format français : « 01/10 »
// dans l'année en cours, « 01/10/2027 » sinon.
func NormaliserDatesObjet(objet string, now time.Time) string {
	return dateISO.ReplaceAllStringFunc(objet, func(m string) string {
		p := dateISO.FindStringSubmatch(m)
		annee, _ := strconv.Atoi(p[1])
		if annee == now.Year() {
			return p[3] + "/" + p[2]
		}
		return p[3] + "/" + p[2] + "/" + p[1]
	})
}

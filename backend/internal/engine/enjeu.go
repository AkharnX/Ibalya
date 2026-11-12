package engine

import (
	"sort"
	"time"
)

// Enjeu d'une priorité, pour n'ouvrir la synthèse que sur ce qui compte.
// Barème : en retard et je dois, 3 ; en retard et on me doit, 2 ; retard
// probable (bloqué par un amont), 2 ; interlocuteur typé client, +1 ;
// priorité haute, +1 ; échéance dans les deux jours, +1. Ce qui ne concerne
// pas le dirigeant vaut 0.
func Enjeu(x EngagementSuivi, typeContact string, now time.Time) int {
	if x.Sens == SensPourInfo {
		return 0
	}
	score := 0
	switch x.Categorie {
	case CatRetard:
		if x.Sens == SensJeDois {
			score += 3
		} else {
			score += 2
		}
	case CatRisque:
		score += 2
	}
	if typeContact == "client" {
		score++
	}
	if x.Priorite == "haute" {
		score++
	}
	if x.Echeance != nil {
		if d := joursCalendaires(*x.Echeance, now); d >= 0 && d <= 2 {
			score++
		}
	}
	return score
}

// trierParEnjeu rend une copie triée par enjeu décroissant, ordre d'origine
// conservé à enjeu égal.
func trierParEnjeu(items []PrioriteItem) []PrioriteItem {
	out := make([]PrioriteItem, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, k int) bool { return out[i].Enjeu > out[k].Enjeu })
	return out
}

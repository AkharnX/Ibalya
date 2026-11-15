package engine

// PersonnePertinente dit si un interlocuteur mérite d'apparaître par défaut
// dans le carnet : jamais un automate ; sinon, il faut un engagement avec le
// dirigeant, ou des messages dans les deux sens. Les adresses en copie qui
// n'ont jamais répondu et les expéditeurs à qui on n'a jamais écrit restent
// accessibles sous « Tous ».
func PersonnePertinente(recus, envoyes, engagements int, automatique bool) bool {
	if automatique {
		return false
	}
	return engagements > 0 || (recus > 0 && envoyes > 0)
}

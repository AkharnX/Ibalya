package engine

import (
	"regexp"
	"strings"
)

// Fiabilité d'un engagement extrait.
//
// Le score était auparavant celui que le modèle déclarait sur lui-même. Mesuré
// sur la base de recette, il était inversé : les engagements notés au-dessus de
// 0,90 étaient corrigés par le dirigeant dans 53 % des cas, contre 9 % pour les
// autres. C'est attendu — demander à un modèle sa propre certitude mesure son
// aisance à formuler, pas son exactitude — et ça se voyait dans la
// distribution, faite de valeurs rondes dont douze fois 0,95.
//
// Le score est désormais construit à partir de signaux vérifiables sans le
// modèle, dont l'avis ne pèse plus qu'un dixième. Chaque signal est enregistré
// avec l'engagement, pour que le score puisse s'expliquer et, plus tard, se
// calibrer sur les corrections réelles du dirigeant.

// SignauxFiabilite rassemble ce qu'on sait de l'engagement au moment de sa
// création, hors avis du modèle.
type SignauxFiabilite struct {
	// AvisModele : la confiance déclarée par le modèle, entre 0 et 1.
	AvisModele float64
	// EcheanceExplicite : une date écrite dans le texte, pas déduite.
	EcheanceExplicite bool
	// FormulationEngageante : le texte source contient une tournure
	// d'engagement explicite, pas une intention vague.
	FormulationEngageante bool
	// FilConversationnel : le fil compte au moins un aller-retour, ce qui le
	// distingue d'une diffusion à sens unique.
	FilConversationnel bool
	// InterlocuteurConnu : au moins trois échanges antérieurs avec lui.
	InterlocuteurConnu bool
	// TauxCorrectionInterlocuteur : part des engagements déjà extraits de cet
	// interlocuteur que le dirigeant a corrigés ou abandonnés, entre 0 et 1.
	// Vaut -1 quand l'historique est trop mince pour conclure.
	TauxCorrectionInterlocuteur float64
}

// poids de chaque signal. Ils somment à 1 pour qu'un engagement portant toutes
// les marques d'un vrai engagement atteigne 1,00.
const (
	poidsBase            = 0.35
	poidsEcheance        = 0.20
	poidsFormulation     = 0.15
	poidsFilConversation = 0.10
	poidsInterlocuteur   = 0.10
	// L'avis du modèle est plafonné à un dixième : il apporte quelque chose,
	// mais ne doit plus pouvoir emporter la décision à lui seul.
	poidsAvisModele = 0.10
	// Pénalité maximale quand un interlocuteur produit surtout des faux
	// positifs. C'est le seul signal qui apprend de l'usage réel.
	penaliteCorrectionMax = 0.20
	// En dessous de cet historique, le taux de correction n'est pas
	// significatif et n'est pas appliqué.
	minimumHistorique = 4
)

// tournures d'engagement explicites, dans les deux sens : celles par lesquelles
// on se lie, et celles par lesquelles on lie l'autre. Une promesse écrite
// « je vous livre jeudi » vaut mieux qu'un « il faudrait qu'on avance ».
var formulationEngageante = regexp.MustCompile(`(?i)(` +
	// l'émetteur s'engage
	`\bje (vous |t')?(livre|envoie|enverrai|transmets|transmettrai|confirme|confirmerai|reviens|rappelle|rappellerai|m'engage|ferai|fais)\b|` +
	`\bnous (vous )?(livrons|livrerons|envoyons|enverrons|transmettons|confirmons|confirmerons|nous engageons)\b|` +
	`\bon (vous )?(livre|envoie|transmet|confirme|rappelle)\b|` +
	`\b(ce sera|ce sera fait|c'est noté|entendu, je)\b|` +
	// l'émetteur engage le destinataire
	`\b(merci de|veuillez|il faudra|vous devez|vous devrez|nous attendons de vous|nous comptons sur vous)\b|` +
	`\b(avant le|d'ici le|au plus tard le|pour le)\s+\d` +
	`)`)

// FormulationEstEngageante dit si le texte source porte une tournure
// d'engagement explicite.
func FormulationEstEngageante(texte string) bool {
	if strings.TrimSpace(texte) == "" {
		return false
	}
	return formulationEngageante.MatchString(texte)
}

// CalculerFiabilite retourne le score et le détail des signaux retenus. Le
// détail est stocké avec l'engagement : un score qu'on ne peut pas expliquer
// ne vaut pas mieux que celui qu'il remplace.
func CalculerFiabilite(s SignauxFiabilite) (float64, map[string]any) {
	score := poidsBase
	detail := map[string]any{"base": poidsBase}

	if s.EcheanceExplicite {
		score += poidsEcheance
		detail["echeance_explicite"] = poidsEcheance
	}
	if s.FormulationEngageante {
		score += poidsFormulation
		detail["formulation_engageante"] = poidsFormulation
	}
	if s.FilConversationnel {
		score += poidsFilConversation
		detail["fil_conversationnel"] = poidsFilConversation
	}
	if s.InterlocuteurConnu {
		score += poidsInterlocuteur
		detail["interlocuteur_connu"] = poidsInterlocuteur
	}

	avis := clamp01(s.AvisModele) * poidsAvisModele
	score += avis
	detail["avis_modele"] = arrondir(avis)

	if s.TauxCorrectionInterlocuteur >= 0 {
		penalite := clamp01(s.TauxCorrectionInterlocuteur) * penaliteCorrectionMax
		score -= penalite
		detail["penalite_corrections"] = -arrondir(penalite)
	}

	score = clamp01(score)
	detail["score"] = arrondir(score)
	return arrondir(score), detail
}

// NiveauFiabilite traduit le score en un des trois niveaux affichés.
//
// Un pourcentage à deux décimales promet une précision que ce score n'a pas.
// Trois niveaux disent la même chose sans mentir sur ce qu'on sait.
func NiveauFiabilite(score float64) string {
	switch {
	case score >= 0.75:
		return "elevee"
	case score >= 0.50:
		return "a_verifier"
	default:
		return "incertaine"
	}
}

func arrondir(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

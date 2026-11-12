package ingest

import (
	"regexp"
	"strings"

	"ibalya/backend/internal/store"
)

// Classe d'un expéditeur, décidée avant toute extraction. Seuls les messages
// « humain » partent au modèle : un suivi de colis, une notification de
// logiciel ou un renouvellement d'abonnement ne promettent rien à personne,
// et un hameçonnage ne doit surtout pas devenir une promesse à relancer.
type Classe string

const (
	ClasseHumain       Classe = "humain"
	ClasseAutomatique  Classe = "automatique"
	ClassePublicitaire Classe = "publicitaire"
	ClasseSecurite     Classe = "securite"
)

// Adresses d'automates : robots d'envoi, notifications, suivis d'expédition.
// Le motif porte sur l'adresse entière (partie locale et domaine), ce qui
// attrape aussi « @notification.operateur.fr ». Les adresses de service d'un
// vrai fournisseur (commandes@, facturation@) restent humaines : ce sont des
// personnes qui y répondent, et c'est souvent là qu'arrive « facture jointe ».
var adresseAutomatique = regexp.MustCompile(`(?i)(` +
	`no[-_.]?reply|do[-_.]?not[-_.]?reply|ne[-_. ]?pas[-_. ]?repondre|nepasrepondre|` +
	`mailer[-_.]?daemon|postmaster|bounces?@|notifications?|notifier|notify@|` +
	`alerts?@|alertes?@|robot@|automat[a-z]*@|auto[-_.]?(confirm|reply|notif)[a-z]*|` +
	`ship(ment|ping)?[-_.]?(tracking|confirm|notif)[a-z]*|tracking@|suivi[-_.]?colis|` +
	`orders?[-_.]?(confirmation|update|status|notification)|commandes?[-_.]?(auto|notification|confirmation)|` +
	`abonnements?@|subscriptions?@|system@|daemon@|newsletter|marketing@|espace[-_.]?client@|` +
	`info@.*\.(mailchimp|sendgrid|brevo)` +
	`)`)

// Domaines qui n'écrivent que par automate : plateformes d'envoi, outils,
// transporteurs, places de marché, hébergeurs, outils de gestion. Pas les
// opérateurs télécom : leurs commerciaux écrivent depuis ces domaines.
var domainesAutomatiques = []string{
	"amazonses.com", "sendgrid.net", "mailchimp.com", "mailgun.org", "brevo.com",
	"notifications.google.com", "docs.google.com", "calendar-notification.google.com",
	"github.com", "gitlab.com", "atlassian.com", "slack.com", "linkedin.com",
	"facebookmail.com", "twitter.com", "x.com", "stripe.com", "paypal.com",
	"amazon.fr", "amazon.com", "amazon.de", "cdiscount.com", "fnac.com",
	"chronopost.fr", "colissimo.fr", "laposte.fr", "ups.com", "dhl.com", "dhl.fr",
	"fedex.com", "tnt.fr", "tnt.com", "dpd.fr", "dpd.com", "gls-group.eu", "gls-france.com",
	"mondialrelay.fr", "ovh.com", "ovhcloud.com", "ovh.net", "ionos.fr", "ionos.com",
	"qonto.com", "qonto.eu", "pennylane.com", "sellsy.com", "axonaut.com",
}

// Phrases par lesquelles un automate se présente. Appliquées au corps
// normalisé (minuscules, sans accents). Une annonce d'expédition écrite par
// une personne n'en fait pas partie : c'est un signal utile, pas du bruit.
var corpsAutomatique = regexp.MustCompile(`(` +
	`message automatique|e-?mail automatique|courriel automatique|` +
	`ne pas repondre a ce|ne repondez pas a ce|merci de ne pas (y )?repondre|` +
	`envoye automatiquement|genere automatiquement|` +
	`do not reply|this is an automated|automated message` +
	`)`)

// Hameçonnage : signaux forts (suffisent seuls) et signaux faibles (il en
// faut deux). « Confirmez vos coordonnées » seul peut venir d'un vrai client.
var securiteForte = regexp.MustCompile(`(` +
	`connexion suspecte|tentative de connexion|activite (inhabituelle|suspecte)|` +
	`compte (sera|a ete|est|va etre) (suspendu|bloque|desactive|verrouille|ferme)|` +
	`mot de passe (expire|a expire|va expirer)|` +
	`unusual sign-?in|suspicious (activity|login|sign-?in)|verify your (account|identity)|` +
	`your account (will be|has been) (suspended|locked|closed)` +
	`)`)

var securiteFaible = regexp.MustCompile(`(` +
	`sous 24 ?h|dans les 24 ?heures|sous 48 ?h|` +
	`verifi(ez|er) votre (identite|compte)|confirm(ez|er) vos (informations|coordonnees|donnees)|` +
	`(mettre a jour|mise a jour de) vos informations de paiement|cliquez (ici|sur le lien) pour` +
	`)`)

// ClasserExpediteur rend la classe du message et le motif d'exclusion
// correspondant (vide pour un humain). Ordre : hameçonnage d'abord, parce que
// c'est l'étiquette la plus utile au dirigeant ; puis automate (en-tête,
// adresse, domaine, corps) ; puis publicité (en-tête de désinscription).
func ClasserExpediteur(m store.Message) (Classe, string) {
	texte := normaliser(m.Subject + "\n" + m.Body)
	if securiteForte.MatchString(texte) || len(securiteFaible.FindAllStringIndex(texte, -1)) >= 2 {
		return ClasseSecurite, "securite_suspect"
	}
	sender := strings.ToLower(strings.TrimSpace(m.Sender))
	if m.AutoSubmitted {
		return ClasseAutomatique, "expediteur_automatique"
	}
	if at := strings.LastIndex(sender, "@"); at >= 0 {
		domaine := sender[at+1:]
		for _, d := range domainesAutomatiques {
			if domaine == d || strings.HasSuffix(domaine, "."+d) {
				return ClasseAutomatique, "domaine_exclu"
			}
		}
	}
	if adresseAutomatique.MatchString(sender) || corpsAutomatique.MatchString(texte) {
		return ClasseAutomatique, "expediteur_automatique"
	}
	if m.ListUnsubscribe {
		return ClassePublicitaire, "newsletter"
	}
	return ClasseHumain, ""
}

package engine

import (
	"strings"
	"testing"
	"time"

	"ibalya/backend/internal/store"
)

func dans(jours int) *time.Time {
	t := time.Now().AddDate(0, 0, jours)
	return &t
}

// suggestAction décide À QUI l'agent propose d'écrire. Une erreur ici envoie
// une relance au mauvais interlocuteur — le défaut le plus visible du produit.
func TestSuggestAction(t *testing.T) {
	base := func(cat, typ string, sortant bool) EngagementSuivi {
		return EngagementSuivi{
			Engagement: store.Engagement{Type: typ},
			Categorie:  cat, Sortant: sortant, Contact: "contact@exemple.fr",
		}
	}

	t.Run("engagement bloqué : on relance la CAUSE, pas le client qui subit", func(t *testing.T) {
		s := base(CatRisque, "livraison", true)
		s.Blocage = &Blocage{AmontObjet: "Livraison vitrages", AmontEmetteur: "fournisseur@exemple.fr"}
		a := suggestAction(s)
		if a.ToEmail != "fournisseur@exemple.fr" {
			t.Errorf("destinataire attendu : le fournisseur amont, obtenu %q", a.ToEmail)
		}
		if a.Intent != "relance_cause" {
			t.Errorf("intention attendue relance_cause, obtenue %q", a.Intent)
		}
	})

	t.Run("rendez-vous bloqué : on écrit au client pour décaler", func(t *testing.T) {
		s := base(CatRisque, "rendez_vous", true)
		s.Blocage = &Blocage{AmontEmetteur: "fournisseur@exemple.fr"}
		a := suggestAction(s)
		if a.Intent != "reporter_rdv" || a.ToEmail != "contact@exemple.fr" {
			t.Errorf("attendu reporter_rdv vers le client, obtenu %s vers %s", a.Intent, a.ToEmail)
		}
	})

	t.Run("retard sur MA promesse : j'informe le client", func(t *testing.T) {
		if a := suggestAction(base(CatRetard, "livraison", true)); a.Intent != "info_retard" {
			t.Errorf("attendu info_retard, obtenu %q", a.Intent)
		}
	})

	// Renommée de relance_fournisseur : la contrepartie n'est pas toujours un
	// fournisseur, ce peut être un client dont on attend une validation.
	t.Run("retard sur SA promesse : je relance", func(t *testing.T) {
		if a := suggestAction(base(CatRetard, "livraison", false)); a.Intent != "relance_retard" {
			t.Errorf("attendu relance_retard, obtenu %q", a.Intent)
		}
	})

	t.Run("devis que je dois envoyer / que j'attends", func(t *testing.T) {
		if a := suggestAction(base(CatEnCours, "devis", true)); a.Intent != "envoi_devis" {
			t.Errorf("devis sortant : attendu envoi_devis, obtenu %q", a.Intent)
		}
		if a := suggestAction(base(CatEnCours, "devis", false)); a.Intent != "relance_devis" {
			t.Errorf("devis entrant : attendu relance_devis, obtenu %q", a.Intent)
		}
	})

	// L'échéance doit être confirmée pour déclencher l'urgence : une date
	// inférée par le modèle n'alimente rien tant que le dirigeant ne l'a pas
	// validée (CDC 7.3). Le cas non confirmé est couvert dans suggestion_test.go.
	t.Run("livraison imminente et confirmée : on confirme la date", func(t *testing.T) {
		s := base(CatEnCours, "livraison", true)
		s.Echeance = dans(1)
		s.EcheanceConfirmee = true
		if a := suggestAction(s); a.Intent != "confirmer_date" {
			t.Errorf("attendu confirmer_date, obtenu %q", a.Intent)
		}
	})

	t.Run("livraison lointaine : simple point d'avancement", func(t *testing.T) {
		s := base(CatEnCours, "livraison", true)
		s.Echeance = dans(20)
		s.EcheanceConfirmee = true
		if a := suggestAction(s); a.Intent != "point_avancement" {
			t.Errorf("attendu point_avancement, obtenu %q", a.Intent)
		}
	})

	t.Run("une action est toujours proposée", func(t *testing.T) {
		for _, typ := range []string{"autre", "facturation", "prise_de_contact", "rendez_vous", ""} {
			a := suggestAction(base(CatEnCours, typ, true))
			if a == nil || a.Label == "" || a.Intent == "" {
				t.Errorf("type %q : action incomplète", typ)
			}
		}
	})
}

// Les libellés affichés au dirigeant doivent rester lisibles quel que soit
// l'état de l'engagement — notamment sans échéance.
func TestLignesAffichees(t *testing.T) {
	s := EngagementSuivi{Engagement: store.Engagement{Objet: "Poser les fenêtres"}, Contact: "mairie@exemple.fr"}

	if l := apercuLigne(s); !strings.Contains(l, "Poser les fenêtres") {
		t.Errorf("l'aperçu doit citer l'objet, obtenu %q", l)
	}
	if c := contexteLigne(s); !strings.Contains(c, "sans échéance") {
		t.Errorf("sans échéance, le contexte doit le dire, obtenu %q", c)
	}

	s.Blocage = &Blocage{AmontObjet: "Livraison vitrages", AmontEcheance: "30/07/2026"}
	if c := contexteLigne(s); !strings.Contains(c, "bloquée par") || !strings.Contains(c, "Livraison vitrages") {
		t.Errorf("le contexte doit nommer la cause du blocage, obtenu %q", c)
	}

	s.Blocage = nil
	s.Statut = "en_retard"
	s.Echeance = dans(-3)
	if c := contexteLigne(s); !strings.Contains(c, "dépassée") {
		t.Errorf("un retard doit être annoncé comme tel, obtenu %q", c)
	}
}

func TestIntentLabelCouvreToutesLesIntentions(t *testing.T) {
	intentions := []string{
		"relance_cause", "relance_fournisseur", "info_retard", "relance_devis",
		"envoi_devis", "envoi_facture", "confirmer_rdv", "reporter_rdv",
		"relance_prospect", "confirmer_date", "point_avancement",
	}
	for _, i := range intentions {
		if l := intentLabel(i); l == "" || l == "Envoyer un message" {
			t.Errorf("intention %q sans libellé dédié (obtenu %q)", i, l)
		}
	}
	if intentLabel("inconnue") != "Envoyer un message" {
		t.Error("une intention inconnue doit retomber sur le libellé générique")
	}
}

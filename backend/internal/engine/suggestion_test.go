package engine

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"ibalya/backend/internal/store"
)

func ref() time.Time { return time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC) }

func eng(typ string, sortant bool, cat string) EngagementSuivi {
	return EngagementSuivi{
		Engagement: store.Engagement{Type: typ},
		Categorie:  cat, Sortant: sortant, Contact: "client@ex.fr",
	}
}

func avecEcheance(s EngagementSuivi, dans time.Duration) EngagementSuivi {
	t := ref().Add(dans)
	s.Echeance = &t
	s.EcheanceConfirmee = true
	return s
}

// Le défaut qui a motivé la réécriture : sept engagements sur huit portaient la
// même phrase, parce que « relance » et « autre » n'avaient aucune branche.
func TestTypesSansBrancheNeTombentPlusDansLeMemeCas(t *testing.T) {
	vus := map[string]string{}
	for _, c := range []struct {
		typ     string
		sortant bool
	}{
		{"relance", true}, {"relance", false},
		{"autre", true}, {"autre", false},
		{"livraison", true}, {"livraison", false},
		{"devis", true}, {"devis", false},
		{"facturation", true}, {"facturation", false},
	} {
		a := suggestActionA(eng(c.typ, c.sortant, CatEnCours), ref())
		if a == nil || a.Label == "" {
			t.Fatalf("%s/%v : aucune suggestion", c.typ, c.sortant)
		}
		cle := c.typ + "/" + map[bool]string{true: "sortant", false: "entrant"}[c.sortant]
		vus[cle] = a.Label
	}
	distinctes := map[string]bool{}
	for _, l := range vus {
		distinctes[l] = true
	}
	if len(distinctes) < 8 {
		t.Fatalf("seulement %d libellés distincts pour 10 situations : %v", len(distinctes), vus)
	}
	t.Logf("%d libellés distincts sur 10 situations", len(distinctes))
}

// Le sens de l'engagement change l'action : réclamer n'est pas envoyer.
func TestLeSensChangeLAction(t *testing.T) {
	for _, typ := range []string{"devis", "facturation", "livraison", "relance", "autre"} {
		sortant := suggestActionA(eng(typ, true, CatEnCours), ref())
		entrant := suggestActionA(eng(typ, false, CatEnCours), ref())
		if sortant.Label == entrant.Label {
			t.Errorf("%s : même suggestion que je doive ou que j'attende — %q", typ, sortant.Label)
		}
	}
}

// L'échéance proche prime sur le type : une promesse due demain et une due dans
// trois mois recevaient la même suggestion.
func TestEcheanceProchePrimeSurLeType(t *testing.T) {
	loin := suggestActionA(avecEcheance(eng("autre", true, CatEnCours), 90*24*time.Hour), ref())
	proche := suggestActionA(avecEcheance(eng("autre", true, CatEnCours), 24*time.Hour), ref())
	if loin.Label == proche.Label {
		t.Fatalf("échéance lointaine et imminente donnent la même chose : %q", loin.Label)
	}
	if proche.Intent != "confirmer_date" {
		t.Fatalf("intention attendue confirmer_date, obtenue %q", proche.Intent)
	}
}

// Une échéance inférée non confirmée ne doit pas déclencher l'urgence : le CDC
// exige la confirmation du dirigeant avant d'alimenter les détecteurs.
func TestEcheanceNonConfirmeeNeDeclenchePasLUrgence(t *testing.T) {
	s := avecEcheance(eng("autre", true, CatEnCours), 24*time.Hour)
	s.EcheanceConfirmee = false
	if a := suggestActionA(s, ref()); a.Intent == "confirmer_date" {
		t.Fatal("une échéance non confirmée a déclenché la branche d'urgence")
	}
}

// Un engagement bloqué vise la cause, pas le client qui la subit.
func TestBlocageViseLaCause(t *testing.T) {
	s := eng("livraison", true, CatRisque)
	s.Blocage = &Blocage{AmontEmetteur: "fournisseur@ex.fr", AmontObjet: "pose du vitrage"}
	a := suggestActionA(s, ref())
	if a.ToEmail != "fournisseur@ex.fr" {
		t.Fatalf("destinataire %q : l'action doit viser la cause du blocage", a.ToEmail)
	}
}

func TestRetardSelonLeSens(t *testing.T) {
	jeDois := suggestActionA(eng("livraison", true, CatRetard), ref())
	onMeDoit := suggestActionA(eng("livraison", false, CatRetard), ref())
	if jeDois.Intent != "info_retard" {
		t.Errorf("quand je suis en retard, il faut prévenir : %q", jeDois.Intent)
	}
	if onMeDoit.Intent != "relance_retard" {
		t.Errorf("quand on me doit, il faut relancer : %q", onMeDoit.Intent)
	}
}

// Toute situation reçoit un destinataire : un engagement sans destinataire ne
// produit aucun brouillon.
func TestToujoursUnDestinataire(t *testing.T) {
	for _, cat := range []string{CatEnCours, CatRetard, CatRisque} {
		for _, typ := range []string{"devis", "livraison", "relance", "autre", "rendez_vous"} {
			a := suggestActionA(eng(typ, true, cat), ref())
			if a.ToEmail == "" {
				t.Errorf("%s/%s : aucun destinataire", cat, typ)
			}
		}
	}
}

// Le prompt du service d'inférence décrit chaque intention une par une. Une
// intention émise par le moteur mais absente du prompt donne un libellé
// distinct dans l'interface et un message générique dans la boîte du client :
// la correction paraît faite alors qu'elle ne l'est pas.
func TestToutesLesIntentionsSontDecritesDansLePrompt(t *testing.T) {
	moteur := map[string]bool{}
	src, err := os.ReadFile("suivi.go")
	if err != nil {
		t.Skipf("source illisible : %v", err)
	}
	for _, m := range regexp.MustCompile(`vers\("[^"]*", "([a-z_]+)"`).FindAllStringSubmatch(string(src), -1) {
		moteur[m[1]] = true
	}
	if len(moteur) == 0 {
		t.Fatal("aucune intention trouvée dans suivi.go : le motif de lecture est à revoir")
	}

	prompt, err := os.ReadFile("../../../llm-service/app/prompts.py")
	if err != nil {
		t.Skipf("prompt illisible : %v", err)
	}
	decrites := map[string]bool{}
	for _, l := range strings.Split(string(prompt), "\n") {
		if m := regexp.MustCompile(`^- ([a-z_ /]+) :`).FindStringSubmatch(l); m != nil {
			for _, x := range strings.Split(m[1], "/") {
				decrites[strings.TrimSpace(x)] = true
			}
		}
	}
	for i := range moteur {
		if !decrites[i] {
			t.Errorf("l'intention %q est émise par le moteur mais n'est décrite nulle part dans le prompt", i)
		}
	}
	t.Logf("%d intentions émises, toutes décrites", len(moteur))
}

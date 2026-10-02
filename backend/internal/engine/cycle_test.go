package engine

import (
	"context"
	"testing"
	"time"
)

// L'état de cycle est cloisonné par boîte (tenant). Hors contexte de tenant, la
// clé est 0 : c'est ce que ces tests exercent.

func poserEtat(c EtatCycle) {
	etatMu.Lock()
	etats[0] = c
	etatMu.Unlock()
}

// L'état est ce que l'interface interroge pour savoir si l'agent travaille.
func TestEtatCycleAuRepos(t *testing.T) {
	poserEtat(EtatCycle{})

	e := &Engine{}
	if c := e.Etat(context.Background()); c.EnCours {
		t.Fatal("aucun cycle ne tourne, EnCours devrait être faux")
	}
}

func TestEtatCyclePendantExecution(t *testing.T) {
	poserEtat(EtatCycle{EnCours: true, Phase: "Analyse des messages par le modèle",
		Origine: "dirigeant", Debut: time.Now().Add(-42 * time.Second)})
	defer poserEtat(EtatCycle{})

	c := (&Engine{}).Etat(context.Background())
	if !c.EnCours {
		t.Fatal("un cycle tourne, EnCours devrait être vrai")
	}
	if c.Phase == "" {
		t.Fatal("la phase doit être annoncée, c'est ce que lit l'utilisateur")
	}
	if c.Secondes < 41 || c.Secondes > 44 {
		t.Fatalf("temps écoulé calculé à %d s, attendu autour de 42", c.Secondes)
	}
	if c.Origine != "dirigeant" {
		t.Fatalf("origine %q : l'interface distingue un cycle demandé d'un cycle automatique", c.Origine)
	}
}

// Le temps écoulé n'a de sens que pendant un cycle.
func TestSecondesNonCalculeesHorsCycle(t *testing.T) {
	poserEtat(EtatCycle{EnCours: false, Debut: time.Now().Add(-10 * time.Minute)})
	defer poserEtat(EtatCycle{})

	if c := (&Engine{}).Etat(context.Background()); c.Secondes != 0 {
		t.Fatalf("hors cycle, le compteur doit rester à zéro, obtenu %d", c.Secondes)
	}
}

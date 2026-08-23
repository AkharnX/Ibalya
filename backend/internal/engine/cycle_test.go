package engine

import (
	"testing"
	"time"
)

// L'état est ce que l'interface interroge pour savoir si l'agent travaille.
func TestEtatCycleAuRepos(t *testing.T) {
	etatMu.Lock()
	etat = EtatCycle{}
	etatMu.Unlock()

	e := &Engine{}
	if c := e.Etat(); c.EnCours {
		t.Fatal("aucun cycle ne tourne, EnCours devrait être faux")
	}
}

func TestEtatCyclePendantExecution(t *testing.T) {
	etatMu.Lock()
	etat = EtatCycle{EnCours: true, Phase: "Analyse des messages par le modèle",
		Origine: "dirigeant", Debut: time.Now().Add(-42 * time.Second)}
	etatMu.Unlock()
	defer func() { etatMu.Lock(); etat = EtatCycle{}; etatMu.Unlock() }()

	c := (&Engine{}).Etat()
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
	etatMu.Lock()
	etat = EtatCycle{EnCours: false, Debut: time.Now().Add(-10 * time.Minute)}
	etatMu.Unlock()
	defer func() { etatMu.Lock(); etat = EtatCycle{}; etatMu.Unlock() }()

	if c := (&Engine{}).Etat(); c.Secondes != 0 {
		t.Fatalf("hors cycle, le compteur doit rester à zéro, obtenu %d", c.Secondes)
	}
}

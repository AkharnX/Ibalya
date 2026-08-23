package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// CycleResult résume un cycle complet ingestion → extraction → graphe → détection.
type CycleResult struct {
	Ingestion  any       `json:"ingestion,omitempty"`
	Extraction any       `json:"extraction"`
	Liens      int       `json:"liens_candidats"`
	Detection  any       `json:"detection"`
	Duree      string    `json:"duree"`
	TermineLe  time.Time `json:"termine_le"`
	Erreur     string    `json:"erreur,omitempty"`
}

var cycleMu sync.Mutex

// EtatCycle décrit ce que l'agent est en train de faire.
//
// Un cycle enchaîne la lecture de la boîte, jusqu'à huit appels au modèle, le
// graphe et les détecteurs : il dure de trente secondes à deux minutes. Le
// scheduler en lance un toutes les quinze minutes. Sans cet état, l'interface
// ne peut rien montrer de ce travail, ni celui déclenché par le dirigeant, ni
// celui de l'arrière-plan.
type EtatCycle struct {
	EnCours   bool      `json:"en_cours"`
	Phase     string    `json:"phase,omitempty"`
	Origine   string    `json:"origine,omitempty"` // dirigeant | automatique
	Debut     time.Time `json:"debut,omitempty"`
	Secondes  int       `json:"secondes"`
	TermineLe time.Time `json:"termine_le,omitempty"`
	Duree     string    `json:"derniere_duree,omitempty"`
}

var (
	etatMu sync.RWMutex
	etat   EtatCycle
)

func majPhase(phase string) {
	etatMu.Lock()
	etat.Phase = phase
	etatMu.Unlock()
}

// Etat retourne une copie de l'état courant, avec le temps écoulé calculé.
func (e *Engine) Etat() EtatCycle {
	etatMu.RLock()
	defer etatMu.RUnlock()
	c := etat
	if c.EnCours && !c.Debut.IsZero() {
		c.Secondes = int(time.Since(c.Debut).Seconds())
	}
	return c
}

// RunCycle exécute la chaîne complète. ingestFn est fourni par l'appelant
// (il dépend du connecteur) ; nil pour ne faire que l'aval.
func (e *Engine) RunCycle(ctx context.Context, ingestFn func(context.Context) (any, error)) CycleResult {
	return e.runCycle(ctx, ingestFn, "automatique")
}

// RunCycleOrigine permet de distinguer un cycle demandé par le dirigeant de
// celui du scheduler : l'interface n'annonce pas la même chose dans les deux cas.
func (e *Engine) RunCycleOrigine(ctx context.Context, ingestFn func(context.Context) (any, error), origine string) CycleResult {
	return e.runCycle(ctx, ingestFn, origine)
}

func (e *Engine) runCycle(ctx context.Context, ingestFn func(context.Context) (any, error), origine string) CycleResult {
	if !cycleMu.TryLock() {
		return CycleResult{Erreur: "un cycle est déjà en cours"}
	}
	defer cycleMu.Unlock()

	start := time.Now()
	res := CycleResult{}

	etatMu.Lock()
	etat = EtatCycle{EnCours: true, Phase: "Démarrage", Origine: origine, Debut: start,
		TermineLe: etat.TermineLe, Duree: etat.Duree}
	etatMu.Unlock()
	defer func() {
		etatMu.Lock()
		etat = EtatCycle{EnCours: false, TermineLe: time.Now(),
			Duree: time.Since(start).Round(time.Second).String()}
		etatMu.Unlock()
	}()

	if ingestFn != nil {
		majPhase("Lecture de la boîte de réception")
		ing, err := ingestFn(ctx)
		if err != nil {
			res.Erreur = fmt.Sprintf("ingestion: %v", err)
			log.Printf("cycle: %s", res.Erreur)
		}
		res.Ingestion = ing
	}

	majPhase("Analyse des messages par le modèle")
	ext, err := e.RunExtraction(ctx, 8)
	if err != nil && res.Erreur == "" {
		res.Erreur = fmt.Sprintf("extraction: %v", err)
		log.Printf("cycle: %s", res.Erreur)
	}
	res.Extraction = ext

	majPhase("Recherche des dépendances")
	links, err := e.RunGraphHeuristics(ctx)
	if err != nil && res.Erreur == "" {
		res.Erreur = fmt.Sprintf("graphe: %v", err)
	}
	res.Liens = links

	majPhase("Surveillance des engagements")
	det, err := e.RunDetectors(ctx)
	if err != nil && res.Erreur == "" {
		res.Erreur = fmt.Sprintf("détecteurs: %v", err)
	}
	res.Detection = det

	res.Duree = time.Since(start).Round(time.Millisecond).String()
	res.TermineLe = time.Now()
	e.Store.SetSetting(ctx, "dernier_cycle", res.TermineLe.Format(time.RFC3339))
	return res
}

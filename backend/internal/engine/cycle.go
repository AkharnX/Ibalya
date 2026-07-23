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

// RunCycle exécute la chaîne complète. ingestFn est fourni par l'appelant
// (il dépend du connecteur) ; nil pour ne faire que l'aval.
func (e *Engine) RunCycle(ctx context.Context, ingestFn func(context.Context) (any, error)) CycleResult {
	if !cycleMu.TryLock() {
		return CycleResult{Erreur: "un cycle est déjà en cours"}
	}
	defer cycleMu.Unlock()

	start := time.Now()
	res := CycleResult{}

	if ingestFn != nil {
		ing, err := ingestFn(ctx)
		if err != nil {
			res.Erreur = fmt.Sprintf("ingestion: %v", err)
			log.Printf("cycle: %s", res.Erreur)
		}
		res.Ingestion = ing
	}

	ext, err := e.RunExtraction(ctx, 8)
	if err != nil && res.Erreur == "" {
		res.Erreur = fmt.Sprintf("extraction: %v", err)
		log.Printf("cycle: %s", res.Erreur)
	}
	res.Extraction = ext

	links, err := e.RunGraphHeuristics(ctx)
	if err != nil && res.Erreur == "" {
		res.Erreur = fmt.Sprintf("graphe: %v", err)
	}
	res.Liens = links

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

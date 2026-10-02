package engine

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"ibalya/backend/internal/store"
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

// L'état de cycle est cloisonné PAR BOÎTE (tenant) : sans cela, un cycle lancé
// sur une boîte affichait « Analyse en cours » sur toutes les autres (et tous
// les comptes), puisque l'état était un singleton de process. La clé est l'id de
// l'espace courant, lu dans le contexte (posé par EnTenant).
var (
	etatMu sync.Mutex
	etats  = map[int64]EtatCycle{}
)

// cléTenant identifie la boîte du contexte courant ; 0 hors tenant (ne devrait
// pas arriver pour un cycle, mais évite un état partagé accidentel).
func cléTenant(ctx context.Context) int64 {
	if id, ok := store.TenantID(ctx); ok {
		return id
	}
	return 0
}

func majPhase(ctx context.Context, phase string) {
	id := cléTenant(ctx)
	etatMu.Lock()
	e := etats[id]
	e.Phase = phase
	etats[id] = e
	etatMu.Unlock()
}

// Etat retourne l'état de cycle de la boîte du contexte, temps écoulé calculé.
func (e *Engine) Etat(ctx context.Context) EtatCycle {
	id := cléTenant(ctx)
	etatMu.Lock()
	c := etats[id]
	etatMu.Unlock()
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
	// Verrou bloquant, pas TryLock : en multi-utilisateur, plusieurs cycles
	// légitimes se présentent (scheduler qui boucle sur les boîtes, onboarding
	// d'un nouveau raccordement). Les sauter en silence faisait perdre des
	// ingestions — un onboarding tombant pendant un cycle du scheduler ne lisait
	// jamais ses 30 jours. Ils s'exécutent maintenant chacun leur tour.
	cycleMu.Lock()
	defer cycleMu.Unlock()

	id := cléTenant(ctx)
	start := time.Now()
	res := CycleResult{}

	etatMu.Lock()
	prec := etats[id]
	etats[id] = EtatCycle{EnCours: true, Phase: "Démarrage", Origine: origine, Debut: start,
		TermineLe: prec.TermineLe, Duree: prec.Duree}
	etatMu.Unlock()
	defer func() {
		etatMu.Lock()
		etats[id] = EtatCycle{EnCours: false, TermineLe: time.Now(),
			Duree: time.Since(start).Round(time.Second).String()}
		etatMu.Unlock()
	}()

	if ingestFn != nil {
		majPhase(ctx, "Lecture de la boîte de réception")
		ing, err := ingestFn(ctx)
		if err != nil {
			res.Erreur = fmt.Sprintf("ingestion: %v", err)
			log.Printf("cycle: %s", res.Erreur)
		}
		res.Ingestion = ing
	}

	majPhase(ctx, "Analyse des messages par le modèle")
	ext, err := e.RunExtraction(ctx, 8)
	if err != nil && res.Erreur == "" {
		res.Erreur = fmt.Sprintf("extraction: %v", err)
		log.Printf("cycle: %s", res.Erreur)
	}
	res.Extraction = ext

	majPhase(ctx, "Recherche des dépendances")
	links, err := e.RunGraphHeuristics(ctx)
	if err != nil && res.Erreur == "" {
		res.Erreur = fmt.Sprintf("graphe: %v", err)
	}
	res.Liens = links

	majPhase(ctx, "Surveillance des engagements")
	det, err := e.RunDetectors(ctx)
	if err != nil && res.Erreur == "" {
		res.Erreur = fmt.Sprintf("détecteurs: %v", err)
	}
	res.Detection = det

	// Rafraîchit le miroir d'activité s'il existe déjà. Il est purement calculé
	// (SQL, sans appel LLM), donc peu coûteux ; le figer à l'onboarding le
	// rendait périmé au fil des nouveaux messages (retour de Stewe).
	if rep, _ := e.Store.LatestReport(ctx, "miroir"); rep != nil {
		if _, err := e.GenerateMiroir(ctx); err != nil {
			log.Printf("cycle: rafraîchissement miroir: %v", err)
		}
	}

	res.Duree = time.Since(start).Round(time.Millisecond).String()
	res.TermineLe = time.Now()
	e.Store.SetSetting(ctx, "dernier_cycle", res.TermineLe.Format(time.RFC3339))
	return res
}

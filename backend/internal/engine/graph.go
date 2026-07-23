package engine

import (
	"context"
	"fmt"
)

// RunGraphHeuristics propose des liens de dépendance candidats (CDC 8.1).
// Le LLM n'est JAMAIS l'autorité finale : les liens naissent d'heuristiques
// et ne servent aux détecteurs qu'une fois confirmés par le dirigeant.
//
// Heuristiques : même fil OU même paire d'interlocuteurs, échéance amont
// antérieure à l'échéance aval, proximité temporelle (≤ 21 jours).
func (e *Engine) RunGraphHeuristics(ctx context.Context) (int, error) {
	rows, err := e.Store.Pool.Query(ctx, `
		SELECT a.id, b.id,
		       CASE WHEN a.thread_id = b.thread_id THEN 'même fil'
		            ELSE 'mêmes interlocuteurs' END
		FROM engagements a
		JOIN engagements b ON a.id <> b.id
		WHERE a.statut IN ('ouvert','confirme','en_retard')
		  AND b.statut IN ('ouvert','confirme','en_retard')
		  AND a.echeance IS NOT NULL AND b.echeance IS NOT NULL
		  AND a.echeance < b.echeance
		  AND b.echeance - a.echeance <= 21
		  AND (
		        a.thread_id = b.thread_id
		     OR (a.emetteur_id IS NOT NULL AND (a.emetteur_id = b.destinataire_id OR a.emetteur_id = b.emetteur_id)
		         AND a.thread_id <> b.thread_id)
		  )
		  AND NOT EXISTS (SELECT 1 FROM dependency_links l WHERE l.amont_id=a.id AND l.aval_id=b.id)`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type cand struct {
		amont, aval int64
		raison      string
	}
	var cands []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.amont, &c.aval, &c.raison); err != nil {
			return 0, err
		}
		cands = append(cands, c)
	}
	rows.Close()
	created := 0
	for _, c := range cands {
		raison := fmt.Sprintf("Heuristique : %s, échéances rapprochées", c.raison)
		if err := e.Store.CreateLink(ctx, c.amont, c.aval, raison); err == nil {
			created++
		}
	}
	if created > 0 {
		e.Store.Audit(ctx, "agent", "liens_candidats_proposes", map[string]int{"nouveaux": created})
	}
	return created, nil
}

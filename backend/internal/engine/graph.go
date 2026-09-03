package engine

import (
	"context"
	"fmt"
	"log"

	"ibalya/backend/internal/llm"
)

// RunGraphHeuristics propose des liens de dépendance candidats (CDC 8.1).
//
// Deux étages. Une heuristique structurelle dresse d'abord une liste large :
// même fil ou mêmes interlocuteurs, échéance amont antérieure à l'aval,
// proximité temporelle (≤ 21 jours). Puis le modèle juge chaque candidat —
// l'aval dépend-il vraiment de l'amont ? — pour écarter les coïncidences que
// l'heuristique ne sait pas distinguer (deux abonnements, un rendez-vous et
// une démarche sans lien). Le modèle n'est JAMAIS l'autorité finale : il filtre
// et classe, le dirigeant confirme, et les détecteurs n'agissent que sur les
// liens confirmés.
//
// Le jugement reçoit les décisions passées du dirigeant comme exemples : ses
// confirmations et ses rejets orientent les suivants, sans aucun entraînement.
func (e *Engine) RunGraphHeuristics(ctx context.Context) (int, error) {
	rows, err := e.Store.Pool.Query(ctx, `
		SELECT a.id, b.id, a.objet, b.objet,
		       to_char(a.echeance,'DD/MM/YYYY'), to_char(b.echeance,'DD/MM/YYYY'),
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
		     -- chaîne type PME : le fournisseur promet AU dirigeant (amont), le
		     -- dirigeant promet à son client (aval)
		     OR (a.destinataire_id IS NOT NULL AND a.destinataire_id = b.emetteur_id
		         AND a.thread_id <> b.thread_id)
		  )
		  AND NOT EXISTS (SELECT 1 FROM dependency_links l WHERE l.amont_id=a.id AND l.aval_id=b.id)`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type cand struct {
		amont, aval           int64
		amontObjet, avalObjet string
		amontEch, avalEch     string
		raison                string
	}
	var cands []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.amont, &c.aval, &c.amontObjet, &c.avalObjet,
			&c.amontEch, &c.avalEch, &c.raison); err != nil {
			return 0, err
		}
		cands = append(cands, c)
	}
	rows.Close()
	if len(cands) == 0 {
		return 0, nil
	}

	// Décisions passées du dirigeant, en exemples pour le jugement.
	var exemples []llm.DependExemple
	if ex, err := e.Store.ExemplesDependance(ctx, 5); err == nil {
		for _, d := range ex {
			exemples = append(exemples, llm.DependExemple{Amont: d.Amont, Aval: d.Aval, Verdict: d.Verdict})
		}
	}

	// Sous ce score, le candidat n'est pas retenu : le modèle le juge trop
	// improbable pour valoir la peine que le dirigeant s'y arrête.
	const seuilRetenu = 0.5

	created := 0
	for _, c := range cands {
		raison := fmt.Sprintf("Heuristique : %s, échéances rapprochées", c.raison)
		score := 0.0

		rep, err := e.LLM.JugerDependance(ctx, llm.DependRequest{
			AmontObjet: c.amontObjet, AmontEcheance: c.amontEch,
			AvalObjet: c.avalObjet, AvalEcheance: c.avalEch,
			RaisonHeuristique: c.raison, Exemples: exemples,
		})
		switch {
		case err != nil:
			// Modèle injoignable : on ne perd pas le candidat, on le propose sur
			// la seule heuristique. Le dirigeant tranchera comme avant.
			log.Printf("dépendance: jugement indisponible, candidat proposé sur heuristique seule: %v", err)
		case !rep.Depend || rep.Score < seuilRetenu:
			// Jugé sans lien : on n'encombre pas le dirigeant avec.
			continue
		default:
			score = rep.Score
			if rep.Raison != "" {
				raison = rep.Raison
			}
		}

		if err := e.Store.CreateLink(ctx, c.amont, c.aval, raison, score); err == nil {
			created++
		}
	}
	if created > 0 {
		e.Store.Audit(ctx, "agent", "liens_candidats_proposes",
			map[string]int{"nouveaux": created, "examines": len(cands)})
	}
	return created, nil
}

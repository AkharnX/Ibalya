package engine

import (
	"context"
	"fmt"
	"log"
	"sort"

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
	rows, err := e.Store.Q(ctx).Query(ctx, `
		SELECT a.id, b.id, a.objet, b.objet, a.type, b.type,
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
	var cands []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.amont, &c.aval, &c.amontObjet, &c.avalObjet, &c.amontType, &c.avalType,
			&c.amontEch, &c.avalEch, &c.raison); err != nil {
			return 0, err
		}
		// Un rendez-vous ne bloque pas un devis : seules les paires de types
		// qui peuvent se bloquer vont au jugement.
		if !dependanceCompatible(c.amontType, c.avalType) {
			continue
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

	// Au plus maxJugementsParCycle appels au modèle : le reste attend le cycle
	// suivant. Les candidats structurels sont recalculés à chaque cycle.
	if len(cands) > maxJugementsParCycle {
		cands = cands[:maxJugementsParCycle]
	}

	var juges []candidatJuge
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
			// Modèle injoignable : on N'ÉCRIT PAS un candidat non jugé. Le faire
			// (score 0) noyait la liste de propositions bruyantes pendant une panne
			// LLM — exactement ce qui est arrivé lors d'une coupure de crédits.
			// Le candidat structurel est recalculé à chaque cycle : il sera jugé
			// au prochain, une fois le modèle de retour.
			log.Printf("dépendance: jugement indisponible, candidat ignoré ce cycle: %v", err)
			continue
		case !rep.Depend || rep.Score < seuilRetenu:
			// Jugé sans lien : on n'encombre pas le dirigeant avec. On MÉMORISE
			// toutefois le verdict (statut 'ecarte') pour ne plus jamais reposer
			// cette paire au modèle. Sans cette trace, une paire structurellement
			// candidate mais sans lien réel était re-jugée à chaque cycle, à vie :
			// des dizaines de milliers d'appels LLM pour zéro lien produit.
			motif := "Jugé sans lien par le modèle"
			if rep.Raison != "" {
				motif = rep.Raison
			}
			if err := e.Store.EcarterLien(ctx, c.amont, c.aval, motif, rep.Score); err != nil {
				log.Printf("dépendance: mémorisation du verdict négatif échouée (amont %d, aval %d): %v", c.amont, c.aval, err)
			}
			continue
		default:
			score = rep.Score
			if rep.Raison != "" {
				raison = rep.Raison
			}
		}
		juges = append(juges, candidatJuge{cand: c, score: score, raison: raison})
	}

	// Les mieux notés d'abord, cinq au plus : mieux vaut aucune proposition
	// qu'une proposition absurde (constat BS ACCESS : 12 proposées, 0 crédible).
	created := 0
	for _, j := range retenirCandidats(juges, seuilRetenu, maxCandidatsParCycle) {
		if err := e.Store.CreateLink(ctx, j.amont, j.aval, j.raison, j.score); err == nil {
			created++
		}
	}
	if created > 0 {
		e.Store.Audit(ctx, "agent", "liens_candidats_proposes",
			map[string]int{"nouveaux": created, "examines": len(cands)})
	}
	return created, nil
}

// Sous ce score, le candidat n'est pas retenu : le modèle le juge trop
// improbable pour valoir la peine que le dirigeant s'y arrête.
const seuilRetenu = 0.7

// Plafonds par cycle : appels au modèle, puis propositions créées.
const (
	maxJugementsParCycle = 15
	maxCandidatsParCycle = 5
)

type cand struct {
	amont, aval           int64
	amontObjet, avalObjet string
	amontType, avalType   string
	amontEch, avalEch     string
	raison                string
}

type candidatJuge struct {
	cand
	score  float64
	raison string
}

// Types qui peuvent se bloquer : l'amont est quelque chose qu'on attend
// (livraison, devis, facture), l'aval quelque chose qu'on a promis derrière.
// Un rendez-vous ne dépend que d'une livraison (la pose après la réception),
// jamais d'un devis ; un devis ou une prise de contact ne bloquent personne
// en aval. « autre » reste possible des deux côtés, faute de savoir.
var avalsParAmont = map[string]map[string]bool{
	"livraison":   {"livraison": true, "rendez_vous": true, "facturation": true, "autre": true, "": true},
	"devis":       {"livraison": true, "facturation": true, "autre": true, "": true},
	"facturation": {"livraison": true, "facturation": true, "autre": true, "": true},
	"autre":       {"livraison": true, "rendez_vous": true, "facturation": true, "autre": true, "": true},
	"":            {"livraison": true, "rendez_vous": true, "facturation": true, "autre": true, "": true},
}

func dependanceCompatible(amontType, avalType string) bool {
	return avalsParAmont[amontType][avalType]
}

// retenirCandidats trie par score décroissant, coupe au seuil puis au plafond.
func retenirCandidats(juges []candidatJuge, seuil float64, max int) []candidatJuge {
	out := make([]candidatJuge, 0, len(juges))
	for _, j := range juges {
		if j.score >= seuil {
			out = append(out, j)
		}
	}
	sort.SliceStable(out, func(i, k int) bool { return out[i].score > out[k].score })
	if len(out) > max {
		out = out[:max]
	}
	return out
}

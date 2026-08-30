// Commande recalibrer : recalcule la fiabilité des engagements existants.
//
// La fiabilité était la certitude que le modèle déclarait sur lui-même. Elle
// est désormais calculée à partir de signaux vérifiables, avec de nouveaux
// seuils d'affichage. Sans reprise, les anciens engagements garderaient leur
// score gonflé et s'afficheraient « Élevée » sous les seuils neufs — c'est-à-
// dire exactement l'inverse de ce que la mesure montre, puisque ce sont eux
// que le dirigeant corrige le plus.
//
// L'avis d'origine du modèle est relu dans l'événement « cree », qui le
// conservait. Rien n'est donc perdu.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"ibalya/backend/internal/config"
	"ibalya/backend/internal/engine"
	"ibalya/backend/internal/store"
)

func main() {
	appliquer := flag.Bool("appliquer", false, "écrire les nouveaux scores")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connexion :", err)
		os.Exit(1)
	}
	defer pool.Close()
	st := &store.Store{Pool: pool}

	rows, err := pool.Query(ctx, `
		SELECT e.id, e.objet, e.confiance, e.echeance IS NOT NULL, e.echeance_inferee,
		       coalesce(e.thread_id, 0), coalesce(p.email, ''),
		       coalesce(m.body, ''),
		       coalesce((SELECT v.details->>'confiance' FROM engagement_events v
		                  WHERE v.engagement_id = e.id AND v.type = 'cree'
		                  ORDER BY v.id LIMIT 1), '')
		  FROM engagements e
		  LEFT JOIN persons p ON p.id = e.emetteur_id
		  LEFT JOIN messages m ON m.id = e.source_message_id
		 ORDER BY e.id`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lecture :", err)
		os.Exit(1)
	}
	type ligne struct {
		id                 int64
		objet              string
		ancien             float64
		aEcheance, inferee bool
		threadID           int64
		email, corps       string
		avisBrut           string
	}
	var lignes []ligne
	for rows.Next() {
		var l ligne
		if err := rows.Scan(&l.id, &l.objet, &l.ancien, &l.aEcheance, &l.inferee,
			&l.threadID, &l.email, &l.corps, &l.avisBrut); err != nil {
			fmt.Fprintln(os.Stderr, "ligne :", err)
			os.Exit(1)
		}
		lignes = append(lignes, l)
	}
	rows.Close()

	fmt.Printf("  %-52s %-11s %s\n", "engagement", "avant", "après")
	fmt.Println("  " + repeter("─", 84))
	baisses, hausses := 0, 0
	type maj struct {
		id    int64
		score float64
	}
	var majs []maj
	for _, l := range lignes {
		avis := 0.0
		if l.avisBrut != "" {
			_ = json.Unmarshal([]byte(l.avisBrut), &avis)
		}
		s := engine.SignauxFiabilite{
			AvisModele:                  avis,
			EcheanceExplicite:           l.aEcheance && !l.inferee,
			FormulationEngageante:       engine.FormulationEstEngageante(l.corps),
			TauxCorrectionInterlocuteur: -1,
		}
		if l.threadID != 0 {
			if entrants, sortants, err := st.MessagesDansFil(ctx, l.threadID); err == nil {
				s.FilConversationnel = entrants > 0 && sortants > 0
			}
		}
		if l.email != "" {
			if n, err := st.EchangesAvecInterlocuteur(ctx, l.email); err == nil {
				s.InterlocuteurConnu = n >= 3
			}
		}
		score, _ := engine.CalculerFiabilite(s)
		objet := l.objet
		if len([]rune(objet)) > 50 {
			objet = string([]rune(objet)[:50]) + "…"
		}
		fleche := "="
		if score < l.ancien {
			fleche, baisses = "↓", baisses+1
		} else if score > l.ancien {
			fleche, hausses = "↑", hausses+1
		}
		fmt.Printf("  %-52s %-11s %s %.2f (%s)\n", objet,
			fmt.Sprintf("%.2f (%s)", l.ancien, niveau(l.ancien)), fleche, score, niveau(score))
		majs = append(majs, maj{l.id, score})
	}
	fmt.Printf("\n  %d engagements : %d en baisse, %d en hausse\n", len(lignes), baisses, hausses)

	// Le seul test qui compte : le nouveau score sépare-t-il mieux les
	// engagements que le dirigeant a corrigés de ceux qu'il a gardés ?
	// Un score utile est corrigé moins souvent quand il est haut.
	fmt.Println("\n  Pouvoir prédictif — part d'engagements corrigés ou abandonnés :")
	fmt.Printf("  %-14s %-22s %s\n", "", "ancien score", "nouveau score")
	for _, seuil := range []struct {
		nom string
		min float64
	}{{"haute", 0.75}, {"basse", 0}} {
		var ancienTot, ancienErr, nouvTot, nouvErr int
		for i, l := range lignes {
			corrige := estCorrige(ctx, pool, l.id)
			hautAncien := l.ancien >= 0.75
			hautNouveau := majs[i].score >= 0.75
			if (seuil.nom == "haute") == hautAncien {
				ancienTot++
				if corrige {
					ancienErr++
				}
			}
			if (seuil.nom == "haute") == hautNouveau {
				nouvTot++
				if corrige {
					nouvErr++
				}
			}
		}
		fmt.Printf("  %-14s %-22s %s\n", "fiabilité "+seuil.nom,
			pourcent(ancienErr, ancienTot), pourcent(nouvErr, nouvTot))
	}

	if !*appliquer {
		fmt.Println("\n  simulation. Relancer avec --appliquer pour écrire.")
		return
	}
	for _, m := range majs {
		if _, err := pool.Exec(ctx,
			`UPDATE engagements SET confiance=$2, maj_le=now() WHERE id=$1`, m.id, m.score); err != nil {
			fmt.Fprintln(os.Stderr, "écriture :", err)
			os.Exit(1)
		}
	}
	fmt.Printf("\n  %d scores réécrits.\n", len(majs))
}

// estCorrige dit si le dirigeant est revenu sur cet engagement.
func estCorrige(ctx context.Context, pool *pgxpool.Pool, id int64) bool {
	var n int
	_ = pool.QueryRow(ctx, `
		SELECT count(*) FROM engagements e
		 WHERE e.id = $1
		   AND (e.statut = 'abandonne'
		        OR EXISTS (SELECT 1 FROM engagement_events v
		                    WHERE v.engagement_id = e.id AND v.type = 'corrige'))`, id).Scan(&n)
	return n > 0
}

func pourcent(n, total int) string {
	if total == 0 {
		return "aucun"
	}
	return fmt.Sprintf("%d %% (%d sur %d)", int(100*float64(n)/float64(total)+0.5), n, total)
}

func niveau(s float64) string {
	switch engine.NiveauFiabilite(s) {
	case "elevee":
		return "élevée"
	case "a_verifier":
		return "à vérifier"
	}
	return "incertaine"
}

func repeter(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

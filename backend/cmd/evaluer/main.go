// Commande evaluer : mesure la qualité de l'extraction sur un corpus étiqueté.
//
// Jusqu'ici, modifier une instruction d'extraction ou la formule de fiabilité
// se faisait à l'aveugle : rien ne disait si le résultat s'améliorait ou se
// dégradait. Cette commande donne le chiffre.
//
// Le corpus (fixtures/corpus_eval.json) est entièrement inventé. Aucun message
// réel n'y figure, ce qui permet de le versionner sans aucune question de
// confidentialité.
//
//	go run ./cmd/evaluer                 # pré-filtre seul, sans appel au modèle
//	go run ./cmd/evaluer --avec-modele   # pipeline complet
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ibalya/backend/internal/config"
	"ibalya/backend/internal/ingest"
	"ibalya/backend/internal/llm"
	"ibalya/backend/internal/store"
)

type attendu struct {
	Engagement bool   `json:"engagement"`
	Echeance   string `json:"echeance"`
	Filtre     bool   `json:"filtre"`
	Pourquoi   string `json:"pourquoi"`
}

type cas struct {
	ExternalID string   `json:"external_id"`
	Subject    string   `json:"subject"`
	Sender     string   `json:"sender"`
	Recipients []string `json:"recipients"`
	Body       string   `json:"body"`
	Attendu    attendu  `json:"attendu"`
}

func main() {
	avecModele := flag.Bool("avec-modele", false, "appeler le service d'inférence (coûte des jetons)")
	chemin := flag.String("corpus", "", "chemin du corpus (défaut : fixtures/corpus_eval.json)")
	pause := flag.Duration("pause", 3*time.Second, "attente entre deux appels au modèle")
	flag.Parse()

	if *chemin == "" {
		*chemin = filepath.Join("..", "fixtures", "corpus_eval.json")
		if _, err := os.Stat(*chemin); err != nil {
			*chemin = filepath.Join("fixtures", "corpus_eval.json")
		}
	}
	brut, err := os.ReadFile(*chemin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "corpus introuvable :", err)
		os.Exit(1)
	}
	var corpus []cas
	if err := json.Unmarshal(brut, &corpus); err != nil {
		fmt.Fprintln(os.Stderr, "corpus illisible :", err)
		os.Exit(1)
	}

	// Le pré-filtre est la première ligne : ce qu'il écarte n'atteint jamais le
	// modèle. On le mesure toujours, sans rien appeler.
	sensibles := ingest.CategoriesParDefaut()
	type resultat struct {
		c        cas
		filtre   string
		extraits int
		echeance string
	}
	var res []resultat
	for _, c := range corpus {
		m := store.Message{
			Sender: c.Sender, Recipients: c.Recipients,
			Subject: c.Subject, Body: c.Body, SentAt: time.Now().AddDate(0, 0, -1),
		}
		r := resultat{c: c}
		if cat := ingest.DetecterCategorie(m.Subject, m.Body, sensibles); cat != "" {
			r.filtre = "categorie_" + string(cat)
		} else if raison := ingest.ExclusionReason(m, false, nil); raison != "" {
			r.filtre = raison
		}
		res = append(res, r)
	}

	if *avecModele {
		cfg := config.Load()
		cli := llm.New(cfg.LLMServiceURL)
		ctx, annule := context.WithTimeout(context.Background(), 4*time.Minute)
		defer annule()
		for i := range res {
			if res[i].filtre != "" {
				continue // écarté avant le modèle : rien à demander
			}
			// Espacer les appels. Enchaînés, ils déclenchent la limitation de
			// débit du fournisseur, et la mesure varie alors du tout au tout
			// d'un passage à l'autre sans que le pipeline ait changé.
			if i > 0 {
				time.Sleep(*pause)
			}
			c := res[i].c
			rep, err := cli.Extract(ctx, llm.ExtractRequest{
				Messages: []llm.ExtractMessage{{
					ID: int64(i + 1), Sender: c.Sender, To: strings.Join(c.Recipients, ","),
					Subject: c.Subject, Body: c.Body,
					SentAt: time.Now().AddDate(0, 0, -1).Format(time.RFC3339),
				}},
				AccountEmail: "dirigeant@menuiserie-dupont.fr",
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s : %v\n", c.ExternalID, err)
				continue
			}
			for _, r := range rep.Results {
				res[i].extraits += len(r.Engagements)
				if res[i].echeance == "" && len(r.Engagements) > 0 {
					res[i].echeance = r.Engagements[0].Echeance
				}
			}
		}
	}

	// Les deux étages se jugent séparément. Le pré-filtre décide qui atteint le
	// modèle, pas ce que le message contient : lui reprocher de ne pas
	// reconnaître une formule de politesse n'aurait aucun sens.
	fmt.Println("  ÉTAGE 1 — pré-filtre : qui atteint le modèle")
	fmt.Println("  " + strings.Repeat("─", 78))
	var fVP, fFP, fFN int
	for _, r := range res {
		ecarte := r.filtre != ""
		devrait := r.c.Attendu.Filtre
		verdict := "ok"
		switch {
		case ecarte && devrait:
			fVP++
		case ecarte && !devrait:
			fFP++
			verdict = "ÉCARTÉ À TORT"
		case !ecarte && devrait:
			fFN++
			verdict = "AURAIT DÛ ÊTRE ÉCARTÉ"
		}
		if verdict != "ok" {
			fmt.Printf("  %-40s %-22s %s\n", tronquer(r.c.Subject, 38), r.filtre, verdict)
			fmt.Printf("  %40s ↳ %s\n", "", r.c.Attendu.Pourquoi)
		}
	}
	fmt.Printf("  %d écartés à raison · %d à tort · %d oubliés\n", fVP, fFP, fFN)

	if !*avecModele {
		fmt.Println("\n  ÉTAGE 2 — extraction : non mesuré (--avec-modele pour l'appeler).")
		if fFP > 0 || fFN > 0 {
			os.Exit(1)
		}
		return
	}

	fmt.Println("\n  ÉTAGE 2 — extraction : y a-t-il un engagement ?")
	fmt.Println("  " + strings.Repeat("─", 78))
	var vp, fp, vn, fn int
	var echeancesJustes, echeancesAttendues int
	for _, r := range res {
		if r.filtre != "" {
			continue // n'a jamais atteint le modèle : hors sujet ici
		}
		predit := r.extraits > 0
		att := r.c.Attendu.Engagement
		verdict := "ok"
		switch {
		case predit && att:
			vp++
		case predit && !att:
			fp++
			verdict = "FAUX POSITIF"
		case !predit && att:
			fn++
			verdict = "MANQUÉ"
		default:
			vn++
		}
		ecart := ""
		if att && r.c.Attendu.Echeance != "" {
			echeancesAttendues++
			if r.echeance == r.c.Attendu.Echeance {
				echeancesJustes++
			} else {
				obtenue := r.echeance
				if obtenue == "" {
					obtenue = "aucune"
				}
				ecart = fmt.Sprintf("échéance %s au lieu de %s", obtenue, r.c.Attendu.Echeance)
			}
		}
		obtenu := fmt.Sprintf("%d engagement(s)", r.extraits)
		fmt.Printf("  %-40s %-22s %s\n", tronquer(r.c.Subject, 38), obtenu, verdict)
		if verdict != "ok" {
			fmt.Printf("  %40s ↳ %s\n", "", r.c.Attendu.Pourquoi)
		}
		if ecart != "" {
			fmt.Printf("  %40s ↳ %s\n", "", ecart)
		}
	}
	fmt.Printf("\n  vrais positifs %d · faux positifs %d · manqués %d · vrais négatifs %d\n", vp, fp, fn, vn)
	fmt.Printf("  précision %s   rappel %s\n", taux(vp, vp+fp), taux(vp, vp+fn))
	if echeancesAttendues > 0 {
		fmt.Printf("  échéances exactes %s\n", taux(echeancesJustes, echeancesAttendues))
	}
	if fFP > 0 || fFN > 0 || fp > 0 || fn > 0 {
		os.Exit(1) // la CI doit voir passer une régression
	}
	return

}

func tronquer(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func taux(n, total int) string {
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%3d %% (%d/%d)", int(100*float64(n)/float64(total)+0.5), n, total)
}

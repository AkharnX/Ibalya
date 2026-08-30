// Commande confidentialite : applique les filtres de catégories sensibles aux
// messages déjà en base.
//
// Les filtres agissent à l'ingestion, donc seulement sur ce qui arrive après
// leur activation. Le courrier déjà stocké — candidatures, arrêts maladie —
// resterait indéfiniment lisible. Cette commande le rattrape.
//
// Sans argument elle ne fait que compter et montrer un échantillon. Il faut
// --appliquer pour écrire, parce que la purge est irréversible côté Ibalya.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"ibalya/backend/internal/config"
	"ibalya/backend/internal/ingest"
)

func main() {
	appliquer := flag.Bool("appliquer", false, "écrire réellement : exclure et purger le contenu")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connexion :", err)
		os.Exit(1)
	}
	defer pool.Close()

	// On applique le réglage réellement en vigueur, pas toutes les catégories :
	// rattraper le passé avec des règles que le dirigeant n'a pas choisies
	// écarterait du courrier qu'il a décidé de garder.
	var brut string
	if err := pool.QueryRow(ctx,
		`SELECT value FROM settings WHERE key=$1`, ingest.CleReglageCategories).Scan(&brut); err != nil {
		brut = ""
	}
	actives := ingest.LireCategories(brut)
	var libelles []string
	for _, c := range ingest.CategoriesConnues {
		if actives[c] {
			libelles = append(libelles, c.Libelle())
		}
	}
	if len(libelles) == 0 {
		fmt.Println("  aucune catégorie active : rien à faire.")
		return
	}
	fmt.Printf("  catégories actives : %s\n\n", strings.Join(libelles, ", "))

	rows, err := pool.Query(ctx,
		`SELECT id, coalesce(subject,''), coalesce(body,''), sender, status
		   FROM messages WHERE body <> '' ORDER BY sent_at DESC`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lecture :", err)
		os.Exit(1)
	}
	type trouve struct {
		id                    int64
		cat                   ingest.Categorie
		sujet, sender, statut string
	}
	var touches []trouve
	total := 0
	for rows.Next() {
		var id int64
		var sujet, corps, sender, statut string
		if err := rows.Scan(&id, &sujet, &corps, &sender, &statut); err != nil {
			fmt.Fprintln(os.Stderr, "lecture ligne :", err)
			os.Exit(1)
		}
		total++
		if c := ingest.DetecterCategorie(sujet, corps, actives); c != "" {
			touches = append(touches, trouve{id, c, sujet, sender, statut})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "parcours :", err)
		os.Exit(1)
	}

	parCat := map[ingest.Categorie]int{}
	atteignaientLeModele := 0
	for _, t := range touches {
		parCat[t.cat]++
		if t.statut != "excluded" {
			atteignaientLeModele++
		}
	}
	fmt.Printf("  %d messages examinés, %d relèvent d'une catégorie sensible\n\n", total, len(touches))
	for _, c := range ingest.CategoriesConnues {
		if parCat[c] > 0 {
			fmt.Printf("  %-22s %d\n", c.Libelle(), parCat[c])
		}
	}
	fmt.Printf("\n  dont %d atteignaient le modèle (les autres étaient déjà écartés\n"+
		"  par le pré-filtre, mais leur contenu reste stocké).\n", atteignaientLeModele)

	fmt.Println("\n  échantillon de ceux qui atteignaient le modèle :")
	montres := 0
	for _, t := range touches {
		if t.statut == "excluded" {
			continue
		}
		if montres >= 12 {
			fmt.Printf("  ... et %d autres\n", atteignaientLeModele-montres)
			break
		}
		montres++
		sujet := strings.TrimSpace(t.sujet)
		if len([]rune(sujet)) > 58 {
			sujet = string([]rune(sujet)[:58]) + "…"
		}
		fmt.Printf("  [%-9s] %-60s %s\n", t.cat, sujet, t.sender)
	}

	if !*appliquer {
		fmt.Println("\n  simulation. Relancer avec --appliquer pour exclure et purger.")
		return
	}

	// Le contenu est purgé, pas la ligne : les métadonnées gardent la trace de
	// l'échange, et la messagerie d'origine reste la source de vérité.
	n := 0
	for _, t := range touches {
		raison := "categorie_sensible_" + string(t.cat)
		if _, err := pool.Exec(ctx,
			`UPDATE messages SET body='', subject='', status='excluded', exclude_reason=$2
			   WHERE id=$1`, t.id, raison); err != nil {
			fmt.Fprintln(os.Stderr, "mise à jour :", err)
			os.Exit(1)
		}
		n++
	}
	fmt.Printf("\n  %d messages exclus et purgés de leur contenu.\n", n)
}

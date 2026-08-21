package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"ibalya/backend/internal/api"
	"ibalya/backend/internal/channel"
	"ibalya/backend/internal/config"
	"ibalya/backend/internal/engine"
	"ibalya/backend/internal/ingest"
	"ibalya/backend/internal/llm"
	"ibalya/backend/internal/store"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("base de données: %v", err)
	}
	log.Println("base de données connectée, schéma appliqué")

	// connecteur derrière l'interface de canal (EF-10)
	var reader channel.Reader
	oauthCfg := channel.GmailOAuthConfig(cfg.GoogleClientID, cfg.GoogleClientSecret,
		cfg.PublicBaseURL+"/api/oauth/google/callback")
	switch cfg.Channel {
	case "fixture":
		if cfg.FixturePath == "" {
			log.Fatal("CHANNEL=fixture exige FIXTURE_PATH")
		}
		reader = channel.NewFixture(cfg.FixturePath)
		log.Printf("connecteur fixture: %s", cfg.FixturePath)
	default:
		reader = channel.NewGmail(oauthCfg, st)
		log.Println("connecteur gmail (OAuth)")
	}

	eng := &engine.Engine{Store: st, LLM: llm.New(cfg.LLMServiceURL), Channel: reader}
	ing := &ingest.Ingester{Store: st, Channel: reader}
	srv := &api.Server{Cfg: cfg, Store: st, Engine: eng, Ingester: ing, OAuth: oauthCfg}

	if cfg.AdminToken == "" {
		log.Fatal("ADMIN_TOKEN obligatoire (protection du tableau de bord)")
	}

	// planificateur : cycle complet + digest quotidien
	go scheduler(ctx, cfg, st, eng, ing)

	log.Printf("Ibalya à l'écoute sur %s", cfg.APIAddr)
	if err := http.ListenAndServe(cfg.APIAddr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

func scheduler(ctx context.Context, cfg config.Config, st *store.Store, eng *engine.Engine, ing *ingest.Ingester) {
	cycleTicker := time.NewTicker(time.Duration(cfg.IngestInterval) * time.Minute)
	digestTicker := time.NewTicker(10 * time.Minute)
	defer cycleTicker.Stop()
	defer digestTicker.Stop()

	connected := func() bool {
		tok, _, _ := st.GetOAuthToken(ctx, "google")
		return tok != nil || cfg.Channel == "fixture"
	}

	for {
		select {
		case <-cycleTicker.C:
			if !connected() {
				continue
			}
			res := eng.RunCycle(ctx, func(ctx context.Context) (any, error) {
				return ing.Run(ctx, time.Now().AddDate(0, 0, -2), 300)
			})
			if res.Erreur != "" {
				log.Printf("cycle planifié: %s", res.Erreur)
			}
		case <-digestTicker.C:
			if !connected() {
				continue
			}
			maybeDailyDigest(ctx, cfg, st, eng)
		case <-ctx.Done():
			return
		}
	}
}

// maybeDailyDigest génère le digest une fois par jour à l'heure configurée
// (hebdomadaire le lundi si le dirigeant a choisi ce rythme).
func maybeDailyDigest(ctx context.Context, cfg config.Config, st *store.Store, eng *engine.Engine) {
	now := time.Now()
	if now.Hour() != cfg.DigestHour {
		return
	}
	dtype := st.GetSetting(ctx, "digest_type", "quotidien")
	if dtype == "hebdo" && now.Weekday() != time.Monday {
		return
	}
	today := now.Format("2006-01-02")
	if st.GetSetting(ctx, "dernier_digest", "") == today {
		return
	}
	if _, err := eng.GenerateDigest(ctx, dtype); err != nil {
		log.Printf("digest planifié: %v", err)
		return
	}
	st.SetSetting(ctx, "dernier_digest", today)
	log.Printf("digest %s généré", dtype)
}

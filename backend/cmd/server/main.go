package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"ibalya/backend/internal/api"
	"ibalya/backend/internal/channel"
	"ibalya/backend/internal/config"
	"ibalya/backend/internal/engine"
	"ibalya/backend/internal/ingest"
	"ibalya/backend/internal/llm"
	"ibalya/backend/internal/store"
	"strconv"
)

func main() {
	creerUtilisateur := flag.String("creer-utilisateur", "", "crée un compte : -creer-utilisateur email")
	nom := flag.String("nom", "", "nom affiché du compte à créer")
	flag.Parse()

	cfg := config.Load()
	if err := cfg.Verifier(); err != nil {
		log.Fatalf("configuration refusée : %v", err)
	}
	ctx := context.Background()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("base de données: %v", err)
	}
	log.Println("base de données connectée, schéma appliqué")

	if *creerUtilisateur != "" {
		if err := creerCompte(ctx, st, *creerUtilisateur, *nom); err != nil {
			log.Fatalf("création du compte: %v", err)
		}
		return
	}

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
	case "imap":
		// Un connecteur IMAP couvre Yahoo, OVH, Gandi, Orange, Free et toute
		// boîte auto-hébergée, là où chaque API propriétaire n'en couvre qu'une.
		if cfg.IMAPHote == "" || cfg.IMAPUtilisateur == "" || cfg.IMAPMotDePasse == "" {
			log.Fatal("CHANNEL=imap exige IMAP_HOST, IMAP_USER et IMAP_PASSWORD")
		}
		reader = channel.NewIMAP(channel.IMAPConfig{
			Hote: cfg.IMAPHote, Port: cfg.IMAPPort,
			Utilisateur: cfg.IMAPUtilisateur, MotDePasse: cfg.IMAPMotDePasse,
			Dossier:  cfg.IMAPDossier,
			SMTPHote: cfg.SMTPHote, SMTPPort: cfg.SMTPPort,
			TLSSansVerification: cfg.IMAPTLSSansVerif,
		})
		log.Printf("connecteur imap: %s@%s", cfg.IMAPUtilisateur, cfg.IMAPHote)
	default:
		reader = channel.NewGmail(oauthCfg, st)
		log.Println("connecteur gmail (OAuth)")
	}

	// Un canal raccordé depuis l'interface prime sur la variable d'environnement :
	// c'est le dirigeant qui décide, pas le fichier de configuration.
	if st.GetSetting(ctx, "canal_type", "") == "outlook" && cfg.MicrosoftClientID != "" {
		reader = channel.NewOutlook(channel.OutlookOAuthConfig(cfg.MicrosoftClientID,
			cfg.MicrosoftClientSecret, cfg.MicrosoftTenant,
			cfg.PublicBaseURL+"/api/oauth/microsoft/callback"), st)
		log.Println("connecteur outlook (Microsoft Graph)")
	}
	if st.GetSetting(ctx, "canal_type", "") == "imap" {
		if r, err := imapDepuisReglages(ctx, st, cfg); err == nil {
			reader = r
			log.Println("connecteur imap (raccordé depuis l'interface)")
		} else {
			log.Printf("canal IMAP enregistré mais inutilisable, repli sur %s : %v", reader.Name(), err)
		}
	}

	coffre, err := store.NouveauCoffre(cfg.CleChiffrement)
	if err != nil {
		log.Fatalf("clé de chiffrement : %v", err)
	}
	commutateur := channel.NewCommutateur(reader)
	eng := &engine.Engine{Store: st, LLM: llm.New(cfg.LLMServiceURL), Channel: commutateur, BaseURL: cfg.PublicBaseURL}
	ing := &ingest.Ingester{Store: st, Channel: commutateur}
	srv := &api.Server{Cfg: cfg, Store: st, Engine: eng, Ingester: ing, OAuth: oauthCfg,
		Commutateur: commutateur, Coffre: coffre}

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

// imapDepuisReglages reconstruit le connecteur depuis ce que le dirigeant a
// enregistré dans l'interface, mot de passe déchiffré au passage.
func imapDepuisReglages(ctx context.Context, st *store.Store, cfg config.Config) (channel.Reader, error) {
	coffre, err := store.NouveauCoffre(cfg.CleChiffrement)
	if err != nil || coffre == nil {
		return nil, fmt.Errorf("aucune clé de chiffrement : impossible de relire le mot de passe")
	}
	mdp, err := coffre.Dechiffrer(st.GetSetting(ctx, "imap_mot_de_passe", ""))
	if err != nil {
		return nil, err
	}
	nombre := func(c string, def int) int {
		if v, e := strconv.Atoi(st.GetSetting(ctx, c, "")); e == nil && v > 0 {
			return v
		}
		return def
	}
	return channel.NewIMAP(channel.IMAPConfig{
		Hote: st.GetSetting(ctx, "imap_hote", ""), Port: nombre("imap_port", 993),
		Utilisateur: st.GetSetting(ctx, "imap_utilisateur", ""), MotDePasse: mdp,
		Dossier:  st.GetSetting(ctx, "imap_dossier", "INBOX"),
		SMTPHote: st.GetSetting(ctx, "smtp_hote", ""), SMTPPort: nombre("smtp_port", 587),
		TLSSansVerification: cfg.IMAPTLSSansVerif,
	}), nil
}

func scheduler(ctx context.Context, cfg config.Config, st *store.Store, eng *engine.Engine, ing *ingest.Ingester) {
	cycleTicker := time.NewTicker(time.Duration(cfg.IngestInterval) * time.Minute)
	digestTicker := time.NewTicker(10 * time.Minute)
	defer cycleTicker.Stop()
	defer digestTicker.Stop()

	connected := func() bool {
		// Un canal à identifiants directs (IMAP, fixtures) est prêt dès le
		// démarrage ; seul OAuth exige un raccordement préalable.
		if cfg.Channel != "gmail" {
			return true
		}
		tok, _, _ := st.GetOAuthToken(ctx, "google")
		return tok != nil
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

// creerCompte crée un compte depuis la ligne de commande. Le mot de passe est
// saisi sans écho et n'apparaît donc ni à l'écran ni dans l'historique du shell.
func creerCompte(ctx context.Context, st *store.Store, email, nom string) error {
	// Un seul lecteur pour toute l'entrée standard : deux bufio.NewReader
	// successifs se disputaient le tampon, le premier avalait la ligne du mot
	// de passe et la création échouait dès que -nom n'était pas fourni.
	lecteur := bufio.NewReader(os.Stdin)
	if nom == "" {
		fmt.Print("Nom affiché : ")
		ligne, _ := lecteur.ReadString('\n')
		nom = strings.TrimSpace(ligne)
	}
	// Terminal : saisie masquée avec confirmation. Sinon (tuyau, script de
	// provisionnement) : le mot de passe est lu sur l'entrée standard.
	var mdp string
	if term.IsTerminal(int(syscall.Stdin)) {
		fmt.Print("Mot de passe (10 caractères minimum) : ")
		a, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return err
		}
		fmt.Print("Confirmation : ")
		b, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return err
		}
		if string(a) != string(b) {
			return fmt.Errorf("les deux saisies diffèrent")
		}
		mdp = string(a)
	} else {
		ligne, err := lecteur.ReadString('\n')
		if err != nil && ligne == "" {
			return fmt.Errorf("mot de passe absent sur l'entrée standard")
		}
		mdp = strings.TrimRight(ligne, "\r\n")
	}
	id, err := st.CreateUser(ctx, email, nom, mdp)
	if err != nil {
		return err
	}
	fmt.Printf("Compte créé : %s (%s), identifiant %d\n", email, nom, id)
	return nil
}

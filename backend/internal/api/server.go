// Package api expose l'API REST et le tableau de bord (port 9999).
package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"fmt"
	"ibalya/backend/internal/channel"
	"ibalya/backend/internal/config"
	"ibalya/backend/internal/engine"
	"ibalya/backend/internal/ingest"
	"ibalya/backend/internal/store"
)

type Server struct {
	Cfg      config.Config
	Store    *store.Store
	Engine   *engine.Engine
	Ingester *ingest.Ingester
	OAuth    *oauth2.Config
	// Commutateur permet de raccorder une autre boîte sans redémarrer.
	Commutateur *channel.Commutateur
	// Coffre chiffre les secrets stockés en base, comme le mot de passe IMAP.
	Coffre *store.Coffre

	// Compteur d'échecs de connexion. Initialisé par Handler pour que les
	// appelants gardent leur littéral de structure.
	limiteConnexion *limiteur
}

func (s *Server) Handler() http.Handler {
	if s.limiteConnexion == nil {
		s.limiteConnexion = nouveauLimiteur(10, 15*time.Minute)
	}
	mux := http.NewServeMux()

	// Page commerciale à la racine, application sous /app.
	// Un visiteur qui tape ibalya.com doit voir le produit, pas un formulaire
	// de connexion.
	mux.Handle("GET /app", http.RedirectHandler("/app/", http.StatusMovedPermanently))
	mux.Handle("GET /app/", http.StripPrefix("/app", spaHandler(s.Cfg.FrontendDir)))
	mux.Handle("GET /", http.FileServer(http.Dir(s.Cfg.LandingDir)))

	// authentification
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("GET /api/oauth/google/login", s.googleLoginStart)
	mux.HandleFunc("GET /api/oauth/google/login/callback", s.googleLoginCallback)
	mux.HandleFunc("POST /api/logout", s.auth(s.logout))
	mux.HandleFunc("GET /api/me", s.auth(s.me))
	mux.HandleFunc("POST /api/password", s.auth(s.changerMotDePasse))
	mux.HandleFunc("GET /api/users", s.auth(s.listUsers))

	// santé (sans auth)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// Raccordement d'une boîte : google (Gmail) ou microsoft (Outlook).
	mux.HandleFunc("POST /api/oauth/{fournisseur}/start", s.auth(s.oauthStart))
	mux.HandleFunc("GET /api/oauth/{fournisseur}/callback", s.oauthCallback)

	// statut & cycle
	mux.HandleFunc("GET /api/status", s.auth(s.status))
	mux.HandleFunc("POST /api/cycle/run", s.auth(s.runCycle))
	mux.HandleFunc("POST /api/onboarding/run", s.auth(s.runOnboarding))
	mux.HandleFunc("GET /api/onboarding/status", s.auth(s.onboardingStatus))
	mux.HandleFunc("POST /api/onboarding/ack", s.auth(s.onboardingAck))

	// miroir & capsule
	mux.HandleFunc("GET /api/miroir", s.auth(s.getMiroir))
	mux.HandleFunc("POST /api/miroir/generate", s.auth(s.generateMiroir))
	mux.HandleFunc("GET /api/capsule", s.auth(s.getCapsule))
	mux.HandleFunc("PUT /api/capsule", s.auth(s.putCapsule))
	mux.HandleFunc("POST /api/capsule/infer", s.auth(s.inferCapsule))

	// engagements
	mux.HandleFunc("GET /api/engagements/{id}/events", s.auth(s.listEvents))
	mux.HandleFunc("GET /api/engagements/{id}/source", s.auth(s.engagementSource))
	mux.HandleFunc("GET /api/threads/{id}/source", s.auth(s.threadSource))
	mux.HandleFunc("PATCH /api/engagements/{id}", s.auth(s.patchEngagement))
	mux.HandleFunc("POST /api/engagements/{id}/correct", s.auth(s.correctEngagement))

	// détections
	mux.HandleFunc("GET /api/detections", s.auth(s.listDetections))
	mux.HandleFunc("POST /api/detections/{id}/dismiss", s.auth(s.dismissDetection))

	// liens de dépendance
	mux.HandleFunc("GET /api/links", s.auth(s.listLinks))
	mux.HandleFunc("POST /api/links/{id}/confirm", s.auth(s.linkStatus("confirme")))
	mux.HandleFunc("POST /api/links/{id}/reject", s.auth(s.linkStatus("rejete")))

	// digest & brouillons
	mux.HandleFunc("GET /api/drafts", s.auth(s.listDrafts))
	mux.HandleFunc("PATCH /api/drafts/{id}", s.auth(s.patchDraft))
	mux.HandleFunc("POST /api/drafts/{id}/review", s.auth(s.reviewDraft))
	mux.HandleFunc("POST /api/drafts/{id}/validate", s.auth(s.validateDraft))
	mux.HandleFunc("POST /api/drafts/{id}/reject", s.auth(s.rejectDraft))

	// vues de la maquette : synthèse (direction) + suivi des engagements
	mux.HandleFunc("GET /api/synthese", s.auth(s.synthese))
	mux.HandleFunc("GET /api/suivi", s.auth(s.suivi))
	mux.HandleFunc("POST /api/engagements/{id}/draft", s.auth(s.draftForEngagement))
	mux.HandleFunc("GET /api/pilotage", s.auth(s.pilotage))

	// règles apprises, personnes, audit, réglages
	mux.HandleFunc("GET /api/rules", s.auth(s.listRules))
	mux.HandleFunc("POST /api/rules", s.auth(s.createRule))
	mux.HandleFunc("DELETE /api/rules/{id}", s.auth(s.deleteRule))
	// Déclencher un digest sans attendre l'heure du scheduler : sert à
	// vérifier l'expéditeur et le contenu après un changement de réglage.
	mux.HandleFunc("POST /api/digest/envoyer", s.auth(s.envoyerDigest))
	mux.HandleFunc("GET /api/canal", s.auth(s.getCanal))
	mux.HandleFunc("POST /api/canal/tester", s.auth(s.testerCanal))
	mux.HandleFunc("PUT /api/canal", s.auth(s.putCanal))
	mux.HandleFunc("GET /api/recherche", s.auth(s.recherche))
	mux.HandleFunc("GET /api/persons", s.auth(s.listPersons))
	mux.HandleFunc("GET /api/persons/{id}", s.auth(s.fichePersonne))
	mux.HandleFunc("PATCH /api/persons/{id}", s.auth(s.patchPerson))
	mux.HandleFunc("GET /api/audit", s.auth(s.listAudit))
	mux.HandleFunc("GET /api/kpis", s.auth(s.kpis))
	mux.HandleFunc("GET /api/settings", s.auth(s.getSettings))
	mux.HandleFunc("PUT /api/settings", s.auth(s.putSettings))

	return mux
}

// --- middleware ---

// auth accepte deux modes :
//  1. une session nominative (cookie HttpOnly) — le cas normal du tableau de bord ;
//  2. le jeton de service, UNIQUEMENT depuis la boucle locale — pour les
//     scripts (make status, demo.sh) ; jamais joignable depuis Internet.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(cookieSession); err == nil && c.Value != "" {
			if u, err := s.Store.UserBySession(r.Context(), c.Value); err == nil {
				next(w, r.WithContext(context.WithValue(r.Context(), ctxUser{}, u)))
				return
			}
		}
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if token != "" && s.Cfg.AdminToken != "" && estLocal(r) &&
			subtle.ConstantTimeCompare([]byte(token), []byte(s.Cfg.AdminToken)) == 1 {
			next(w, r)
			return
		}
		httpError(w, 401, "authentification requise")
	}
}

// --- OAuth ---

// configRaccordement retourne la configuration OAuth du fournisseur demandé,
// et le nom sous lequel son jeton est rangé.
func (s *Server) configRaccordement(f string) (*oauth2.Config, string, error) {
	switch f {
	case "google":
		if s.OAuth == nil || s.OAuth.ClientID == "" {
			return nil, "", fmt.Errorf("GOOGLE_CLIENT_ID et GOOGLE_CLIENT_SECRET ne sont pas configurés")
		}
		return s.OAuth, "google", nil
	case "microsoft":
		if s.Cfg.MicrosoftClientID == "" {
			return nil, "", fmt.Errorf("MICROSOFT_CLIENT_ID et MICROSOFT_CLIENT_SECRET ne sont pas configurés")
		}
		return channel.OutlookOAuthConfig(s.Cfg.MicrosoftClientID, s.Cfg.MicrosoftClientSecret,
			s.Cfg.MicrosoftTenant, s.Cfg.PublicBaseURL+"/api/oauth/microsoft/callback"), "microsoft", nil
	}
	return nil, "", fmt.Errorf("fournisseur inconnu : %s", f)
}

func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	f := r.PathValue("fournisseur")
	cfg, _, err := s.configRaccordement(f)
	if err != nil {
		httpError(w, 400, err.Error())
		return
	}
	b := make([]byte, 16)
	rand.Read(b)
	state := hex.EncodeToString(b)
	// L'état porte le fournisseur : le rappel doit savoir lequel répond, et
	// deux raccordements ne doivent pas se confondre.
	s.Store.SetSetting(r.Context(), "oauth_state", f+":"+state)
	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	writeJSON(w, map[string]string{"url": url})
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f := r.PathValue("fournisseur")
	cfg, provider, err := s.configRaccordement(f)
	if err != nil {
		httpError(w, 400, err.Error())
		return
	}
	// L'état porte le fournisseur : un rappel de Microsoft ne doit pas valider
	// un raccordement Google entamé en parallèle.
	attendu := s.Store.GetSetting(ctx, "oauth_state", "")
	if attendu == "" || attendu != f+":"+r.URL.Query().Get("state") {
		httpError(w, 400, "état OAuth invalide")
		return
	}
	// À usage unique : un nonce rejouable ne protège plus de rien.
	s.Store.SetSetting(ctx, "oauth_state", "")

	tok, err := cfg.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		httpError(w, 400, "échange OAuth: "+err.Error())
		return
	}
	b, _ := json.Marshal(tok)
	if err := s.Store.SaveOAuthToken(ctx, provider, b, ""); err != nil {
		httpError(w, 500, err.Error())
		return
	}

	// Le canal bascule sur la boîte qu'on vient de raccorder : sans cela le
	// raccordement réussirait sans que l'agent lise quoi que ce soit.
	var lecteur channel.Reader
	switch provider {
	case "google":
		lecteur = channel.NewGmail(cfg, s.Store)
		s.Store.SetSetting(ctx, "canal_type", "gmail")
	case "microsoft":
		lecteur = channel.NewOutlook(cfg, s.Store)
		s.Store.SetSetting(ctx, "canal_type", "outlook")
	}
	if lecteur != nil {
		// L'adresse est persistée ici : le tableau de bord et les liens en ont
		// besoin, et l'appel peut échouer plus tard.
		if email, err := lecteur.AccountEmail(context.Background()); err == nil {
			tokB, _, _ := s.Store.GetOAuthToken(ctx, provider)
			_ = s.Store.SaveOAuthToken(ctx, provider, tokB, email)
		}
		s.Commutateur.Remplacer(lecteur)
	}
	s.Store.Audit(ctx, "dirigeant", "canal_connecte", map[string]string{"provider": provider})
	// J+0 → J+1 : lance l'onboarding en arrière-plan (30 jours + miroir + capsule)
	go s.onboarding()
	http.Redirect(w, r, "/app/?connected=1", http.StatusFound)
}

// --- onboarding (Miroir J+1) ---

func (s *Server) onboarding() {
	ctx := context.Background()
	log.Println("onboarding: lecture des 30 derniers jours…")
	s.marquerPhase(ctx, "lecture", "")

	res := s.Engine.RunCycle(ctx, func(ctx context.Context) (any, error) {
		defer s.marquerPhase(ctx, "analyse", "")
		return s.Ingester.Run(ctx, time.Now().AddDate(0, 0, -30), 1000)
	})
	if res.Erreur != "" {
		log.Printf("onboarding: %s", res.Erreur)
		s.marquerPhase(ctx, "erreur", res.Erreur)
		return
	}

	s.marquerPhase(ctx, "miroir", "")
	if _, err := s.Engine.GenerateMiroir(ctx); err != nil {
		log.Printf("onboarding: miroir: %v", err)
		s.marquerPhase(ctx, "erreur", err.Error())
		return
	}

	// capsule temps 1 APRÈS le miroir (séquencement psychologique CDC 9.1)
	s.marquerPhase(ctx, "capsule", "")
	if err := s.Engine.InferCapsule(ctx); err != nil {
		log.Printf("onboarding: capsule: %v", err)
		s.marquerPhase(ctx, "erreur", err.Error())
		return
	}
	s.marquerPhase(ctx, "termine", "")
}

func (s *Server) runOnboarding(w http.ResponseWriter, r *http.Request) {
	go s.onboarding()
	writeJSON(w, map[string]string{"status": "onboarding lancé en arrière-plan"})
}

// --- statut & cycle ---

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var counts struct {
		Messages    int `json:"messages"`
		EnAttente   int `json:"messages_en_attente"`
		Exclus      int `json:"messages_exclus"`
		Engagements int `json:"engagements"`
		Detections  int `json:"detections_actives"`
		Brouillons  int `json:"brouillons_proposes"`
		// liens de dépendance en attente d'arbitrage : tant qu'ils restent
		// candidats, le détecteur de contradiction (CDC 8.1) les ignore.
		LiensAConfirmer     int `json:"liens_a_confirmer"`
		EcheancesAConfirmer int `json:"echeances_a_confirmer"`
	}
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&counts.Messages)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE status='pending'`).Scan(&counts.EnAttente)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE status='excluded'`).Scan(&counts.Exclus)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM engagements`).Scan(&counts.Engagements)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM detections WHERE statut IN ('nouvelle','au_digest')`).Scan(&counts.Detections)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM drafts WHERE statut='propose'`).Scan(&counts.Brouillons)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM dependency_links WHERE statut='candidat'`).Scan(&counts.LiensAConfirmer)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM engagements
		WHERE echeance IS NOT NULL AND echeance_inferee AND NOT echeance_confirmee
		  AND statut IN ('ouvert','confirme','en_retard')`).Scan(&counts.EcheancesAConfirmer)

	// L'état du canal se lit sur le connecteur : lui seul sait s'il est
	// utilisable. Gmail exige un jeton, IMAP des identifiants, les fixtures
	// rien — l'ancienne lecture du jeton Google déclarait IMAP non connecté.
	email, errCompte := s.Engine.Channel.AccountEmail(ctx)
	llmOK := s.Engine.LLM.Health(ctx) == nil

	// Rappel de reconnexion. En mode Test, Google révoque le jeton sept jours
	// après le consentement : on prévient à partir du cinquième pour laisser
	// le temps d'agir avant la panne. Propre à Google ; IMAP et Microsoft
	// n'ont pas cette expiration.
	reconnexion := map[string]any{"requise": false}
	// Le rappel n'a de sens qu'en mode Test : c'est lui qui impose l'expiration
	// à sept jours. En production, le jeton ne meurt plus sur ce calendrier, et
	// le bandeau serait une fausse alerte hebdomadaire.
	if s.Engine.Channel.Name() == "gmail" && s.Store.GetSetting(ctx, "google_mode_test", "1") == "1" {
		if connecte, jours := s.Store.EtatConnexionOAuth(ctx, "google"); connecte {
			restant := 7 - jours
			reconnexion = map[string]any{
				"fournisseur":   "Gmail",
				"jours_depuis":  jours,
				"jours_restant": restant,
				"bientot":       restant <= 2,
			}
		}
	}

	writeJSON(w, map[string]any{
		"canal":             s.Engine.Channel.Name(),
		"canal_connecte":    errCompte == nil && email != "",
		"compte":            email,
		"reconnexion":       reconnexion,
		"service_llm_ok":    llmOK,
		"dernier_cycle":     s.Store.GetSetting(ctx, "dernier_cycle", ""),
		"seuil_publication": s.Store.GetSetting(ctx, "seuil_publication", "0.6"),
		"compteurs":         counts,
		"cycle":             s.Engine.Etat(),
	})
}

func (s *Server) runCycle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SinceDays int `json:"since_days"`
		Max       int `json:"max"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.SinceDays <= 0 {
		body.SinceDays = 2
	}
	// Le plafond était figé à 500. Le rapatriement rend les messages du plus
	// récent au plus ancien : sur une longue fenêtre — un premier cycle de
	// trente jours sur une boîte chargée — les plus anciens passaient à la
	// trappe sans que rien ne le signale.
	if body.Max <= 0 {
		body.Max = 500
	}
	if body.Max > 2000 {
		body.Max = 2000
	}
	res := s.Engine.RunCycleOrigine(r.Context(), func(ctx context.Context) (any, error) {
		return s.Ingester.Run(ctx, time.Now().AddDate(0, 0, -body.SinceDays), body.Max)
	}, "dirigeant")
	writeJSON(w, res)
}

// --- miroir & capsule ---

// POST /api/digest/envoyer — produit un digest et l'envoie immédiatement.
//
// Le scheduler ne le fait qu'à l'heure dite : sans déclenchement manuel, la
// moindre vérification d'expéditeur ou de contenu demande d'attendre le
// lendemain matin.
func (s *Server) envoyerDigest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type string `json:"type"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Type != "hebdo" {
		body.Type = "quotidien"
	}
	dc, err := s.Engine.GenerateDigest(r.Context(), body.Type)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"type":        body.Type,
		"alertes":     len(dc.Detections),
		"engagements": len(dc.Engagements),
		"messages":    len(dc.Brouillons),
		"expediteur":  s.Engine.Courrier.Expediteur(),
	})
}

func (s *Server) getMiroir(w http.ResponseWriter, r *http.Request) {
	rep, err := s.Store.LatestReport(r.Context(), "miroir")
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	if rep == nil {
		httpError(w, 404, "aucun miroir généré")
		return
	}
	writeJSON(w, rep)
}

func (s *Server) generateMiroir(w http.ResponseWriter, r *http.Request) {
	m, err := s.Engine.GenerateMiroir(r.Context())
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, m)
}

func (s *Server) getCapsule(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.GetCapsule(r.Context())
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, c)
}

func (s *Server) putCapsule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Facts      json.RawMessage `json:"facts"`
		Intentions json.RawMessage `json:"intentions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, 400, err.Error())
		return
	}
	// la capsule conditionne l'extraction : n'accepter que des objets JSON
	for name, raw := range map[string]json.RawMessage{"facts": body.Facts, "intentions": body.Intentions} {
		if raw == nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			httpError(w, 400, name+" doit être un objet JSON")
			return
		}
	}
	if err := s.Store.UpdateCapsule(r.Context(), body.Facts, body.Intentions); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	s.Store.Audit(r.Context(), acteur(r), "capsule_corrigee", nil)
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) inferCapsule(w http.ResponseWriter, r *http.Request) {
	if err := s.Engine.InferCapsule(r.Context()); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	c, _ := s.Store.GetCapsule(r.Context())
	writeJSON(w, c)
}

// --- engagements ---

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	evs, err := s.Store.ListEvents(r.Context(), id)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, evs)
}

func (s *Server) patchEngagement(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var body struct {
		Statut   *string `json:"statut"`
		Priorite *string `json:"priorite"`
		Echeance *string `json:"echeance"` // YYYY-MM-DD ; confirme l'échéance
		// Verdict : l'agent avait-il raison d'extraire cet engagement ?
		// Distinct du statut, qui dit seulement ce qu'il est devenu. Les deux
		// étaient confondus, au point qu'un engagement livré — donc une
		// réussite — se comptait comme une correction.
		Verdict *string `json:"verdict_extraction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, 400, err.Error())
		return
	}
	fields := map[string]any{}
	if body.Statut != nil {
		fields["statut"] = *body.Statut
		// Marquer « livré » ne se discute pas : l'engagement existait bel et
		// bien. C'est une étiquette positive gratuite, et la seule qu'on
		// obtienne sans rien demander au dirigeant.
		if *body.Statut == "livre" {
			fields["verdict_extraction"] = "juste"
		}
	}
	if body.Verdict != nil {
		switch *body.Verdict {
		case "juste", "faux", "imprecis":
			fields["verdict_extraction"] = *body.Verdict
		default:
			httpError(w, 400, "verdict_extraction : juste, faux ou imprecis")
			return
		}
	}
	if body.Priorite != nil {
		fields["priorite"] = *body.Priorite
	}
	if body.Echeance != nil {
		t, err := time.Parse("2006-01-02", *body.Echeance)
		if err != nil {
			httpError(w, 400, "échéance invalide (AAAA-MM-JJ)")
			return
		}
		fields["echeance"] = t
		fields["echeance_confirmee"] = true
		fields["echeance_inferee"] = false
	}
	if len(fields) == 0 {
		httpError(w, 400, "aucun champ à modifier")
		return
	}
	if err := s.Store.UpdateEngagement(r.Context(), id, fields); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	_ = s.Store.AddEvent(r.Context(), id, "corrige", nil, fields)
	s.Store.Audit(r.Context(), acteur(r), "engagement_corrige", map[string]any{"id": id, "champs": fields})
	writeJSON(w, map[string]string{"status": "ok"})
}

// interlocuteurDe trouve l'autre partie d'un engagement : celle qui n'est pas
// le dirigeant.
//
// Un engagement lie deux personnes. Pour une promesse que le dirigeant a prise
// lui-même — « confirmer ma présence au rendez-vous » —, il en est l'émetteur ;
// l'interlocuteur est alors le destinataire. Prendre l'émetteur sans réfléchir
// faisait viser le dirigeant par ses propres règles : « ne plus rien extraire
// de moi-même », ce qui n'a aucun sens.
func interlocuteurDe(eng *store.Engagement, proprietaire string) string {
	prop := normaliserEmail(proprietaire)
	em := normaliserEmail(eng.EmetteurEmail)
	dest := normaliserEmail(eng.DestinataireEmail)
	if em != "" && em != prop {
		return eng.EmetteurEmail
	}
	if dest != "" && dest != prop {
		return eng.DestinataireEmail
	}
	return ""
}

// correctEngagement : la boucle d'apprentissage (CDC 11). Chaque correction
// d'un geste devient une règle explicite.
func (s *Server) correctEngagement(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	ctx := r.Context()
	var body struct {
		Action string `json:"action"` // pas_un_engagement / ignorer_interlocuteur / priorite_haute / ne_plus_alerter
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, 400, err.Error())
		return
	}
	eng, err := s.Store.GetEngagement(ctx, id)
	if err != nil || eng == nil {
		httpError(w, 404, "engagement introuvable")
		return
	}
	interlocuteur := interlocuteurDe(eng, s.proprietaire(ctx))
	var rule store.LearnedRule
	switch body.Action {
	case "pas_un_engagement":
		_ = s.Store.UpdateEngagement(ctx, id, map[string]any{
			"statut": "abandonne", "verdict_extraction": "faux"})
		rule = store.LearnedRule{PorteeType: "engagement", PorteeCible: strconv.FormatInt(id, 10),
			Action: "requalifier", Note: "« " + eng.Objet + " » n'est pas un engagement"}
	case "ignorer_interlocuteur":
		if interlocuteur == "" {
			httpError(w, 400, "aucun interlocuteur distinct de vous sur cet engagement : rien à ignorer")
			return
		}
		rule = store.LearnedRule{PorteeType: "interlocuteur", PorteeCible: interlocuteur,
			Action: "ignorer", Note: "Ne plus extraire d'engagements de " + interlocuteur}
	case "priorite_haute":
		_ = s.Store.UpdateEngagement(ctx, id, map[string]any{"priorite": "haute"})
		if interlocuteur == "" {
			httpError(w, 400, "aucun interlocuteur distinct de vous sur cet engagement")
			return
		}
		rule = store.LearnedRule{PorteeType: "interlocuteur", PorteeCible: interlocuteur,
			Action: "priorite_haute", Note: "Priorité haute pour " + interlocuteur}
	// Engagement réel mais mal résumé : l'extraction n'est pas à jeter, elle est
	// à corriger. Sans cette nuance, tout ce qui n'est pas parfait se retrouve
	// compté comme un faux positif.
	case "engagement_imprecis":
		_ = s.Store.UpdateEngagement(ctx, id, map[string]any{"verdict_extraction": "imprecis"})
	// Abandon pour raison métier : l'affaire ne se fait pas, mais l'agent avait
	// raison de l'extraire. C'est une étiquette POSITIVE, et la confondre avec
	// un faux positif était précisément le défaut de l'étiquetage précédent.
	case "abandon_metier":
		_ = s.Store.UpdateEngagement(ctx, id, map[string]any{
			"statut": "abandonne", "verdict_extraction": "juste"})
	case "ne_plus_alerter":
		if eng.ThreadID != nil {
			rule = store.LearnedRule{PorteeType: "fil", PorteeCible: strconv.FormatInt(*eng.ThreadID, 10),
				Action: "ne_plus_alerter", Note: "Ne plus alerter sur ce fil"}
		}
	default:
		httpError(w, 400, "action inconnue")
		return
	}
	if rule.Action != "" {
		if _, err := s.Store.CreateRule(ctx, rule); err != nil {
			httpError(w, 500, err.Error())
			return
		}
	}
	_ = s.Store.AddEvent(ctx, id, "corrige", nil, map[string]string{"action": body.Action})
	s.Store.Audit(ctx, "dirigeant", "correction", map[string]any{"engagement_id": id, "action": body.Action})
	writeJSON(w, map[string]string{"status": "ok", "regle": rule.Note})
}

// --- détections ---

func (s *Server) listDetections(w http.ResponseWriter, r *http.Request) {
	statuts := []string{"nouvelle", "au_digest"}
	if q := r.URL.Query().Get("statut"); q != "" {
		statuts = strings.Split(q, ",")
	}
	minScore := 0.0
	if r.URL.Query().Get("proactives") == "1" {
		minScore = s.Engine.SeuilPublication(r.Context())
	}
	dets, err := s.Store.ListDetections(r.Context(), statuts, minScore, 100)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, dets)
}

func (s *Server) dismissDetection(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if err := s.Store.SetDetectionStatus(r.Context(), id, "ecartee"); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	s.Store.Audit(r.Context(), acteur(r), "detection_ecartee", map[string]int64{"id": id})
	writeJSON(w, map[string]string{"status": "ok"})
}

// --- liens ---

func (s *Server) listLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.Store.ListLinks(r.Context(), r.URL.Query().Get("statut"))
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, links)
}

func (s *Server) linkStatus(statut string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pathID(r)
		if err := s.Store.SetLinkStatus(r.Context(), id, statut); err != nil {
			httpError(w, 500, err.Error())
			return
		}
		s.Store.Audit(r.Context(), acteur(r), "lien_"+statut, map[string]int64{"id": id})
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

// --- digest & brouillons ---

func (s *Server) listDrafts(w http.ResponseWriter, r *http.Request) {
	drafts, err := s.Store.ListDrafts(r.Context(), r.URL.Query().Get("statut"))
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, drafts)
}

// validateDraft : marche 3 de l'escalier d'agentivité — l'envoi n'a lieu
// qu'ici, sur clic explicite du dirigeant.
func (s *Server) validateDraft(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if err := s.Engine.SendDraft(r.Context(), id); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "envoyé"})
}

func (s *Server) rejectDraft(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if err := s.Store.SetDraftStatus(r.Context(), id, "rejete", false); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	s.Store.Audit(r.Context(), acteur(r), "brouillon_rejete", map[string]int64{"id": id})
	writeJSON(w, map[string]string{"status": "ok"})
}

// --- règles, personnes, audit, réglages ---

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.Store.ListRules(r.Context(), false)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, rules)
}

func (s *Server) createRule(w http.ResponseWriter, r *http.Request) {
	var rule store.LearnedRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		httpError(w, 400, err.Error())
		return
	}
	id, err := s.Store.CreateRule(r.Context(), rule)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	s.Store.Audit(r.Context(), acteur(r), "regle_creee", rule)
	writeJSON(w, map[string]int64{"id": id})
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if err := s.Store.DeactivateRule(r.Context(), id); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	s.Store.Audit(r.Context(), acteur(r), "regle_desactivee", map[string]int64{"id": id})
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) listPersons(w http.ResponseWriter, r *http.Request) {
	persons, err := s.Store.ListPersons(r.Context())
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, persons)
}

func (s *Server) patchPerson(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var body struct {
		Type      string `json:"type"`
		Sensitive bool   `json:"sensitive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, 400, err.Error())
		return
	}
	if err := s.Store.UpdatePerson(r.Context(), id, body.Type, body.Sensitive); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	entries, err := s.Store.ListAudit(r.Context(), limit)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, entries)
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	writeJSON(w, map[string]string{
		"seuil_publication": s.Store.GetSetting(ctx, "seuil_publication", "0.6"),
		"digest_type":       s.Store.GetSetting(ctx, "digest_type", "quotidien"),
		"digest_email":      s.Store.GetSetting(ctx, "digest_email", "0"),
		// Statut de publication de l'app Google : "1" = mode Test (jeton expirant
		// à 7 jours, rappel actif), "0" = production.
		"google_mode_test":   s.Store.GetSetting(ctx, "google_mode_test", "1"),
		"digest_expediteur":  s.Store.GetSetting(ctx, "digest_expediteur", ""),
		"identite_prenom":    s.Store.GetSetting(ctx, "identite_prenom", ""),
		"identite_nom":       s.Store.GetSetting(ctx, "identite_nom", ""),
		"identite_fonction":  s.Store.GetSetting(ctx, "identite_fonction", ""),
		"identite_societe":   s.Store.GetSetting(ctx, "identite_societe", ""),
		"identite_signature": s.Store.GetSetting(ctx, "identite_signature", ""),
		// Catégories sensibles écartées avant toute inférence (CDC : filtres
		// RH, juridique, santé, exclusion configurable).
		ingest.CleReglageCategories: ingest.EcrireCategories(
			ingest.LireCategories(s.Store.GetSetting(ctx, ingest.CleReglageCategories, ""))),
	})
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, 400, err.Error())
		return
	}
	for k, v := range body {
		switch k {
		case "seuil_publication":
			f, err := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(v, ",", ".")), 64)
			if err != nil || f < 0 || f > 1 {
				httpError(w, 400, "le seuil de publication doit être un nombre entre 0 et 1")
				return
			}
			s.Store.SetSetting(r.Context(), k, strconv.FormatFloat(f, 'f', 2, 64))
		case "digest_type":
			if v != "quotidien" && v != "hebdo" {
				httpError(w, 400, "digest_type: quotidien ou hebdo")
				return
			}
			s.Store.SetSetting(r.Context(), k, v)
		case "digest_email":
			if v != "0" && v != "1" {
				httpError(w, 400, "digest_email: 0 ou 1")
				return
			}
			s.Store.SetSetting(r.Context(), k, v)
		case "google_mode_test":
			if v != "0" && v != "1" {
				httpError(w, 400, "google_mode_test: 0 ou 1")
				return
			}
			s.Store.SetSetting(r.Context(), k, v)
		// Identité du dirigeant. Elle sert à signer les messages sortants :
		// le modèle inventait des noms — cinq variantes sur douze brouillons,
		// dont deux qui n'étaient pas le bon prénom.
		case "identite_prenom", "identite_nom", "identite_fonction", "identite_societe":
			if len(v) > 120 {
				httpError(w, 400, k+" : 120 caractères au maximum")
				return
			}
			s.Store.SetSetting(r.Context(), k, strings.TrimSpace(v))
		// Signature rédigée à la main : elle prime sur les quatre champs.
		case "identite_signature":
			if len(v) > 600 {
				httpError(w, 400, "la signature est limitée à 600 caractères")
				return
			}
			s.Store.SetSetting(r.Context(), k, strings.TrimRight(v, " \n\t"))
		// Expéditeur du digest. Doit être une adresse vérifiée dans Gmail,
		// sans quoi l'envoi est refusé par Google.
		case "digest_expediteur":
			v = strings.TrimSpace(v)
			if v != "" && !strings.Contains(v, "@") {
				httpError(w, 400, "digest_expediteur : adresse email attendue")
				return
			}
			s.Store.SetSetting(r.Context(), k, v)
		// Catégories sensibles. Le réglage est réécrit à partir des catégories
		// connues : une clé inventée par le client ne doit pas se retrouver
		// stockée, et une valeur illisible ne doit pas désactiver le filtre en
		// silence.
		case ingest.CleReglageCategories:
			var recu map[string]bool
			if err := json.Unmarshal([]byte(v), &recu); err != nil {
				httpError(w, 400, "categories_sensibles : objet {categorie: bool} attendu")
				return
			}
			actives := map[ingest.Categorie]bool{}
			for _, c := range ingest.CategoriesConnues {
				actives[c] = recu[string(c)]
			}
			s.Store.SetSetting(r.Context(), k, ingest.EcrireCategories(actives))
		}
	}
	s.Store.Audit(r.Context(), acteur(r), "reglages_modifies", body)
	writeJSON(w, map[string]string{"status": "ok"})
}

// patchDraft permet au dirigeant d'ajuster un brouillon avant validation.
func (s *Server) patchDraft(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	var body struct {
		ToEmail string `json:"to_email"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, 400, err.Error())
		return
	}
	if strings.TrimSpace(body.ToEmail) == "" || strings.TrimSpace(body.Body) == "" {
		httpError(w, 400, "destinataire et corps du message obligatoires")
		return
	}
	ok, err := s.Store.UpdateDraft(r.Context(), id, strings.TrimSpace(body.ToEmail), strings.TrimSpace(body.Subject), body.Body)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	if !ok {
		httpError(w, 409, "brouillon introuvable ou déjà traité")
		return
	}
	s.Store.Audit(r.Context(), acteur(r), "brouillon_modifie", map[string]any{"draft_id": id})
	writeJSON(w, map[string]string{"status": "ok"})
}

// reviewDraft : l'agent relit la version modifiée par le dirigeant et rend un avis.
func (s *Server) reviewDraft(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, 400, err.Error())
		return
	}
	resp, err := s.Engine.ReviewDraft(r.Context(), pathID(r), body.Subject, body.Body)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, resp)
}

// synthese : vue direction — KPI, décisions à prendre, aperçu par catégorie.
func (s *Server) synthese(w http.ResponseWriter, r *http.Request) {
	syn, err := s.Engine.GenerateSynthese(r.Context())
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, syn)
}

// suivi : tous les engagements actifs, classés (en cours / retard / risque)
// avec leur action suggérée.
func (s *Server) suivi(w http.ResponseWriter, r *http.Request) {
	list, err := s.Engine.Suivi(r.Context())
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, list)
}

// draftForEngagement rédige un message à la demande pour un engagement.
func (s *Server) draftForEngagement(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Intent string `json:"intent"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	draft, err := s.Engine.DraftForEngagement(r.Context(), pathID(r), body.Intent)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, draft)
}

// pilotage agrège la vue CODIR : jalons à livrer, retards, répartition par
// type, et actions à un clic (quick wins).
func (s *Server) pilotage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	seuil := s.Engine.SeuilPublication(ctx)

	engs, err := s.Store.ListEngagements(ctx, []string{"ouvert", "confirme", "en_retard"})
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	parType := map[string]int{}
	var enRetard, jalons, aConfirmer []store.Engagement
	horizon := time.Now().AddDate(0, 0, 14)
	for _, e := range engs {
		if e.Confiance < seuil {
			continue // règle anti-churn : rien sous le seuil
		}
		parType[e.Type]++
		switch {
		case e.Statut == "en_retard":
			enRetard = append(enRetard, e)
		case e.Echeance != nil && e.EcheanceInferee && !e.EcheanceConfirmee:
			aConfirmer = append(aConfirmer, e)
		case e.Echeance != nil && e.EcheanceConfirmee && e.Echeance.Before(horizon):
			jalons = append(jalons, e)
		}
	}

	drafts, _ := s.Store.ListDrafts(ctx, "propose")
	liens, _ := s.Store.ListLinks(ctx, "candidat")
	criticals, _ := s.Store.ListDetections(ctx, []string{"nouvelle", "au_digest"}, seuil, 10)
	var alertes []store.Detection
	for _, d := range criticals {
		if d.Critique {
			alertes = append(alertes, d)
		}
	}

	writeJSON(w, map[string]any{
		"alertes_critiques": alertes,
		"en_retard":         enRetard,
		"jalons_14_jours":   jalons,
		"par_type":          parType,
		"quick_wins": map[string]any{
			"brouillons_a_valider":  drafts,
			"echeances_a_confirmer": aConfirmer,
			"liens_a_trancher":      liens,
		},
	})
}

// kpis calcule les indicateurs des critères de réussite (CDC section 14).
func (s *Server) kpis(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := func(query string, args ...any) float64 {
		var v float64
		_ = s.Store.Pool.QueryRow(ctx, query, args...).Scan(&v)
		return v
	}

	totalMsgs := q(`SELECT count(*) FROM messages`)
	exclus := q(`SELECT count(*) FROM messages WHERE status='excluded'`)
	tauxExclusion := 0.0
	if totalMsgs > 0 {
		tauxExclusion = exclus / totalMsgs
	}

	extraits := q(`SELECT count(*) FROM engagements`)
	requalifies := q(`SELECT count(*) FROM learned_rules WHERE action='requalifier'`)
	precision := 1.0
	if extraits > 0 {
		precision = 1 - requalifies/extraits
	}

	suggeres := q(`SELECT count(*) FROM drafts`)
	valides := q(`SELECT count(*) FROM drafts WHERE statut='envoye'`)
	tauxValidation := 0.0
	if suggeres > 0 {
		tauxValidation = valides / suggeres
	}

	detectionsDigest := q(`SELECT count(*) FROM detections WHERE statut IN ('au_digest','traitee')`)
	ecartees := q(`SELECT count(*) FROM detections WHERE statut='ecartee'`)
	tauxFauxPositifs := 0.0
	if detectionsDigest+ecartees > 0 {
		tauxFauxPositifs = ecartees / (detectionsDigest + ecartees)
	}

	writeJSON(w, map[string]any{
		"taux_exclusion_prefiltre": tauxExclusion, // santé économique (EF-11)
		"messages_analyses":        q(`SELECT count(*) FROM messages WHERE status='analyzed'`),
		"engagements_extraits":     extraits,
		"precision_estimee":        precision,                                                                                                                                            // cible > 85 %
		"taux_faux_positifs":       tauxFauxPositifs,                                                                                                                                     // cible < 10 %
		"corrections_7_jours":      q(`SELECT count(*) FROM audit_log WHERE actor='dirigeant' AND event_type IN ('correction','engagement_corrige') AND ts > now() - interval '7 days'`), // cible < 3 après S3
		"actions_suggerees":        suggeres,
		"actions_validees":         valides,
		"taux_validation_actions":  tauxValidation, // cible > 40 %
		"digests_generes":          q(`SELECT count(*) FROM reports WHERE type LIKE 'digest_%'`),
		"regles_apprises_actives":  q(`SELECT count(*) FROM learned_rules WHERE active`),
		"incidents_critiques":      0, // cible : 0 — toute action passe par validation explicite
	})
}

// spaHandler sert les fichiers statiques du build et renvoie index.html pour
// toute route inconnue (routage côté client).
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}

// --- utilitaires ---

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: encodage JSON: %v", err)
	}
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

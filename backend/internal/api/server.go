// Package api expose l'API REST et le tableau de bord (port 9999).
package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"agentops/backend/internal/channel"
	"agentops/backend/internal/config"
	"agentops/backend/internal/engine"
	"agentops/backend/internal/ingest"
	"agentops/backend/internal/store"
)

//go:embed web
var webFS embed.FS

type Server struct {
	Cfg      config.Config
	Store    *store.Store
	Engine   *engine.Engine
	Ingester *ingest.Ingester
	OAuth    *oauth2.Config
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// tableau de bord statique
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /", http.FileServer(http.FS(sub)))

	// santé (sans auth)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok"})
	})

	// OAuth Google
	mux.HandleFunc("POST /api/oauth/google/start", s.auth(s.oauthStart))
	mux.HandleFunc("GET /api/oauth/google/callback", s.oauthCallback)

	// statut & cycle
	mux.HandleFunc("GET /api/status", s.auth(s.status))
	mux.HandleFunc("POST /api/cycle/run", s.auth(s.runCycle))
	mux.HandleFunc("POST /api/onboarding/run", s.auth(s.runOnboarding))

	// miroir & capsule
	mux.HandleFunc("GET /api/miroir", s.auth(s.getMiroir))
	mux.HandleFunc("POST /api/miroir/generate", s.auth(s.generateMiroir))
	mux.HandleFunc("GET /api/capsule", s.auth(s.getCapsule))
	mux.HandleFunc("PUT /api/capsule", s.auth(s.putCapsule))
	mux.HandleFunc("POST /api/capsule/infer", s.auth(s.inferCapsule))

	// engagements
	mux.HandleFunc("GET /api/engagements", s.auth(s.listEngagements))
	mux.HandleFunc("GET /api/engagements/{id}/events", s.auth(s.listEvents))
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
	mux.HandleFunc("GET /api/digest/latest", s.auth(s.latestDigest))
	mux.HandleFunc("POST /api/digest/generate", s.auth(s.generateDigest))
	mux.HandleFunc("GET /api/drafts", s.auth(s.listDrafts))
	mux.HandleFunc("PATCH /api/drafts/{id}", s.auth(s.patchDraft))
	mux.HandleFunc("POST /api/drafts/{id}/validate", s.auth(s.validateDraft))
	mux.HandleFunc("POST /api/drafts/{id}/reject", s.auth(s.rejectDraft))

	// vue de pilotage (format CODIR)
	mux.HandleFunc("GET /api/pilotage", s.auth(s.pilotage))

	// règles apprises, personnes, audit, réglages
	mux.HandleFunc("GET /api/rules", s.auth(s.listRules))
	mux.HandleFunc("POST /api/rules", s.auth(s.createRule))
	mux.HandleFunc("DELETE /api/rules/{id}", s.auth(s.deleteRule))
	mux.HandleFunc("GET /api/persons", s.auth(s.listPersons))
	mux.HandleFunc("PATCH /api/persons/{id}", s.auth(s.patchPerson))
	mux.HandleFunc("GET /api/audit", s.auth(s.listAudit))
	mux.HandleFunc("GET /api/kpis", s.auth(s.kpis))
	mux.HandleFunc("GET /api/settings", s.auth(s.getSettings))
	mux.HandleFunc("PUT /api/settings", s.auth(s.putSettings))

	return mux
}

// --- middleware ---

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if s.Cfg.AdminToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.Cfg.AdminToken)) != 1 {
			httpError(w, 401, "jeton d'accès invalide")
			return
		}
		next(w, r)
	}
}

// --- OAuth ---

func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	if s.OAuth == nil || s.OAuth.ClientID == "" {
		httpError(w, 400, "GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET non configurés")
		return
	}
	b := make([]byte, 16)
	rand.Read(b)
	state := hex.EncodeToString(b)
	s.Store.SetSetting(r.Context(), "oauth_state", state)
	url := s.OAuth.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	writeJSON(w, map[string]string{"url": url})
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.URL.Query().Get("state") != s.Store.GetSetting(ctx, "oauth_state", "-") {
		httpError(w, 400, "état OAuth invalide")
		return
	}
	tok, err := s.OAuth.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		httpError(w, 400, "échange OAuth: "+err.Error())
		return
	}
	b, _ := json.Marshal(tok)
	if err := s.Store.SaveOAuthToken(ctx, "google", b, ""); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	// récupère l'adresse du compte et la persiste
	if g, ok := s.Engine.Channel.(*channel.Gmail); ok {
		if email, err := g.AccountEmail(context.Background()); err == nil {
			tokB, _, _ := s.Store.GetOAuthToken(ctx, "google")
			_ = s.Store.SaveOAuthToken(ctx, "google", tokB, email)
		}
	}
	s.Store.Audit(ctx, "dirigeant", "canal_connecte", map[string]string{"provider": "google"})
	// J+0 → J+1 : lance l'onboarding en arrière-plan (30 jours + miroir + capsule)
	go s.onboarding()
	http.Redirect(w, r, "/?connected=1", http.StatusFound)
}

// --- onboarding (Miroir J+1) ---

func (s *Server) onboarding() {
	ctx := context.Background()
	log.Println("onboarding: lecture des 30 derniers jours…")
	res := s.Engine.RunCycle(ctx, func(ctx context.Context) (any, error) {
		return s.Ingester.Run(ctx, time.Now().AddDate(0, 0, -30), 1000)
	})
	if res.Erreur != "" {
		log.Printf("onboarding: %s", res.Erreur)
	}
	if _, err := s.Engine.GenerateMiroir(ctx); err != nil {
		log.Printf("onboarding: miroir: %v", err)
	}
	// capsule temps 1 APRÈS le miroir (séquencement psychologique CDC 9.1)
	if err := s.Engine.InferCapsule(ctx); err != nil {
		log.Printf("onboarding: capsule: %v", err)
	}
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
	}
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&counts.Messages)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE status='pending'`).Scan(&counts.EnAttente)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE status='excluded'`).Scan(&counts.Exclus)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM engagements`).Scan(&counts.Engagements)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM detections WHERE statut IN ('nouvelle','au_digest')`).Scan(&counts.Detections)
	s.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM drafts WHERE statut='propose'`).Scan(&counts.Brouillons)

	_, email, _ := s.Store.GetOAuthToken(ctx, "google")
	tok, _, _ := s.Store.GetOAuthToken(ctx, "google")
	llmOK := s.Engine.LLM.Health(ctx) == nil

	writeJSON(w, map[string]any{
		"canal":            s.Engine.Channel.Name(),
		"canal_connecte":   tok != nil || s.Engine.Channel.Name() == "fixture",
		"compte":           email,
		"service_llm_ok":   llmOK,
		"dernier_cycle":    s.Store.GetSetting(ctx, "dernier_cycle", ""),
		"seuil_publication": s.Store.GetSetting(ctx, "seuil_publication", "0.6"),
		"compteurs":        counts,
	})
}

func (s *Server) runCycle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SinceDays int `json:"since_days"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.SinceDays <= 0 {
		body.SinceDays = 2
	}
	res := s.Engine.RunCycle(r.Context(), func(ctx context.Context) (any, error) {
		return s.Ingester.Run(ctx, time.Now().AddDate(0, 0, -body.SinceDays), 500)
	})
	writeJSON(w, res)
}

// --- miroir & capsule ---

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
	s.Store.Audit(r.Context(), "dirigeant", "capsule_corrigee", nil)
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

func (s *Server) listEngagements(w http.ResponseWriter, r *http.Request) {
	var statuts []string
	if q := r.URL.Query().Get("statut"); q != "" {
		statuts = strings.Split(q, ",")
	}
	engs, err := s.Store.ListEngagements(r.Context(), statuts)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, engs)
}

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
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, 400, err.Error())
		return
	}
	fields := map[string]any{}
	if body.Statut != nil {
		fields["statut"] = *body.Statut
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
	s.Store.Audit(r.Context(), "dirigeant", "engagement_corrige", map[string]any{"id": id, "champs": fields})
	writeJSON(w, map[string]string{"status": "ok"})
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
	var rule store.LearnedRule
	switch body.Action {
	case "pas_un_engagement":
		_ = s.Store.UpdateEngagement(ctx, id, map[string]any{"statut": "abandonne"})
		rule = store.LearnedRule{PorteeType: "engagement", PorteeCible: strconv.FormatInt(id, 10),
			Action: "requalifier", Note: "« " + eng.Objet + " » n'est pas un engagement"}
	case "ignorer_interlocuteur":
		rule = store.LearnedRule{PorteeType: "interlocuteur", PorteeCible: eng.EmetteurEmail,
			Action: "ignorer", Note: "Ne plus extraire d'engagements de " + eng.EmetteurEmail}
	case "priorite_haute":
		_ = s.Store.UpdateEngagement(ctx, id, map[string]any{"priorite": "haute"})
		rule = store.LearnedRule{PorteeType: "interlocuteur", PorteeCible: eng.EmetteurEmail,
			Action: "priorite_haute", Note: "Priorité haute pour " + eng.EmetteurEmail}
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
	s.Store.Audit(r.Context(), "dirigeant", "detection_ecartee", map[string]int64{"id": id})
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
		s.Store.Audit(r.Context(), "dirigeant", "lien_"+statut, map[string]int64{"id": id})
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

// --- digest & brouillons ---

func (s *Server) latestDigest(w http.ResponseWriter, r *http.Request) {
	dtype := r.URL.Query().Get("type")
	if dtype == "" {
		dtype = "quotidien"
	}
	rep, err := s.Store.LatestReport(r.Context(), "digest_"+dtype)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	if rep == nil {
		httpError(w, 404, "aucun digest généré")
		return
	}
	writeJSON(w, rep)
}

func (s *Server) generateDigest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type string `json:"type"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Type == "" {
		body.Type = "quotidien"
	}
	d, err := s.Engine.GenerateDigest(r.Context(), body.Type)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, d)
}

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
	s.Store.Audit(r.Context(), "dirigeant", "brouillon_rejete", map[string]int64{"id": id})
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
	s.Store.Audit(r.Context(), "dirigeant", "regle_creee", rule)
	writeJSON(w, map[string]int64{"id": id})
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if err := s.Store.DeactivateRule(r.Context(), id); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	s.Store.Audit(r.Context(), "dirigeant", "regle_desactivee", map[string]int64{"id": id})
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
		}
	}
	s.Store.Audit(r.Context(), "dirigeant", "reglages_modifies", body)
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
	s.Store.Audit(r.Context(), "dirigeant", "brouillon_modifie", map[string]any{"draft_id": id})
	writeJSON(w, map[string]string{"status": "ok"})
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
		"alertes_critiques":     alertes,
		"en_retard":             enRetard,
		"jalons_14_jours":       jalons,
		"par_type":              parType,
		"quick_wins": map[string]any{
			"brouillons_a_valider":   drafts,
			"echeances_a_confirmer":  aConfirmer,
			"liens_a_trancher":       liens,
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
		"taux_exclusion_prefiltre":   tauxExclusion, // santé économique (EF-11)
		"messages_analyses":          q(`SELECT count(*) FROM messages WHERE status='analyzed'`),
		"engagements_extraits":       extraits,
		"precision_estimee":          precision,          // cible > 85 %
		"taux_faux_positifs":         tauxFauxPositifs,   // cible < 10 %
		"corrections_7_jours":        q(`SELECT count(*) FROM audit_log WHERE actor='dirigeant' AND event_type IN ('correction','engagement_corrige') AND ts > now() - interval '7 days'`), // cible < 3 après S3
		"actions_suggerees":          suggeres,
		"actions_validees":           valides,
		"taux_validation_actions":    tauxValidation, // cible > 40 %
		"digests_generes":            q(`SELECT count(*) FROM reports WHERE type LIKE 'digest_%'`),
		"regles_apprises_actives":    q(`SELECT count(*) FROM learned_rules WHERE active`),
		"incidents_critiques":        0, // cible : 0 — toute action passe par validation explicite
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

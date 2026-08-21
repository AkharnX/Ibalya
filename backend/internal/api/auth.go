package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"ibalya/backend/internal/store"
)

const cookieSession = "ibalya_session"

type ctxUser struct{}

// utilisateur renvoie l'utilisateur de la requête, ou nil pour un accès service.
func utilisateur(r *http.Request) *store.User {
	if u, ok := r.Context().Value(ctxUser{}).(*store.User); ok {
		return u
	}
	return nil
}

// acteur nomme l'auteur d'une action dans l'audit trail : l'email de
// l'utilisateur connecté, ou « service » pour les scripts locaux.
func acteur(r *http.Request) string {
	if u := utilisateur(r); u != nil {
		return u.Email
	}
	return "service"
}

// estLocal indique si la requête provient réellement de la machine elle-même.
//
// Attention au piège du reverse proxy : nginx relaie les requêtes d'Internet
// depuis 127.0.0.1, donc l'adresse de pair ne suffit PAS à conclure. On exige
// en plus l'absence des en-têtes que nginx ajoute systématiquement à ce qu'il
// relaie. Ces en-têtes ne sont pas usurpables depuis l'extérieur : nginx les
// réécrit avec l'adresse réelle du client (proxy_set_header X-Real-IP
// $remote_addr), et le port 9999 est fermé au public.
func estLocal(r *http.Request) bool {
	if r.Header.Get("X-Real-IP") != "" || r.Header.Get("X-Forwarded-For") != "" {
		return false // requête relayée : elle vient d'Internet
	}
	hote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		hote = r.RemoteAddr
	}
	ip := net.ParseIP(hote)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) poserCookie(w http.ResponseWriter, token string, expire time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieSession,
		Value:    token,
		Path:     "/",
		Expires:  expire,
		HttpOnly: true, // inaccessible au JavaScript : protège d'un XSS
		Secure:   strings.HasPrefix(s.Cfg.PublicBaseURL, "https://"),
		SameSite: http.SameSiteLaxMode, // Lax : le retour OAuth de Google reste authentifié
	})
}

func (s *Server) effacerCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieSession, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: strings.HasPrefix(s.Cfg.PublicBaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
}

// POST /api/login
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email      string `json:"email"`
		MotDePasse string `json:"mot_de_passe"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, 400, "requête invalide")
		return
	}
	u, err := s.Store.Authenticate(r.Context(), body.Email, body.MotDePasse)
	if err != nil {
		s.Store.Audit(r.Context(), "anonyme", "connexion_refusee", map[string]string{"email": body.Email})
		httpError(w, 401, err.Error())
		return
	}
	token, expire, err := s.Store.CreateSession(r.Context(), u.ID)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	s.poserCookie(w, token, expire)
	s.Store.Audit(r.Context(), u.Email, "connexion", nil)
	writeJSON(w, u)
}

// POST /api/logout
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieSession); err == nil {
		_ = s.Store.DeleteSession(r.Context(), c.Value)
	}
	s.effacerCookie(w)
	s.Store.Audit(r.Context(), acteur(r), "deconnexion", nil)
	writeJSON(w, map[string]string{"status": "ok"})
}

// GET /api/me
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	if u := utilisateur(r); u != nil {
		writeJSON(w, u)
		return
	}
	writeJSON(w, map[string]string{"email": "service", "nom": "Accès service local"})
}

// POST /api/password — changement de son propre mot de passe
func (s *Server) changerMotDePasse(w http.ResponseWriter, r *http.Request) {
	u := utilisateur(r)
	if u == nil {
		httpError(w, 403, "réservé aux comptes nominatifs")
		return
	}
	var body struct {
		Actuel   string `json:"actuel"`
		Nouveau  string `json:"nouveau"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, 400, "requête invalide")
		return
	}
	if _, err := s.Store.Authenticate(r.Context(), u.Email, body.Actuel); err != nil {
		httpError(w, 401, "mot de passe actuel incorrect")
		return
	}
	if err := s.Store.SetPassword(r.Context(), u.ID, body.Nouveau); err != nil {
		httpError(w, 400, err.Error())
		return
	}
	s.Store.Audit(r.Context(), u.Email, "mot_de_passe_modifie", nil)
	writeJSON(w, map[string]string{"status": "ok"})
}

// GET /api/users — la liste des comptes, pour l'écran Réglages
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.Store.ListUsers(r.Context())
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, users)
}

// ─── Connexion via Google ───
// Flux distinct de celui qui donne accès à Gmail : ici on ne demande que
// l'identité (openid, email, profil), jamais le contenu de la boîte.
// Règle : Google prouve QUI vous êtes, il ne décide pas si vous avez le droit
// d'entrer. Seules les adresses déjà provisionnées ouvrent une session.

const cookieEtatOAuth = "ibalya_oauth_login"

func (s *Server) configLoginGoogle() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.Cfg.GoogleClientID,
		ClientSecret: s.Cfg.GoogleClientSecret,
		RedirectURL:  s.Cfg.PublicBaseURL + "/api/oauth/google/login/callback",
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

// GET /api/oauth/google/login — départ du parcours (lien direct, pas de session requise)
func (s *Server) googleLoginStart(w http.ResponseWriter, r *http.Request) {
	if s.Cfg.GoogleClientID == "" {
		httpError(w, 400, "connexion Google non configurée")
		return
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	etat := hex.EncodeToString(b)
	// l'état vit dans un cookie éphémère : deux connexions simultanées ne se
	// marchent pas dessus, contrairement à un état global en base
	http.SetCookie(w, &http.Cookie{
		Name: cookieEtatOAuth, Value: etat, Path: "/", MaxAge: 600,
		HttpOnly: true, Secure: strings.HasPrefix(s.Cfg.PublicBaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, s.configLoginGoogle().AuthCodeURL(etat), http.StatusFound)
}

// GET /api/oauth/google/login/callback
func (s *Server) googleLoginCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	echec := func(motif string) {
		http.Redirect(w, r, "/?erreur="+url.QueryEscape(motif), http.StatusFound)
	}

	c, err := r.Cookie(cookieEtatOAuth)
	if err != nil || c.Value == "" || c.Value != r.URL.Query().Get("state") {
		echec("Requête de connexion invalide ou expirée. Réessayez.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieEtatOAuth, Value: "", Path: "/", MaxAge: -1})

	tok, err := s.configLoginGoogle().Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		echec("Échec de l'échange avec Google.")
		return
	}
	email, verifie, err := infosCompteGoogle(ctx, s.configLoginGoogle(), tok)
	if err != nil {
		echec("Impossible de lire votre compte Google.")
		return
	}
	if !verifie {
		echec("Cette adresse Google n'est pas vérifiée.")
		return
	}

	// Aucune création implicite : le compte doit exister et être actif.
	u, err := s.Store.UserByEmail(ctx, email)
	if err != nil || u == nil || !u.Actif {
		s.Store.Audit(ctx, "anonyme", "connexion_google_refusee", map[string]string{"email": email})
		echec("Aucun compte Ibalya n'est associé à " + email + ".")
		return
	}
	token, expire, err := s.Store.CreateSession(ctx, u.ID)
	if err != nil {
		echec("Création de session impossible.")
		return
	}
	s.poserCookie(w, token, expire)
	s.Store.Audit(ctx, u.Email, "connexion", map[string]string{"methode": "google"})
	http.Redirect(w, r, "/", http.StatusFound)
}

// infosCompteGoogle interroge le point d'accès userinfo : plus simple et plus
// sûr que de décoder soi-même le jeton d'identité.
func infosCompteGoogle(ctx context.Context, cfg *oauth2.Config, tok *oauth2.Token) (string, bool, error) {
	resp, err := cfg.Client(ctx, tok).Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("userinfo: statut %d", resp.StatusCode)
	}
	var infos struct {
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&infos); err != nil {
		return "", false, err
	}
	return infos.Email, infos.VerifiedEmail, nil
}

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ibalya/backend/internal/channel"
)

// Raccordement du canal depuis l'interface.
//
// Le connecteur IMAP existait mais aucune porte d'entrée : raccorder une autre
// boîte que Gmail supposait d'éditer le fichier d'environnement sur le serveur
// et de redémarrer. Un client ne peut pas faire ça.

type ConfigCanal struct {
	Type        string `json:"type"` // gmail | imap
	Hote        string `json:"hote"`
	Port        int    `json:"port"`
	Utilisateur string `json:"utilisateur"`
	MotDePasse  string `json:"mot_de_passe,omitempty"` // jamais renvoyé en lecture
	Dossier     string `json:"dossier"`
	SMTPHote    string `json:"smtp_hote"`
	SMTPPort    int    `json:"smtp_port"`
	// MotDePasseEnregistre signale qu'un secret existe, sans le divulguer :
	// l'interface affiche un champ pré-rempli de points plutôt que vide.
	MotDePasseEnregistre bool `json:"mot_de_passe_enregistre"`
}

func (s *Server) lireConfigCanal(ctx context.Context) ConfigCanal {
	g := func(c, def string) string { return s.Store.GetSetting(ctx, c, def) }
	n := func(c string, def int) int {
		if v, err := strconv.Atoi(g(c, "")); err == nil && v > 0 {
			return v
		}
		return def
	}
	return ConfigCanal{
		Type:                 g("canal_type", s.Cfg.Channel),
		Hote:                 g("imap_hote", s.Cfg.IMAPHote),
		Port:                 n("imap_port", 993),
		Utilisateur:          g("imap_utilisateur", s.Cfg.IMAPUtilisateur),
		Dossier:              g("imap_dossier", "INBOX"),
		SMTPHote:             g("smtp_hote", ""),
		SMTPPort:             n("smtp_port", 587),
		MotDePasseEnregistre: g("imap_mot_de_passe", "") != "",
	}
}

// GET /api/canal
func (s *Server) getCanal(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.lireConfigCanal(r.Context()))
}

// construireIMAP assemble un connecteur depuis une configuration soumise. Un
// mot de passe vide signifie « garder celui déjà enregistré » : l'interface ne
// le reçoit jamais en lecture, elle ne peut donc pas le renvoyer.
func (s *Server) construireIMAP(ctx context.Context, c ConfigCanal) (*channel.IMAP, error) {
	mdp := c.MotDePasse
	if strings.TrimSpace(mdp) == "" {
		chiffre := s.Store.GetSetting(ctx, "imap_mot_de_passe", "")
		if chiffre == "" {
			return nil, fmt.Errorf("mot de passe requis")
		}
		clair, err := s.Coffre.Dechiffrer(chiffre)
		if err != nil {
			return nil, err
		}
		mdp = clair
	}
	if strings.TrimSpace(c.Hote) == "" || strings.TrimSpace(c.Utilisateur) == "" {
		return nil, fmt.Errorf("hôte et adresse sont requis")
	}
	return channel.NewIMAP(channel.IMAPConfig{
		Hote: strings.TrimSpace(c.Hote), Port: c.Port,
		Utilisateur: strings.TrimSpace(c.Utilisateur), MotDePasse: mdp,
		Dossier:  c.Dossier,
		SMTPHote: c.SMTPHote, SMTPPort: c.SMTPPort,
		TLSSansVerification: s.Cfg.IMAPTLSSansVerif,
	}), nil
}

// POST /api/canal/tester — éprouve réellement la connexion.
//
// Sans cette épreuve, une erreur de saisie ne se découvre qu'au cycle suivant,
// jusqu'à quinze minutes plus tard, sous la forme d'une absence de résultat.
func (s *Server) testerCanal(w http.ResponseWriter, r *http.Request) {
	var c ConfigCanal
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		httpError(w, 400, "requête invalide")
		return
	}
	if c.Type != "imap" {
		httpError(w, 400, "seul le canal IMAP se teste ici ; Gmail passe par OAuth")
		return
	}
	imap, err := s.construireIMAP(r.Context(), c)
	if err != nil {
		httpError(w, 400, err.Error())
		return
	}
	// Un délai borné : un hôte injoignable ne doit pas faire attendre indéfiniment.
	ctx, annuler := context.WithTimeout(r.Context(), 25*time.Second)
	defer annuler()

	msgs, err := imap.FetchSince(ctx, time.Now().AddDate(0, 0, -7), 5)
	if err != nil {
		s.Store.Audit(r.Context(), acteur(r), "canal_test_echec",
			map[string]string{"hote": c.Hote, "erreur": err.Error()})
		httpError(w, 400, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"ok":       true,
		"messages": len(msgs),
		"dossier":  strings.TrimSpace(c.Dossier),
		"message": fmt.Sprintf("Connexion réussie : %d message(s) lus sur les 7 derniers jours.",
			len(msgs)),
	})
}

// PUT /api/canal — enregistre et bascule à chaud.
func (s *Server) putCanal(w http.ResponseWriter, r *http.Request) {
	var c ConfigCanal
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		httpError(w, 400, "requête invalide")
		return
	}
	ctx := r.Context()

	switch c.Type {
	case "gmail":
		s.Store.SetSetting(ctx, "canal_type", "gmail")
		s.Commutateur.Remplacer(channel.NewGmail(s.OAuth, s.Store))
	case "imap":
		imap, err := s.construireIMAP(ctx, c)
		if err != nil {
			httpError(w, 400, err.Error())
			return
		}
		// On n'enregistre qu'une configuration éprouvée : sinon le canal
		// bascule vers une boîte injoignable et l'agent s'arrête en silence.
		ctxTest, annuler := context.WithTimeout(ctx, 25*time.Second)
		defer annuler()
		if _, err := imap.FetchSince(ctxTest, time.Now().AddDate(0, 0, -7), 1); err != nil {
			httpError(w, 400, "connexion refusée, rien n'a été enregistré : "+err.Error())
			return
		}
		if mdp := strings.TrimSpace(c.MotDePasse); mdp != "" {
			chiffre, err := s.Coffre.Chiffrer(mdp)
			if err != nil {
				httpError(w, 500, "impossible de chiffrer le mot de passe : "+err.Error())
				return
			}
			s.Store.SetSetting(ctx, "imap_mot_de_passe", chiffre)
		}
		s.Store.SetSetting(ctx, "canal_type", "imap")
		s.Store.SetSetting(ctx, "imap_hote", strings.TrimSpace(c.Hote))
		s.Store.SetSetting(ctx, "imap_port", strconv.Itoa(c.Port))
		s.Store.SetSetting(ctx, "imap_utilisateur", strings.TrimSpace(c.Utilisateur))
		s.Store.SetSetting(ctx, "imap_dossier", strings.TrimSpace(c.Dossier))
		s.Store.SetSetting(ctx, "smtp_hote", strings.TrimSpace(c.SMTPHote))
		s.Store.SetSetting(ctx, "smtp_port", strconv.Itoa(c.SMTPPort))
		s.Commutateur.Remplacer(imap)
	default:
		httpError(w, 400, "type de canal inconnu : gmail ou imap")
		return
	}
	s.Store.Audit(ctx, acteur(r), "canal_raccorde",
		map[string]string{"type": c.Type, "hote": c.Hote, "compte": c.Utilisateur})
	writeJSON(w, s.lireConfigCanal(ctx))
}

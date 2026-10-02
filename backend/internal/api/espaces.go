package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"ibalya/backend/internal/engine"
	"ibalya/backend/internal/store"
)

// Gestion des boîtes (espaces) d'un compte. Un compte (login) possède plusieurs
// espaces isolés ; la session mémorise lequel est actif. Ces handlers raisonnent
// au niveau COMPTE : ils vérifient toujours l'appartenance de l'espace visé au
// compte connecté avant d'agir, sans jamais faire confiance à l'id envoyé.

// GET /api/espaces — les boîtes du compte connecté, l'active marquée.
func (s *Server) listEspaces(w http.ResponseWriter, r *http.Request) {
	c := compteDe(r)
	u := utilisateur(r)
	if c == nil || u == nil {
		httpError(w, 403, "réservé aux comptes nominatifs")
		return
	}
	esp, err := s.Store.ListEspaces(r.Context(), c.ID, u.ID)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, esp)
}

// POST /api/espaces — crée une boîte vide. Le raccordement se fait ensuite en
// basculant dessus, puis en lançant le flux OAuth/IMAP habituel (qui écrit dans
// le tenant de l'espace actif).
func (s *Server) creerEspace(w http.ResponseWriter, r *http.Request) {
	c := compteDe(r)
	if c == nil {
		httpError(w, 403, "réservé aux comptes nominatifs")
		return
	}
	var body struct {
		Libelle string `json:"libelle"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	id, err := s.Store.CreateEspace(r.Context(), c.ID, body.Libelle, "")
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	s.Store.Audit(r.Context(), acteur(r), "espace_cree", map[string]any{"espace": id})
	writeJSON(w, map[string]any{"id": id, "libelle": strings.TrimSpace(body.Libelle)})
}

// POST /api/espaces/{id}/activer — bascule la session sur une autre boîte.
func (s *Server) activerEspace(w http.ResponseWriter, r *http.Request) {
	c := compteDe(r)
	if c == nil {
		httpError(w, 403, "réservé aux comptes nominatifs")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpError(w, 400, "identifiant invalide")
		return
	}
	ok, err := s.Store.EspaceAppartientA(r.Context(), c.ID, id)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	if !ok {
		httpError(w, 404, "boîte introuvable")
		return
	}
	cook, err := r.Cookie(cookieSession)
	if err != nil {
		httpError(w, 401, "session requise")
		return
	}
	if err := s.Store.SetEspaceActif(r.Context(), cook.Value, id); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	writeJSON(w, map[string]any{"status": "ok", "espace_actif": id})
}

// DELETE /api/espaces/{id} — supprime une boîte et TOUTES ses données (jamais la
// messagerie d'origine). Refuse la dernière boîte du compte.
func (s *Server) supprimerEspace(w http.ResponseWriter, r *http.Request) {
	c := compteDe(r)
	u := utilisateur(r)
	if c == nil || u == nil {
		httpError(w, 403, "réservé aux comptes nominatifs")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpError(w, 400, "identifiant invalide")
		return
	}
	ok, err := s.Store.EspaceAppartientA(r.Context(), c.ID, id)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	if !ok {
		httpError(w, 404, "boîte introuvable")
		return
	}
	// On ne laisse jamais un compte sans aucune boîte.
	esp, err := s.Store.ListEspaces(r.Context(), c.ID, u.ID)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	if len(esp) <= 1 {
		httpError(w, 400, "impossible de supprimer votre dernière boîte")
		return
	}
	// Si on supprime la boîte active, basculer la session sur une autre AVANT la
	// purge, sinon on détruirait la session courante avec l'espace.
	if id == u.ID {
		if cook, err := r.Cookie(cookieSession); err == nil {
			for _, e := range esp {
				if e.ID != id {
					_ = s.Store.SetEspaceActif(r.Context(), cook.Value, e.ID)
					break
				}
			}
		}
	}
	// Révocation best-effort du consentement Google avant la purge.
	s.revoquerJetonEspace(r.Context(), id)
	if err := s.Store.SupprimerEspace(r.Context(), c.ID, id); err != nil {
		httpError(w, 500, err.Error())
		return
	}
	s.Store.Audit(r.Context(), acteur(r), "espace_supprime", map[string]any{"espace": id})
	writeJSON(w, map[string]any{"status": "ok"})
}

// ─── Vue combinée : toutes les boîtes du compte ──────────────────────────────
// Chaque boîte est un tenant RLS isolé : impossible de tout lire en une requête.
// On boucle sur les boîtes du compte, on lit chacune dans son propre EnTenant, et
// on fusionne en marquant chaque ligne par sa boîte d'origine. ListEspaces ne
// renvoie que les boîtes du compte connecté : on ne croise jamais deux comptes.
// Lecture seule : agir sur un élément (envoyer, résoudre) se fait dans le tenant
// de sa boîte, donc le front renvoie l'utilisateur dans la boîte concernée.

// GET /api/synthese/global — synthèse agrégée sur toutes les boîtes du compte.
func (s *Server) syntheseGlobale(w http.ResponseWriter, r *http.Request) {
	c := compteDe(r)
	u := utilisateur(r)
	if c == nil || u == nil {
		httpError(w, 403, "réservé aux comptes nominatifs")
		return
	}
	espaces, err := s.Store.ListEspaces(r.Context(), c.ID, u.ID)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}

	type prioriteGlobale struct {
		engine.PrioriteItem
		Boite   string `json:"boite"`
		BoiteID int64  `json:"boite_id"`
	}
	type alerteGlobale struct {
		store.Detection
		Boite   string `json:"boite"`
		BoiteID int64  `json:"boite_id"`
	}
	var out struct {
		KPI struct {
			EngagementsSuivis int `json:"engagements_suivis"`
			Retards           int `json:"retards"`
			Risques           int `json:"risques"`
			MessagesAValider  int `json:"messages_a_valider"`
			MessagesLus       int `json:"messages_lus"`
		} `json:"kpi"`
		Priorites  []prioriteGlobale               `json:"priorites"`
		Alertes    []alerteGlobale                 `json:"alertes"`
		Categories map[string]engine.CategorieBloc `json:"categories"`
		Boites     int                             `json:"boites"`
	}
	out.Priorites = []prioriteGlobale{}
	out.Alertes = []alerteGlobale{}
	out.Categories = map[string]engine.CategorieBloc{}
	out.Boites = len(espaces)

	for _, e := range espaces {
		boite := e
		_ = s.Store.EnTenant(r.Context(), boite.ID, func(tctx context.Context) error {
			syn, err := s.Engine.GenerateSynthese(tctx)
			if err != nil || syn == nil {
				return nil // une boîte en échec ne casse pas la vue d'ensemble
			}
			out.KPI.EngagementsSuivis += syn.KPI.EngagementsSuivis
			out.KPI.Retards += syn.KPI.Retards
			out.KPI.Risques += syn.KPI.Risques
			out.KPI.MessagesAValider += syn.KPI.MessagesAValider
			out.KPI.MessagesLus += syn.KPI.MessagesLus
			for _, p := range syn.Priorites {
				out.Priorites = append(out.Priorites, prioriteGlobale{PrioriteItem: p, Boite: boite.Libelle, BoiteID: boite.ID})
			}
			for _, a := range syn.Alertes {
				out.Alertes = append(out.Alertes, alerteGlobale{Detection: a, Boite: boite.Libelle, BoiteID: boite.ID})
			}
			for cat, bloc := range syn.Categories {
				cur := out.Categories[cat]
				cur.Nombre += bloc.Nombre
				for _, ap := range bloc.Apercu {
					if len(cur.Apercu) < 3 {
						cur.Apercu = append(cur.Apercu, ap)
					}
				}
				out.Categories[cat] = cur
			}
			return nil
		})
	}

	// Mêmes règles de tri que la synthèse d'une boîte : le risque avant le retard.
	rang := func(cat string) int {
		if cat == engine.CatRisque {
			return 0
		}
		return 1
	}
	sort.SliceStable(out.Priorites, func(i, j int) bool {
		return rang(out.Priorites[i].Categorie) < rang(out.Priorites[j].Categorie)
	})

	writeJSON(w, out)
}

// GET /api/drafts/global — messages à valider de toutes les boîtes du compte.
func (s *Server) draftsGlobaux(w http.ResponseWriter, r *http.Request) {
	c := compteDe(r)
	u := utilisateur(r)
	if c == nil || u == nil {
		httpError(w, 403, "réservé aux comptes nominatifs")
		return
	}
	espaces, err := s.Store.ListEspaces(r.Context(), c.ID, u.ID)
	if err != nil {
		httpError(w, 500, err.Error())
		return
	}
	type draftGlobal struct {
		store.Draft
		Boite   string `json:"boite"`
		BoiteID int64  `json:"boite_id"`
	}
	out := []draftGlobal{}
	for _, e := range espaces {
		boite := e
		_ = s.Store.EnTenant(r.Context(), boite.ID, func(tctx context.Context) error {
			drafts, err := s.Store.ListDrafts(tctx, "propose")
			if err != nil {
				return nil
			}
			for _, d := range drafts {
				out = append(out, draftGlobal{Draft: d, Boite: boite.Libelle, BoiteID: boite.ID})
			}
			return nil
		})
	}
	writeJSON(w, out)
}

// revoquerJetonEspace tente de révoquer chez Google le consentement de la boîte
// d'un espace. Best-effort : la suppression des données ne dépend pas de sa
// réussite (l'utilisateur peut aussi révoquer côté compte Google).
func (s *Server) revoquerJetonEspace(ctx context.Context, espaceID int64) {
	_ = s.Store.EnTenant(ctx, espaceID, func(tctx context.Context) error {
		raw, _, err := s.Store.GetOAuthToken(tctx, "google")
		if err != nil || len(raw) == 0 {
			return nil
		}
		var tok oauth2.Token
		if json.Unmarshal(raw, &tok) != nil {
			return nil
		}
		jeton := tok.AccessToken
		if jeton == "" {
			jeton = tok.RefreshToken
		}
		if jeton == "" {
			return nil
		}
		ctxT, cancel := context.WithTimeout(tctx, 8*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctxT, "POST",
			"https://oauth2.googleapis.com/revoke",
			strings.NewReader(url.Values{"token": {jeton}}.Encode()))
		if err != nil {
			return nil
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
		}
		return nil
	})
}

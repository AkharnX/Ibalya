package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ibalya/backend/internal/store"
)

// Vue de suivi : chaque engagement est classé en une catégorie unique et porte
// l'action que le dirigeant peut déclencher d'un clic. Principe de la maquette :
// une cause racine (ex. livraison fournisseur en retard) produit UNE action de
// relance, même si elle menace plusieurs engagements en aval.

const (
	CatEnCours = "encours"
	CatRetard  = "retard"
	CatRisque  = "risque"
)

// Blocage décrit la cause amont qui met un engagement en risque.
type Blocage struct {
	AmontID      int64  `json:"amont_id"`
	AmontObjet   string `json:"amont_objet"`
	AmontEmetteur string `json:"amont_emetteur"`
	AmontEcheance string `json:"amont_echeance"`
}

// ActionSuggeree : libellé affiché et intention passée au rédacteur de message.
type ActionSuggeree struct {
	Label  string `json:"label"`
	Intent string `json:"intent"`
	// destinataire réel de l'action (peut différer du contact de l'engagement :
	// pour un blocage amont, on relance le fournisseur, pas le client)
	ToEmail string `json:"to_email"`
}

// EngagementSuivi enrichit un engagement des informations de la vue de suivi.
type EngagementSuivi struct {
	store.Engagement
	Categorie string          `json:"categorie"`
	Sortant   bool            `json:"sortant"` // promesse faite PAR le dirigeant
	Blocage   *Blocage        `json:"blocage,omitempty"`
	Action    *ActionSuggeree `json:"action,omitempty"`
	Contact   string          `json:"contact"` // l'autre partie
}

// blocages retourne, pour chaque engagement aval, la cause amont qui le bloque.
// Seuls les liens CONFIRMÉS comptent (CDC 8.1 : jamais sur un lien candidat).
func (e *Engine) blocages(ctx context.Context) map[int64]Blocage {
	out := map[int64]Blocage{}
	rows, err := e.Store.Pool.Query(ctx, `
		SELECT l.aval_id, a.id, a.objet, coalesce(pe.email, ''), a.echeance
		FROM dependency_links l
		JOIN engagements a ON a.id = l.amont_id
		LEFT JOIN persons pe ON pe.id = a.emetteur_id
		WHERE l.statut = 'confirme' AND a.statut = 'en_retard'`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var avalID int64
		var b Blocage
		var ech *time.Time
		if err := rows.Scan(&avalID, &b.AmontID, &b.AmontObjet, &b.AmontEmetteur, &ech); err != nil {
			continue
		}
		if ech != nil {
			b.AmontEcheance = ech.Format("02/01/2006")
		}
		out[avalID] = b
	}
	return out
}

// Suivi construit la liste complète des engagements actifs, classés et
// assortis de leur action suggérée.
func (e *Engine) Suivi(ctx context.Context) ([]EngagementSuivi, error) {
	seuil := e.SeuilPublication(ctx)
	engs, err := e.Store.ListEngagements(ctx, []string{"ouvert", "confirme", "en_retard"})
	if err != nil {
		return nil, err
	}
	blocages := e.blocages(ctx)
	account, _ := e.Channel.AccountEmail(ctx)

	out := make([]EngagementSuivi, 0, len(engs))
	for _, eng := range engs {
		if eng.Confiance < seuil {
			continue // règle anti-churn : rien sous le seuil en vue proactive
		}
		s := EngagementSuivi{Engagement: eng}
		s.Sortant = account != "" && strings.EqualFold(eng.EmetteurEmail, account)
		s.Contact = eng.DestinataireEmail
		if !s.Sortant {
			s.Contact = eng.EmetteurEmail
		}
		switch {
		case eng.Statut == "en_retard":
			s.Categorie = CatRetard
		default:
			if b, ok := blocages[eng.ID]; ok {
				s.Categorie = CatRisque
				bb := b
				s.Blocage = &bb
			} else {
				s.Categorie = CatEnCours
			}
		}
		s.Action = suggestAction(s)
		out = append(out, s)
	}
	return out, nil
}

// suggestAction déduit l'action la plus utile selon la catégorie, le type,
// le sens de l'engagement et la cause de blocage éventuelle.
func suggestAction(s EngagementSuivi) *ActionSuggeree {
	// Un engagement bloqué par un amont en retard : l'action utile est de
	// relancer la CAUSE, pas d'écrire au client qui subit.
	if s.Categorie == CatRisque && s.Blocage != nil {
		if s.Type == "rendez_vous" {
			return &ActionSuggeree{Label: "Proposer un nouveau créneau", Intent: "reporter_rdv", ToEmail: s.Contact}
		}
		return &ActionSuggeree{
			Label:   "Relancer le fournisseur (cause du blocage)",
			Intent:  "relance_cause",
			ToEmail: s.Blocage.AmontEmetteur,
		}
	}
	if s.Categorie == CatRetard {
		if s.Sortant {
			return &ActionSuggeree{Label: "Informer le client du retard", Intent: "info_retard", ToEmail: s.Contact}
		}
		return &ActionSuggeree{Label: "Relancer le fournisseur", Intent: "relance_fournisseur", ToEmail: s.Contact}
	}
	// en cours : l'action dépend du type d'engagement
	switch s.Type {
	case "devis":
		if s.Sortant {
			return &ActionSuggeree{Label: "Envoyer le devis promis", Intent: "envoi_devis", ToEmail: s.Contact}
		}
		return &ActionSuggeree{Label: "Relancer pour validation du devis", Intent: "relance_devis", ToEmail: s.Contact}
	case "facturation":
		return &ActionSuggeree{Label: "Envoyer la facture", Intent: "envoi_facture", ToEmail: s.Contact}
	case "rendez_vous":
		return &ActionSuggeree{Label: "Confirmer le rendez-vous", Intent: "confirmer_rdv", ToEmail: s.Contact}
	case "prise_de_contact":
		return &ActionSuggeree{Label: "Relancer le prospect", Intent: "relance_prospect", ToEmail: s.Contact}
	case "livraison":
		if s.Echeance != nil && time.Until(*s.Echeance) < 72*time.Hour {
			return &ActionSuggeree{Label: "Confirmer la date d'intervention", Intent: "confirmer_date", ToEmail: s.Contact}
		}
		return &ActionSuggeree{Label: "Envoyer un point d'avancement", Intent: "point_avancement", ToEmail: s.Contact}
	}
	return &ActionSuggeree{Label: "Envoyer un point d'avancement", Intent: "point_avancement", ToEmail: s.Contact}
}

// --- Synthèse (vue direction) ---

type PrioriteItem struct {
	EngagementID int64           `json:"engagement_id"`
	Categorie    string          `json:"categorie"`
	Titre        string          `json:"titre"`
	Contexte     string          `json:"contexte"`
	Action       *ActionSuggeree `json:"action,omitempty"`
}

type CategorieBloc struct {
	Nombre  int      `json:"nombre"`
	Apercu  []string `json:"apercu"`
}

type Synthese struct {
	KPI struct {
		EngagementsSuivis int `json:"engagements_suivis"`
		Retards           int `json:"retards"`
		Risques           int `json:"risques"`
		MessagesAValider  int `json:"messages_a_valider"`
		MessagesLus       int `json:"messages_lus"`
	} `json:"kpi"`
	Priorites  []PrioriteItem           `json:"priorites"`
	Categories map[string]CategorieBloc `json:"categories"`
}

// GenerateSynthese consolide la vue direction : ce qui bloque, ce qui arrive,
// ce qui se traite en un clic — sans dupliquer une même cause racine.
func (e *Engine) GenerateSynthese(ctx context.Context) (*Synthese, error) {
	suivi, err := e.Suivi(ctx)
	if err != nil {
		return nil, err
	}
	s := &Synthese{Categories: map[string]CategorieBloc{}}
	s.KPI.EngagementsSuivis = len(suivi)
	_ = e.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&s.KPI.MessagesLus)
	_ = e.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM drafts WHERE statut='propose'`).Scan(&s.KPI.MessagesAValider)

	parCat := map[string][]EngagementSuivi{}
	for _, x := range suivi {
		parCat[x.Categorie] = append(parCat[x.Categorie], x)
	}
	s.KPI.Retards = len(parCat[CatRetard])
	s.KPI.Risques = len(parCat[CatRisque])

	for _, cat := range []string{CatEnCours, CatRetard, CatRisque} {
		list := parCat[cat]
		bloc := CategorieBloc{Nombre: len(list)}
		for i, x := range list {
			if i == 2 {
				break
			}
			bloc.Apercu = append(bloc.Apercu, apercuLigne(x))
		}
		s.Categories[cat] = bloc
	}

	// priorités : les engagements à risque d'abord, puis les retards
	for _, cat := range []string{CatRisque, CatRetard} {
		for _, x := range parCat[cat] {
			s.Priorites = append(s.Priorites, PrioriteItem{
				EngagementID: x.ID,
				Categorie:    x.Categorie,
				Titre:        x.Objet,
				Contexte:     contexteLigne(x),
				Action:       x.Action,
			})
		}
	}
	return s, nil
}

func apercuLigne(x EngagementSuivi) string {
	switch {
	case x.Blocage != nil:
		return fmt.Sprintf("%s — dépend de « %s », en retard", x.Objet, x.Blocage.AmontObjet)
	case x.Statut == "en_retard" && x.Echeance != nil:
		return fmt.Sprintf("%s — échéance dépassée le %s", x.Objet, x.Echeance.Format("02/01"))
	case x.Echeance != nil && !x.EcheanceConfirmee:
		return fmt.Sprintf("%s — échéance %s, à confirmer", x.Objet, x.Echeance.Format("02/01"))
	case x.Echeance != nil:
		return fmt.Sprintf("%s — échéance %s", x.Objet, x.Echeance.Format("02/01"))
	}
	return x.Objet
}

func contexteLigne(x EngagementSuivi) string {
	ech := "sans échéance"
	if x.Echeance != nil {
		ech = "échéance " + x.Echeance.Format("02/01")
	}
	if x.Blocage != nil {
		cause := x.Blocage.AmontObjet
		if x.Blocage.AmontEcheance != "" {
			cause += ", attendue le " + x.Blocage.AmontEcheance
		}
		return fmt.Sprintf("%s — bloquée par : %s", ech, cause)
	}
	if x.Statut == "en_retard" && x.Echeance != nil {
		jours := int(time.Since(*x.Echeance).Hours() / 24)
		return fmt.Sprintf("Échéance dépassée le %s (%d jour(s)) — %s",
			x.Echeance.Format("02/01"), jours, x.Contact)
	}
	return fmt.Sprintf("%s — %s", ech, x.Contact)
}

// DraftForEngagement rédige à la demande un message pour un engagement donné,
// selon l'intention choisie. Le brouillon reste au statut « propose » : aucun
// envoi sans validation explicite (marche 3).
func (e *Engine) DraftForEngagement(ctx context.Context, engagementID int64, intent string) (*store.Draft, error) {
	suivi, err := e.Suivi(ctx)
	if err != nil {
		return nil, err
	}
	var target *EngagementSuivi
	for i := range suivi {
		if suivi[i].ID == engagementID {
			target = &suivi[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("engagement %d introuvable ou sous le seuil", engagementID)
	}
	action := target.Action
	if intent != "" && (action == nil || action.Intent != intent) {
		action = &ActionSuggeree{Label: intentLabel(intent), Intent: intent, ToEmail: target.Contact}
	}
	if action == nil || action.ToEmail == "" {
		return nil, fmt.Errorf("aucun destinataire identifié pour cet engagement")
	}
	return e.draftFor(ctx, *target, *action)
}

func intentLabel(intent string) string {
	labels := map[string]string{
		"relance_cause":       "Relancer le fournisseur (cause du blocage)",
		"relance_fournisseur": "Relancer le fournisseur",
		"info_retard":         "Informer le client du retard",
		"relance_devis":       "Relancer pour validation du devis",
		"envoi_devis":         "Envoyer le devis promis",
		"envoi_facture":       "Envoyer la facture",
		"confirmer_rdv":       "Confirmer le rendez-vous",
		"reporter_rdv":        "Proposer un nouveau créneau",
		"relance_prospect":    "Relancer le prospect",
		"confirmer_date":      "Confirmer la date d'intervention",
		"point_avancement":    "Envoyer un point d'avancement",
	}
	if l, ok := labels[intent]; ok {
		return l
	}
	return "Envoyer un message"
}

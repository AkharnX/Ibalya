package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"ibalya/backend/internal/llm"
	"ibalya/backend/internal/store"
)

// --- Miroir d'activité (EF-2, livrable J+1) ---

type Miroir struct {
	GenereLe           time.Time          `json:"genere_le"`
	PeriodeJours       int                `json:"periode_jours"`
	MessagesLus        int                `json:"messages_lus"`
	EngagementsOuverts []store.Engagement `json:"engagements_ouverts"`
	EnRetardProbable   []store.Engagement `json:"en_retard_probable"`
	FilsSansReponse    []FilSansReponse   `json:"fils_sans_reponse"`
	Note               string             `json:"note"`
}

type FilSansReponse struct {
	ThreadID      int64  `json:"thread_id"`
	Sujet         string `json:"sujet"`
	Interlocuteur string `json:"interlocuteur"`
	JoursSilence  int    `json:"jours_silence"`
}

// GenerateMiroir produit le premier rapport après lecture de l'historique.
// Il est généré AVANT les questions de setup (séquencement psychologique, CDC 9.1).
func (e *Engine) GenerateMiroir(ctx context.Context) (*Miroir, error) {
	seuil := e.SeuilPublication(ctx)
	m := &Miroir{GenereLe: time.Now(), PeriodeJours: 30,
		Note: "Voici ce que je comprends de votre activité après lecture de vos 30 derniers jours. Corrigez-moi d'un geste : chaque correction m'apprend votre réalité."}

	_ = e.Store.Pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&m.MessagesLus)

	engs, err := e.Store.ListEngagements(ctx, []string{"ouvert", "confirme", "en_retard"})
	if err != nil {
		return nil, err
	}
	for _, eng := range engs {
		if eng.Confiance < seuil {
			continue // jamais présenté proactivement sous le seuil (CDC 7.2)
		}
		if eng.Statut == "en_retard" {
			m.EnRetardProbable = append(m.EnRetardProbable, eng)
		} else {
			m.EngagementsOuverts = append(m.EngagementsOuverts, eng)
		}
	}

	rows, err := e.Store.Pool.Query(ctx, `
		SELECT t.id, t.subject,
		       coalesce((SELECT m.sender FROM messages m WHERE m.thread_id=t.id AND NOT m.outbound ORDER BY m.sent_at DESC LIMIT 1), ''),
		       EXTRACT(EPOCH FROM now() - t.last_message_at)/86400.0
		FROM threads t
		WHERE NOT t.excluded AND t.last_message_at IS NOT NULL
		  AND t.last_message_at < now() - interval '4 days'
		  AND EXISTS (SELECT 1 FROM engagements e WHERE e.thread_id = t.id AND e.statut IN ('ouvert','confirme','en_retard'))
		ORDER BY t.last_message_at ASC LIMIT 20`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f FilSansReponse
			var jours float64
			if err := rows.Scan(&f.ThreadID, &f.Sujet, &f.Interlocuteur, &jours); err == nil {
				f.JoursSilence = int(jours)
				m.FilsSansReponse = append(m.FilsSansReponse, f)
			}
		}
		rows.Close()
	}

	if _, err := e.Store.SaveReport(ctx, "miroir", m); err != nil {
		return nil, err
	}
	e.Store.Audit(ctx, "agent", "miroir_genere", map[string]any{
		"engagements": len(m.EngagementsOuverts), "en_retard": len(m.EnRetardProbable), "fils_silencieux": len(m.FilsSansReponse)})
	return m, nil
}

// --- Capsule temps 1 : inférence des faits ---

func (e *Engine) InferCapsule(ctx context.Context) error {
	accountEmail, _ := e.Channel.AccountEmail(ctx)
	rows, err := e.Store.Pool.Query(ctx, `SELECT id, sender, array_to_string(recipients, ', '), subject, left(body, 1500), sent_at
		FROM messages WHERE status='analyzed' ORDER BY sent_at DESC LIMIT 40`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var sample []llm.ExtractMessage
	for rows.Next() {
		var m llm.ExtractMessage
		var sentAt time.Time
		if err := rows.Scan(&m.ID, &m.Sender, &m.To, &m.Subject, &m.Body, &sentAt); err == nil {
			m.SentAt = sentAt.Format(time.RFC3339)
			sample = append(sample, m)
		}
	}
	rows.Close()
	if len(sample) == 0 {
		return fmt.Errorf("aucun message analysé : lancer d'abord un cycle d'ingestion")
	}
	resp, err := e.LLM.InferCapsule(ctx, llm.CapsuleRequest{MessagesSample: sample, AccountEmail: accountEmail})
	if err != nil {
		return err
	}
	if err := e.Store.UpdateCapsule(ctx, resp.Facts, nil); err != nil {
		return err
	}
	e.Store.Audit(ctx, "agent", "capsule_inferee", nil)
	return nil
}

// --- Digest (EF-6) ---

type DigestContent struct {
	GenereLe    time.Time          `json:"genere_le"`
	Type        string             `json:"type"`
	Engagements []store.Engagement `json:"engagements_a_risque"`
	Detections  []store.Detection  `json:"detections"`
	Brouillons  []store.Draft      `json:"brouillons_proposes"`
}

// GenerateDigest assemble le digest : détections au-dessus du seuil uniquement
// (règle anti-churn), engagements triés par risque, brouillons d'action.
func (e *Engine) GenerateDigest(ctx context.Context, dtype string) (*DigestContent, error) {
	seuil := e.SeuilPublication(ctx)
	dc := &DigestContent{GenereLe: time.Now(), Type: dtype}

	dets, err := e.Store.ListDetections(ctx, []string{"nouvelle"}, seuil, 30)
	if err != nil {
		return nil, err
	}
	dc.Detections = dets

	// brouillons pour les détections actionnables (EF-7)
	for _, d := range dets {
		if draft := e.maybeDraft(ctx, d); draft != nil {
			dc.Brouillons = append(dc.Brouillons, *draft)
		}
	}

	engs, err := e.Store.ListEngagements(ctx, []string{"en_retard", "ouvert", "confirme"})
	if err == nil {
		for _, eng := range engs {
			if eng.Confiance < seuil {
				continue
			}
			risque := eng.Statut == "en_retard" ||
				(eng.Echeance != nil && eng.EcheanceConfirmee && time.Until(*eng.Echeance) < 7*24*time.Hour)
			if risque {
				dc.Engagements = append(dc.Engagements, eng)
			}
			if len(dc.Engagements) >= 15 {
				break
			}
		}
	}

	if _, err := e.Store.SaveReport(ctx, "digest_"+dtype, dc); err != nil {
		return nil, err
	}
	// les détections ne sont consommées qu'une fois le rapport sauvegardé :
	// en cas d'échec, elles restent « nouvelle » et reviendront au prochain digest
	for _, d := range dets {
		_ = e.Store.SetDetectionStatus(ctx, d.ID, "au_digest")
	}
	e.Store.Audit(ctx, "agent", "digest_genere", map[string]any{
		"type": dtype, "detections": len(dc.Detections), "brouillons": len(dc.Brouillons)})

	// envoi du digest par email au dirigeant si activé (Réglages)
	if e.Store.GetSetting(ctx, "digest_email", "0") == "1" {
		if to, _ := e.Channel.AccountEmail(ctx); to != "" {
			subject := "Votre digest Ibalya — " + time.Now().Format("02/01/2006")
			// Le digest vient d'Ibalya, pas du dirigeant : il part par
			// l'expéditeur de service quand il est configuré. Passer par sa
			// boîte le fait apparaître dans ses messages envoyés, et Gmail y
			// remplace en silence toute adresse qui n'est pas un alias vérifié.
			var err error
			if e.Courrier.Configure() {
				err = e.Courrier.Envoyer(ctx, to, subject, e.renderDigestText(dc))
			} else {
				exp := e.Store.GetSetting(ctx, "digest_expediteur", "")
				err = e.Channel.SendFrom(ctx, exp, "Digest Ibalya", to, subject, e.renderDigestText(dc))
			}
			if err != nil {
				e.Store.Audit(ctx, "agent", "digest_email_echec", map[string]string{"erreur": err.Error()})
			} else {
				e.Store.Audit(ctx, "agent", "digest_email_envoye", map[string]string{"to": to})
			}
		}
	}
	return dc, nil
}

// renderDigestText met en forme le digest pour l'email (texte simple, lisible partout).
func (e *Engine) renderDigestText(dc *DigestContent) string {
	var b strings.Builder
	b.WriteString("Bonjour,\n\nVoici votre point du jour.\n")
	if len(dc.Detections) > 0 {
		b.WriteString("\n— ALERTES —\n")
		for _, d := range dc.Detections {
			marque := ""
			if d.Critique {
				marque = " [CRITIQUE]"
			}
			fmt.Fprintf(&b, "•%s %s\n  %s\n", marque, d.Titre, d.Detail)
		}
	}
	if len(dc.Engagements) > 0 {
		b.WriteString("\n— ENGAGEMENTS À RISQUE —\n")
		for _, e := range dc.Engagements {
			ech := "sans échéance"
			if e.Echeance != nil {
				ech = "échéance " + e.Echeance.Format("02/01/2006")
			}
			fmt.Fprintf(&b, "• %s (%s, %s)\n", e.Objet, ech, e.Statut)
		}
	}
	if len(dc.Brouillons) > 0 {
		fmt.Fprintf(&b, "\n— SUGGESTIONS —\n%d message(s) pré-rédigé(s) vous attendent.\n", len(dc.Brouillons))
	}
	if len(dc.Detections) == 0 && len(dc.Engagements) == 0 {
		b.WriteString("\nRien à signaler au-dessus du seuil aujourd'hui.\n")
	}
	// Un digest sans lien de retour est un constat sans suite : c'est dans
	// l'application que le dirigeant valide, corrige et relance.
	if e.BaseURL != "" {
		fmt.Fprintf(&b, "\nOuvrir Ibalya : %s/app/\n", strings.TrimSuffix(e.BaseURL, "/"))
	}
	b.WriteString("\n— Ibalya\n")
	return b.String()
}

// maybeDraft pré-rédige un brouillon d'action pour une détection actionnable.
// Aucun envoi : le brouillon reste « propose » jusqu'à validation d'un clic (marche 3).
func (e *Engine) maybeDraft(ctx context.Context, d store.Detection) *store.Draft {
	switch d.Type {
	case "silence_anormal", "echeance_a_risque", "contradiction", "orphelin":
	default:
		return nil
	}
	// silence : ne relancer l'interlocuteur que si c'est bien lui qu'on attend
	// (dernier message sortant). Sinon c'est au dirigeant de répondre — un
	// « je suis sans nouvelles » serait une inversion factuelle.
	if d.Type == "silence_anormal" {
		var p struct {
			DernierSortant bool `json:"dernier_sortant"`
		}
		if json.Unmarshal(d.Payload, &p) != nil || !p.DernierSortant {
			return nil
		}
	}
	if has, _ := e.Store.HasActiveDraftForDetection(ctx, d.ID); has {
		return nil
	}
	toEmail, toName, engObjet := "", "", ""
	var engID *int64
	// contradiction : la recommandation croisée (CDC 9.6) relance l'interlocuteur
	// AMONT (celui qui bloque), pas le client aval
	if d.Type == "contradiction" {
		var payload struct {
			AmontID int64 `json:"amont_id"`
		}
		if json.Unmarshal(d.Payload, &payload) == nil && payload.AmontID != 0 {
			if amont, err := e.Store.GetEngagement(ctx, payload.AmontID); err == nil && amont != nil {
				toEmail = amont.EmetteurEmail
				engObjet = amont.Objet
				engID = &amont.ID
			}
		}
	}
	if toEmail == "" && d.EngagementID != nil {
		if eng, err := e.Store.GetEngagement(ctx, *d.EngagementID); err == nil && eng != nil {
			engObjet = eng.Objet
			engID = &eng.ID
			toEmail = eng.EmetteurEmail
		}
	}
	if toEmail == "" && d.ThreadID != nil {
		_ = e.Store.Pool.QueryRow(ctx,
			`SELECT sender FROM messages WHERE thread_id=$1 AND NOT outbound ORDER BY sent_at DESC LIMIT 1`,
			*d.ThreadID).Scan(&toEmail)
	}
	if engObjet == "" && d.ThreadID != nil {
		if t, err := e.Store.GetThread(ctx, *d.ThreadID); err == nil && t != nil {
			engObjet = strings.TrimPrefix(t.Subject, "RE: ")
		}
	}
	accountEmail, _ := e.Channel.AccountEmail(ctx)
	if toEmail == "" || strings.EqualFold(toEmail, accountEmail) {
		return nil
	}
	facts := e.capsuleLLM(ctx)
	extraits := e.threadExtraits(ctx, d.ThreadID)
	resp, err := e.LLM.Draft(ctx, llm.DraftRequest{
		DetectionType: d.Type, DetectionTitre: d.Titre, DetectionDetail: d.Detail,
		EngagementObjet: engObjet, ToEmail: toEmail, ToName: toName,
		FromEmail: accountEmail, Capsule: facts, ThreadExtraits: extraits,
	})
	if err != nil {
		return nil
	}
	draft := store.Draft{DetectionID: &d.ID, EngagementID: engID, ToEmail: toEmail,
		Subject: resp.Subject, Body: e.apposerSignature(ctx, resp.Body), Statut: "propose"}
	id, err := e.Store.CreateDraft(ctx, draft)
	if err != nil {
		return nil
	}
	draft.ID = id
	e.Store.Audit(ctx, "agent", "brouillon_propose", map[string]any{"draft_id": id, "detection_id": d.ID, "to": toEmail})
	return &draft
}

// contexteClient rassemble ce que l'agent sait de l'interlocuteur au-delà du
// fil courant : les autres engagements ouverts avec lui et l'ancienneté de la
// relation. Sans cela, un client suivi sur trois dossiers n'est vu qu'à travers
// un seul.
func (e *Engine) contexteClient(ctx context.Context, email string, excludeID int64) []string {
	out := []string{}
	if email == "" {
		return out
	}
	if last, n, err := e.Store.LastExchangeWithContact(ctx, email); err == nil && n > 0 {
		ligne := fmt.Sprintf("%d message(s) échangé(s) avec %s", n, email)
		if last != nil {
			ligne += fmt.Sprintf(", dernier le %s", last.Format("02/01/2006"))
		}
		out = append(out, ligne)
	}
	engs, err := e.Store.OpenEngagementsByContact(ctx, email, excludeID)
	if err != nil || len(engs) == 0 {
		return out
	}
	out = append(out, fmt.Sprintf("Autres engagements en cours avec cet interlocuteur (%d) :", len(engs)))
	for _, x := range engs {
		ech := "sans échéance"
		if x.Echeance != nil {
			ech = "échéance " + x.Echeance.Format("02/01/2006")
		}
		sens := "il s'est engagé"
		if accountEmail, _ := e.Channel.AccountEmail(ctx); strings.EqualFold(x.EmetteurEmail, accountEmail) {
			sens = "vous vous êtes engagé"
		}
		out = append(out, fmt.Sprintf("  · %s (%s, %s, %s)", x.Objet, x.Type, ech, sens))
	}
	return out
}

// threadExtraits renvoie les 3 derniers messages d'un fil, tronqués : ils
// ancrent le brouillon dans la conversation réelle.
func (e *Engine) threadExtraits(ctx context.Context, threadID *int64) []string {
	extraits := []string{}
	if threadID == nil {
		return extraits
	}
	msgs, err := e.Store.ThreadMessages(ctx, *threadID)
	if err != nil {
		return extraits
	}
	if len(msgs) > 3 {
		msgs = msgs[len(msgs)-3:]
	}
	for _, m := range msgs {
		body := m.Body
		if len(body) > 300 {
			body = body[:300] + "…"
		}
		extraits = append(extraits, fmt.Sprintf("[%s] %s : %s", m.SentAt.Format("02/01"), m.Sender, body))
	}
	return extraits
}

// draftFor rédige un message pour un engagement selon l'action choisie.
// Réutilise le brouillon déjà en attente s'il y en a un (clics répétés).
func (e *Engine) draftFor(ctx context.Context, s EngagementSuivi, action ActionSuggeree) (*store.Draft, error) {
	if existing, _ := e.Store.FindProposedDraftByEngagement(ctx, s.ID); existing != nil {
		// Un brouillon produit avant la mise en place de la signature, ou avant
		// que le dirigeant ne la modifie, doit la porter lui aussi : sinon il
		// suffit d'un brouillon déjà en attente pour que le changement reste
		// invisible.
		if corrige := e.apposerSignature(ctx, existing.Body); corrige != existing.Body {
			if err := e.Store.SetDraftBody(ctx, existing.ID, corrige); err == nil {
				existing.Body = corrige
			}
		}
		return existing, nil
	}
	accountEmail, _ := e.Channel.AccountEmail(ctx)
	if strings.EqualFold(action.ToEmail, accountEmail) {
		return nil, fmt.Errorf("le destinataire de l'action est votre propre adresse")
	}
	facts := e.capsuleLLM(ctx)
	contexte := contexteLigne(s)
	if s.Blocage != nil {
		contexte += fmt.Sprintf(" — cause amont : « %s » (%s)", s.Blocage.AmontObjet, s.Blocage.AmontEmetteur)
	}
	resp, err := e.LLM.Draft(ctx, llm.DraftRequest{
		DetectionType: s.Categorie, DetectionTitre: action.Label, DetectionDetail: contexte,
		EngagementObjet: s.Objet, ToEmail: action.ToEmail, FromEmail: accountEmail,
		Capsule: facts, ThreadExtraits: e.threadExtraits(ctx, s.ThreadID),
		Intent: action.Intent, IntentLabel: action.Label,
		ContexteClient: e.contexteClient(ctx, action.ToEmail, s.ID),
	})
	if err != nil {
		return nil, err
	}
	engID := s.ID
	draft := store.Draft{EngagementID: &engID, ToEmail: action.ToEmail,
		Subject: resp.Subject, Body: e.apposerSignature(ctx, resp.Body), Statut: "propose"}
	id, err := e.Store.CreateDraft(ctx, draft)
	if err != nil {
		return nil, err
	}
	draft.ID = id
	draft.EngagementObjet = s.Objet
	draft.DetectionTitre = action.Label
	e.Store.Audit(ctx, "agent", "brouillon_propose", map[string]any{
		"draft_id": id, "engagement_id": s.ID, "intent": action.Intent, "to": action.ToEmail})
	return &draft, nil
}

// ReviewDraft fait relire par l'agent la version modifiée par le dirigeant.
// Aucune écriture en base : c'est un avis, le dirigeant reste décideur.
func (e *Engine) ReviewDraft(ctx context.Context, draftID int64, subject, body string) (*llm.ReviewResponse, error) {
	d, err := e.Store.GetDraft(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("le message est vide")
	}
	if subject == "" {
		subject = d.Subject
	}
	facts := e.capsuleLLM(ctx)
	req := llm.ReviewRequest{
		ToEmail: d.ToEmail, Subject: subject, Body: body,
		Capsule: facts, ContexteClient: e.contexteClient(ctx, d.ToEmail, 0),
	}
	// contexte de l'engagement rattaché, s'il existe
	if d.EngagementID != nil {
		if eng, err := e.Store.GetEngagement(ctx, *d.EngagementID); err == nil && eng != nil {
			req.EngagementObjet = eng.Objet
			req.ThreadExtraits = e.threadExtraits(ctx, eng.ThreadID)
			if eng.Echeance != nil {
				req.Contexte = "échéance " + eng.Echeance.Format("02/01/2006") + ", statut " + eng.Statut
			}
		}
	}
	if d.DetectionID != nil {
		if det, err := e.Store.GetDetection(ctx, *d.DetectionID); err == nil && det != nil {
			req.IntentLabel = det.Titre
			if req.Contexte == "" {
				req.Contexte = det.Detail
			}
		}
	}
	resp, err := e.LLM.Review(ctx, req)
	if err != nil {
		return nil, err
	}
	e.Store.Audit(ctx, "dirigeant", "brouillon_relu", map[string]any{
		"draft_id": draftID, "verdict": resp.Verdict, "remarques": len(resp.Remarques)})
	return resp, nil
}

// SendDraft envoie un brouillon validé par le dirigeant (marche 3) et trace tout.
// La réclamation du brouillon est atomique : deux validations simultanées
// (double-clic) ne peuvent pas envoyer deux fois.
func (e *Engine) SendDraft(ctx context.Context, draftID int64) error {
	var d store.Draft
	err := e.Store.Pool.QueryRow(ctx, `UPDATE drafts SET statut='envoye', sent_at=now()
		WHERE id=$1 AND statut IN ('propose','valide')
		RETURNING id, detection_id, engagement_id, to_email, subject, body`, draftID).
		Scan(&d.ID, &d.DetectionID, &d.EngagementID, &d.ToEmail, &d.Subject, &d.Body)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("brouillon %d déjà traité ou introuvable", draftID)
	}
	if err != nil {
		return err
	}
	if err := e.Channel.Send(ctx, d.ToEmail, d.Subject, d.Body); err != nil {
		// échec d'envoi : on rend le brouillon validable à nouveau
		_ = e.Store.SetDraftStatus(ctx, draftID, "propose", false)
		return fmt.Errorf("envoi: %w", err)
	}
	if d.EngagementID != nil {
		_ = e.Store.AddEvent(ctx, *d.EngagementID, "relance", nil, map[string]any{"draft_id": draftID})
	}
	if d.DetectionID != nil {
		_ = e.Store.SetDetectionStatus(ctx, *d.DetectionID, "traitee")
	}
	e.Store.Audit(ctx, "dirigeant", "action_validee_envoyee", map[string]any{
		"draft_id": draftID, "to": d.ToEmail, "subject": d.Subject})
	return nil
}

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"agentops/backend/internal/llm"
	"agentops/backend/internal/store"
)

// --- Miroir d'activité (EF-2, livrable J+1) ---

type Miroir struct {
	GenereLe          time.Time          `json:"genere_le"`
	PeriodeJours      int                `json:"periode_jours"`
	MessagesLus       int                `json:"messages_lus"`
	EngagementsOuverts []store.Engagement `json:"engagements_ouverts"`
	EnRetardProbable  []store.Engagement `json:"en_retard_probable"`
	FilsSansReponse   []FilSansReponse   `json:"fils_sans_reponse"`
	Note              string             `json:"note"`
}

type FilSansReponse struct {
	ThreadID     int64  `json:"thread_id"`
	Sujet        string `json:"sujet"`
	Interlocuteur string `json:"interlocuteur"`
	JoursSilence int    `json:"jours_silence"`
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
			subject := "Votre digest AgentOps — " + time.Now().Format("02/01/2006")
			if err := e.Channel.Send(ctx, to, subject, renderDigestText(dc)); err != nil {
				e.Store.Audit(ctx, "agent", "digest_email_echec", map[string]string{"erreur": err.Error()})
			} else {
				e.Store.Audit(ctx, "agent", "digest_email_envoye", map[string]string{"to": to})
			}
		}
	}
	return dc, nil
}

// renderDigestText met en forme le digest pour l'email (texte simple, lisible partout).
func renderDigestText(dc *DigestContent) string {
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
		fmt.Fprintf(&b, "\n— SUGGESTIONS —\n%d message(s) pré-rédigé(s) vous attendent : ouvrez le tableau de bord pour valider d'un clic.\n", len(dc.Brouillons))
	}
	if len(dc.Detections) == 0 && len(dc.Engagements) == 0 {
		b.WriteString("\nRien à signaler au-dessus du seuil aujourd'hui.\n")
	}
	b.WriteString("\n— AgentOps\n")
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
		if t, err := e.Store.GetThread(ctx, *d.ThreadID); err == nil {
			engObjet = strings.TrimPrefix(t.Subject, "RE: ")
		}
	}
	accountEmail, _ := e.Channel.AccountEmail(ctx)
	if toEmail == "" || strings.EqualFold(toEmail, accountEmail) {
		return nil
	}
	capsule, _ := e.Store.GetCapsule(ctx)
	var facts json.RawMessage
	if capsule != nil {
		facts = capsule.Facts
	}
	resp, err := e.LLM.Draft(ctx, llm.DraftRequest{
		DetectionType: d.Type, DetectionTitre: d.Titre, DetectionDetail: d.Detail,
		EngagementObjet: engObjet, ToEmail: toEmail, ToName: toName,
		FromEmail: accountEmail, Capsule: facts,
	})
	if err != nil {
		return nil
	}
	draft := store.Draft{DetectionID: &d.ID, EngagementID: engID, ToEmail: toEmail,
		Subject: resp.Subject, Body: resp.Body, Statut: "propose"}
	id, err := e.Store.CreateDraft(ctx, draft)
	if err != nil {
		return nil
	}
	draft.ID = id
	e.Store.Audit(ctx, "agent", "brouillon_propose", map[string]any{"draft_id": id, "detection_id": d.ID, "to": toEmail})
	return &draft
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

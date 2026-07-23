package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agentops/backend/internal/store"
)

// Les 5 détecteurs permanents (CDC section 8.2). Tous s'exécutent côté
// backend sur les données structurées, à chaque cycle. Chaque détection
// porte un score ; seules celles au-dessus du seuil entrent au digest.

type DetectStats struct {
	EnRetard   int            `json:"passes_en_retard"`
	Detections map[string]int `json:"detections"`
}

func (e *Engine) RunDetectors(ctx context.Context) (DetectStats, error) {
	st := DetectStats{Detections: map[string]int{}}

	// transition automatique vers en_retard (journal d'événements CDC 5.2)
	n, err := e.markOverdue(ctx)
	if err != nil {
		return st, err
	}
	st.EnRetard = n

	capsule := e.capsuleParams(ctx)
	today := time.Now().Format("2006-01-02")

	for name, fn := range map[string]func(context.Context, capsuleParams, string) (int, error){
		"echeance_a_risque": e.detectEcheanceARisque,
		"silence_anormal":   e.detectSilenceAnormal,
		"contradiction":     e.detectContradiction,
		"orphelin":          e.detectOrphelin,
		"surcharge":         e.detectSurcharge,
	} {
		count, err := fn(ctx, capsule, today)
		if err != nil {
			return st, fmt.Errorf("détecteur %s: %w", name, err)
		}
		st.Detections[name] = count
	}
	e.Store.Audit(ctx, "agent", "cycle_detection", st)
	return st, nil
}

type capsuleParams struct {
	HorizonJours        int     // X du détecteur 1
	SilenceDefautHeures float64 // cold-start du détecteur 2
	OrphelinJours       int     // délai du détecteur 4
	SurchargeSeuil      int     // seuil du détecteur 5
}

// capsuleParams lit les paramètres calibrés depuis la capsule (facts),
// avec des valeurs par défaut raisonnables en cold-start.
func (e *Engine) capsuleParams(ctx context.Context) capsuleParams {
	p := capsuleParams{HorizonJours: 7, SilenceDefautHeures: 96, OrphelinJours: 5, SurchargeSeuil: 5}
	c, err := e.Store.GetCapsule(ctx)
	if err != nil {
		return p
	}
	var facts map[string]any
	if json.Unmarshal(c.Facts, &facts) == nil {
		if v, ok := facts["horizon_jours"].(float64); ok && v > 0 {
			p.HorizonJours = int(v)
		}
		if v, ok := facts["silence_defaut_heures"].(float64); ok && v > 0 {
			p.SilenceDefautHeures = v
		}
	}
	return p
}

func (e *Engine) markOverdue(ctx context.Context) (int, error) {
	rows, err := e.Store.Pool.Query(ctx, `
		UPDATE engagements SET statut='en_retard', maj_le=now()
		WHERE statut IN ('ouvert','confirme')
		  AND echeance IS NOT NULL AND echeance_confirmee AND echeance < CURRENT_DATE
		RETURNING id`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		_ = e.Store.AddEvent(ctx, id, "passe_en_retard", nil, nil)
	}
	return len(ids), nil
}

// Détecteur 1 — Échéance à risque : échéance < X jours ET aucun signal de
// progression dans la fenêtre récente. Les échéances inférées non confirmées
// n'alimentent jamais ce détecteur (CDC 7.3).
func (e *Engine) detectEcheanceARisque(ctx context.Context, p capsuleParams, today string) (int, error) {
	engs, err := e.Store.ListEngagements(ctx, []string{"ouvert", "confirme"})
	if err != nil {
		return 0, err
	}
	count := 0
	now := time.Now()
	for _, eng := range engs {
		if eng.Echeance == nil || !eng.EcheanceConfirmee {
			continue
		}
		days := int(eng.Echeance.Sub(now).Hours() / 24)
		if days < 0 || days > p.HorizonJours {
			continue
		}
		last, err := e.Store.LastEventOfTypes(ctx, eng.ID, []string{"signal_progression", "livre", "confirme"})
		if err != nil {
			continue
		}
		fenetre := now.AddDate(0, 0, -p.HorizonJours/2-1)
		if last != nil && last.After(fenetre) {
			continue // signal récent → pas de risque
		}
		score := 0.7
		critique := false
		if days <= 2 {
			score = 0.9
			critique = true
		}
		if eng.Priorite == "haute" {
			score += 0.05
		}
		d := store.Detection{
			Type: "echeance_a_risque", EngagementID: &eng.ID, ThreadID: eng.ThreadID,
			Score: clamp01(score), Critique: critique,
			Titre:  fmt.Sprintf("Échéance dans %d jour(s) sans signal de progression", days),
			Detail: fmt.Sprintf("« %s » — échéance le %s, aucun signal récent.", eng.Objet, eng.Echeance.Format("02/01/2006")),
		}
		key := fmt.Sprintf("echeance_a_risque:%d:%s", eng.ID, today)
		if id, _ := e.Store.CreateDetection(ctx, d, key); id != 0 {
			count++
		}
	}
	return count, nil
}

// Détecteur 2 — Silence anormal : fil avec engagement ouvert sans réponse
// depuis plus longtemps que son rythme appris (seuil PAR FIL, anti-faux-positif).
func (e *Engine) detectSilenceAnormal(ctx context.Context, p capsuleParams, today string) (int, error) {
	rows, err := e.Store.Pool.Query(ctx, `
		SELECT DISTINCT t.id, t.subject, t.last_message_at, t.response_rhythm_hours,
		       (SELECT m.sender FROM messages m WHERE m.thread_id=t.id AND NOT m.outbound ORDER BY m.sent_at DESC LIMIT 1)
		FROM threads t
		JOIN engagements e ON e.thread_id = t.id AND e.statut IN ('ouvert','confirme','en_retard')
		WHERE NOT t.excluded AND t.last_message_at IS NOT NULL`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type row struct {
		id         int64
		subject    string
		lastMsg    time.Time
		rhythm     *float64
		lastSender *string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.subject, &r.lastMsg, &r.rhythm, &r.lastSender); err != nil {
			return 0, err
		}
		list = append(list, r)
	}
	rows.Close()

	rules, _ := e.Store.ListRules(ctx, true)
	count := 0
	now := time.Now()
	for _, r := range list {
		seuil := p.SilenceDefautHeures // cold-start : défaut dérivé de la capsule
		if r.rhythm != nil && *r.rhythm > 0 {
			seuil = *r.rhythm * 2 // marge : 2× le rythme habituel du fil
			if seuil < 24 {
				seuil = 24
			}
		}
		silence := now.Sub(r.lastMsg).Hours()
		if silence <= seuil {
			continue
		}
		if e.threadMuted(r.id, rules) {
			continue
		}
		interlocuteur := ""
		if r.lastSender != nil {
			interlocuteur = *r.lastSender
		}
		jours := int(silence / 24)
		d := store.Detection{
			Type: "silence_anormal", ThreadID: &r.id,
			Score: 0.65,
			Titre: fmt.Sprintf("Silence anormal depuis %d jour(s)", jours),
			Detail: fmt.Sprintf("Fil « %s » (%s) : sans réponse depuis %d jour(s), au-delà du rythme habituel.",
				r.subject, interlocuteur, jours),
			Payload: mustJSON(map[string]any{"interlocuteur": interlocuteur, "jours_silence": jours}),
		}
		key := fmt.Sprintf("silence_anormal:%d:%s", r.id, today)
		if id, _ := e.Store.CreateDetection(ctx, d, key); id != 0 {
			count++
		}
	}
	return count, nil
}

func (e *Engine) threadMuted(threadID int64, rules []store.LearnedRule) bool {
	cible := fmt.Sprintf("%d", threadID)
	for _, r := range rules {
		if r.PorteeType == "fil" && r.PorteeCible == cible &&
			(r.Action == "ne_plus_alerter" || r.Action == "ignorer") {
			return true
		}
	}
	return false
}

// Détecteur 3 — Contradiction : promesse aval intenable à cause d'un retard
// amont. Uniquement sur liens CONFIRMÉS (CDC 8.2 — jamais sur candidat).
func (e *Engine) detectContradiction(ctx context.Context, p capsuleParams, today string) (int, error) {
	links, err := e.Store.ListLinks(ctx, "confirme")
	if err != nil {
		return 0, err
	}
	count := 0
	now := time.Now()
	for _, l := range links {
		amont, err1 := e.Store.GetEngagement(ctx, l.AmontID)
		aval, err2 := e.Store.GetEngagement(ctx, l.AvalID)
		if err1 != nil || err2 != nil || amont == nil || aval == nil {
			continue
		}
		if aval.Statut != "ouvert" && aval.Statut != "confirme" {
			continue
		}
		amontBloque := amont.Statut == "en_retard"
		if !amontBloque && amont.ThreadID != nil {
			amontBloque, _ = e.Store.HasSilenceDetection(ctx, *amont.ThreadID)
		}
		if !amontBloque || aval.Echeance == nil {
			continue
		}
		days := int(aval.Echeance.Sub(now).Hours() / 24)
		if days > p.HorizonJours {
			continue
		}
		d := store.Detection{
			Type: "contradiction", EngagementID: &aval.ID, ThreadID: aval.ThreadID,
			Score: 0.9, Critique: true,
			Titre: "Contradiction : promesse aval menacée par un blocage amont",
			Detail: fmt.Sprintf("« %s » (échéance %s) dépend de « %s » qui est %s.",
				aval.Objet, aval.Echeance.Format("02/01/2006"), amont.Objet, statutLabel(amont.Statut)),
			Payload: mustJSON(map[string]any{"amont_id": amont.ID, "aval_id": aval.ID}),
		}
		key := fmt.Sprintf("contradiction:%d:%d:%s", amont.ID, aval.ID, today)
		if id, _ := e.Store.CreateDetection(ctx, d, key); id != 0 {
			count++
		}
	}
	return count, nil
}

// Détecteur 4 — Engagement orphelin : aucun événement après la création
// au-delà d'un délai (le « trou noir » des PME).
func (e *Engine) detectOrphelin(ctx context.Context, p capsuleParams, today string) (int, error) {
	engs, err := e.Store.ListEngagements(ctx, []string{"ouvert"})
	if err != nil {
		return 0, err
	}
	count := 0
	limite := time.Now().AddDate(0, 0, -p.OrphelinJours)
	for _, eng := range engs {
		if eng.CreeLe.After(limite) {
			continue
		}
		n, err := e.Store.CountEventsAfterCreation(ctx, eng.ID)
		if err != nil || n > 0 {
			continue
		}
		jours := int(time.Since(eng.CreeLe).Hours() / 24)
		d := store.Detection{
			Type: "orphelin", EngagementID: &eng.ID, ThreadID: eng.ThreadID,
			Score: 0.6,
			Titre: fmt.Sprintf("Engagement sans suite depuis %d jours", jours),
			Detail: fmt.Sprintf("« %s » : aucune confirmation, livraison ni relance depuis sa détection.",
				eng.Objet),
		}
		key := fmt.Sprintf("orphelin:%d", eng.ID) // une seule fois par engagement
		if id, _ := e.Store.CreateDetection(ctx, d, key); id != 0 {
			count++
		}
	}
	return count, nil
}

// Détecteur 5 — Surcharge ponctuelle : concentration d'échéances sur un même
// responsable / une même semaine.
func (e *Engine) detectSurcharge(ctx context.Context, p capsuleParams, today string) (int, error) {
	rows, err := e.Store.Pool.Query(ctx, `
		SELECT coalesce(pe.email,'(inconnu)'), date_trunc('week', e.echeance)::date, count(*)
		FROM engagements e
		LEFT JOIN persons pe ON pe.id = e.emetteur_id
		WHERE e.statut IN ('ouvert','confirme','en_retard') AND e.echeance IS NOT NULL
		  AND e.echeance >= CURRENT_DATE AND e.echeance < CURRENT_DATE + 28
		GROUP BY 1, 2 HAVING count(*) >= $1`, p.SurchargeSeuil)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	type surcharge struct {
		email string
		week  time.Time
		n     int
	}
	var list []surcharge
	for rows.Next() {
		var s surcharge
		if err := rows.Scan(&s.email, &s.week, &s.n); err != nil {
			return 0, err
		}
		list = append(list, s)
	}
	rows.Close()
	for _, s := range list {
		d := store.Detection{
			Type:  "surcharge",
			Score: 0.6,
			Titre: fmt.Sprintf("%d échéances la semaine du %s pour %s", s.n, s.week.Format("02/01"), s.email),
			Detail: fmt.Sprintf("Concentration inhabituelle de %d engagements à échéance sur la même semaine — lissage de charge recommandé.",
				s.n),
			Payload: mustJSON(map[string]any{"responsable": s.email, "semaine": s.week.Format("2006-01-02"), "nombre": s.n}),
		}
		key := fmt.Sprintf("surcharge:%s:%s", s.email, s.week.Format("2006-01-02"))
		if id, _ := e.Store.CreateDetection(ctx, d, key); id != 0 {
			count++
		}
	}
	return count, nil
}

func statutLabel(s string) string {
	switch s {
	case "en_retard":
		return "en retard"
	case "ouvert":
		return "ouvert"
	default:
		return s
	}
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

package store

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Personnes ---

func (s *Store) UpsertPerson(ctx context.Context, email, name string) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO persons (email, name) VALUES ($1,$2)
		ON CONFLICT (email) DO UPDATE SET name = CASE WHEN persons.name = '' THEN EXCLUDED.name ELSE persons.name END
		RETURNING id`, email, name).Scan(&id)
	return id, err
}

func (s *Store) ListPersons(ctx context.Context) ([]Person, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, name, email, type, sensitive, created_at FROM persons ORDER BY email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name, &p.Email, &p.Type, &p.Sensitive, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpdatePerson(ctx context.Context, id int64, ptype string, sensitive bool) error {
	_, err := s.Pool.Exec(ctx, `UPDATE persons SET type=$2, sensitive=$3 WHERE id=$1`, id, ptype, sensitive)
	return err
}

// --- Threads & messages ---

func (s *Store) UpsertThread(ctx context.Context, channel, externalID, subject string, lastMsgAt time.Time) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO threads (channel, external_id, subject, last_message_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (channel, external_id) DO UPDATE SET
		  subject = CASE WHEN threads.subject = '' THEN EXCLUDED.subject ELSE threads.subject END,
		  last_message_at = GREATEST(coalesce(threads.last_message_at, EXCLUDED.last_message_at), EXCLUDED.last_message_at)
		RETURNING id`, channel, externalID, subject, lastMsgAt).Scan(&id)
	return id, err
}

func (s *Store) GetThread(ctx context.Context, id int64) (*Thread, error) {
	var t Thread
	err := s.Pool.QueryRow(ctx, `SELECT id, channel, external_id, subject, last_message_at, response_rhythm_hours, excluded
		FROM threads WHERE id=$1`, id).
		Scan(&t.ID, &t.Channel, &t.ExternalID, &t.Subject, &t.LastMessageAt, &t.ResponseRhythmHours, &t.Excluded)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) SetThreadExcluded(ctx context.Context, id int64, excluded bool) error {
	_, err := s.Pool.Exec(ctx, `UPDATE threads SET excluded=$2 WHERE id=$1`, id, excluded)
	return err
}

func (s *Store) SetThreadRhythm(ctx context.Context, id int64, hours float64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE threads SET response_rhythm_hours=$2 WHERE id=$1`, id, hours)
	return err
}

// InsertMessage retourne (id, inséré). inséré=false si déjà connu (dédoublonnage EF-11).
func (s *Store) InsertMessage(ctx context.Context, m Message) (int64, bool, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO messages
		(thread_id, external_id, channel, sender, recipients, sent_at, subject, body, outbound, list_unsubscribe, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending')
		ON CONFLICT (channel, external_id) DO NOTHING RETURNING id`,
		m.ThreadID, m.ExternalID, m.Channel, m.Sender, m.Recipients, m.SentAt, m.Subject, m.Body, m.Outbound, m.ListUnsubscribe).
		Scan(&id)
	if err == pgx.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func (s *Store) MarkMessage(ctx context.Context, id int64, status string, reason *string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE messages SET status=$2, exclude_reason=$3 WHERE id=$1`, id, status, reason)
	return err
}

func (s *Store) ListPendingMessages(ctx context.Context, limit int) ([]Message, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, thread_id, external_id, channel, sender, recipients, sent_at, subject, body, outbound, list_unsubscribe, status
		FROM messages WHERE status='pending' ORDER BY sent_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.ExternalID, &m.Channel, &m.Sender, &m.Recipients, &m.SentAt, &m.Subject, &m.Body, &m.Outbound, &m.ListUnsubscribe, &m.Status); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetMessage(ctx context.Context, id int64) (*Message, error) {
	var m Message
	err := s.Pool.QueryRow(ctx, `SELECT id, thread_id, external_id, channel, sender, recipients, sent_at, subject, body, outbound, list_unsubscribe, status
		FROM messages WHERE id=$1`, id).
		Scan(&m.ID, &m.ThreadID, &m.ExternalID, &m.Channel, &m.Sender, &m.Recipients, &m.SentAt, &m.Subject, &m.Body, &m.Outbound, &m.ListUnsubscribe, &m.Status)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ThreadMessages liste les messages d'un fil, du plus ancien au plus récent.
func (s *Store) ThreadMessages(ctx context.Context, threadID int64) ([]Message, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, thread_id, external_id, channel, sender, recipients, sent_at, subject, body, outbound, list_unsubscribe, status
		FROM messages WHERE thread_id=$1 ORDER BY sent_at ASC`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.ExternalID, &m.Channel, &m.Sender, &m.Recipients, &m.SentAt, &m.Subject, &m.Body, &m.Outbound, &m.ListUnsubscribe, &m.Status); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// --- Engagements ---

func (s *Store) CreateEngagement(ctx context.Context, e Engagement) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO engagements
		(emetteur_id, destinataire_id, objet, echeance, echeance_inferee, echeance_confirmee, statut, confiance, priorite, source_message_id, thread_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		e.EmetteurID, e.DestinataireID, e.Objet, e.Echeance, e.EcheanceInferee, e.EcheanceConfirmee,
		e.Statut, e.Confiance, e.Priorite, e.SourceMessageID, e.ThreadID).Scan(&id)
	return id, err
}

const engagementSelect = `SELECT e.id, e.emetteur_id, e.destinataire_id,
	coalesce(pe.email,''), coalesce(pd.email,''),
	e.objet, e.echeance, e.echeance_inferee, e.echeance_confirmee, e.statut, e.confiance, e.priorite,
	e.source_message_id, e.thread_id, e.cree_le, e.maj_le
	FROM engagements e
	LEFT JOIN persons pe ON pe.id = e.emetteur_id
	LEFT JOIN persons pd ON pd.id = e.destinataire_id `

func scanEngagements(rows pgx.Rows) ([]Engagement, error) {
	defer rows.Close()
	var out []Engagement
	for rows.Next() {
		var e Engagement
		if err := rows.Scan(&e.ID, &e.EmetteurID, &e.DestinataireID, &e.EmetteurEmail, &e.DestinataireEmail,
			&e.Objet, &e.Echeance, &e.EcheanceInferee, &e.EcheanceConfirmee, &e.Statut, &e.Confiance,
			&e.Priorite, &e.SourceMessageID, &e.ThreadID, &e.CreeLe, &e.MajLe); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ListEngagements(ctx context.Context, statuts []string) ([]Engagement, error) {
	q := engagementSelect
	var rows pgx.Rows
	var err error
	if len(statuts) > 0 {
		rows, err = s.Pool.Query(ctx, q+`WHERE e.statut = ANY($1) ORDER BY e.echeance NULLS LAST, e.id DESC`, statuts)
	} else {
		rows, err = s.Pool.Query(ctx, q+`ORDER BY e.echeance NULLS LAST, e.id DESC`)
	}
	if err != nil {
		return nil, err
	}
	return scanEngagements(rows)
}

func (s *Store) GetEngagement(ctx context.Context, id int64) (*Engagement, error) {
	rows, err := s.Pool.Query(ctx, engagementSelect+`WHERE e.id=$1`, id)
	if err != nil {
		return nil, err
	}
	list, err := scanEngagements(rows)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

func (s *Store) OpenEngagementsByThread(ctx context.Context, threadID int64) ([]Engagement, error) {
	rows, err := s.Pool.Query(ctx, engagementSelect+
		`WHERE e.thread_id=$1 AND e.statut IN ('ouvert','confirme','en_retard') ORDER BY e.id`, threadID)
	if err != nil {
		return nil, err
	}
	return scanEngagements(rows)
}

func (s *Store) UpdateEngagement(ctx context.Context, id int64, fields map[string]any) error {
	q := `UPDATE engagements SET maj_le=now()`
	args := []any{id}
	i := 2
	for k, v := range fields {
		switch k {
		case "statut", "priorite", "objet", "echeance", "echeance_inferee", "echeance_confirmee", "confiance":
			q += `, ` + k + ` = $` + itoa(i)
			args = append(args, v)
			i++
		}
	}
	q += ` WHERE id=$1`
	_, err := s.Pool.Exec(ctx, q, args...)
	return err
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func (s *Store) AddEvent(ctx context.Context, engagementID int64, evType string, sourceMessageID *int64, details any) error {
	b, _ := json.Marshal(details)
	if b == nil || string(b) == "null" {
		b = []byte(`{}`)
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO engagement_events (engagement_id, type, source_message_id, details)
		VALUES ($1,$2,$3,$4)`, engagementID, evType, sourceMessageID, b)
	return err
}

func (s *Store) ListEvents(ctx context.Context, engagementID int64) ([]EngagementEvent, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id, engagement_id, type, horodatage, source_message_id, details
		FROM engagement_events WHERE engagement_id=$1 ORDER BY horodatage`, engagementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EngagementEvent
	for rows.Next() {
		var e EngagementEvent
		if err := rows.Scan(&e.ID, &e.EngagementID, &e.Type, &e.Horodatage, &e.SourceMessageID, &e.Details); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// LastEventOfTypes retourne l'horodatage du dernier événement d'un des types donnés.
func (s *Store) LastEventOfTypes(ctx context.Context, engagementID int64, types []string) (*time.Time, error) {
	var t *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT max(horodatage) FROM engagement_events
		WHERE engagement_id=$1 AND type = ANY($2)`, engagementID, types).Scan(&t)
	return t, err
}

func (s *Store) CountEventsAfterCreation(ctx context.Context, engagementID int64) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM engagement_events
		WHERE engagement_id=$1 AND type <> 'cree'`, engagementID).Scan(&n)
	return n, err
}

// --- Liens de dépendance ---

func (s *Store) CreateLink(ctx context.Context, amont, aval int64, raison string) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO dependency_links (amont_id, aval_id, raison)
		VALUES ($1,$2,$3) ON CONFLICT (amont_id, aval_id) DO NOTHING`, amont, aval, raison)
	return err
}

func (s *Store) ListLinks(ctx context.Context, statut string) ([]DependencyLink, error) {
	q := `SELECT l.id, l.amont_id, l.aval_id, l.statut, l.raison, l.created_at, ea.objet, ev.objet
		FROM dependency_links l
		JOIN engagements ea ON ea.id = l.amont_id
		JOIN engagements ev ON ev.id = l.aval_id `
	var rows pgx.Rows
	var err error
	if statut != "" {
		rows, err = s.Pool.Query(ctx, q+`WHERE l.statut=$1 ORDER BY l.id DESC`, statut)
	} else {
		rows, err = s.Pool.Query(ctx, q+`ORDER BY l.id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DependencyLink
	for rows.Next() {
		var l DependencyLink
		if err := rows.Scan(&l.ID, &l.AmontID, &l.AvalID, &l.Statut, &l.Raison, &l.CreatedAt, &l.AmontObjet, &l.AvalObjet); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) SetLinkStatus(ctx context.Context, id int64, statut string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE dependency_links SET statut=$2 WHERE id=$1`, id, statut)
	return err
}

// --- Capsule ---

func (s *Store) GetCapsule(ctx context.Context) (*Capsule, error) {
	var c Capsule
	err := s.Pool.QueryRow(ctx, `SELECT facts, intentions, updated_at FROM capsule WHERE id=1`).
		Scan(&c.Facts, &c.Intentions, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) UpdateCapsule(ctx context.Context, facts, intentions json.RawMessage) error {
	if facts != nil && intentions != nil {
		_, err := s.Pool.Exec(ctx, `UPDATE capsule SET facts=$1, intentions=$2, updated_at=now() WHERE id=1`, facts, intentions)
		return err
	}
	if facts != nil {
		_, err := s.Pool.Exec(ctx, `UPDATE capsule SET facts=$1, updated_at=now() WHERE id=1`, facts)
		return err
	}
	if intentions != nil {
		_, err := s.Pool.Exec(ctx, `UPDATE capsule SET intentions=$1, updated_at=now() WHERE id=1`, intentions)
		return err
	}
	return nil
}

// --- Règles apprises ---

func (s *Store) CreateRule(ctx context.Context, r LearnedRule) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO learned_rules (portee_type, portee_cible, action, note)
		VALUES ($1,$2,$3,$4) RETURNING id`, r.PorteeType, r.PorteeCible, r.Action, r.Note).Scan(&id)
	return id, err
}

func (s *Store) ListRules(ctx context.Context, activeOnly bool) ([]LearnedRule, error) {
	q := `SELECT id, portee_type, portee_cible, action, note, active, created_at FROM learned_rules`
	if activeOnly {
		q += ` WHERE active`
	}
	q += ` ORDER BY id DESC`
	rows, err := s.Pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LearnedRule
	for rows.Next() {
		var r LearnedRule
		if err := rows.Scan(&r.ID, &r.PorteeType, &r.PorteeCible, &r.Action, &r.Note, &r.Active, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeactivateRule(ctx context.Context, id int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE learned_rules SET active=false WHERE id=$1`, id)
	return err
}

// --- Détections ---

// CreateDetection insère si la clé de dédoublonnage est nouvelle. Retourne l'id ou 0 si doublon.
func (s *Store) CreateDetection(ctx context.Context, d Detection, dedupKey string) (int64, error) {
	if d.Payload == nil {
		d.Payload = []byte(`{}`)
	}
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO detections
		(type, engagement_id, thread_id, score, titre, detail, critique, payload, dedup_key)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (dedup_key) DO NOTHING RETURNING id`,
		d.Type, d.EngagementID, d.ThreadID, d.Score, d.Titre, d.Detail, d.Critique, d.Payload, dedupKey).Scan(&id)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	return id, err
}

func (s *Store) ListDetections(ctx context.Context, statuts []string, minScore float64, limit int) ([]Detection, error) {
	q := `SELECT id, type, engagement_id, thread_id, score, titre, detail, critique, payload, statut, created_at
		FROM detections WHERE score >= $1`
	args := []any{minScore}
	if len(statuts) > 0 {
		q += ` AND statut = ANY($2)`
		args = append(args, statuts)
	}
	q += ` ORDER BY critique DESC, score DESC, id DESC LIMIT ` + itoaN(limit)
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Detection
	for rows.Next() {
		var d Detection
		if err := rows.Scan(&d.ID, &d.Type, &d.EngagementID, &d.ThreadID, &d.Score, &d.Titre, &d.Detail, &d.Critique, &d.Payload, &d.Statut, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDetection(ctx context.Context, id int64) (*Detection, error) {
	var d Detection
	err := s.Pool.QueryRow(ctx, `SELECT id, type, engagement_id, thread_id, score, titre, detail, critique, payload, statut, created_at
		FROM detections WHERE id=$1`, id).
		Scan(&d.ID, &d.Type, &d.EngagementID, &d.ThreadID, &d.Score, &d.Titre, &d.Detail, &d.Critique, &d.Payload, &d.Statut, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) SetDetectionStatus(ctx context.Context, id int64, statut string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE detections SET statut=$2 WHERE id=$1`, id, statut)
	return err
}

// HasSilenceDetection indique une détection de silence récente et active sur un fil.
func (s *Store) HasSilenceDetection(ctx context.Context, threadID int64) (bool, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM detections
		WHERE type='silence_anormal' AND thread_id=$1 AND statut IN ('nouvelle','au_digest')
		AND created_at > now() - interval '7 days'`, threadID).Scan(&n)
	return n > 0, err
}

// --- Brouillons ---

func (s *Store) CreateDraft(ctx context.Context, d Draft) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `INSERT INTO drafts (detection_id, engagement_id, to_email, subject, body)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`,
		d.DetectionID, d.EngagementID, d.ToEmail, d.Subject, d.Body).Scan(&id)
	return id, err
}

func (s *Store) ListDrafts(ctx context.Context, statut string) ([]Draft, error) {
	q := `SELECT id, detection_id, engagement_id, to_email, subject, body, statut, created_at, sent_at FROM drafts`
	var rows pgx.Rows
	var err error
	if statut != "" {
		rows, err = s.Pool.Query(ctx, q+` WHERE statut=$1 ORDER BY id DESC`, statut)
	} else {
		rows, err = s.Pool.Query(ctx, q+` ORDER BY id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Draft
	for rows.Next() {
		var d Draft
		if err := rows.Scan(&d.ID, &d.DetectionID, &d.EngagementID, &d.ToEmail, &d.Subject, &d.Body, &d.Statut, &d.CreatedAt, &d.SentAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDraft(ctx context.Context, id int64) (*Draft, error) {
	var d Draft
	err := s.Pool.QueryRow(ctx, `SELECT id, detection_id, engagement_id, to_email, subject, body, statut, created_at, sent_at
		FROM drafts WHERE id=$1`, id).
		Scan(&d.ID, &d.DetectionID, &d.EngagementID, &d.ToEmail, &d.Subject, &d.Body, &d.Statut, &d.CreatedAt, &d.SentAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) SetDraftStatus(ctx context.Context, id int64, statut string, sent bool) error {
	if sent {
		_, err := s.Pool.Exec(ctx, `UPDATE drafts SET statut=$2, sent_at=now() WHERE id=$1`, id, statut)
		return err
	}
	_, err := s.Pool.Exec(ctx, `UPDATE drafts SET statut=$2 WHERE id=$1`, id, statut)
	return err
}

func (s *Store) HasActiveDraftForDetection(ctx context.Context, detectionID int64) (bool, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM drafts WHERE detection_id=$1 AND statut IN ('propose','valide','envoye')`, detectionID).Scan(&n)
	return n > 0, err
}

// --- Rapports ---

func (s *Store) SaveReport(ctx context.Context, rtype string, content any) (int64, error) {
	b, err := json.Marshal(content)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.Pool.QueryRow(ctx, `INSERT INTO reports (type, content) VALUES ($1,$2) RETURNING id`, rtype, b).Scan(&id)
	return id, err
}

func (s *Store) LatestReport(ctx context.Context, rtype string) (*Report, error) {
	var r Report
	err := s.Pool.QueryRow(ctx, `SELECT id, type, content, created_at FROM reports WHERE type=$1 ORDER BY id DESC LIMIT 1`, rtype).
		Scan(&r.ID, &r.Type, &r.Content, &r.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func itoaN(n int) string {
	if n <= 0 {
		n = 100
	}
	return strconv.Itoa(n)
}

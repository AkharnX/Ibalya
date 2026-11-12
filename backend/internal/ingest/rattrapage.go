package ingest

import (
	"context"
	"fmt"

	"ibalya/backend/internal/store"
)

// Bilan d'un rattrapage du pré-filtre sur une boîte déjà lue.
type Bilan struct {
	Examines           int
	Exclus             int
	ParMotif           map[string]int
	EngagementsEcartes int
	Echantillon        []string // « expéditeur | objet | motif », dix au plus
}

// Reappliquer reclasse les messages déjà en base avec les règles actuelles du
// pré-filtre. Le filtre n'agit qu'à l'ingestion : une boîte lue avant une
// règle garde ses suivis de colis et ses notifications analysés, et les faux
// engagements qui en sont sortis. Cette fonction les rattrape, pour UN tenant
// (le contexte doit venir de Store.EnTenant).
//
// Sans `appliquer`, elle compte et prélève un échantillon, sans rien écrire.
// Avec, le message passe « excluded » avec son motif ; un engagement créé
// depuis ce message et jamais touché depuis (aucun événement après « cree »)
// passe « abandonne » avec un événement « exclu_automatique », ses alertes
// ouvertes sont écartées et ses brouillons en attente rejetés.
func Reappliquer(ctx context.Context, st *store.Store, appliquer bool) (Bilan, error) {
	b := Bilan{ParMotif: map[string]int{}}
	rules, err := st.ListRules(ctx, true)
	if err != nil {
		return b, err
	}
	rows, err := st.Q(ctx).Query(ctx, `SELECT m.id, m.thread_id, m.sender, m.subject, m.body,
			m.list_unsubscribe, m.auto_submitted, t.excluded
		FROM messages m JOIN threads t ON t.id = m.thread_id
		WHERE m.status IN ('pending','analyzed') AND NOT m.outbound
		ORDER BY m.id`)
	if err != nil {
		return b, err
	}
	type cible struct {
		id    int64
		motif string
	}
	var cibles []cible
	for rows.Next() {
		var m store.Message
		var filExclu bool
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.Sender, &m.Subject, &m.Body,
			&m.ListUnsubscribe, &m.AutoSubmitted, &filExclu); err != nil {
			rows.Close()
			return b, err
		}
		b.Examines++
		motif := ExclusionReason(m, filExclu, rules)
		if motif == "" {
			continue
		}
		b.Exclus++
		b.ParMotif[motif]++
		if len(b.Echantillon) < 10 {
			b.Echantillon = append(b.Echantillon, fmt.Sprintf("%s | %s | %s", m.Sender, tronquer(m.Subject, 60), motif))
		}
		cibles = append(cibles, cible{m.ID, motif})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return b, err
	}
	if !appliquer {
		return b, nil
	}
	for _, c := range cibles {
		motif := c.motif
		if err := st.MarkMessage(ctx, c.id, "excluded", &motif); err != nil {
			return b, err
		}
		n, err := ecarterEngagementsIssusDe(ctx, st, c.id, motif)
		if err != nil {
			return b, err
		}
		b.EngagementsEcartes += n
	}
	return b, nil
}

// ecarterEngagementsIssusDe écarte les engagements nés d'un message désormais
// exclu, à condition que rien ne se soit passé depuis : un engagement relancé,
// confirmé ou corrigé par le dirigeant a une vie propre, on n'y touche pas.
func ecarterEngagementsIssusDe(ctx context.Context, st *store.Store, messageID int64, motif string) (int, error) {
	rows, err := st.Q(ctx).Query(ctx, `SELECT e.id FROM engagements e
		JOIN engagement_events c ON c.engagement_id = e.id AND c.type = 'cree' AND c.source_message_id = $1
		WHERE e.statut IN ('ouvert','confirme','en_retard')
		  AND NOT EXISTS (SELECT 1 FROM engagement_events x
		                  WHERE x.engagement_id = e.id AND x.type <> 'cree')`, messageID)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := st.UpdateEngagement(ctx, id, map[string]any{"statut": "abandonne"}); err != nil {
			return 0, err
		}
		_ = st.AddEvent(ctx, id, "exclu_automatique", &messageID, map[string]string{"motif": motif})
		_, _ = st.Q(ctx).Exec(ctx, `UPDATE detections SET statut='ecartee'
			WHERE engagement_id=$1 AND statut IN ('nouvelle','au_digest')`, id)
		_, _ = st.Q(ctx).Exec(ctx, `UPDATE drafts SET statut='rejete'
			WHERE engagement_id=$1 AND statut='propose'`, id)
	}
	return len(ids), nil
}

func tronquer(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

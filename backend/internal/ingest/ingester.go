package ingest

import (
	"context"
	"log"
	"time"

	"ibalya/backend/internal/channel"
	"ibalya/backend/internal/store"
)

type Ingester struct {
	Store   *store.Store
	Channel channel.Reader
}

type Stats struct {
	Fetched  int `json:"fetched"`
	Inserted int `json:"inserted"`
	Excluded int `json:"excluded"`
	Kept     int `json:"kept"`
}

// Run récupère les messages depuis `since`, les normalise, applique le
// pré-filtre EF-11 et persiste. Retourne les statistiques du cycle
// (le taux d'exclusion est un indicateur de santé économique — CDC 6.4).
func (ing *Ingester) Run(ctx context.Context, since time.Time, max int) (Stats, error) {
	var st Stats
	msgs, err := ing.Channel.FetchSince(ctx, since, max)
	if err != nil {
		return st, err
	}
	st.Fetched = len(msgs)
	rules, err := ing.Store.ListRules(ctx, true)
	if err != nil {
		return st, err
	}

	for _, cm := range msgs {
		threadID, err := ing.Store.UpsertThread(ctx, ing.Channel.Name(), cm.ThreadExternalID, cm.Subject, cm.SentAt)
		if err != nil {
			log.Printf("ingest: upsert thread: %v", err)
			continue
		}
		thread, err := ing.Store.GetThread(ctx, threadID)
		if err != nil {
			continue
		}
		m := store.Message{
			ThreadID:        threadID,
			ExternalID:      cm.ExternalID,
			Channel:         ing.Channel.Name(),
			Sender:          cm.Sender,
			Recipients:      cm.Recipients,
			SentAt:          cm.SentAt,
			Subject:         cm.Subject,
			Body:            cm.Body,
			Outbound:        cm.Outbound,
			ListUnsubscribe: cm.ListUnsubscribe,
		}
		id, inserted, err := ing.Store.InsertMessage(ctx, m)
		if err != nil {
			log.Printf("ingest: insert message: %v", err)
			continue
		}
		if !inserted { // dédoublonnage : déjà connu
			continue
		}
		st.Inserted++
		if _, err := ing.Store.UpsertPerson(ctx, cm.Sender, cm.SenderName); err == nil {
			for _, r := range cm.Recipients {
				_, _ = ing.Store.UpsertPerson(ctx, r, "")
			}
		}
		if reason := ExclusionReason(m, thread.Excluded, rules); reason != "" {
			st.Excluded++
			_ = ing.Store.MarkMessage(ctx, id, "excluded", &reason)
		} else {
			st.Kept++
		}
	}

	// met à jour le rythme de réponse appris par fil (détecteur silence anormal)
	ing.updateRhythms(ctx)

	ing.Store.Audit(ctx, "agent", "ingestion_cycle", st)
	return st, nil
}

// updateRhythms calcule le rythme de réponse habituel par fil : moyenne des
// écarts entre messages d'expéditeurs différents (CDC 5.3 / détecteur 2).
func (ing *Ingester) updateRhythms(ctx context.Context) {
	rows, err := ing.Store.Pool.Query(ctx, `
		WITH gaps AS (
		  SELECT thread_id,
		         EXTRACT(EPOCH FROM sent_at - lag(sent_at) OVER (PARTITION BY thread_id ORDER BY sent_at))/3600.0 AS gap_h,
		         sender <> lag(sender) OVER (PARTITION BY thread_id ORDER BY sent_at) AS speaker_change
		  FROM messages
		)
		SELECT thread_id, avg(gap_h) FROM gaps
		WHERE speaker_change AND gap_h IS NOT NULL AND gap_h > 0
		GROUP BY thread_id HAVING count(*) >= 2`)
	if err != nil {
		return
	}
	defer rows.Close()
	type tr struct {
		id int64
		h  float64
	}
	var updates []tr
	for rows.Next() {
		var t tr
		if err := rows.Scan(&t.id, &t.h); err == nil {
			updates = append(updates, t)
		}
	}
	rows.Close()
	for _, u := range updates {
		_ = ing.Store.SetThreadRhythm(ctx, u.id, u.h)
	}
}

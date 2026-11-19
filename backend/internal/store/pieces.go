package store

import (
	"context"
	"strings"
)

// RechercherPieces cherche dans les pièces jointes du tenant : plein texte
// français sur le nom et le texte (colonne générée `recherche`), ou
// correspondance simple sur le nom, l'expéditeur et l'objet du message porteur
// (« Qonto » même si le PDF ne reprend pas le nom). Sans borne de date : la
// facture d'octobre doit se trouver fin novembre. Les plus récentes d'abord.
// Les messages à motif interdit (confidentiel, hameçonnage) sont écartés même
// s'ils portent encore des pièces d'avant leur reclassement : filet de
// sécurité, en plus du nettoyage au démarrage (schema.go).
func (s *Store) RechercherPieces(ctx context.Context, q string, limite int) ([]PieceTrouvee, error) {
	q = strings.TrimSpace(q)
	motif := "%" + strings.ToLower(q) + "%"
	rows, err := s.q(ctx).Query(ctx, `
		WITH req AS (SELECT websearch_to_tsquery('french', $1) AS q)
		SELECT a.id, a.nom, a.type_mime, m.id, m.thread_id, m.external_id, m.channel,
		       COALESCE(NULLIF(p.name, ''), m.sender), m.subject, m.sent_at,
		       CASE WHEN a.recherche @@ req.q
		            THEN ts_headline('french', left(a.texte, 20000), req.q, 'MaxWords=24, MinWords=10')
		            ELSE '' END
		FROM attachments a
		JOIN messages m ON m.id = a.message_id
		LEFT JOIN persons p ON p.email = m.sender, req
		WHERE (a.recherche @@ req.q OR lower(a.nom) LIKE $2 OR lower(m.sender) LIKE $2 OR lower(m.subject) LIKE $2)
		  AND NOT (m.status = 'excluded' AND (m.exclude_reason LIKE 'categorie_sensible\_%' OR m.exclude_reason LIKE 'securite\_%'))
		ORDER BY m.sent_at DESC, a.id LIMIT $3`, q, motif, limite)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PieceTrouvee{}
	for rows.Next() {
		var p PieceTrouvee
		if err := rows.Scan(&p.ID, &p.Nom, &p.TypeMime, &p.MessageID, &p.ThreadID, &p.ExternalID, &p.Channel,
			&p.Expediteur, &p.Objet, &p.Date, &p.Extrait); err != nil {
			return nil, err
		}
		// ts_headline balise la correspondance en <b>…</b> ; l'interface affiche
		// du texte, pas du HTML.
		p.Extrait = strings.NewReplacer("<b>", "", "</b>", "").Replace(p.Extrait)
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeleteAttachments retire toutes les pièces d'un message : appelé quand un
// message bascule après coup vers un motif interdit (catégorie confidentielle,
// hameçonnage), pour que rien de ce qu'il portait ne reste cherchable.
func (s *Store) DeleteAttachments(ctx context.Context, messageID int64) error {
	_, err := s.q(ctx).Exec(ctx, `DELETE FROM attachments WHERE message_id=$1`, messageID)
	return err
}

// MessagesConnusSansPiece : parmi des identifiants externes rapportés par le
// canal, les messages déjà en base qui n'ont aucune pièce enregistrée. Sert au
// rattrapage : ce sont eux qu'on relit, et eux seuls.
func (s *Store) MessagesConnusSansPiece(ctx context.Context, channel string, externalIDs []string) ([]Message, error) {
	if len(externalIDs) == 0 {
		return []Message{}, nil
	}
	rows, err := s.q(ctx).Query(ctx, `
		SELECT m.id, m.external_id, m.status, m.exclude_reason, m.subject, m.sender
		FROM messages m
		WHERE m.channel = $1 AND m.external_id = ANY($2)
		  AND NOT EXISTS (SELECT 1 FROM attachments a WHERE a.message_id = m.id)
		ORDER BY m.sent_at`, channel, externalIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ExternalID, &m.Status, &m.ExcludeReason, &m.Subject, &m.Sender); err != nil {
			return nil, err
		}
		m.Channel = channel
		out = append(out, m)
	}
	return out, rows.Err()
}

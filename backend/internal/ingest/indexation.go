package ingest

import (
	"context"
	"fmt"
	"time"

	"ibalya/backend/internal/channel"
)

// BilanPieces : ce qu'un rattrapage des pièces jointes a trouvé ou écrit.
type BilanPieces struct {
	Lus         int            `json:"lus"`         // messages rapportés par le canal
	Concernes   int            `json:"concernes"`   // déjà connus, sans pièce, avec au moins un document
	Pieces      int            `json:"pieces"`      // pièces retenues (écrites si appliquer)
	ParMotif    map[string]int `json:"par_motif"`   // messages concernés par motif (« conserve » pour les non écartés)
	Echantillon []string       `json:"echantillon"` // « expéditeur | objet | n pièce(s) », dix au plus
}

// IndexerPieces relit les messages des N derniers jours via le canal de la
// boîte et enregistre les documents joints des messages déjà connus qui n'en
// ont aucun : les boîtes raccordées avant cette fonctionnalité ont été lues
// sans les pièces de leurs messages écartés. Le mode est plafonné à texte seul,
// donc jamais d'OCR : sinon chaque passage renverrait au service d'inférence
// les images de signature dont l'OCR n'avait rien tiré, et la commande serait
// idempotente en écriture mais pas en coût. Sans appliquer, rien n'est écrit.
func (ing *Ingester) IndexerPieces(ctx context.Context, since time.Time, max int, appliquer bool) (BilanPieces, error) {
	b := BilanPieces{ParMotif: map[string]int{}}
	canal := ing.canal(ctx)
	msgs, err := canal.FetchSince(ctx, since, max)
	if err != nil {
		return b, err
	}
	b.Lus = len(msgs)
	sensibles := LireCategories(ing.Store.GetSetting(ctx, CleReglageCategories, ""))

	parID := map[string]channel.Message{}
	var ids []string
	for _, m := range msgs {
		if len(m.Attachments) == 0 {
			continue
		}
		parID[m.ExternalID] = m
		ids = append(ids, m.ExternalID)
	}
	connus, err := ing.Store.MessagesConnusSansPiece(ctx, canal.Name(), ids)
	if err != nil {
		return b, err
	}
	for _, m := range connus {
		mode := ModePour(m.Status, m.ExcludeReason)
		if mode == ModeComplete {
			mode = ModeTexteSeul // jamais d'OCR en rattrapage
		}
		pjs := parID[m.ExternalID].Attachments
		n := 0
		for _, pj := range pjs {
			if traiter, _ := planPiece(mode, pj.Nom, pj.Type); traiter {
				n++
			}
		}
		if n == 0 {
			continue
		}
		motif := "conserve"
		if m.Status == "excluded" && m.ExcludeReason != nil {
			motif = *m.ExcludeReason
		}
		b.Concernes++
		b.ParMotif[motif]++
		if appliquer {
			n = ing.enregistrerPiecesJointes(ctx, m.ID, pjs, mode, sensibles)
		}
		b.Pieces += n
		if len(b.Echantillon) < 10 {
			b.Echantillon = append(b.Echantillon, fmt.Sprintf("%s | %s | %d pièce(s)", m.Sender, tronquer(m.Subject, 60), n))
		}
	}
	return b, nil
}

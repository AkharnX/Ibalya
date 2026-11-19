package ingest

import (
	"context"
	"log"
	"strings"
	"time"

	"ibalya/backend/internal/channel"
	"ibalya/backend/internal/piecejointe"
	"ibalya/backend/internal/store"
)

type Ingester struct {
	Store     *store.Store
	Channel   channel.Reader
	CanalPour func(context.Context) channel.Reader
	// OCR (optionnel) : texte d'une image ou d'un PDF scanné, via le service LLM
	// (Mistral OCR, UE). Nil = pas d'OCR, on ignore simplement ces pièces.
	OCR func(ctx context.Context, nom, typeMime string, data []byte) (string, error)
}

// canal résout le canal du tenant courant (voir Engine.Canal).
func (ing *Ingester) canal(ctx context.Context) channel.Reader {
	if ing.CanalPour != nil {
		return ing.CanalPour(ctx)
	}
	return ing.Channel
}

type Stats struct {
	Fetched  int `json:"fetched"`
	Inserted int `json:"inserted"`
	Excluded int `json:"excluded"`
	Kept     int `json:"kept"`
	// Sensibles compte les messages écartés pour catégorie sensible. Le filtre
	// ne doit pas être silencieux : le dirigeant doit pouvoir constater qu'il
	// travaille, sans qu'on lui montre ce qui a été écarté.
	Sensibles int `json:"sensibles"`
}

// Run récupère les messages depuis `since`, les normalise, applique le
// pré-filtre EF-11 et persiste. Retourne les statistiques du cycle
// (le taux d'exclusion est un indicateur de santé économique — CDC 6.4).
func (ing *Ingester) Run(ctx context.Context, since time.Time, max int) (Stats, error) {
	var st Stats
	msgs, err := ing.canal(ctx).FetchSince(ctx, since, max)
	if err != nil {
		return st, err
	}
	st.Fetched = len(msgs)
	rules, err := ing.Store.ListRules(ctx, true)
	if err != nil {
		return st, err
	}
	sensibles := LireCategories(ing.Store.GetSetting(ctx, CleReglageCategories, ""))

	// Ensemble des adresses « moi » (boîte connectée + login + alias) : un
	// message dont l'expéditeur est le dirigeant est SORTANT, même s'il arrive
	// par une autre de ses adresses. Sinon il serait pris pour un interlocuteur.
	compteCanal, _ := ing.canal(ctx).AccountEmail(ctx)
	soi := ing.Store.AdressesSoi(ctx, compteCanal)

	for _, cm := range msgs {
		threadID, err := ing.Store.UpsertThread(ctx, ing.canal(ctx).Name(), cm.ThreadExternalID, cm.Subject, cm.SentAt)
		if err != nil {
			log.Printf("ingest: upsert thread: %v", err)
			continue
		}
		thread, err := ing.Store.GetThread(ctx, threadID)
		if err != nil || thread == nil {
			continue
		}
		m := store.Message{
			ThreadID:        threadID,
			ExternalID:      cm.ExternalID,
			Channel:         ing.canal(ctx).Name(),
			Sender:          cm.Sender,
			Recipients:      cm.Recipients,
			SentAt:          cm.SentAt,
			Subject:         cm.Subject,
			Body:            cm.Body,
			Outbound:        cm.Outbound || soi[strings.ToLower(strings.TrimSpace(cm.Sender))],
			ListUnsubscribe: cm.ListUnsubscribe,
			AutoSubmitted:   cm.AutoSubmitted,
		}
		// Les catégories sensibles se décident AVANT l'écriture : le contenu
		// d'un arrêt maladie ou d'une candidature n'est pas conservé du tout,
		// seules les métadonnées le sont pour garder trace de l'échange. La
		// messagerie d'origine reste la source de vérité.
		categorie := DetecterCategorie(m.Subject, m.Body, sensibles)
		if categorie != "" {
			m.Subject, m.Body = "", ""
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
		// Les pièces jointes suivent le sort du message, mais pas tout ou rien :
		// les factures et relevés arrivent de robots que le pré-filtre écarte, et
		// l'utilisateur veut les retrouver. Les messages écartés gardent leurs
		// documents en texte seul (sans OCR, sans image) ; les confidentiels et
		// les hameçonnages n'ont rien, la pièce n'est pas ouverte (ModePour).
		if categorie != "" {
			raison := "categorie_sensible_" + string(categorie)
			st.Excluded++
			st.Sensibles++
			_ = ing.Store.MarkMessage(ctx, id, "excluded", &raison)
			ing.enregistrerPiecesJointes(ctx, id, cm.Attachments, ModePour("excluded", &raison))
		} else if reason := ExclusionReason(m, thread.Excluded, rules); reason != "" {
			st.Excluded++
			_ = ing.Store.MarkMessage(ctx, id, "excluded", &reason)
			ing.enregistrerPiecesJointes(ctx, id, cm.Attachments, ModePour("excluded", &reason))
		} else {
			st.Kept++
			ing.enregistrerPiecesJointes(ctx, id, cm.Attachments, ModeComplete)
		}
	}

	// met à jour le rythme de réponse appris par fil (détecteur silence anormal)
	ing.updateRhythms(ctx)

	ing.Store.Audit(ctx, "agent", "ingestion_cycle", st)
	return st, nil
}

// maxPiecesParMessage : garde-fou, aligné sur le connecteur Gmail.
const maxPiecesParMessage = 8

// enregistrerPiecesJointes extrait le texte des pièces jointes d'un message et le
// stocke (jamais les octets), selon le mode (voir ModePieces). Un document est
// enregistré même sans texte, pour rester trouvable par son nom ; une image ne
// l'est que si l'OCR en tire du texte. Une pièce illisible n'empêche rien : le
// corps du mail reste traité normalement. Retourne le nombre de lignes écrites.
func (ing *Ingester) enregistrerPiecesJointes(ctx context.Context, messageID int64, pjs []channel.PieceJointe, mode ModePieces) int {
	ecrites := 0
	for _, pj := range pjs {
		if ecrites >= maxPiecesParMessage {
			break
		}
		traiter, ocr := planPiece(mode, pj.Nom, pj.Type)
		if !traiter {
			continue
		}
		texte := ""
		r, err := piecejointe.Extraire(pj.Nom, pj.Type, pj.Donnees)
		if err == nil {
			texte = r.Texte
			// Image ou PDF scanné : pas de couche texte -> OCR si le mode le permet
			// (les octets sont encore là, on ne les persiste jamais).
			if r.BesoinOCR && ocr && ing.OCR != nil {
				if t, err := ing.OCR(ctx, pj.Nom, pj.Type, pj.Donnees); err == nil {
					texte = strings.TrimSpace(t)
				} else {
					log.Printf("ingest: OCR %q: %v", pj.Nom, err)
				}
			}
		}
		if texte == "" && !garderSansTexte(pj.Nom, pj.Type) {
			continue
		}
		if err := ing.Store.InsertAttachment(ctx, messageID, store.Attachment{
			Nom: pj.Nom, TypeMime: pj.Type, Texte: texte, BesoinOCR: false,
		}); err != nil {
			log.Printf("ingest: pièce jointe %q: %v", pj.Nom, err)
			continue
		}
		ecrites++
	}
	return ecrites
}

// ScanContexte lit l'historique en MÉTADONNÉES seules (expéditeur,
// destinataires, date, objet) et en reconstruit le contexte : carnet de
// contacts et fils connus. Aucun corps n'est téléchargé ni stocké (agrégats
// seulement), aucun engagement n'est extrait — l'extraction, coûteuse, reste
// bornée à la fenêtre récente. Peu coûteux (SQL/heuristique, zéro LLM) et
// propre côté RGPD. Règle deux retours de Stewe : « connais tous mes contacts »
// et « pas d'alertes de vieux mails ».
func (ing *Ingester) ScanContexte(ctx context.Context, since time.Time, max int) (int, error) {
	msgs, err := ing.canal(ctx).FetchMetaSince(ctx, since, max)
	if err != nil {
		return 0, err
	}
	compteCanal, _ := ing.canal(ctx).AccountEmail(ctx)
	soi := ing.Store.AdressesSoi(ctx, compteCanal)
	nom := ing.canal(ctx).Name()

	// Un contact est une personne : on n'ajoute jamais le dirigeant à son
	// propre carnet, ni les automates (no-reply, notifications, newsletters,
	// transporteurs), qui noyaient la liste sous des adresses sans intérêt.
	humain := func(adresse string) bool {
		a := strings.ToLower(strings.TrimSpace(adresse))
		if a == "" || soi[a] {
			return false
		}
		classe, _ := ClasserExpediteur(store.Message{Sender: a})
		return classe == ClasseHumain
	}
	for _, cm := range msgs {
		if humain(cm.Sender) {
			ing.Store.UpsertPerson(ctx, cm.Sender, cm.SenderName)
		}
		for _, r := range cm.Recipients {
			if humain(r) {
				ing.Store.UpsertPerson(ctx, r, "")
			}
		}
		if cm.ThreadExternalID != "" {
			ing.Store.UpsertThread(ctx, nom, cm.ThreadExternalID, cm.Subject, cm.SentAt)
		}
	}
	return len(msgs), nil
}

// updateRhythms calcule le rythme de réponse habituel par fil : moyenne des
// écarts entre messages d'expéditeurs différents (CDC 5.3 / détecteur 2).
func (ing *Ingester) updateRhythms(ctx context.Context) {
	rows, err := ing.Store.Q(ctx).Query(ctx, `
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

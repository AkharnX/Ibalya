package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ibalya/backend/internal/llm"
	"ibalya/backend/internal/store"
)

// Assistant conversationnel : le dirigeant interroge sa boîte en langage
// naturel (« combien de devis en attente ? », « où en est Martin ? »).
//
// Le chatbot ne relit pas Gmail en direct : il répond par-dessus les données
// déjà extraites et stockées (engagements, alertes, messages). Le contexte est
// assemblé ICI, dans la connexion du tenant courant : RLS garantit qu'on ne
// rassemble que la boîte de l'utilisateur. Le modèle informe, il n'agit jamais.
//
// Chaque élément de contexte porte une `ref` ; le modèle renvoie les refs qu'il
// a utilisées, que le backend résout en sources cliquables (lien vers le fil).

type chatReq struct {
	Question   string         `json:"question"`
	Historique []llm.ChatTour `json:"historique,omitempty"`
}

type chatResponse struct {
	Reponse string             `json:"reponse"`
	Sources []store.SourceChat `json:"sources"`
}

// POST /api/chat
func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	var in chatReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "corps invalide", http.StatusBadRequest)
		return
	}
	in.Question = strings.TrimSpace(in.Question)
	if in.Question == "" {
		http.Error(w, "question vide", http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	// refs : ref d'un élément de contexte -> source cliquable correspondante.
	refs := map[string]store.SourceChat{}
	engagements := s.chatEngagements(ctx, refs)
	alertes := s.chatAlertes(ctx, refs)
	messages := s.chatMessages(ctx, in.Question, refs)

	var reponse string
	var sources []store.SourceChat
	// Garde-fou anti-hallucination : sans aucune donnée, on ne consulte PAS le
	// modèle. Un LLM à qui l'on ne fournit aucun contexte comble le vide en
	// inventant (faux devis, faux clients). Le seul cas sûr est de répondre
	// nous-mêmes que la boîte n'a rien à analyser. C'est le cas typique d'un
	// nouvel utilisateur qui n'a pas encore raccordé sa boîte.
	if len(engagements) == 0 && len(alertes) == 0 && len(messages) == 0 {
		reponse = "Je n'ai encore aucune donnée à analyser : ta boîte n'est pas raccordée, " +
			"ou aucun cycle de lecture n'a encore tourné. Va dans Réglages → Connexion pour " +
			"raccorder ta boîte, puis reviens me poser tes questions. Je ne réponds qu'à partir " +
			"de tes vrais échanges, jamais d'exemples inventés."
		sources = []store.SourceChat{}
	} else {
		stats := s.chatStats(ctx)
		req := llm.ChatRequest{
			Question:    in.Question,
			Aujourdhui:  time.Now().Format("2006-01-02"),
			Stats:       &stats,
			Engagements: engagements,
			Alertes:     alertes,
			Messages:    messages,
			Historique:  bornerHistorique(in.Historique, 8),
		}
		r2, err := s.Engine.LLM.Chat(ctx, req)
		if err != nil {
			http.Error(w, "assistant indisponible", http.StatusBadGateway)
			return
		}
		reponse = r2.Reponse
		sources = resoudreSources(r2.Sources, refs)
	}

	// Historisation : on conserve le tour (question puis réponse). Un échec
	// d'écriture ne doit pas priver l'utilisateur de sa réponse.
	_ = s.Store.AjouterTourChat(ctx, "user", in.Question, nil)
	_ = s.Store.AjouterTourChat(ctx, "assistant", reponse, sources)

	writeJSON(w, chatResponse{Reponse: reponse, Sources: sources})
}

// resoudreSources traduit les refs renvoyées par le modèle en sources
// cliquables, en ignorant celles qu'il aurait inventées et en dédoublonnant.
func resoudreSources(refsModele []string, refs map[string]store.SourceChat) []store.SourceChat {
	out := []store.SourceChat{}
	vus := map[string]bool{}
	for _, ref := range refsModele {
		src, ok := refs[strings.TrimSpace(ref)]
		if !ok {
			continue // ref hors contexte : jamais inventer un lien
		}
		cle := fmt.Sprintf("%s|%d", src.Label, src.ThreadID)
		if vus[cle] {
			continue
		}
		vus[cle] = true
		out = append(out, src)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

// GET /api/chat/historique — relit la conversation persistée du tenant.
func (s *Server) chatHistorique(w http.ResponseWriter, r *http.Request) {
	tours, err := s.Store.HistoriqueChat(r.Context(), 200)
	if err != nil {
		http.Error(w, "historique indisponible", http.StatusBadGateway)
		return
	}
	writeJSON(w, tours)
}

// DELETE /api/chat/historique — efface la conversation (« nouvelle conversation »).
func (s *Server) chatEffacer(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.EffacerHistoriqueChat(r.Context()); err != nil {
		http.Error(w, "effacement impossible", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// bornerHistorique ne garde que les derniers tours : au-delà, le contexte
// gonfle sans améliorer la réponse.
func bornerHistorique(h []llm.ChatTour, max int) []llm.ChatTour {
	if len(h) > max {
		return h[len(h)-max:]
	}
	return h
}

func (s *Server) chatEngagements(ctx context.Context, refs map[string]store.SourceChat) []llm.ChatEngagement {
	out := []llm.ChatEngagement{}
	rows, err := s.Store.Q(ctx).Query(ctx, `
		SELECT e.objet, e.type, e.statut,
		       coalesce(to_char(e.echeance,'DD/MM/YYYY'),''),
		       coalesce(nullif(pd.name,''), pd.email, ''),
		       (e.echeance IS NOT NULL AND e.echeance < current_date
		        AND e.statut NOT IN ('livre','abandonne')) AS en_retard,
		       e.thread_id
		  FROM engagements e
		  LEFT JOIN persons pd ON pd.id = e.destinataire_id
		 WHERE e.statut <> 'abandonne'
		 ORDER BY e.echeance ASC NULLS LAST, e.maj_le DESC
		 LIMIT 50`)
	if err != nil {
		return out
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var e llm.ChatEngagement
		var enRetard *bool
		var threadID *int64
		if err := rows.Scan(&e.Objet, &e.Type, &e.Statut, &e.Echeance, &e.Interlocuteur, &enRetard, &threadID); err != nil {
			return out
		}
		e.EnRetard = enRetard != nil && *enRetard
		n++
		e.Ref = fmt.Sprintf("e%d", n)
		label := e.Objet
		if e.Interlocuteur != "" {
			label += " — " + e.Interlocuteur
		}
		refs[e.Ref] = store.SourceChat{Label: label, ThreadID: deref(threadID)}
		out = append(out, e)
	}
	return out
}

// chatStats calcule en base les décomptes exacts (le LLM compte mal sur de
// grandes listes). « En cours » = statut ouvert ou en_retard.
func (s *Server) chatStats(ctx context.Context) llm.ChatStats {
	st := llm.ChatStats{ParTypeEnCours: map[string]int{}}
	q := s.Store.Q(ctx)
	_ = q.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (WHERE statut IN ('ouvert','en_retard')),
		  count(*) FILTER (WHERE echeance IS NOT NULL AND echeance < current_date
		                   AND statut NOT IN ('livre','abandonne'))
		  FROM engagements`).Scan(&st.EngagementsEnCours, &st.EnRetard)
	_ = q.QueryRow(ctx, `SELECT count(*) FROM detections WHERE statut='nouvelle'`).Scan(&st.Alertes)

	rows, err := q.Query(ctx, `
		SELECT type, count(*) FROM engagements
		 WHERE statut IN ('ouvert','en_retard') GROUP BY type`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var n int
			if rows.Scan(&t, &n) == nil {
				st.ParTypeEnCours[t] = n
			}
		}
	}
	return st
}

func (s *Server) chatAlertes(ctx context.Context, refs map[string]store.SourceChat) []llm.ChatAlerte {
	out := []llm.ChatAlerte{}
	rows, err := s.Store.Q(ctx).Query(ctx, `
		SELECT type, titre, thread_id FROM detections
		 WHERE statut = 'nouvelle'
		 ORDER BY critique DESC, created_at DESC
		 LIMIT 30`)
	if err != nil {
		return out
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var a llm.ChatAlerte
		var threadID *int64
		if err := rows.Scan(&a.Type, &a.Objet, &threadID); err != nil {
			return out
		}
		n++
		a.Ref = fmt.Sprintf("a%d", n)
		refs[a.Ref] = store.SourceChat{Label: a.Objet, ThreadID: deref(threadID)}
		out = append(out, a)
	}
	return out
}

// chatMessages retrouve les extraits de messages pertinents pour la question,
// par recherche de mots-clés (ILIKE). MVP : suffisant pour retrouver un dossier
// par nom d'interlocuteur ou par sujet. On passera au plein-texte (tsvector)
// si le rappel « par sens » manque.
func (s *Server) chatMessages(ctx context.Context, question string, refs map[string]store.SourceChat) []llm.ChatMessage {
	out := []llm.ChatMessage{}
	motifs := motsClesMotifs(question)
	if len(motifs) == 0 {
		return out
	}
	rows, err := s.Store.Q(ctx).Query(ctx, `
		SELECT coalesce(nullif(t.subject,''),'(sans objet)'),
		       m.sender,
		       coalesce(to_char(m.sent_at,'DD/MM/YYYY'),''),
		       left(m.body, 300),
		       t.id
		  FROM messages m
		  JOIN threads t ON t.id = m.thread_id
		 WHERE lower(m.subject || ' ' || m.body) LIKE ANY($1)
		 ORDER BY m.sent_at DESC NULLS LAST
		 LIMIT 8`, motifs)
	if err != nil {
		return out
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var m llm.ChatMessage
		var threadID int64
		if err := rows.Scan(&m.Fil, &m.De, &m.Date, &m.Extrait, &threadID); err != nil {
			return out
		}
		m.Extrait = strings.Join(strings.Fields(m.Extrait), " ") // aplatit les blancs
		n++
		m.Ref = fmt.Sprintf("m%d", n)
		label := "Fil « " + m.Fil + " »"
		if m.Date != "" {
			label += " — " + m.Date
		}
		refs[m.Ref] = store.SourceChat{Label: label, ThreadID: threadID}
		out = append(out, m)
	}
	return out
}

func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// motsClesMotifs extrait de la question les mots discriminants (≥ 4 lettres,
// hors mots vides) et les transforme en motifs ILIKE. Six au plus : au-delà,
// on élargit la recherche pour rien.
func motsClesMotifs(question string) []string {
	champs := strings.FieldsFunc(strings.ToLower(question), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == 'é' || r == 'è' || r == 'ê' || r == 'à' || r == 'ç' ||
			r == 'ù' || r == 'â' || r == 'î' || r == 'ô' || r == 'û')
	})
	vus := map[string]bool{}
	motifs := []string{}
	for _, mot := range champs {
		if len(mot) < 4 || motsVides[mot] || vus[mot] {
			continue
		}
		vus[mot] = true
		motifs = append(motifs, "%"+mot+"%")
		if len(motifs) >= 6 {
			break
		}
	}
	return motifs
}

// Mots trop fréquents pour discriminer : les inclure ferait tout remonter.
var motsVides = map[string]bool{
	"avec": true, "dans": true, "pour": true, "cette": true, "quel": true,
	"quelle": true, "quels": true, "quelles": true, "combien": true, "est": true,
	"sont": true, "mais": true, "donc": true, "tous": true, "toutes": true,
	"plus": true, "moins": true, "leur": true, "leurs": true, "nous": true,
	"vous": true, "elle": true, "elles": true, "être": true, "cela": true,
	"comme": true, "sans": true, "aussi": true, "encore": true, "depuis": true,
	"chez": true, "mail": true, "mails": true, "email": true, "emails": true,
}

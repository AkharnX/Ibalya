package engine

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"ibalya/backend/internal/store"
)

// Détecteur 6 — « C'est fait ? » : un engagement en retard dont plus personne
// ne parle, ou dont le fil annonce la fin (livré, payé, facture jointe), ne
// mérite pas une alerte de plus : il mérite une question à un clic. Sans
// cela, une commande livrée et payée reste « en retard de 41 jours »
// (constat BS ACCESS, 11 nov. 2026).

// DelaiAClore : au-delà de ce retard sans activité, on pose la question.
const DelaiAClore = 7 * 24 * time.Hour

var sansAccents = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c",
)

var signauxFin = regexp.MustCompile(`(` +
	`\blivre(e|s|es)?\b|receptionne(e|s|es)?|bien recu(e|s|es)?|recu(e|s|es)? ce (jour|matin)|` +
	`\bpaye(e|s|es)?\b|\bregle(e|s|es)?\b|virement (effectue|recu|fait)|reglement (effectue|recu)|` +
	`merci pour (la|votre|cette) (livraison|intervention|reactivite|rapidite)|` +
	`facture (ci-?jointe|jointe|acquittee|en piece jointe)|` +
	`c'est fait|est fait|termine(e|s|es)?\b|cloture(e|s|es)?\b` +
	`)`)

// SignalFin dit si un texte annonce qu'un engagement est accompli, et rend la
// phrase qui le dit (120 caractères au plus), pour l'afficher au dirigeant.
func SignalFin(texte string) (string, bool) {
	norm := sansAccents.Replace(strings.ToLower(texte))
	loc := signauxFin.FindStringIndex(norm)
	if loc == nil {
		return "", false
	}
	// La normalisation ne change pas la longueur en octets des lettres
	// remplacées une par une ? Si : « é » fait deux octets, « e » un seul.
	// On travaille donc sur les runes pour retrouver la phrase d'origine.
	runesNorm := []rune(norm)
	runesOrig := []rune(texte)
	debut := len([]rune(norm[:loc[0]]))
	fin := len([]rune(norm[:loc[1]]))
	if len(runesOrig) != len(runesNorm) {
		// Un remplacement n'a pas conservé le nombre de runes : on rend le
		// signal lui-même plutôt qu'une phrase décalée.
		return tronquerRunes(string(runesNorm[debut:fin]), 120), true
	}
	a, b := debut, fin
	for a > 0 && !strings.ContainsRune(".!?\n", runesOrig[a-1]) {
		a--
	}
	for b < len(runesOrig) && !strings.ContainsRune(".!?\n", runesOrig[b]) {
		b++
	}
	phrase := strings.TrimSpace(string(runesOrig[a:b]))
	if len([]rune(phrase)) > 120 {
		// Trop long : on centre sur le signal.
		marge := 50
		a2 := debut - marge
		if a2 < a {
			a2 = a
		}
		b2 := fin + marge
		if b2 > b {
			b2 = b
		}
		phrase = "…" + strings.TrimSpace(string(runesOrig[a2:b2])) + "…"
	}
	return tronquerRunes(phrase, 120), true
}

func tronquerRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// aClorePourDelai : échéance dépassée depuis DelaiAClore au moins, et aucun
// événement depuis autant. Un retard frais, ou un engagement relancé il y a
// trois jours, garde ses alertes ordinaires.
func aClorePourDelai(echeance, dernierEvenement, now time.Time) bool {
	limite := now.Add(-DelaiAClore)
	return !echeance.After(limite) && !dernierEvenement.After(limite)
}

func (e *Engine) detectAClore(ctx context.Context, p capsuleParams, today string) (int, error) {
	engs, err := e.Store.ListEngagements(ctx, []string{"en_retard"})
	if err != nil {
		return 0, err
	}
	now := time.Now()
	count := 0
	for _, eng := range engs {
		if eng.Echeance == nil {
			continue
		}
		detail, raison, messageID := "", "", int64(0)
		if eng.ThreadID != nil {
			msgs, _ := e.Store.ThreadMessages(ctx, *eng.ThreadID)
			for _, m := range msgs {
				if !m.SentAt.After(*eng.Echeance) {
					continue
				}
				if extrait, ok := SignalFin(m.Subject + "\n" + m.Body); ok {
					detail = fmt.Sprintf("« %s » : un message du %s semble dire que c'est fait : « %s »",
						eng.Objet, m.SentAt.Format("02/01"), extrait)
					raison, messageID = "signal", m.ID
					break
				}
			}
		}
		if detail == "" {
			dernier := eng.CreeLe
			if t, err := e.Store.LastEventAt(ctx, eng.ID); err == nil && t != nil && t.After(dernier) {
				dernier = *t
			}
			if !aClorePourDelai(*eng.Echeance, dernier, now) {
				continue
			}
			detail = fmt.Sprintf("« %s » : échéance dépassée depuis %s, sans activité.",
				eng.Objet, Jours(-joursCalendaires(*eng.Echeance, now)))
			raison = "delai"
		}
		d := store.Detection{
			Type: "a_clore", EngagementID: &eng.ID, ThreadID: eng.ThreadID,
			Score: 0.6, Critique: false,
			Titre:   "C'est fait ?",
			Detail:  detail,
			Payload: mustJSON(map[string]any{"raison": raison, "message_id": messageID}),
		}
		if id, _ := e.Store.CreateDetection(ctx, d, cleAlerte("a_clore", eng.ID)); id != 0 {
			count++
		}
	}
	return count, nil
}

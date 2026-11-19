package ingest

import (
	"strings"

	"ibalya/backend/internal/piecejointe"
)

// ModePieces dit ce qu'on fait des pièces jointes d'un message selon le sort
// que le pré-filtre lui a réservé. Les factures, relevés et bons de commande
// arrivent presque toujours de robots (facturation@, noreply@) : n'indexer que
// les pièces des messages conservés revenait à ne connaître aucune facture.
// Voir docs/superpowers/specs/2026-11-19-recherche-pieces-jointes-design.md.
type ModePieces string

const (
	// ModeAucune : la pièce n'est même pas ouverte. Catégories confidentielles
	// (on promet de ne garder que l'expéditeur et la date) et hameçonnage (c'est
	// précisément le fichier qu'on refuse d'ouvrir).
	ModeAucune ModePieces = "aucune"
	// ModeTexteSeul : documents seulement, couche texte seulement, jamais d'OCR
	// (donc rien ne part au service d'inférence, comme la page Données le
	// promet pour les emails automatiques), pas d'image (ce sont des logos).
	ModeTexteSeul ModePieces = "texte_seul"
	// ModeComplete : le comportement historique des messages conservés,
	// OCR des images et PDF scannés compris.
	ModeComplete ModePieces = "complete"
)

// ModePour décide du mode d'après le statut du message et son motif
// d'exclusion. Règle de préfixe pour les motifs interdits : un motif ajouté
// plus tard sous « categorie_sensible_ » ou « securite_ » reste interdit sans
// qu'on y pense. Un motif inconnu donne texte seul, jamais complet.
func ModePour(statut string, motif *string) ModePieces {
	if statut != "excluded" {
		return ModeComplete
	}
	if motif == nil {
		return ModeTexteSeul
	}
	if strings.HasPrefix(*motif, "categorie_sensible_") || strings.HasPrefix(*motif, "securite_") {
		return ModeAucune
	}
	return ModeTexteSeul
}

// planPiece dit si une pièce est traitée dans ce mode, et si l'OCR est permis.
func planPiece(mode ModePieces, nom, mime string) (traiter, ocr bool) {
	switch mode {
	case ModeTexteSeul:
		return piecejointe.Document(nom, mime), false
	case ModeComplete:
		traiter = piecejointe.Document(nom, mime) || piecejointe.Image(nom, mime)
		return traiter, traiter
	}
	return false, false
}

// garderSansTexte : un document sans texte extractible (PDF scanné sans OCR,
// archive Office illisible) est enregistré quand même, nom et type, pour qu'on
// le retrouve par son nom. Une image dont l'OCR n'a rien tiré n'est pas un
// document, elle n'est pas gardée.
func garderSansTexte(nom, mime string) bool { return piecejointe.Document(nom, mime) }

// Package piecejointe extrait le texte d'une pièce jointe (PDF, image, Office).
//
// Stratégie « hybride, local d'abord » : le texte des PDF et des documents
// Office est lu localement, sans que rien ne sorte du serveur. Les images et les
// PDF scannés (sans texte extractible) sont signalés BesoinOCR=true : l'appelant
// délègue alors au service d'OCR (Mistral, UE). On ne stocke jamais les octets
// bruts : seul le texte extrait est conservé, cloisonné comme le reste.
package piecejointe

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

// TailleMax borne la taille d'une pièce jointe traitée. Au-delà, on ignore :
// un fichier énorme est rarement un engagement, et le coût (lecture, OCR,
// modèle) ne le justifie pas.
const TailleMax = 12 << 20 // 12 Mo

// seuilTextePDF : en-dessous de ce nombre de caractères utiles, on considère que
// le PDF n'a pas de couche texte (scanné) et qu'il faut l'OCR.
const seuilTextePDF = 24

// Resultat porte le texte extrait et, le cas échéant, le besoin d'OCR.
type Resultat struct {
	Texte     string // texte extrait (vide si BesoinOCR)
	BesoinOCR bool   // true : image ou PDF scanné, à envoyer à l'OCR
	Methode   string // "pdf-texte", "docx", "xlsx", "image", "pdf-scanne"
}

var (
	reEspaces = regexp.MustCompile(`[ \t]+`)
	reLignes  = regexp.MustCompile(`\n{3,}`)
	// Texte des éléments Word (<w:t>…</w:t>) et des chaînes partagées xlsx (<t>…</t>).
	reBaliseT = regexp.MustCompile(`(?s)<(?:w:)?t(?:\s[^>]*)?>(.*?)</(?:w:)?t>`)
	reBalises = regexp.MustCompile(`<[^>]+>`)
)

// Traitable indique si on sait (ou saura via OCR) extraire ce type.
// On se fie à l'extension ET au type MIME : les deux sont parfois trompeurs.
func Traitable(nom, mime string) bool {
	switch famille(nom, mime) {
	case "pdf", "docx", "xlsx", "image":
		return true
	}
	return false
}

func famille(nom, mime string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(nom), "."))
	mime = strings.ToLower(mime)
	switch {
	case ext == "pdf" || strings.Contains(mime, "pdf"):
		return "pdf"
	case ext == "docx" || strings.Contains(mime, "wordprocessingml"):
		return "docx"
	case ext == "xlsx" || strings.Contains(mime, "spreadsheetml"):
		return "xlsx"
	case ext == "png" || ext == "jpg" || ext == "jpeg" || ext == "webp" ||
		ext == "gif" || ext == "tiff" || ext == "tif" ||
		strings.HasPrefix(mime, "image/"):
		return "image"
	}
	return ""
}

// Extraire tente l'extraction locale. Pour une image (ou un PDF sans texte), il
// renvoie BesoinOCR=true sans erreur : à l'appelant de passer par l'OCR.
func Extraire(nom, mime string, data []byte) (Resultat, error) {
	if len(data) == 0 {
		return Resultat{}, fmt.Errorf("pièce jointe vide")
	}
	if len(data) > TailleMax {
		return Resultat{}, fmt.Errorf("pièce jointe trop volumineuse (%d octets)", len(data))
	}
	switch famille(nom, mime) {
	case "pdf":
		return extrairePDF(data)
	case "docx":
		t, err := extraireOOXML(data, reBaliseT)
		return Resultat{Texte: nettoyer(t), Methode: "docx"}, err
	case "xlsx":
		t, err := extraireOOXML(data, reBaliseT)
		return Resultat{Texte: nettoyer(t), Methode: "xlsx"}, err
	case "image":
		return Resultat{BesoinOCR: true, Methode: "image"}, nil
	}
	return Resultat{}, fmt.Errorf("type non pris en charge : %s (%s)", nom, mime)
}

func extrairePDF(data []byte) (Resultat, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		// Illisible en tant que PDF texte : on tente l'OCR (scan, PDF exotique).
		return Resultat{BesoinOCR: true, Methode: "pdf-scanne"}, nil
	}
	var buf bytes.Buffer
	flux, err := r.GetPlainText()
	if err == nil {
		_, _ = io.Copy(&buf, flux)
	}
	texte := nettoyer(buf.String())
	if len([]rune(texte)) < seuilTextePDF {
		// Pas de couche texte utile : PDF scanné -> OCR.
		return Resultat{BesoinOCR: true, Methode: "pdf-scanne"}, nil
	}
	return Resultat{Texte: texte, Methode: "pdf-texte"}, nil
}

// extraireOOXML lit une archive Office (docx/xlsx) et concatène le texte des
// balises ciblées. docx : word/document.xml. xlsx : xl/sharedStrings.xml (toutes
// les chaînes de cellules) + les éventuels textes inline des feuilles.
func extraireOOXML(data []byte, re *regexp.Regexp) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("archive Office illisible : %w", err)
	}
	var morceaux []string
	for _, f := range zr.File {
		nom := f.Name
		pertinent := nom == "word/document.xml" ||
			nom == "xl/sharedStrings.xml" ||
			(strings.HasPrefix(nom, "xl/worksheets/") && strings.HasSuffix(nom, ".xml")) ||
			(strings.HasPrefix(nom, "word/") && strings.HasSuffix(nom, ".xml") && strings.Contains(nom, "header") )
		if !pertinent {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		contenu, _ := io.ReadAll(io.LimitReader(rc, TailleMax))
		rc.Close()
		for _, m := range re.FindAllSubmatch(contenu, -1) {
			if len(m) > 1 {
				morceaux = append(morceaux, string(m[1]))
			}
		}
	}
	// Repli : certains documents n'ont pas la structure attendue -> on dégrade en
	// retirant toutes les balises du document principal.
	if len(morceaux) == 0 {
		for _, f := range zr.File {
			if f.Name == "word/document.xml" || f.Name == "xl/sharedStrings.xml" {
				rc, err := f.Open()
				if err != nil {
					continue
				}
				contenu, _ := io.ReadAll(io.LimitReader(rc, TailleMax))
				rc.Close()
				morceaux = append(morceaux, string(reBalises.ReplaceAll(contenu, []byte(" "))))
			}
		}
	}
	return strings.Join(morceaux, " "), nil
}

// nettoyer normalise les blancs : OCR, PDF et XML produisent des espacements
// erratiques qui gonflent le texte envoyé au modèle sans rien apporter.
func nettoyer(s string) string {
	s = strings.ReplaceAll(s, "\r", "\n")
	s = reEspaces.ReplaceAllString(s, " ")
	lignes := strings.Split(s, "\n")
	for i, l := range lignes {
		lignes[i] = strings.TrimSpace(l)
	}
	s = strings.Join(lignes, "\n")
	s = reLignes.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

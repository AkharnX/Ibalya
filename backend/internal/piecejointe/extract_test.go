package piecejointe

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func zipAvec(t *testing.T, fichiers map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for nom, contenu := range fichiers {
		f, err := w.Create(nom)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(contenu)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestTraitableEtFamille(t *testing.T) {
	cas := []struct {
		nom, mime string
		veut      bool
	}{
		{"devis.pdf", "application/pdf", true},
		{"photo.JPG", "", true},
		{"scan.png", "image/png", true},
		{"contrat.docx", "", true},
		{"prix.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true},
		{"note.txt", "text/plain", false},
		{"archive.zip", "application/zip", false},
		{"", "application/pdf", true}, // type MIME seul
	}
	for _, c := range cas {
		if got := Traitable(c.nom, c.mime); got != c.veut {
			t.Errorf("Traitable(%q,%q)=%v, veut %v", c.nom, c.mime, got, c.veut)
		}
	}
}

func TestExtraireDocx(t *testing.T) {
	doc := `<?xml version="1.0"?><w:document><w:body>` +
		`<w:p><w:r><w:t>Bon de commande</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t xml:space="preserve">Livraison avant le 15 octobre</w:t></w:r></w:p>` +
		`</w:body></w:document>`
	data := zipAvec(t, map[string]string{
		"[Content_Types].xml": "<Types/>",
		"word/document.xml":   doc,
	})
	r, err := Extraire("contrat.docx", "", data)
	if err != nil {
		t.Fatal(err)
	}
	if r.BesoinOCR {
		t.Fatal("docx ne doit pas demander d'OCR")
	}
	if !strings.Contains(r.Texte, "Bon de commande") || !strings.Contains(r.Texte, "Livraison avant le 15 octobre") {
		t.Fatalf("texte docx manquant : %q", r.Texte)
	}
}

func TestExtraireXlsx(t *testing.T) {
	ss := `<?xml version="1.0"?><sst><si><t>Devis Dupont</t></si><si><t>Échéance 30/11</t></si></sst>`
	data := zipAvec(t, map[string]string{
		"[Content_Types].xml":      "<Types/>",
		"xl/sharedStrings.xml":     ss,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData/></worksheet>`,
	})
	r, err := Extraire("prix.xlsx", "", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Texte, "Devis Dupont") || !strings.Contains(r.Texte, "Échéance 30/11") {
		t.Fatalf("texte xlsx manquant : %q", r.Texte)
	}
}

func TestImageDemandeOCR(t *testing.T) {
	r, err := Extraire("scan.jpg", "image/jpeg", []byte{0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if !r.BesoinOCR || r.Methode != "image" {
		t.Fatalf("image doit demander l'OCR, eu %+v", r)
	}
}

func TestPDFIllisibleBasculeOCR(t *testing.T) {
	// Des octets qui ne sont pas un PDF valide : extraction locale impossible,
	// on bascule proprement sur l'OCR plutôt que d'échouer.
	r, err := Extraire("scan.pdf", "application/pdf", []byte("%PDF-1.4 pas vraiment un pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if !r.BesoinOCR {
		t.Fatalf("un PDF sans texte doit demander l'OCR, eu %+v", r)
	}
}

func TestBornes(t *testing.T) {
	if _, err := Extraire("x.pdf", "application/pdf", nil); err == nil {
		t.Error("pièce vide doit échouer")
	}
	gros := make([]byte, TailleMax+1)
	if _, err := Extraire("x.pdf", "application/pdf", gros); err == nil {
		t.Error("pièce trop grosse doit échouer")
	}
	if _, err := Extraire("x.txt", "text/plain", []byte("coucou")); err == nil {
		t.Error("type non géré doit échouer")
	}
}

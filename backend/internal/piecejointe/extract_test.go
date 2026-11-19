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
		{"note.txt", "text/plain", true}, // document texte depuis la recherche de pièces jointes
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
	if _, err := Extraire("x.dwg", "application/octet-stream", []byte("coucou")); err == nil {
		t.Error("type non géré doit échouer")
	}
}

func TestDocumentImageEtLibelle(t *testing.T) {
	cas := []struct {
		nom, mime string
		doc, img  bool
		lib       string
	}{
		{"facture.pdf", "application/pdf", true, false, "PDF"},
		{"devis.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", true, false, "Word"},
		{"releve.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true, false, "Excel"},
		{"pres.pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", true, false, "PowerPoint"},
		{"notes.txt", "text/plain", true, false, "Texte"},
		{"export.csv", "text/csv", true, false, "Texte"},
		{"scan.jpg", "image/jpeg", false, true, "Image"},
		{"plan.dwg", "application/octet-stream", false, false, ""},
		{"archive.zip", "application/zip", false, false, ""},
		{"sans-extension", "application/pdf", true, false, "PDF"},
	}
	for _, c := range cas {
		if Document(c.nom, c.mime) != c.doc || Image(c.nom, c.mime) != c.img || Libelle(c.nom, c.mime) != c.lib {
			t.Errorf("%s : doc=%v img=%v lib=%q", c.nom, Document(c.nom, c.mime), Image(c.nom, c.mime), Libelle(c.nom, c.mime))
		}
	}
}

func TestExtrairePptxEtTexte(t *testing.T) {
	r, err := Extraire("pres.pptx", "", zipAvec(t, map[string]string{
		"ppt/slides/slide1.xml": `<p:sld><p:txBody><a:p><a:r><a:t>Budget 2026</a:t></a:r></a:p></p:txBody></p:sld>`,
	}))
	if err != nil || !strings.Contains(r.Texte, "Budget 2026") || r.Methode != "pptx" {
		t.Fatalf("pptx : %+v %v", r, err)
	}
	r, err = Extraire("notes.txt", "text/plain", []byte("Rendez-vous lundi\r\n\r\n\r\n\r\nà 9 h"))
	if err != nil || r.Texte != "Rendez-vous lundi\n\nà 9 h" || r.Methode != "texte" {
		t.Fatalf("txt : %+v %v", r, err)
	}
}

// Exports Excel « Texte Unicode » (UTF-16 avec BOM) et CSV Windows-1252 : le
// texte doit être lisible, sans octet nul et avec ses accents.
func TestExtraireTexteEncodages(t *testing.T) {
	utf16 := []byte{0xFF, 0xFE}
	for _, r := range "Relevé de compte" {
		utf16 = append(utf16, byte(r), byte(r>>8))
	}
	r, err := Extraire("export.txt", "text/plain", utf16)
	if err != nil || r.Texte != "Relevé de compte" {
		t.Fatalf("utf-16 : %q %v", r.Texte, err)
	}
	cp1252 := []byte("Relev\xe9 de compte;Montant\r\n12;3,50 \x80")
	r, err = Extraire("export.csv", "text/csv", cp1252)
	if err != nil || !strings.HasPrefix(r.Texte, "Relevé de compte;Montant") || !strings.HasSuffix(r.Texte, "3,50 €") {
		t.Fatalf("cp1252 : %q %v", r.Texte, err)
	}
}

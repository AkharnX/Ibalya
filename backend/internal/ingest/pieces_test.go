package ingest

import "testing"

func TestModePour(t *testing.T) {
	m := func(s string) *string { return &s }
	cas := []struct {
		statut  string
		motif   *string
		attendu ModePieces
	}{
		{"pending", nil, ModeComplete},
		{"analyzed", nil, ModeComplete},
		{"excluded", m("expediteur_automatique"), ModeTexteSeul},
		{"excluded", m("newsletter"), ModeTexteSeul},
		{"excluded", m("publicitaire"), ModeTexteSeul},
		{"excluded", m("domaine_exclu"), ModeTexteSeul},
		{"excluded", m("fil_exclu"), ModeTexteSeul},
		{"excluded", m("regle_apprise"), ModeTexteSeul},
		{"excluded", m("corps_vide"), ModeTexteSeul},
		{"excluded", m("motif_inconnu_futur"), ModeTexteSeul},
		{"excluded", nil, ModeTexteSeul},
		{"excluded", m("securite_suspect"), ModeAucune},
		{"excluded", m("securite_nouveau_motif"), ModeAucune},
		{"excluded", m("categorie_sensible_sante"), ModeAucune},
		{"excluded", m("categorie_sensible_rh"), ModeAucune},
		{"excluded", m("categorie_sensible_juridique"), ModeAucune},
	}
	for _, c := range cas {
		if got := ModePour(c.statut, c.motif); got != c.attendu {
			t.Errorf("%s/%v : %s, attendu %s", c.statut, c.motif, got, c.attendu)
		}
	}
}

func TestPlanPiece(t *testing.T) {
	cas := []struct {
		mode         ModePieces
		nom, mime    string
		traiter, ocr bool
	}{
		{ModeAucune, "facture.pdf", "application/pdf", false, false},
		{ModeTexteSeul, "facture.pdf", "application/pdf", true, false},
		{ModeTexteSeul, "logo.png", "image/png", false, false},
		{ModeTexteSeul, "archive.zip", "application/zip", false, false},
		{ModeComplete, "facture.pdf", "application/pdf", true, true},
		{ModeComplete, "recu.jpg", "image/jpeg", true, true},
		{ModeComplete, "archive.zip", "application/zip", false, false},
	}
	for _, c := range cas {
		tr, ocr := planPiece(c.mode, c.nom, c.mime)
		if tr != c.traiter || ocr != c.ocr {
			t.Errorf("%s %s : traiter=%v ocr=%v", c.mode, c.nom, tr, ocr)
		}
	}
}

// Un document sans texte est quand même une ligne (retrouvable par son nom) ;
// une image sans texte OCR n'en est pas une.
func TestPlanPieceDocumentSansTexte(t *testing.T) {
	if !garderSansTexte("scan.pdf", "application/pdf") {
		t.Fatal("un document sans texte doit être enregistré (nom et type)")
	}
	if garderSansTexte("photo.jpg", "image/jpeg") {
		t.Fatal("une image sans texte OCR ne doit pas être enregistrée")
	}
}

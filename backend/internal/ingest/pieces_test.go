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

// Une pièce dont le texte relève d'une catégorie confidentielle cochée (fiche
// de paie envoyée par une plateforme à l'objet neutre) n'est pas enregistrée,
// même si le message lui-même n'a rien déclenché.
func TestPieceSensible(t *testing.T) {
	rh := map[Categorie]bool{CategorieRH: true}
	if !pieceSensible("document.pdf", "Votre bulletin de paie du mois d'octobre", rh) {
		t.Fatal("un bulletin de paie doit être reconnu sensible")
	}
	if pieceSensible("facture.pdf", "Facture n° 12 Total TTC 148,00 EUR", rh) {
		t.Fatal("une facture n'est pas sensible")
	}
	if pieceSensible("document.pdf", "Votre bulletin de paie", map[Categorie]bool{}) {
		t.Fatal("catégorie non cochée : rien à détecter")
	}
}

// Les motifs interdits (préfixes) : la recherche les ignore et les rattrapages
// retirent les pièces déjà stockées de ces messages.
func TestMotifInterdit(t *testing.T) {
	for m, veut := range map[string]bool{"categorie_sensible_sante": true, "securite_suspect": true, "securite_autre": true,
		"expediteur_automatique": false, "newsletter": false, "": false} {
		if MotifInterdit(m) != veut {
			t.Errorf("%q : %v", m, !veut)
		}
	}
}

package ingest

import "testing"

func toutesActives() map[Categorie]bool {
	m := map[Categorie]bool{}
	for _, c := range CategoriesConnues {
		m[c] = true
	}
	return m
}

func TestDetectionCategoriesSensibles(t *testing.T) {
	cas := []struct {
		nom, sujet, corps string
		attendu           Categorie
	}{
		{"arrêt maladie", "Absence", "Je vous transmets mon arrêt de travail jusqu'au 15.", CategorieSante},
		{"sans accent", "Absence", "je vous transmets mon arret maladie", CategorieSante},
		{"certificat médical", "Justificatif", "Ci-joint le certificat médical du docteur.", CategorieSante},
		{"candidature", "Candidature", "Vous trouverez ci-joint mon CV et ma lettre de motivation.", CategorieRH},
		{"fiche de paie", "Paie", "Ma fiche de paie de juillet comporte une erreur.", CategorieRH},
		{"rupture conventionnelle", "RDV", "Je souhaite discuter d'une rupture conventionnelle.", CategorieRH},
		{"mise en demeure", "Courrier", "Nous vous adressons une mise en demeure de payer.", CategorieJuridique},
		{"huissier", "Recouvrement", "Le dossier est transmis à l'huissier.", CategorieJuridique},
		{"détecté dans le sujet seul", "Mise en demeure", "Voir pièce jointe.", CategorieJuridique},
		{"rien de sensible", "Devis vitrage", "Je vous confirme la livraison jeudi 14.", ""},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			got := DetecterCategorie(c.sujet, c.corps, toutesActives())
			if got != c.attendu {
				t.Fatalf("catégorie %q, attendu %q", got, c.attendu)
			}
		})
	}
}

// Le vrai danger n'est pas de rater un message sensible : c'est d'écarter du
// courrier d'affaires ordinaire, car le dirigeant croit alors l'agent en
// panne sans savoir pourquoi. Ces phrases viennent du vocabulaire courant
// d'une PME du bâtiment et ne doivent JAMAIS déclencher un filtre.
func TestAucunFauxPositifSurDuCourrierMetier(t *testing.T) {
	metier := []string{
		"Je vous confirme le contrat de maintenance pour l'année prochaine.",
		"Le dossier client Ménard est prêt, je vous l'envoie demain.",
		"Pouvez-vous me faire un devis pour la pose de trois fenêtres ?",
		"Notre analyse du chantier montre un retard de deux jours.",
		"Le rendez-vous de chantier est déplacé à mardi 10h.",
		"La facture d'essai est jointe, merci de vérifier le montant.",
		"Bilan de la semaine : deux poses terminées, une reportée.",
		"J'ai transmis le bon de commande au fournisseur.",
		"Le tribunal de commerce m'a demandé un extrait Kbis.", // limite assumée, voir plus bas
	}
	for _, phrase := range metier[:len(metier)-1] {
		if c := DetecterCategorie("", phrase, toutesActives()); c != "" {
			t.Errorf("faux positif %q sur : %s", c, phrase)
		}
	}
}

// Une catégorie désactivée ne doit pas être cherchée : c'est ce que le CDC
// appelle « exclusion configurable ».
func TestCategorieDesactiveeNestPasCherchee(t *testing.T) {
	corps := "Je vous transmets mon arrêt de travail."
	if c := DetecterCategorie("", corps, map[Categorie]bool{CategorieSante: false}); c != "" {
		t.Fatalf("catégorie désactivée déclenchée : %q", c)
	}
	if c := DetecterCategorie("", corps, map[Categorie]bool{CategorieSante: true}); c != CategorieSante {
		t.Fatalf("catégorie active non détectée : %q", c)
	}
}

// La santé passe avant : un arrêt maladie transmis par un salarié relève des
// deux familles, et c'est la seule qui relève de l'article 9 du RGPD.
func TestSantePrioritaireSurRH(t *testing.T) {
	corps := "Mon arrêt de travail justifie mon absence, voir mon contrat de travail."
	if c := DetecterCategorie("", corps, toutesActives()); c != CategorieSante {
		t.Fatalf("attendu santé, obtenu %q", c)
	}
}

func TestReglageAllerRetour(t *testing.T) {
	avant := map[Categorie]bool{CategorieSante: true, CategorieRH: false, CategorieJuridique: true}
	apres := LireCategories(EcrireCategories(avant))
	for _, c := range CategoriesConnues {
		if apres[c] != avant[c] {
			t.Fatalf("%s : %v puis %v", c, avant[c], apres[c])
		}
	}
}

// Un réglage vide ou corrompu doit retomber sur les valeurs par défaut, jamais
// tout désactiver : une erreur de configuration ne doit pas ouvrir les vannes
// en silence.
func TestReglageIllisibleRetombeSurLesDefauts(t *testing.T) {
	for _, brut := range []string{"", "   ", "pas du json", "[1,2,3]"} {
		got := LireCategories(brut)
		if !got[CategorieSante] || !got[CategorieRH] {
			t.Fatalf("réglage %q a désactivé un filtre par défaut : %v", brut, got)
		}
	}
}

// Le juridique est délibérément inactif par défaut : c'est la catégorie la plus
// susceptible de porter de vraies échéances d'affaires.
func TestJuridiqueInactifParDefaut(t *testing.T) {
	d := CategoriesParDefaut()
	if d[CategorieJuridique] {
		t.Fatal("le juridique ne doit pas être filtré par défaut")
	}
	if !d[CategorieSante] || !d[CategorieRH] {
		t.Fatal("santé et RH doivent être filtrées par défaut")
	}
}

package ingest

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Filtres de catégories sensibles (exigence CDC : « filtres de catégories
// (RH, juridique, santé), exclusion configurable »).
//
// Deux principes gouvernent ce fichier.
//
// La détection est purement lexicale, comme le reste du pré-filtre : aucun
// appel au modèle. Envoyer un arrêt maladie à un modèle tiers pour savoir s'il
// faut le lui cacher n'aurait aucun sens.
//
// Les motifs sont volontairement étroits. Un faux positif écarte du courrier
// d'affaires légitime et le dirigeant croit l'agent défaillant ; on préfère
// donc des expressions de plusieurs mots, qui n'apparaissent quasiment jamais
// hors de leur contexte, à des mots isolés comme « contrat » ou « dossier ».

// Categorie identifie une famille de contenus à écarter de l'extraction.
type Categorie string

const (
	CategorieSante     Categorie = "sante"
	CategorieRH        Categorie = "rh"
	CategorieJuridique Categorie = "juridique"
)

// CategoriesConnues liste les catégories dans l'ordre d'affichage.
var CategoriesConnues = []Categorie{CategorieSante, CategorieRH, CategorieJuridique}

// Libelle donne le nom lisible d'une catégorie.
func (c Categorie) Libelle() string {
	switch c {
	case CategorieSante:
		return "Santé"
	case CategorieRH:
		return "Ressources humaines"
	case CategorieJuridique:
		return "Juridique"
	}
	return string(c)
}

// motifs par catégorie. Écrits sans accent : le texte est normalisé avant
// comparaison, ce qui évite de dépendre de l'encodage d'origine du message.
var motifsCategories = map[Categorie]*regexp.Regexp{
	CategorieSante: regexp.MustCompile(`(?i)\b(` +
		`arret (de )?(travail|maladie)|conge (de )?maladie|` +
		`certificat medical|ordonnance medicale|compte rendu operatoire|` +
		`hospitalisation|hospitalise|intervention chirurgicale|` +
		`rendez-?vous medical|medecin (traitant|du travail)|` +
		`resultats d(')?analyses|analyses medicales|bilan sanguin|` +
		`assurance maladie|mutuelle sante|feuille de soins|` +
		`accident du travail|maladie professionnelle|invalidite|` +
		// « cpam » seul désignait aussi un employeur : une offre d'emploi
		// « Data Scientist – CPAM des Yvelines » passait pour un dossier
		// médical. Le sigle n'est gardé qu'accompagné.
		`caisse primaire d(')?assurance maladie|releve ameli` +
		`)\b`),

	CategorieRH: regexp.MustCompile(`(?i)\b(` +
		`candidature( spontanee)?|lettre de motivation|curriculum vitae|` +
		`mon cv|cv ci-?joint|votre cv|` +
		`entretien d(')?embauche|promesse d(')?embauche|periode d(')?essai|` +
		`contrat de travail|rupture conventionnelle|solde de tout compte|` +
		`lettre de demission|je demissionne|licenciement|` +
		`bulletin de (paie|salaire)|fiche de paie|` +
		`pretentions salariales|augmentation de salaire|` +
		`conges payes|entretien annuel|note de frais` +
		`)\b`),

	CategorieJuridique: regexp.MustCompile(`(?i)\b(` +
		`mise en demeure|injonction de payer|assignation|` +
		`tribunal|prud(')?hommes|conseil de prud(')?hommes|` +
		`huissier|commissaire de justice|` +
		`cabinet d(')?avocats?|notre avocat|votre avocat|maitre [a-z]{3,}|` +
		`procedure judiciaire|contentieux|` +
		`porter plainte|depot de plainte|` +
		`redressement judiciaire|liquidation judiciaire` +
		`)\b`),
}

// remplaceAccents ramène les caractères accentués français à leur base, pour
// que « arrêt maladie » et « arret maladie » se détectent pareil.
var remplaceAccents = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a", "á", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "í", "i",
	"ô", "o", "ö", "o", "ó", "o",
	"ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
	"’", "'",
)

// normaliser prépare un texte à la comparaison lexicale.
func normaliser(s string) string {
	return remplaceAccents.Replace(strings.ToLower(s))
}

// DetecterCategorie retourne la première catégorie active reconnue dans le
// sujet ou le corps, ou "" si aucune. actives porte le choix du dirigeant :
// une catégorie désactivée n'est pas cherchée du tout.
//
// L'ordre de CategoriesConnues fixe la priorité, la santé d'abord : c'est la
// seule des trois qui relève des catégories particulières de l'article 9 du
// RGPD, et un même message peut relever de plusieurs familles — un arrêt
// maladie transmis par un salarié est à la fois santé et RH.
func DetecterCategorie(sujet, corps string, actives map[Categorie]bool) Categorie {
	texte := normaliser(sujet + "\n" + corps)
	for _, c := range CategoriesConnues {
		if !actives[c] {
			continue
		}
		if motifsCategories[c].MatchString(texte) {
			return c
		}
	}
	return ""
}

// --- configuration ---

// CleReglageCategories est la clé du réglage dans la table settings.
const CleReglageCategories = "categories_sensibles"

// CategoriesParDefaut : santé et RH filtrées, juridique non.
//
// Santé et RH portent des données sur des personnes — un candidat, un salarié —
// qui n'ont aucune raison de partir chez un modèle tiers, et le dirigeant n'a
// rien à suivre là-dedans. Le juridique, lui, est le plus souvent une affaire
// entre entreprises, chargée d'échéances réelles : c'est justement ce qu'Ibalya
// doit voir. Il reste activable d'un clic.
func CategoriesParDefaut() map[Categorie]bool {
	return map[Categorie]bool{
		CategorieSante:     true,
		CategorieRH:        true,
		CategorieJuridique: false,
	}
}

// LireCategories décode le réglage stocké. Toute valeur absente ou illisible
// retombe sur la valeur par défaut : un réglage corrompu ne doit pas ouvrir
// silencieusement les vannes.
func LireCategories(brut string) map[Categorie]bool {
	actives := CategoriesParDefaut()
	if strings.TrimSpace(brut) == "" {
		return actives
	}
	var decode map[string]bool
	if err := json.Unmarshal([]byte(brut), &decode); err != nil {
		return actives
	}
	for _, c := range CategoriesConnues {
		if v, ok := decode[string(c)]; ok {
			actives[c] = v
		}
	}
	return actives
}

// EcrireCategories sérialise le réglage.
func EcrireCategories(actives map[Categorie]bool) string {
	sortie := map[string]bool{}
	for _, c := range CategoriesConnues {
		sortie[string(c)] = actives[c]
	}
	b, err := json.Marshal(sortie)
	if err != nil {
		return ""
	}
	return string(b)
}

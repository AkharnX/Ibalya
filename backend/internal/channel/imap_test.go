package channel

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func connecteur() *IMAP {
	return NewIMAP(IMAPConfig{Hote: "imap.exemple.fr", Utilisateur: "dirigeant@pme.fr"})
}

func brut(entetes, corps string) []byte {
	return []byte(strings.ReplaceAll(entetes, "\n", "\r\n") + "\r\n\r\n" + corps)
}

// IMAP ne connaît pas la notion de fil : on la reconstruit depuis References et
// In-Reply-To. C'est le vrai risque du connecteur.
func TestReconstructionDuFil(t *testing.T) {
	i := connecteur()
	cas := []struct {
		nom, entetes, filAttendu string
	}{
		{
			"message racine : son propre fil",
			"From: client@ex.fr\nTo: dirigeant@pme.fr\nSubject: Devis pergola\nMessage-ID: <a@ex.fr>",
			"a@ex.fr",
		},
		{
			"réponse : le fil est la racine de References",
			"From: dirigeant@pme.fr\nTo: client@ex.fr\nSubject: Re: Devis pergola\nMessage-ID: <b@pme.fr>\nReferences: <a@ex.fr>\nIn-Reply-To: <a@ex.fr>",
			"a@ex.fr",
		},
		{
			"réponse de réponse : toujours la racine, pas le parent",
			"From: client@ex.fr\nTo: dirigeant@pme.fr\nSubject: Re: Devis pergola\nMessage-ID: <c@ex.fr>\nReferences: <a@ex.fr> <b@pme.fr>\nIn-Reply-To: <b@pme.fr>",
			"a@ex.fr",
		},
		{
			"In-Reply-To seul, sans References",
			"From: client@ex.fr\nTo: dirigeant@pme.fr\nSubject: Re: Devis\nMessage-ID: <d@ex.fr>\nIn-Reply-To: <a@ex.fr>",
			"a@ex.fr",
		},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			m, err := i.convertirBrut(brut(c.entetes, "Bonjour."), time.Now())
			if err != nil {
				t.Fatalf("conversion : %v", err)
			}
			if m.ThreadExternalID != c.filAttendu {
				t.Fatalf("fil %q, attendu %q", m.ThreadExternalID, c.filAttendu)
			}
		})
	}
}

// Le sens détermine toute la suite : qui a promis, et donc quelle action
// suggérer. Une erreur ici fausse le moteur entier.
func TestSensDuMessage(t *testing.T) {
	i := connecteur()
	sortant, _ := i.convertirBrut(brut(
		"From: Dirigeant <DIRIGEANT@PME.FR>\nTo: client@ex.fr\nSubject: Devis\nMessage-ID: <x@pme.fr>", "."), time.Now())
	if !sortant.Outbound {
		t.Error("un message envoyé par le compte connecté est sortant, casse comprise")
	}
	entrant, _ := i.convertirBrut(brut(
		"From: client@ex.fr\nTo: dirigeant@pme.fr\nSubject: Devis\nMessage-ID: <y@ex.fr>", "."), time.Now())
	if entrant.Outbound {
		t.Error("un message reçu n'est pas sortant")
	}
}

// Le pré-filtre EF-11 s'appuie sur cet en-tête pour écarter les infolettres
// avant tout appel au modèle.
func TestEnteteDeDesinscription(t *testing.T) {
	i := connecteur()
	m, _ := i.convertirBrut(brut(
		"From: news@ex.fr\nTo: dirigeant@pme.fr\nSubject: Lettre\nMessage-ID: <n@ex.fr>\nList-Unsubscribe: <https://ex.fr/u>", "."), time.Now())
	if !m.ListUnsubscribe {
		t.Error("l'en-tête de désinscription doit être détecté")
	}
}

// Les hébergeurs français servent encore beaucoup d'ISO-8859-1 et d'objets
// encodés : un objet illisible fausse l'extraction.
func TestObjetEncode(t *testing.T) {
	i := connecteur()
	m, err := i.convertirBrut(brut(
		"From: client@ex.fr\nTo: dirigeant@pme.fr\nSubject: =?UTF-8?B?RGV2aXMgcG91ciBsYSBww6lyZ29sYQ==?=\nMessage-ID: <e@ex.fr>", "."), time.Now())
	if err != nil {
		t.Fatalf("conversion : %v", err)
	}
	if m.Subject != "Devis pour la pérgola" {
		t.Fatalf("objet %q, attendu \"Devis pour la pérgola\"", m.Subject)
	}
}

// text/plain est privilégié ; à défaut on dégrade le HTML plutôt que de
// laisser un corps vide, qui empêcherait toute extraction.
func TestCorpsMultipart(t *testing.T) {
	i := connecteur()
	msg := "From: client@ex.fr\r\nTo: dirigeant@pme.fr\r\nSubject: Devis\r\n" +
		"Message-ID: <f@ex.fr>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=\"sep\"\r\n\r\n" +
		"--sep\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nVersion texte.\r\n" +
		"--sep\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n<p>Version HTML.</p>\r\n--sep--\r\n"
	m, err := i.convertirBrut([]byte(msg), time.Now())
	if err != nil {
		t.Fatalf("conversion : %v", err)
	}
	if !strings.Contains(m.Body, "Version texte") {
		t.Fatalf("le texte doit primer sur le HTML, obtenu %q", m.Body)
	}
}

func TestHTMLSeulEstDegrade(t *testing.T) {
	i := connecteur()
	msg := "From: client@ex.fr\r\nTo: dirigeant@pme.fr\r\nSubject: Devis\r\n" +
		"Message-ID: <g@ex.fr>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n\r\n<p>Bonjour <b>Marc</b></p>\r\n"
	m, _ := i.convertirBrut([]byte(msg), time.Now())
	if !strings.Contains(m.Body, "Bonjour") || strings.Contains(m.Body, "<b>") {
		t.Fatalf("le HTML doit être dégradé en texte, obtenu %q", m.Body)
	}
}

// Un message sans identifiant ne peut pas être dédoublonné : on le refuse
// plutôt que de le réanalyser à chaque cycle.
func TestMessageSansIdentifiantRefuse(t *testing.T) {
	i := connecteur()
	if _, err := i.convertirBrut(brut("From: a@b.fr\nTo: c@d.fr\nSubject: x", "."), time.Now()); err == nil {
		t.Fatal("un message sans Message-ID doit être refusé")
	}
}

func TestDestinatairesCollectes(t *testing.T) {
	i := connecteur()
	m, _ := i.convertirBrut(brut(
		"From: client@ex.fr\nTo: dirigeant@pme.fr, associe@pme.fr\nCc: compta@pme.fr\nSubject: x\nMessage-ID: <h@ex.fr>", "."), time.Now())
	if len(m.Recipients) != 3 {
		t.Fatalf("attendu 3 destinataires (To et Cc), obtenu %v", m.Recipients)
	}
}

// Le commutateur doit être indiscernable du canal qu'il porte, sans quoi le
// moteur et l'ingestion devraient savoir qu'un remplacement est possible.
func TestCommutateurDelegue(t *testing.T) {
	var _ Reader = (*Commutateur)(nil)

	g := &Gmail{}
	c := NewCommutateur(g)
	if c.Name() != g.Name() {
		t.Fatalf("le commutateur annonce %q au lieu de %q", c.Name(), g.Name())
	}
	if c.LienWeb("a@b.fr", "42") != g.LienWeb("a@b.fr", "42") {
		t.Fatal("le lien web doit être celui du canal actif")
	}

	i := NewIMAP(IMAPConfig{Hote: "imap.ex.fr", Utilisateur: "a@b.fr"})
	c.Remplacer(i)
	if c.Name() != "imap" {
		t.Fatalf("après remplacement, attendu imap, obtenu %q", c.Name())
	}
	if c.LienWeb("a@b.fr", "42") != "" {
		t.Fatal("IMAP n'a pas de lien web : le commutateur doit refléter ce fait")
	}
}

// Une pièce jointe nommée est rapportée comme en Gmail (nom, type, octets) ;
// une image inline sans nom (signature) n'en est pas une.
func TestPieceJointeIMAP(t *testing.T) {
	i := connecteur()
	pdf := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 faux contenu"))
	msg := "From: noreply@qonto.com\r\nTo: dirigeant@pme.fr\r\nSubject: Facture\r\nMessage-ID: <pj@ex.fr>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"mx\"\r\n\r\n" +
		"--mx\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nVotre facture est jointe.\r\n" +
		"--mx\r\nContent-Type: image/png\r\nContent-Disposition: inline\r\nContent-Transfer-Encoding: base64\r\n\r\naGVsbG8=\r\n" +
		"--mx\r\nContent-Type: application/pdf; name=\"Facture-2026-10.pdf\"\r\nContent-Disposition: attachment; filename=\"Facture-2026-10.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\n" + pdf + "\r\n--mx--\r\n"
	m, err := i.convertirBrut([]byte(msg), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Body, "Votre facture est jointe") {
		t.Fatalf("corps : %q", m.Body)
	}
	if len(m.Attachments) != 1 {
		t.Fatalf("1 pièce attendue (l'image inline sans nom n'en est pas une), %d", len(m.Attachments))
	}
	pj := m.Attachments[0]
	if pj.Nom != "Facture-2026-10.pdf" || !strings.HasPrefix(pj.Type, "application/pdf") || string(pj.Donnees) != "%PDF-1.4 faux contenu" {
		t.Fatalf("%+v", pj)
	}
}

// Une partie inline qui porte un nom de fichier est une pièce, comme en Gmail
// (Filename non vide).
func TestPieceJointeIMAPInlineNommee(t *testing.T) {
	i := connecteur()
	msg := "From: a@ex.fr\r\nTo: dirigeant@pme.fr\r\nSubject: Devis\r\nMessage-ID: <in@ex.fr>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"mx\"\r\n\r\n" +
		"--mx\r\nContent-Type: text/plain\r\n\r\nCi-joint.\r\n" +
		"--mx\r\nContent-Type: application/pdf; name=\"devis.pdf\"\r\nContent-Disposition: inline; filename=\"devis.pdf\"\r\n\r\n%PDF-1.4 devis\r\n--mx--\r\n"
	m, err := i.convertirBrut([]byte(msg), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].Nom != "devis.pdf" || string(m.Attachments[0].Donnees) != "%PDF-1.4 devis" {
		t.Fatalf("%+v", m.Attachments)
	}
}

// Au-delà de TailleMaxPJ, la pièce est ignorée mais le message est converti.
func TestPieceJointeIMAPTropGrosse(t *testing.T) {
	i := connecteur()
	gros := strings.Repeat("x", TailleMaxPJ+1)
	msg := "From: a@ex.fr\r\nTo: dirigeant@pme.fr\r\nSubject: Gros\r\nMessage-ID: <g@ex.fr>\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=\"mx\"\r\n\r\n" +
		"--mx\r\nContent-Type: text/plain\r\n\r\nCorps intact.\r\n" +
		"--mx\r\nContent-Type: application/pdf; name=\"gros.pdf\"\r\nContent-Disposition: attachment; filename=\"gros.pdf\"\r\nContent-Transfer-Encoding: binary\r\n\r\n" + gros + "\r\n--mx--\r\n"
	m, err := i.convertirBrut([]byte(msg), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Attachments) != 0 || !strings.Contains(m.Body, "Corps intact") {
		t.Fatalf("pièces=%d corps=%q", len(m.Attachments), m.Body)
	}
}

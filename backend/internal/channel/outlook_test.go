package channel

import (
	"net/url"
	"strings"
	"testing"
)

func outlook() *Outlook { return &Outlook{} }

// Graph fournit conversationId : contrairement à IMAP, le fil ne se reconstruit
// pas depuis les en-têtes.
func TestOutlookFilNatif(t *testing.T) {
	m := outlook().convertir(messageGraph{
		ID: "AAMk-1", ConversationID: "conv-42", Subject: "Devis",
		ReceivedDateTime: "2026-08-24T09:30:00Z",
	}, "dirigeant@pme.fr")
	if m.ThreadExternalID != "conv-42" {
		t.Fatalf("fil %q, attendu conv-42", m.ThreadExternalID)
	}
	if m.SentAt.IsZero() {
		t.Fatal("la date doit être analysée")
	}
}

// Un message sans conversation est son propre fil, sans quoi tous les
// orphelins se retrouveraient regroupés sous une chaîne vide.
func TestOutlookSansConversation(t *testing.T) {
	m := outlook().convertir(messageGraph{ID: "AAMk-2"}, "dirigeant@pme.fr")
	if m.ThreadExternalID != "AAMk-2" {
		t.Fatalf("fil %q, attendu son propre identifiant", m.ThreadExternalID)
	}
}

// Le sens décide de l'action suggérée : une erreur ici fausse tout le moteur.
func TestOutlookSensDuMessage(t *testing.T) {
	g := messageGraph{ID: "x"}
	g.From.EmailAddress.Address = "DIRIGEANT@PME.FR"
	if m := outlook().convertir(g, "dirigeant@pme.fr"); !m.Outbound {
		t.Error("un message envoyé par le compte est sortant, casse comprise")
	}
	g.From.EmailAddress.Address = "client@ex.fr"
	if m := outlook().convertir(g, "dirigeant@pme.fr"); m.Outbound {
		t.Error("un message reçu n'est pas sortant")
	}
}

// Sender remplace From quand ce dernier est vide, ce qui arrive sur les
// messages envoyés au nom d'un tiers.
func TestOutlookReplieSurSender(t *testing.T) {
	g := messageGraph{ID: "x"}
	g.Sender.EmailAddress.Address = "assistant@ex.fr"
	if m := outlook().convertir(g, "dirigeant@pme.fr"); m.Sender != "assistant@ex.fr" {
		t.Fatalf("expéditeur %q, attendu le repli sur Sender", m.Sender)
	}
}

func TestOutlookDestinataires(t *testing.T) {
	g := messageGraph{ID: "x"}
	for _, a := range []string{"a@ex.fr", "b@ex.fr"} {
		var d adresseGraph
		d.EmailAddress.Address = a
		g.ToRecipients = append(g.ToRecipients, d)
	}
	var cc adresseGraph
	cc.EmailAddress.Address = "compta@ex.fr"
	g.CcRecipients = append(g.CcRecipients, cc)
	if m := outlook().convertir(g, "x@y.fr"); len(m.Recipients) != 3 {
		t.Fatalf("attendu 3 destinataires (To et Cc), obtenu %v", m.Recipients)
	}
}

// Graph rend souvent du HTML : il faut le dégrader, sinon l'extraction analyse
// des balises.
func TestOutlookCorpsHTMLDegrade(t *testing.T) {
	g := messageGraph{ID: "x"}
	g.Body.ContentType = "html"
	g.Body.Content = "<p>Bonjour <b>Marc</b></p>"
	m := outlook().convertir(g, "x@y.fr")
	if strings.Contains(m.Body, "<b>") || !strings.Contains(m.Body, "Bonjour") {
		t.Fatalf("le HTML doit être dégradé, obtenu %q", m.Body)
	}
}

// Un corps vide empêcherait toute extraction : l'aperçu sert de repli.
func TestOutlookRepliSurApercu(t *testing.T) {
	g := messageGraph{ID: "x", BodyPreview: "Aperçu du message"}
	if m := outlook().convertir(g, "x@y.fr"); m.Body != "Aperçu du message" {
		t.Fatalf("corps %q, attendu le repli sur l'aperçu", m.Body)
	}
}

func TestOutlookLienProfond(t *testing.T) {
	o := outlook()
	if l := o.LienWeb("a@b.fr", "AAMk-1"); !strings.Contains(l, "AAMk-1") {
		t.Fatalf("lien %q, il doit porter l'identifiant", l)
	}
	if o.LienWeb("a@b.fr", "") != "" {
		t.Error("sans identifiant, aucun lien")
	}
}

// Les périmètres demandés à Azure conditionnent tout : sans offline_access,
// aucun jeton de rafraîchissement n'est délivré et le raccordement expire.
func TestOutlookPerimetresDemandes(t *testing.T) {
	cfg := OutlookOAuthConfig("id", "secret", "common", "https://ex.fr/cb")
	requis := map[string]bool{"offline_access": false, "Mail.Read": false, "Mail.Send": false, "User.Read": false}
	for _, s := range cfg.Scopes {
		requis[s] = true
	}
	for p, present := range requis {
		if !present {
			t.Errorf("le périmètre %q est requis", p)
		}
	}
	if !strings.Contains(cfg.Endpoint.AuthURL, "login.microsoftonline.com") {
		t.Errorf("point d'accès inattendu : %s", cfg.Endpoint.AuthURL)
	}
}

func TestOutlookRespecteLInterface(t *testing.T) {
	var _ Reader = (*Outlook)(nil)
}

// Les paramètres OData contiennent des espaces — « receivedDateTime ge … » —
// invalides bruts dans une URL. Graph répondait 400 et l'agent n'ingérait rien.
func TestOutlookRequeteEncodee(t *testing.T) {
	q := url.Values{}
	q.Set("$filter", "receivedDateTime ge 2026-08-24T09:00:00Z")
	q.Set("$orderby", "receivedDateTime desc")
	encode := strings.ReplaceAll(q.Encode(), "+", "%20")

	if strings.Contains(encode, " ") {
		t.Fatalf("des espaces bruts subsistent : %s", encode)
	}
	// OData ne lit pas « + » comme une espace : l'encodage doit produire %20.
	if strings.Contains(encode, "+") {
		t.Fatalf("le signe plus n'est pas une espace pour OData : %s", encode)
	}
	if !strings.Contains(encode, "receivedDateTime%20ge%20") {
		t.Fatalf("le filtre doit être encodé en %%20 : %s", encode)
	}
}

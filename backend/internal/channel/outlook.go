// Connecteur Outlook via Microsoft Graph.
//
// Le connecteur IMAP ne couvre pas Outlook : Microsoft a désactivé
// l'authentification simple sur Exchange Online, et l'a annoncée en fin de vie
// pour les comptes personnels. Seul OAuth 2.0 donne accès à ces boîtes.
//
// Graph est interrogé en HTTP direct plutôt qu'avec le kit officiel : quatre
// points d'accès suffisent, là où le kit pèse plusieurs dizaines de mégaoctets
// et impose son propre modèle d'objets.
//
// Deux avantages sur IMAP : Graph fournit conversationId, donc le fil sans
// reconstruction, et webLink, donc le lien profond sans le fabriquer.
package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

const graphBase = "https://graph.microsoft.com/v1.0"

type Outlook struct {
	cfg   *oauth2.Config
	store TokenStore
	email string
}

// OutlookOAuthConfig décrit l'application enregistrée dans Azure. Le locataire
// « common » accepte les comptes professionnels comme personnels ; un
// identifiant de locataire précis restreint à une seule organisation.
func OutlookOAuthConfig(clientID, clientSecret, tenant, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"offline_access", // sans quoi aucun jeton de rafraîchissement n'est délivré
			"User.Read",
			"Mail.Read",
			"Mail.Send",
		},
		Endpoint: microsoft.AzureADEndpoint(tenant),
	}
}

func NewOutlook(cfg *oauth2.Config, store TokenStore) *Outlook {
	return &Outlook{cfg: cfg, store: store}
}

func (o *Outlook) Name() string { return "outlook" }

func (o *Outlook) client(ctx context.Context) (*http.Client, error) {
	raw, _, err := o.store.GetOAuthToken(ctx, "microsoft")
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("aucune boîte Outlook raccordée")
	}
	var tok oauth2.Token
	if err := json.Unmarshal(raw, &tok); err != nil {
		return nil, fmt.Errorf("jeton Outlook illisible : %w", err)
	}
	ts := &persistingTokenSource{src: o.cfg.TokenSource(ctx, &tok), store: o.store, fournisseur: "microsoft"}
	return oauth2.NewClient(ctx, ts), nil
}

// appeler interroge Graph et décode la réponse. Les erreurs de Graph portent un
// message exploitable qu'il serait dommage de perdre derrière un code HTTP.
func (o *Outlook) appeler(ctx context.Context, methode, chemin string, corps any, sortie any) error {
	cli, err := o.client(ctx)
	if err != nil {
		return err
	}
	var lecteur *bytes.Reader
	if corps != nil {
		b, err := json.Marshal(corps)
		if err != nil {
			return err
		}
		lecteur = bytes.NewReader(b)
	} else {
		lecteur = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, methode, graphBase+chemin, lecteur)
	if err != nil {
		return err
	}
	if corps != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		// On lit le corps une fois : un décodage raté laissait « Graph a
		// répondu 400 » sans rien d'exploitable, alors que Graph explique
		// toujours ce qui ne va pas.
		brut, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		var e struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(brut, &e)
		if e.Error.Message != "" {
			return fmt.Errorf("Graph %s : %s", e.Error.Code, e.Error.Message)
		}
		if txt := strings.TrimSpace(string(brut)); txt != "" {
			return fmt.Errorf("Graph a répondu %d : %s", resp.StatusCode, txt)
		}
		return fmt.Errorf("Graph a répondu %d sans explication", resp.StatusCode)
	}
	if sortie == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(sortie)
}

func (o *Outlook) AccountEmail(ctx context.Context) (string, error) {
	if o.email != "" {
		return o.email, nil
	}
	var profil struct {
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
	}
	if err := o.appeler(ctx, http.MethodGet, "/me?$select=mail,userPrincipalName", nil, &profil); err != nil {
		return "", err
	}
	// mail est vide sur certains comptes : userPrincipalName fait alors foi.
	adresse := profil.Mail
	if adresse == "" {
		adresse = profil.UserPrincipalName
	}
	if adresse == "" {
		return "", fmt.Errorf("Graph n'a renvoyé aucune adresse pour ce compte")
	}
	o.email = strings.ToLower(adresse)
	_ = o.store.SetOAuthAccountEmail(ctx, "microsoft", o.email)
	return o.email, nil
}

// LienWeb : Graph fournit webLink par message, il n'y a rien à construire. Le
// lien générique ne sert que si l'identifiant est inconnu.
func (o *Outlook) LienWeb(compte, externalID string) string {
	if externalID == "" {
		return ""
	}
	return "https://outlook.office.com/mail/deeplink/read/" + url.PathEscape(externalID)
}

type messageGraph struct {
	ID               string `json:"id"`
	ConversationID   string `json:"conversationId"`
	Subject          string `json:"subject"`
	ReceivedDateTime string `json:"receivedDateTime"`
	BodyPreview      string `json:"bodyPreview"`
	Body             struct {
		ContentType string `json:"contentType"`
		Content     string `json:"content"`
	} `json:"body"`
	From         adresseGraph   `json:"from"`
	Sender       adresseGraph   `json:"sender"`
	ToRecipients []adresseGraph `json:"toRecipients"`
	CcRecipients []adresseGraph `json:"ccRecipients"`
}

type adresseGraph struct {
	EmailAddress struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	} `json:"emailAddress"`
}

func (o *Outlook) FetchSince(ctx context.Context, since time.Time, max int) ([]Message, error) {
	compte, err := o.AccountEmail(ctx)
	if err != nil {
		return nil, err
	}
	if max <= 0 || max > 500 {
		max = 500
	}
	// Les paramètres OData contiennent des espaces — « receivedDateTime ge … »,
	// « receivedDateTime desc » — qui sont invalides bruts dans une URL et que
	// Graph rejette par un 400. On encode donc, en %20 plutôt qu'en « + » :
	// OData ne traite pas le signe plus comme une espace.
	q := url.Values{}
	q.Set("$filter", "receivedDateTime ge "+since.UTC().Format(time.RFC3339))
	q.Set("$orderby", "receivedDateTime desc")
	q.Set("$top", strconv.Itoa(max))
	q.Set("$select", "id,conversationId,subject,receivedDateTime,bodyPreview,body,from,sender,toRecipients,ccRecipients")
	chemin := "/me/messages?" + strings.ReplaceAll(q.Encode(), "+", "%20")

	var page struct {
		Value []messageGraph `json:"value"`
	}
	if err := o.appeler(ctx, http.MethodGet, chemin, nil, &page); err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(page.Value))
	for _, g := range page.Value {
		out = append(out, o.convertir(g, compte))
	}
	return out, nil
}

func (o *Outlook) convertir(g messageGraph, compte string) Message {
	exp := g.From
	if exp.EmailAddress.Address == "" {
		exp = g.Sender
	}
	m := Message{
		ExternalID: g.ID,
		// Graph fournit le fil nativement : contrairement à IMAP, rien à
		// reconstruire depuis les en-têtes.
		ThreadExternalID: g.ConversationID,
		Subject:          g.Subject,
		Sender:           strings.ToLower(exp.EmailAddress.Address),
		SenderName:       exp.EmailAddress.Name,
		Recipients:       []string{},
	}
	if t, err := time.Parse(time.RFC3339, g.ReceivedDateTime); err == nil {
		m.SentAt = t
	}
	for _, lst := range [][]adresseGraph{g.ToRecipients, g.CcRecipients} {
		for _, a := range lst {
			if a.EmailAddress.Address != "" {
				m.Recipients = append(m.Recipients, strings.ToLower(a.EmailAddress.Address))
			}
		}
	}
	m.Outbound = strings.EqualFold(m.Sender, compte)
	if strings.EqualFold(g.Body.ContentType, "html") {
		m.Body = stripHTML(g.Body.Content)
	} else {
		m.Body = g.Body.Content
	}
	if strings.TrimSpace(m.Body) == "" {
		m.Body = g.BodyPreview
	}
	if m.ThreadExternalID == "" {
		m.ThreadExternalID = g.ID
	}
	return m
}

func (o *Outlook) Send(ctx context.Context, to, subject, body string) error {
	return o.SendFrom(ctx, "", "", to, subject, body)
}

// SendFrom : Graph n'accepte une autre adresse d'expédition que si le compte a
// la délégation correspondante. On la passe quand elle est fournie, et Graph
// refuse explicitement si le droit manque — mieux vaut une erreur claire qu'un
// envoi silencieux depuis la mauvaise adresse.
func (o *Outlook) SendFrom(ctx context.Context, from, fromNom, to, subject, body string) error {
	msg := map[string]any{
		"subject": subject,
		"body":    map[string]string{"contentType": "Text", "content": body},
		"toRecipients": []map[string]any{
			{"emailAddress": map[string]string{"address": to}},
		},
	}
	if f := strings.TrimSpace(from); f != "" {
		adresse := map[string]string{"address": f}
		if n := strings.TrimSpace(fromNom); n != "" {
			adresse["name"] = n
		}
		msg["from"] = map[string]any{"emailAddress": adresse}
	}
	return o.appeler(ctx, http.MethodPost, "/me/sendMail",
		map[string]any{"message": msg, "saveToSentItems": true}, nil)
}

package channel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmail "google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// TokenStore persiste le token OAuth (rafraîchi automatiquement).
type TokenStore interface {
	SaveOAuthToken(ctx context.Context, provider string, token []byte, email string) error
	GetOAuthToken(ctx context.Context, provider string) ([]byte, string, error)
	UpdateOAuthTokenOnly(ctx context.Context, provider string, token []byte) error
	SetOAuthAccountEmail(ctx context.Context, provider, email string) error
}

type Gmail struct {
	cfg   *oauth2.Config
	store TokenStore
}

func GmailOAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			gmail.GmailReadonlyScope,
			gmail.GmailSendScope,
		},
		Endpoint: google.Endpoint,
	}
}

func NewGmail(cfg *oauth2.Config, store TokenStore) *Gmail {
	return &Gmail{cfg: cfg, store: store}
}

func (g *Gmail) Name() string { return "gmail" }

// persistingTokenSource sauvegarde le jeton à chaque rafraîchissement. Le
// fournisseur est porté par le champ : Gmail et Outlook partagent la mécanique
// mais pas la ligne de stockage.
type persistingTokenSource struct {
	src         oauth2.TokenSource
	store       TokenStore
	fournisseur string
	last        string
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.src.Token()
	if err != nil {
		return nil, traduireErreurJeton(p.fournisseur, err)
	}
	if tok.AccessToken != p.last {
		p.last = tok.AccessToken
		b, _ := json.Marshal(tok)
		// surtout pas SaveOAuthToken : il écraserait account_email
		_ = p.store.UpdateOAuthTokenOnly(context.Background(), p.fournisseur, b)
	}
	return tok, nil
}

// traduireErreurJeton transforme un refus d'OAuth en consigne claire.
//
// « invalid_grant » signifie que le jeton de rafraîchissement est mort :
// révoqué, ou expiré. En mode Test, Google révoque ce jeton sept jours après
// le consentement — la panne se répète alors chaque semaine tant que l'app
// n'est pas vérifiée. Sans traduction, le dirigeant ne voyait qu'une URL et un
// code 400, sans savoir qu'il lui suffit de reconnecter sa boîte.
func traduireErreurJeton(fournisseur string, err error) error {
	boite := "votre boîte"
	switch fournisseur {
	case "google":
		boite = "votre compte Gmail"
	case "microsoft":
		boite = "votre compte Outlook"
	}
	if strings.Contains(err.Error(), "invalid_grant") {
		return fmt.Errorf("%s doit être reconnecté : l'autorisation a expiré ou été révoquée. "+
			"Rendez-vous dans Réglages, Connexion, pour la renouveler. (%w)", boite, err)
	}
	return err
}

func (g *Gmail) service(ctx context.Context) (*gmail.Service, error) {
	raw, _, err := g.store.GetOAuthToken(ctx, "google")
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("gmail non connecté : lancer le parcours OAuth")
	}
	var tok oauth2.Token
	if err := json.Unmarshal(raw, &tok); err != nil {
		return nil, err
	}
	ts := &persistingTokenSource{src: g.cfg.TokenSource(ctx, &tok), store: g.store, fournisseur: "google"}
	return gmail.NewService(ctx, option.WithTokenSource(ts))
}

// AccountEmail lit l'adresse dans le store scopé par tenant, jamais depuis un
// cache d'instance : un même canal Gmail sert tous les tenants (chacun lit SON
// jeton), et un email mémorisé sur la struct fuiterait d'un utilisateur à
// l'autre.
func (g *Gmail) AccountEmail(ctx context.Context) (string, error) {
	if _, email, err := g.store.GetOAuthToken(ctx, "google"); err == nil && email != "" {
		return email, nil
	}
	svc, err := g.service(ctx)
	if err != nil {
		return "", err
	}
	prof, err := svc.Users.GetProfile("me").Context(ctx).Do()
	if err != nil {
		return "", err
	}
	// Persistée dans le store (scopé tenant) : le tableau de bord et les liens
	// Gmail en ont besoin, et l'appel au profil peut échouer au callback.
	_ = g.store.SetOAuthAccountEmail(ctx, "google", prof.EmailAddress)
	return prof.EmailAddress, nil
}

// avecRepriseGmail rejoue un appel Gmail quand l'API refuse le débit.
//
// La quête d'un fil fait un appel par message ; sur une large fenêtre, le coût
// dépasse le quota par minute et Gmail répond 403 rateLimitExceeded (ou 429).
// Ce n'est pas une vraie erreur, juste « ralentis ». On attend, en doublant à
// chaque fois, avec un peu de hasard pour ne pas repartir tous en même temps.
func avecRepriseGmail[T any](ctx context.Context, appel func() (T, error)) (T, error) {
	const tentatives = 5
	attente := time.Second
	var zero T
	for i := 0; ; i++ {
		v, err := appel()
		if err == nil || !estLimiteDebitGmail(err) || i == tentatives-1 {
			return v, err
		}
		pause := attente + time.Duration(rand.Int63n(int64(attente/2+1)))
		select {
		case <-time.After(pause):
		case <-ctx.Done():
			return zero, ctx.Err()
		}
		attente *= 2
	}
}

// estLimiteDebitGmail reconnaît un refus pour cause de débit, pas une panne.
func estLimiteDebitGmail(err error) bool {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		if gerr.Code == 429 {
			return true
		}
		if gerr.Code == 403 {
			for _, e := range gerr.Errors {
				if e.Reason == "rateLimitExceeded" || e.Reason == "userRateLimitExceeded" {
					return true
				}
			}
			return strings.Contains(gerr.Message, "Quota exceeded")
		}
	}
	return false
}

func (g *Gmail) FetchSince(ctx context.Context, since time.Time, max int) ([]Message, error) {
	svc, err := g.service(ctx)
	if err != nil {
		return nil, err
	}
	self, _ := g.AccountEmail(ctx)

	query := fmt.Sprintf("after:%d -in:spam -in:trash", since.Unix())
	var out []Message
	pageToken := ""
	for len(out) < max {
		call := svc.Users.Messages.List("me").Q(query).MaxResults(100).Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := avecRepriseGmail(ctx, func() (*gmail.ListMessagesResponse, error) {
			return call.Do()
		})
		if err != nil {
			return nil, fmt.Errorf("gmail list: %w", err)
		}
		for _, ref := range resp.Messages {
			if len(out) >= max {
				break
			}
			full, err := avecRepriseGmail(ctx, func() (*gmail.Message, error) {
				return svc.Users.Messages.Get("me", ref.Id).Format("full").Context(ctx).Do()
			})
			if err != nil {
				continue
			}
			m := parseGmailMessage(full, self)
			out = append(out, m)
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return out, nil
}

func parseGmailMessage(full *gmail.Message, selfEmail string) Message {
	m := Message{
		ExternalID:       full.Id,
		ThreadExternalID: full.ThreadId,
		SentAt:           time.UnixMilli(full.InternalDate),
	}
	var fromRaw string
	for _, h := range full.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "from":
			fromRaw = h.Value
		case "to", "cc":
			for _, a := range strings.Split(h.Value, ",") {
				if addr, err := mail.ParseAddress(strings.TrimSpace(a)); err == nil {
					m.Recipients = append(m.Recipients, strings.ToLower(addr.Address))
				}
			}
		case "subject":
			m.Subject = h.Value
		case "list-unsubscribe":
			m.ListUnsubscribe = true
		}
	}
	if addr, err := mail.ParseAddress(fromRaw); err == nil {
		m.Sender = strings.ToLower(addr.Address)
		m.SenderName = addr.Name
	} else {
		m.Sender = strings.ToLower(strings.TrimSpace(fromRaw))
	}
	if selfEmail != "" && m.Sender == strings.ToLower(selfEmail) {
		m.Outbound = true
	}
	m.Body = cleanBody(extractBody(full.Payload))
	return m
}

func extractBody(p *gmail.MessagePart) string {
	if p == nil {
		return ""
	}
	if p.MimeType == "text/plain" && p.Body != nil && p.Body.Data != "" {
		if b, err := base64.URLEncoding.DecodeString(p.Body.Data); err == nil {
			return string(b)
		}
	}
	for _, part := range p.Parts {
		if s := extractBody(part); s != "" {
			return s
		}
	}
	// repli : html si aucun text/plain
	if p.MimeType == "text/html" && p.Body != nil && p.Body.Data != "" {
		if b, err := base64.URLEncoding.DecodeString(p.Body.Data); err == nil {
			return stripHTML(string(b))
		}
	}
	return ""
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	s = htmlTag.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// cleanBody retire les citations de réponses (économie de tokens LLM) et tronque.
func cleanBody(body string) string {
	var lines []string
	for _, l := range strings.Split(body, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, ">") {
			continue
		}
		if strings.HasPrefix(t, "Le ") && strings.Contains(t, "a écrit :") {
			break
		}
		if strings.HasPrefix(t, "On ") && strings.Contains(t, "wrote:") {
			break
		}
		lines = append(lines, l)
	}
	s := strings.TrimSpace(strings.Join(lines, "\n"))
	// Tronquer et non s[:4000] : une coupe à l'octet près brise un caractère
	// accentué et fait rejeter l'insertion par la base.
	return NettoyerUTF8(Tronquer(s, 4000))
}

// LienWeb : le paramètre authuser évite d'atterrir sur le mauvais compte quand
// l'utilisateur est connecté à plusieurs boîtes Google dans son navigateur.
func (g *Gmail) LienWeb(compte, externalID string) string {
	if externalID == "" {
		return ""
	}
	base := "https://mail.google.com/mail/"
	if compte != "" {
		base += "?authuser=" + url.QueryEscape(compte)
	}
	return base + "#all/" + externalID
}

func (g *Gmail) Send(ctx context.Context, to, subject, body string) error {
	return g.SendFrom(ctx, "", "", to, subject, body)
}

func (g *Gmail) SendFrom(ctx context.Context, from, fromNom, to, subject, body string) error {
	svc, err := g.service(ctx)
	if err != nil {
		return err
	}
	self, _ := g.AccountEmail(ctx)
	expediteur := strings.TrimSpace(from)
	if expediteur == "" {
		expediteur = self
	}
	if n := strings.TrimSpace(fromNom); n != "" {
		expediteur = fmt.Sprintf("%s <%s>", encodeSubject(n), expediteur)
	}
	raw := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		expediteur, to, encodeSubject(subject), body)
	msg := &gmail.Message{Raw: base64.URLEncoding.EncodeToString([]byte(raw))}
	_, err = svc.Users.Messages.Send("me", msg).Context(ctx).Do()
	return err
}

func encodeSubject(s string) string {
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
}

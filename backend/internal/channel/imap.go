// Connecteur IMAP générique.
//
// Le CDC exige que tout connecteur passe par l'interface commune (EF-10), pour
// qu'un second canal s'ajoute sans refonte. Celui-ci en couvre beaucoup d'un
// coup : Yahoo, OVH, Gandi, Orange, Free et toute boîte auto-hébergée parlent
// IMAP. Pour des PME françaises c'est décisif, la majorité n'étant ni sur
// Google Workspace ni sur Microsoft 365 mais sur la messagerie de son
// hébergeur.
//
// Deux différences assumées avec l'API Gmail :
//
//   - IMAP ne connaît pas la notion de fil. On la reconstruit depuis les
//     en-têtes References et In-Reply-To (RFC 5322), ce qui est la méthode que
//     tous les clients de messagerie emploient.
//   - IMAP n'a pas d'interface web à laquelle renvoyer : LienWeb retourne une
//     chaîne vide, et l'interface masque alors le bouton d'ouverture.
package channel

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
	gomail "github.com/emersion/go-message/mail"
)

type IMAPConfig struct {
	Hote        string // imap.example.fr
	Port        int    // 993
	Utilisateur string
	MotDePasse  string
	Dossier     string // INBOX par défaut
	// DossierEnvoyes force le nom du dossier des messages envoyés. Vide (défaut),
	// il est détecté automatiquement via l'attribut \Sent, avec repli sur les
	// noms usuels. Ne sert qu'aux serveurs exotiques qui n'annoncent pas \Sent et
	// nomment ce dossier autrement que les variantes connues.
	DossierEnvoyes string
	SMTPHote       string
	SMTPPort       int
	// TLSSansVerification accepte un certificat auto-signé. Réservé au
	// développement : config.Verifier refuse le démarrage avec cette option
	// sur une installation publique.
	TLSSansVerification bool
}

type IMAP struct {
	cfg IMAPConfig
}

func NewIMAP(cfg IMAPConfig) *IMAP {
	if cfg.Port == 0 {
		cfg.Port = 993
	}
	if cfg.Dossier == "" {
		cfg.Dossier = "INBOX"
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 587
	}
	if cfg.SMTPHote == "" {
		cfg.SMTPHote = strings.Replace(cfg.Hote, "imap.", "smtp.", 1)
	}
	return &IMAP{cfg: cfg}
}

func (i *IMAP) Name() string { return "imap" }

// AccountEmail : l'adresse est celle des identifiants, IMAP n'expose pas de
// profil comme le fait l'API Gmail.
func (i *IMAP) AccountEmail(ctx context.Context) (string, error) {
	if i.cfg.Utilisateur == "" {
		return "", fmt.Errorf("aucun identifiant IMAP configuré")
	}
	return i.cfg.Utilisateur, nil
}

// LienWeb : IMAP ne désigne aucune interface web. Retour vide, l'appelant
// n'affiche alors pas de lien plutôt que d'en fabriquer un faux.
func (i *IMAP) LienWeb(compte, externalID string) string { return "" }

func (i *IMAP) connecter() (*imapclient.Client, error) {
	// JoinHostPort et non Sprintf : une adresse IPv6 littérale contient des
	// deux-points, que "%s:%d" rendrait ambiguë (« too many colons »).
	adresse := net.JoinHostPort(i.cfg.Hote, strconv.Itoa(i.cfg.Port))
	// charset.Reader permet de décoder les corps en ISO-8859-1 et consorts,
	// encore très répandus chez les hébergeurs français.
	opts := &imapclient.Options{
		WordDecoder: &mime.WordDecoder{CharsetReader: charset.Reader},
		// Le dialer refuse les destinations internes : sans lui, le test de
		// connexion sert de scanner du réseau. Le contournement TLS n'existe
		// qu'en développement, où l'on veut justement joindre un serveur local.
		Dialer: dialerRestreint(i.cfg.TLSSansVerification, 20*time.Second),
	}
	if i.cfg.TLSSansVerification {
		opts.TLSConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 — refusé en production
	}
	c, err := imapclient.DialTLS(adresse, opts)
	if err != nil {
		return nil, fmt.Errorf("connexion à %s : %w", adresse, err)
	}
	if err := c.Login(i.cfg.Utilisateur, i.cfg.MotDePasse).Wait(); err != nil {
		c.Close()
		return nil, fmt.Errorf("authentification IMAP refusée : %w", err)
	}
	return c, nil
}

// dossiersALire liste les dossiers à parcourir : la boîte de réception et, quand
// on le trouve, le dossier des messages envoyés. Lire les Envoyés est ce qui
// permet de voir les réponses du dirigeant — ses promesses sortantes, et de quoi
// clore un engagement qu'il a déjà honoré. Sans cela, IMAP ne voyait que ce
// qu'on lui écrit, jamais ce qu'il répond.
func (i *IMAP) dossiersALire(c *imapclient.Client) []string {
	liste := []string{i.cfg.Dossier}
	if env := i.dossierEnvoyes(c); env != "" && !strings.EqualFold(env, i.cfg.Dossier) {
		liste = append(liste, env)
	}
	return liste
}

// dossierEnvoyes localise le dossier « Envoyés », dont le nom varie d'un
// hébergeur à l'autre (Sent, Envoyés, INBOX.Sent…). On interroge d'abord
// l'attribut standard \Sent (RFC 6154, SPECIAL-USE), que la plupart des serveurs
// annoncent ; à défaut on teste les noms usuels. Un nom forcé en configuration
// l'emporte. Chaîne vide si rien n'est trouvé : on se contente alors d'INBOX.
func (i *IMAP) dossierEnvoyes(c *imapclient.Client) string {
	if forced := strings.TrimSpace(i.cfg.DossierEnvoyes); forced != "" {
		return forced
	}
	// 1) Attribut \Sent — la voie fiable, indépendante de la langue.
	if data, err := c.List("", "*", &imap.ListOptions{ReturnSpecialUse: true}).Collect(); err == nil {
		for _, d := range data {
			for _, attr := range d.Attrs {
				if attr == imap.MailboxAttrSent {
					return d.Mailbox
				}
			}
		}
	}
	// 2) Repli : noms courants, en LIST exacte pour ne retenir qu'un dossier
	// réellement présent sur le serveur.
	for _, nom := range []string{
		"Sent", "Sent Items", "Sent Messages",
		"Envoyés", "Éléments envoyés", "Messages envoyés",
		"INBOX.Sent", "INBOX.Envoyés",
		"[Gmail]/Messages envoyés", "[Gmail]/Sent Mail",
	} {
		if data, err := c.List("", nom, nil).Collect(); err == nil && len(data) > 0 {
			return data[0].Mailbox
		}
	}
	return ""
}

func (i *IMAP) FetchSince(ctx context.Context, since time.Time, max int) ([]Message, error) {
	c, err := i.connecter()
	if err != nil {
		return nil, err
	}
	// defer c.Logout().Wait() évaluerait c.Logout() immédiatement et ne
	// différerait que l'attente : la déconnexion partait juste après
	// l'authentification et le SELECT recevait une connexion fermée.
	defer func() { _ = c.Logout().Wait() }()

	var out []Message
	for idx, dossier := range i.dossiersALire(c) {
		budget := 0 // 0 = sans plafond, cohérent avec les tests « max > 0 »
		if max > 0 {
			if budget = max - len(out); budget <= 0 {
				break
			}
		}
		msgs, err := i.fetchDossier(c, dossier, since, budget)
		if err != nil {
			if idx == 0 {
				return nil, err // la boîte de réception est obligatoire
			}
			continue // le dossier Envoyés est optionnel : ne pas casser le cycle
		}
		out = append(out, msgs...)
	}
	return out, nil
}

// fetchDossier lit un dossier : sélection, recherche depuis la date, puis
// récupération du corps. Isolée pour servir chaque dossier (INBOX, Envoyés).
func (i *IMAP) fetchDossier(c *imapclient.Client, dossier string, since time.Time, max int) ([]Message, error) {
	if _, err := c.Select(dossier, &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return nil, fmt.Errorf("dossier %q : %w", dossier, err)
	}

	// SINCE ne compare que la date, l'heure est ignorée par le protocole : on
	// recule d'un jour pour ne rien perdre, le dédoublonnage en base écarte
	// ensuite ce qui a déjà été lu.
	res, err := c.Search(&imap.SearchCriteria{Since: since.AddDate(0, 0, -1)}, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("recherche : %w", err)
	}
	nums := res.AllSeqNums()
	if len(nums) == 0 {
		return []Message{}, nil
	}
	// Les plus récents d'abord : au-delà du plafond, mieux vaut perdre les
	// vieux messages que les nouveaux.
	if len(nums) > max && max > 0 {
		nums = nums[len(nums)-max:]
	}

	var set imap.SeqSet
	set.AddNum(nums...)
	cmd := c.Fetch(set, &imap.FetchOptions{
		Envelope:     true,
		InternalDate: true,
		BodySection:  []*imap.FetchItemBodySection{{}},
	})
	defer cmd.Close()

	out := make([]Message, 0, len(nums))
	for {
		msg := cmd.Next()
		if msg == nil {
			break
		}
		buf, err := msg.Collect()
		if err != nil {
			continue // un message illisible ne doit pas interrompre le cycle
		}
		m, err := i.convertir(buf)
		if err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, cmd.Close()
}

func (i *IMAP) convertir(buf *imapclient.FetchMessageBuffer) (Message, error) {
	var brut []byte
	for _, v := range buf.BodySection {
		brut = v.Bytes
		break
	}
	return i.convertirBrut(brut, buf.InternalDate)
}

// FetchMetaSince ne récupère que l'enveloppe (expéditeur, destinataires, date,
// objet) — aucun corps n'est téléchargé. Sert au scan de contexte sur tout
// l'historique.
func (i *IMAP) FetchMetaSince(ctx context.Context, since time.Time, max int) ([]Message, error) {
	c, err := i.connecter()
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Logout().Wait() }()

	var out []Message
	for idx, dossier := range i.dossiersALire(c) {
		budget := 0
		if max > 0 {
			if budget = max - len(out); budget <= 0 {
				break
			}
		}
		msgs, err := i.fetchMetaDossier(c, dossier, since, budget)
		if err != nil {
			if idx == 0 {
				return nil, err
			}
			continue
		}
		out = append(out, msgs...)
	}
	return out, nil
}

// fetchMetaDossier lit un dossier en enveloppe seule (aucun corps rapatrié),
// pour le scan de contexte sur l'historique.
func (i *IMAP) fetchMetaDossier(c *imapclient.Client, dossier string, since time.Time, max int) ([]Message, error) {
	if _, err := c.Select(dossier, &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return nil, fmt.Errorf("dossier %q : %w", dossier, err)
	}
	res, err := c.Search(&imap.SearchCriteria{Since: since.AddDate(0, 0, -1)}, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("recherche : %w", err)
	}
	nums := res.AllSeqNums()
	if len(nums) == 0 {
		return []Message{}, nil
	}
	if len(nums) > max && max > 0 {
		nums = nums[len(nums)-max:]
	}
	var set imap.SeqSet
	set.AddNum(nums...)
	// Envelope seul : pas de BodySection, donc pas de corps rapatrié.
	cmd := c.Fetch(set, &imap.FetchOptions{Envelope: true, InternalDate: true})
	defer cmd.Close()

	out := make([]Message, 0, len(nums))
	for {
		msg := cmd.Next()
		if msg == nil {
			break
		}
		buf, err := msg.Collect()
		if err != nil || buf.Envelope == nil {
			continue
		}
		out = append(out, i.depuisEnveloppe(buf.Envelope, buf.InternalDate))
	}
	return out, cmd.Close()
}

// depuisEnveloppe construit un message SANS corps à partir de l'enveloppe IMAP.
func (i *IMAP) depuisEnveloppe(env *imap.Envelope, recu time.Time) Message {
	adr := func(a imap.Address) string {
		if a.Mailbox == "" || a.Host == "" {
			return ""
		}
		return strings.ToLower(a.Mailbox + "@" + a.Host)
	}
	m := Message{ExternalID: env.MessageID, Subject: decoder(env.Subject), SentAt: recu, Recipients: []string{}}
	if !env.Date.IsZero() {
		m.SentAt = env.Date
	}
	if len(env.From) > 0 {
		m.Sender = adr(env.From[0])
		m.SenderName = env.From[0].Name
	}
	for _, a := range append(append([]imap.Address{}, env.To...), env.Cc...) {
		if e := adr(a); e != "" {
			m.Recipients = append(m.Recipients, e)
		}
	}
	m.ThreadExternalID = strings.Trim(m.ExternalID, "<>")
	if len(env.InReplyTo) > 0 {
		if r := strings.Trim(strings.TrimSpace(env.InReplyTo[0]), "<>"); r != "" {
			m.ThreadExternalID = r
		}
	}
	m.Outbound = strings.EqualFold(m.Sender, i.cfg.Utilisateur)
	return m
}

// convertirBrut transforme un message RFC 5322 en forme normalisée. Séparée de
// la lecture réseau pour être testable sur des messages bruts : c'est ici que
// se joue le vrai risque du connecteur — l'analyse MIME et la reconstruction
// des fils, qu'IMAP ne fournit pas.
func (i *IMAP) convertirBrut(brut []byte, recu time.Time) (Message, error) {
	if len(brut) == 0 {
		return Message{}, fmt.Errorf("corps absent")
	}
	lecteur, err := gomail.CreateReader(strings.NewReader(string(brut)))
	if err != nil {
		return Message{}, err
	}
	en := lecteur.Header

	m := Message{
		ExternalID: premierNonVide(entete(en, "Message-Id"), entete(en, "Message-ID")),
		Subject:    decoder(entete(en, "Subject")),
		SentAt:     recu,
	}
	if d, err := en.Date(); err == nil && !d.IsZero() {
		m.SentAt = d
	}
	if exp, err := en.AddressList("From"); err == nil && len(exp) > 0 {
		m.Sender = strings.ToLower(exp[0].Address)
		m.SenderName = exp[0].Name
	}
	for _, champ := range []string{"To", "Cc"} {
		if lst, err := en.AddressList(champ); err == nil {
			for _, a := range lst {
				m.Recipients = append(m.Recipients, strings.ToLower(a.Address))
			}
		}
	}
	if m.Recipients == nil {
		m.Recipients = []string{}
	}
	m.ListUnsubscribe = entete(en, "List-Unsubscribe") != ""
	m.Outbound = strings.EqualFold(m.Sender, i.cfg.Utilisateur)
	m.ThreadExternalID = filDe(en, m.ExternalID)
	m.Body = corpsTexte(lecteur)
	if m.ExternalID == "" {
		return Message{}, fmt.Errorf("message sans identifiant")
	}
	return m, nil
}

// filDe reconstruit l'identifiant de fil. IMAP n'en fournit aucun : la racine
// d'une conversation est le premier Message-Id de la chaîne References, et à
// défaut celui auquel ce message répond. Un message isolé est son propre fil.
func filDe(en gomail.Header, propreID string) string {
	if refs := strings.Fields(entete(en, "References")); len(refs) > 0 {
		return strings.Trim(refs[0], "<>")
	}
	if r := strings.TrimSpace(entete(en, "In-Reply-To")); r != "" {
		return strings.Trim(strings.Fields(r)[0], "<>")
	}
	return propreID
}

// corpsTexte privilégie text/plain ; à défaut on dégrade le HTML, un corps
// vide empêcherait toute extraction.
func corpsTexte(lecteur *gomail.Reader) string {
	var texte, html string
	for {
		partie, err := lecteur.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		switch h := partie.Header.(type) {
		case *gomail.InlineHeader:
			ct, _, _ := h.ContentType()
			b, _ := io.ReadAll(partie.Body)
			if strings.HasPrefix(ct, "text/plain") && texte == "" {
				texte = string(b)
			} else if strings.HasPrefix(ct, "text/html") && html == "" {
				html = string(b)
			}
		}
	}
	if strings.TrimSpace(texte) != "" {
		return texte
	}
	return stripHTML(html)
}

func (i *IMAP) Send(ctx context.Context, to, subject, body string) error {
	return i.SendFrom(ctx, "", "", to, subject, body)
}

// SendFrom passe par SMTP : IMAP est un protocole de lecture, il n'envoie rien.
// L'authentification réutilise les identifiants de la boîte, ce qui est la
// configuration habituelle chez les hébergeurs.
func (i *IMAP) SendFrom(ctx context.Context, from, fromNom, to, subject, body string) error {
	if i.cfg.SMTPHote == "" {
		return fmt.Errorf("aucun serveur SMTP configuré")
	}
	expediteur := strings.TrimSpace(from)
	if expediteur == "" {
		expediteur = i.cfg.Utilisateur
	}
	entete := expediteur
	if n := strings.TrimSpace(fromNom); n != "" {
		entete = fmt.Sprintf("%s <%s>", encodeSubject(n), expediteur)
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		entete, to, encodeSubject(subject), time.Now().Format(time.RFC1123Z), body)

	adresse := net.JoinHostPort(i.cfg.SMTPHote, strconv.Itoa(i.cfg.SMTPPort))
	auth := smtp.PlainAuth("", i.cfg.Utilisateur, i.cfg.MotDePasse, i.cfg.SMTPHote)
	// Le port 465 attend du TLS dès la connexion (SSL implicite) : smtp.SendMail
	// dialogue en clair puis tente STARTTLS, ce qui échoue sur 465. On ouvre
	// donc la connexion en TLS d'emblée. Le 587 (et autres) reste sur SendMail,
	// qui gère STARTTLS.
	if i.cfg.SMTPPort == 465 {
		if err := i.envoyerTLSImplicite(adresse, auth, expediteur, to, []byte(msg)); err != nil {
			return fmt.Errorf("envoi SMTP via %s : %w", adresse, err)
		}
		return nil
	}
	if err := smtp.SendMail(adresse, auth, expediteur, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("envoi SMTP via %s : %w", adresse, err)
	}
	return nil
}

// envoyerTLSImplicite envoie via une connexion TLS ouverte dès le départ (465).
func (i *IMAP) envoyerTLSImplicite(adresse string, auth smtp.Auth, from, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", adresse, &tls.Config{
		ServerName:         i.cfg.SMTPHote,
		InsecureSkipVerify: i.cfg.TLSSansVerification, // #nosec G402 — refusé en production
	})
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, i.cfg.SMTPHote)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Auth(auth); err != nil {
		return err
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func entete(en gomail.Header, champ string) string {
	v, _ := en.Text(champ)
	if v == "" {
		v = en.Get(champ)
	}
	return strings.Trim(strings.TrimSpace(v), "<>")
}

func decoder(s string) string {
	d := mime.WordDecoder{CharsetReader: charset.Reader}
	if out, err := d.DecodeHeader(s); err == nil {
		return out
	}
	return s
}

func premierNonVide(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

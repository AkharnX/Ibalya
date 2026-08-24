// Envoi des messages de service.
//
// Un digest n'est pas un message adressé à un client : il vient d'Ibalya, pas
// du dirigeant. Le faire partir de sa boîte personnelle est doublement gênant.
// Il apparaît dans ses messages envoyés, et l'adresse affichée n'est pas celle
// qu'on prétend — Gmail ignore un expéditeur qui n'est pas un alias vérifié du
// compte, si bien que « digest@ibalya.com » était remplacé en silence par
// l'adresse personnelle.
//
// Les messages aux clients continuent de partir de la boîte du dirigeant :
// c'est lui qui relance, pas le service.
package courrier

import (
	"context"
	"fmt"
	"mime"
	"net/smtp"
	"strings"
	"time"
)

type Config struct {
	Hote  string // smtp-relay.brevo.com
	Port  int
	Login string
	Cle   string
	// De est l'adresse affichée. Elle doit être validée chez le fournisseur,
	// sans quoi le message est refusé ou classé en indésirable.
	De    string
	DeNom string
}

type Service struct{ cfg Config }

// Nouveau retourne nil si la configuration est incomplète : l'appelant se
// rabat alors sur le canal du dirigeant plutôt que d'échouer.
func Nouveau(c Config) *Service {
	if strings.TrimSpace(c.Hote) == "" || strings.TrimSpace(c.Cle) == "" ||
		strings.TrimSpace(c.De) == "" {
		return nil
	}
	if c.Port == 0 {
		c.Port = 587
	}
	if strings.TrimSpace(c.Login) == "" {
		c.Login = c.De
	}
	if strings.TrimSpace(c.DeNom) == "" {
		c.DeNom = "Ibalya"
	}
	return &Service{cfg: c}
}

func (s *Service) Configure() bool { return s != nil }

func (s *Service) Expediteur() string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("%s <%s>", s.cfg.DeNom, s.cfg.De)
}

func (s *Service) Envoyer(ctx context.Context, a, sujet, corps string) error {
	if s == nil {
		return fmt.Errorf("aucun expéditeur de service configuré")
	}
	entetes := []string{
		"From: " + fmt.Sprintf("%s <%s>", encoder(s.cfg.DeNom), s.cfg.De),
		"To: " + a,
		"Subject: " + encoder(sujet),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	msg := strings.Join(entetes, "\r\n") + "\r\n\r\n" + corps

	adresse := fmt.Sprintf("%s:%d", s.cfg.Hote, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Login, s.cfg.Cle, s.cfg.Hote)
	if err := smtp.SendMail(adresse, auth, s.cfg.De, []string{a}, []byte(msg)); err != nil {
		return fmt.Errorf("envoi via %s : %w", adresse, err)
	}
	return nil
}

// encoder gère les accents dans les en-têtes, qui ne tolèrent que l'ASCII.
func encoder(s string) string {
	if s == "" {
		return s
	}
	return mime.QEncoding.Encode("UTF-8", s)
}

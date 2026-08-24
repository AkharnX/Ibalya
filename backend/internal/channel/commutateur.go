package channel

import (
	"context"
	"sync"
	"time"
)

// Commutateur permet de changer de canal sans redémarrer le service.
//
// Le connecteur était construit une fois au démarrage et injecté dans le moteur
// et l'ingestion : raccorder une autre boîte supposait d'éditer le fichier
// d'environnement sur le serveur et de relancer. En s'interposant, il satisfait
// lui-même l'interface (EF-10) et délègue au canal actif : ni le moteur ni
// l'ingestion ne savent qu'il y a eu remplacement.
type Commutateur struct {
	mu    sync.RWMutex
	actif Reader
}

func NewCommutateur(initial Reader) *Commutateur {
	return &Commutateur{actif: initial}
}

// Remplacer bascule le canal. Les cycles en cours terminent avec l'ancien : le
// verrou n'est pris qu'à l'échange de la référence, jamais pendant un appel.
func (c *Commutateur) Remplacer(r Reader) {
	c.mu.Lock()
	c.actif = r
	c.mu.Unlock()
}

func (c *Commutateur) Actif() Reader {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actif
}

func (c *Commutateur) Name() string { return c.Actif().Name() }

func (c *Commutateur) AccountEmail(ctx context.Context) (string, error) {
	return c.Actif().AccountEmail(ctx)
}

func (c *Commutateur) FetchSince(ctx context.Context, since time.Time, max int) ([]Message, error) {
	return c.Actif().FetchSince(ctx, since, max)
}

func (c *Commutateur) Send(ctx context.Context, to, subject, body string) error {
	return c.Actif().Send(ctx, to, subject, body)
}

func (c *Commutateur) SendFrom(ctx context.Context, from, fromNom, to, subject, body string) error {
	return c.Actif().SendFrom(ctx, from, fromNom, to, subject, body)
}

func (c *Commutateur) LienWeb(compte, externalID string) string {
	return c.Actif().LienWeb(compte, externalID)
}

package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

// Chiffrement des secrets stockés en base.
//
// Le mot de passe d'une boîte IMAP donne accès à toute la correspondance du
// dirigeant. Une sauvegarde qui fuite, ou une base lue par un tiers, ne doit
// pas le livrer. La clé vient de l'environnement : elle n'est donc jamais dans
// la même sauvegarde que les données qu'elle protège.
type Coffre struct {
	aead cipher.AEAD
}

// NouveauCoffre dérive une clé AES-256 de la phrase fournie. Retourne nil si
// aucune phrase n'est configurée : l'appelant refuse alors d'enregistrer un
// secret plutôt que de l'écrire en clair.
func NouveauCoffre(phrase string) (*Coffre, error) {
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return nil, nil
	}
	if len(phrase) < 32 {
		return nil, fmt.Errorf("la clé de chiffrement doit faire au moins 32 caractères")
	}
	somme := sha256.Sum256([]byte(phrase))
	bloc, err := aes.NewCipher(somme[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(bloc)
	if err != nil {
		return nil, err
	}
	return &Coffre{aead: aead}, nil
}

// Chiffrer retourne une chaîne transportable, nonce compris. GCM authentifie :
// une valeur altérée en base est rejetée au déchiffrement plutôt que d'être
// interprétée.
func (c *Coffre) Chiffrer(clair string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("aucune clé de chiffrement configurée")
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	scelle := c.aead.Seal(nonce, nonce, []byte(clair), nil)
	return base64.RawURLEncoding.EncodeToString(scelle), nil
}

func (c *Coffre) Dechiffrer(chiffre string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("aucune clé de chiffrement configurée")
	}
	if strings.TrimSpace(chiffre) == "" {
		return "", nil
	}
	brut, err := base64.RawURLEncoding.DecodeString(chiffre)
	if err != nil {
		return "", fmt.Errorf("secret illisible : %w", err)
	}
	n := c.aead.NonceSize()
	if len(brut) < n {
		return "", fmt.Errorf("secret tronqué")
	}
	clair, err := c.aead.Open(nil, brut[:n], brut[n:], nil)
	if err != nil {
		// Se produit si la clé a changé, ou si la valeur a été altérée.
		return "", fmt.Errorf("déchiffrement impossible : clé incorrecte ou donnée altérée")
	}
	return string(clair), nil
}

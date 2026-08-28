package channel

import (
	"strings"
	"unicode/utf8"
)

// Tronquer coupe une chaîne à une longueur maximale en octets, sans jamais
// briser un caractère.
//
// Une coupe à l'octet près sectionne un caractère accentué en deux : il reste
// le premier octet d'une séquence UTF-8 incomplète, que PostgreSQL rejette —
// « invalid byte sequence for encoding UTF8: 0xc3 ». Le message entier est
// alors perdu en silence, et l'agent ne voit jamais cette conversation.
//
// La limite reste exprimée en octets : c'est ce que la base contraint, et un
// texte accentué compte deux octets par caractère.
func Tronquer(s string, maxOctets int) string {
	if maxOctets <= 0 || len(s) <= maxOctets {
		return s
	}
	coupe := s[:maxOctets]
	// On recule jusqu'au dernier caractère entier.
	for len(coupe) > 0 && !utf8.ValidString(coupe) {
		coupe = coupe[:len(coupe)-1]
	}
	return coupe
}

// NettoyerUTF8 retire les séquences invalides d'une chaîne venue de
// l'extérieur. Un message mal encodé à la source ne doit pas faire échouer son
// insertion : mieux vaut un caractère perdu qu'une conversation entière.
func NettoyerUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "")
}

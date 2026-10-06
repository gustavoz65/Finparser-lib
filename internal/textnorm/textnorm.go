// Package textnorm normaliza texto para comparação: sem acento, sem caixa e
// com espaços colapsados (inclusive o não-quebrável).
package textnorm

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Fold remove acentos ("Descrição" → "Descricao").
func Fold(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return out
}

// Spaces troca qualquer sequência de espaços (inclusive  ) por um
// espaço só e apara as pontas.
func Spaces(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == ' '
	}), " ")
}

// Key devolve a forma canônica de s para comparação: minúsculas, sem
// acento e com espaços normalizados.
func Key(s string) string {
	return Spaces(strings.ToLower(Fold(s)))
}

// Package textnorm normaliza texto para comparação: sem acento, sem caixa e
// com espaços colapsados (inclusive o não-quebrável).
package textnorm

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// latin1 mapeia U+00C0..U+00FF para a letra base; 0 = sem acento a remover.
var latin1 = [64]byte{
	'A', 'A', 'A', 'A', 'A', 'A', 0, 'C', 'E', 'E', 'E', 'E', 'I', 'I', 'I', 'I',
	0, 'N', 'O', 'O', 'O', 'O', 'O', 0, 0, 'U', 'U', 'U', 'U', 'Y', 0, 0,
	'a', 'a', 'a', 'a', 'a', 'a', 0, 'c', 'e', 'e', 'e', 'e', 'i', 'i', 'i', 'i',
	0, 'n', 'o', 'o', 'o', 'o', 'o', 0, 0, 'u', 'u', 'u', 'u', 'y', 0, 'y',
}

// Fold remove acentos ("Descrição" → "Descricao").
func Fold(s string) string {
	// Caminho rápido: quase todo token de extrato é ASCII puro, e o resto
	// quase sempre só tem acentos do Latin-1 (tabela direta, sem NFD).
	i := 0
	for i < len(s) && s[i] < 0x80 {
		i++
	}
	if i == len(s) {
		return s
	}
	out := make([]byte, i, len(s))
	copy(out, s[:i])
	for _, r := range s[i:] {
		switch {
		case r < 0x80:
			out = append(out, byte(r))
		case r >= 0xC0 && r <= 0xFF && latin1[r-0xC0] != 0:
			out = append(out, latin1[r-0xC0])
		case r >= 0xA0 && r <= 0xFF:
			out = utf8.AppendRune(out, r)
		default:
			return foldSlow(s)
		}
	}
	return string(out)
}

func foldSlow(s string) string {
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

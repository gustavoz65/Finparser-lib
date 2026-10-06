package extract

import (
	"sort"
	"unicode"
)

// GarbledPages devolve as páginas em que mais de 30% dos runes estão fora de
// letras latinas, dígitos, pontuação e símbolos comuns: sinal de fonte com
// CMap ausente, que gera texto trocado.
func GarbledPages(toks []Token) []int {
	total := map[int]int{}
	bad := map[int]int{}
	for _, t := range toks {
		for _, r := range t.Text {
			if unicode.IsSpace(r) {
				continue
			}
			total[t.Page]++
			if !readable(r) {
				bad[t.Page]++
			}
		}
	}
	var pages []int
	for p, n := range total {
		if bad[p]*10 > n*3 {
			pages = append(pages, p)
		}
	}
	sort.Ints(pages)
	return pages
}

func readable(r rune) bool {
	switch {
	case r < 128:
		return unicode.IsPrint(r)
	case unicode.Is(unicode.Latin, r):
		return true
	case r >= 0xA0 && r <= 0xFF: // símbolos Latin-1: º ª § °
		return true
	}
	switch r {
	case '•', '–', '—', '‘', '’', '“', '”', '€', '…', '·', '−':
		return true
	}
	return false
}

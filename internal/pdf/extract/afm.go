package extract

import (
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

// helvetica guarda as larguras AFM da Helvetica para ASCII 32..126, em
// milésimos do tamanho da fonte. Serve de aproximação para qualquer fonte
// proporcional sem /Widths; a precisão basta para separar palavras.
var helvetica = [95]int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278, // ' '..'/'
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, // 0..9
	278, 278, 584, 584, 584, 556, 1015, // ':'..'@'
	667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, // A..M
	722, 778, 667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, // N..Z
	278, 278, 278, 469, 556, 333, // '['..'`'
	556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, // a..m
	556, 556, 556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, // n..z
	334, 260, 334, 584, // '{'..'~'
}

// textWidth estima a largura de s em milésimos do tamanho da fonte.
func textWidth(font, s string) float64 {
	mono := strings.Contains(strings.ToLower(font), "courier")
	total := 0.0
	for _, r := range s {
		if mono {
			total += 600
			continue
		}
		total += float64(runeWidth(r))
	}
	return total
}

func runeWidth(r rune) int {
	if r >= 32 && r <= 126 {
		return helvetica[r-32]
	}
	if r == ' ' {
		return helvetica[0]
	}
	// Letra acentuada: largura da letra base.
	if base := []rune(textnorm.Fold(string(r))); len(base) == 1 && base[0] >= 32 && base[0] <= 126 {
		return helvetica[base[0]-32]
	}
	return 556
}

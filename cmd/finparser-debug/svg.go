package main

import (
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/pdf"
	"github.com/gustavoz65/finparser-lib/internal/pdf/extract"
	"github.com/gustavoz65/finparser-lib/internal/pdf/spatial"
)

// writeSVGs desenha, por página: a caixa de cada token (azul), cada linha
// detectada (verde, com o texto reconstruído) e as fronteiras de coluna
// (vermelho). O eixo Y é invertido para ficar como na tela.
func writeSVGs(dir, base string, toks []extract.Token, tolerance float64) ([]string, error) {
	lines := spatial.Build(toks, tolerance)
	bands := pdf.Columns(lines)

	pages := map[int][]extract.Token{}
	maxPage := 0
	for _, t := range toks {
		pages[t.Page] = append(pages[t.Page], t)
		maxPage = max(maxPage, t.Page)
	}

	var files []string
	for p := 1; p <= maxPage; p++ {
		pt := pages[p]
		if len(pt) == 0 {
			continue
		}
		w, h := 595.0, 842.0 // A4; cresce se o conteúdo passar
		for _, t := range pt {
			w = math.Max(w, t.X+t.W+10)
			h = math.Max(h, t.Y+t.FontSize+10)
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="Helvetica, Arial, sans-serif">`+"\n", w, h, w, h)
		fmt.Fprintf(&sb, `<rect width="100%%" height="100%%" fill="white"/>`+"\n")
		for _, b := range bands {
			fmt.Fprintf(&sb, `<rect x="%.2f" y="0" width="%.2f" height="%.0f" fill="red" fill-opacity="0.05" stroke="red" stroke-width="0.5" stroke-dasharray="4 2"/>`+"\n", b.X0, b.X1-b.X0, h)
		}
		for _, t := range pt {
			fs := t.FontSize
			if fs <= 0 {
				fs = 10
			}
			fmt.Fprintf(&sb, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="none" stroke="#1f6feb" stroke-width="0.3"/>`+"\n", t.X, h-t.Y-fs*0.8, math.Max(t.W, 0.5), fs)
		}
		for _, l := range lines {
			if l.Page != p {
				continue
			}
			y := h - l.Y
			fmt.Fprintf(&sb, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="green" stroke-width="0.6"/>`+"\n", l.X0(), y+1, l.X1(), y+1)
			fmt.Fprintf(&sb, `<text x="%.2f" y="%.2f" font-size="%.1f" fill="#555">%s</text>`+"\n", l.X0(), y, l.FontSize, html.EscapeString(l.Text()))
		}
		sb.WriteString("</svg>\n")

		name := filepath.Join(dir, fmt.Sprintf("%s-p%d.svg", base, p))
		if err := os.WriteFile(name, []byte(sb.String()), 0o644); err != nil {
			return files, err
		}
		files = append(files, name)
	}
	return files, nil
}

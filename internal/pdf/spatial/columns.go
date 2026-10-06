package spatial

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

// Cell é um grupo de palavras vizinhas numa linha.
type Cell struct {
	X0, X1 float64
	Text   string
}

// Cells agrupa as palavras de uma linha em células: um espaço maior que
// gap × fonte separa células. Usado para ler a linha de cabeçalho, onde
// "Valor (R$)" é uma célula de duas palavras.
func Cells(l Line, gap float64) []Cell {
	var out []Cell
	for i, w := range l.Words {
		if i > 0 && w.X0-l.Words[i-1].X1 <= gap*fontOr(l.FontSize) {
			c := &out[len(out)-1]
			c.X1 = w.X1
			c.Text += " " + w.Text
			continue
		}
		out = append(out, Cell{X0: w.X0, X1: w.X1, Text: w.Text})
	}
	return out
}

// Band é a faixa horizontal de uma coluna.
type Band struct{ X0, X1 float64 }

// Assign distribui as palavras de l nas colunas (passo 4). Cada palavra
// vai para a faixa com maior sobreposição horizontal; sem sobreposição,
// para a mais próxima. Valores monetários são alinhados à direita, então
// para tokens numéricos a distância usa a borda direita (X1).
func Assign(l Line, bands []Band) []string {
	parts := make([][]string, len(bands))
	for _, w := range l.Words {
		best, bestOverlap := -1, 0.0
		for i, b := range bands {
			if ov := math.Min(w.X1, b.X1) - math.Max(w.X0, b.X0); ov > bestOverlap {
				best, bestOverlap = i, ov
			}
		}
		if best < 0 {
			numeric := isNumeric(w.Text)
			bestDist := math.Inf(1)
			for i, b := range bands {
				var d float64
				if numeric {
					d = math.Abs(w.X1 - b.X1)
				} else {
					d = intervalDist(w.X0, w.X1, b.X0, b.X1)
				}
				if d < bestDist {
					best, bestDist = i, d
				}
			}
		}
		if best >= 0 {
			parts[best] = append(parts[best], w.Text)
		}
	}
	out := make([]string, len(bands))
	for i, p := range parts {
		out[i] = strings.Join(p, " ")
	}
	return out
}

func intervalDist(a0, a1, b0, b1 float64) float64 {
	switch {
	case a1 < b0:
		return b0 - a1
	case b1 < a0:
		return a0 - b1
	}
	return 0
}

func isNumeric(s string) bool {
	digits := 0
	for _, r := range s {
		switch {
		case unicode.IsDigit(r):
			digits++
		case strings.ContainsRune("R$.,-+()DC ", r):
		default:
			return false
		}
	}
	return digits > 0
}

// Gutters encontra colunas pelo perfil de projeção (fallback do passo 4):
// projeta as faixas [X0, X1] de todas as linhas no eixo X; as "calhas" de
// espaço vazio que atravessam a maioria das linhas (≥ 90%) e têm pelo menos
// minGap × fonte de largura são fronteiras entre colunas.
func Gutters(lines []Line, minGap float64) []Band {
	if len(lines) == 0 {
		return nil
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	var fonts []float64
	for _, l := range lines {
		lo = math.Min(lo, l.X0())
		hi = math.Max(hi, l.X1())
		fonts = append(fonts, l.FontSize)
	}
	n := int(math.Ceil(hi-lo)) + 1
	cover := make([]int, n)
	for _, l := range lines {
		seen := make([]bool, n)
		for _, w := range l.Words {
			for x := int(w.X0 - lo); x <= int(math.Ceil(w.X1-lo)) && x < n; x++ {
				if x >= 0 && !seen[x] {
					seen[x] = true
					cover[x]++
				}
			}
		}
	}
	limit := len(lines) / 10
	width := minGap * fontOr(median(fonts))

	var bands []Band
	start := 0
	for x := 0; x < n; {
		if cover[x] > limit {
			x++
			continue
		}
		g := x
		for g < n && cover[g] <= limit {
			g++
		}
		if float64(g-x) >= width && x > start {
			bands = append(bands, Band{X0: lo + float64(start), X1: lo + float64(x)})
			start = g
		}
		x = g
	}
	if start < n {
		bands = append(bands, Band{X0: lo + float64(start), X1: hi})
	}
	return bands
}

// noiseKey compara linhas ignorando dígitos: "1 de 2" e "2 de 2" são o
// mesmo rodapé.
func noiseKey(l Line) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return '#'
		}
		return r
	}, textnorm.Key(l.Text()))
}

var pageNumber = regexp.MustCompile(`(^|\s)(pag|pagina|page|folha)\.?\s*\d+|^\d+\s*(de|/|of)\s*\d+$`)

// RemoveNoise descarta cabeçalho e rodapé de página (passo 3): linhas que se
// repetem (ignorando dígitos) entre as 3 primeiras ou 3 últimas de várias
// páginas ("Página 2 de 5", nome do banco, CPF mascarado). keep protege
// linhas que devem ficar mesmo repetidas (cabeçalho da tabela, linhas com
// valor monetário).
func RemoveNoise(lines []Line, keep func(Line) bool) []Line {
	byPage := map[int][]int{}
	var pages []int
	for i, l := range lines {
		if _, ok := byPage[l.Page]; !ok {
			pages = append(pages, l.Page)
		}
		byPage[l.Page] = append(byPage[l.Page], i)
	}
	sort.Ints(pages)

	edge := map[int]bool{}
	count := map[string]map[int]bool{}
	for _, p := range pages {
		idx := byPage[p]
		for k, i := range idx {
			if k < 3 || k >= len(idx)-3 {
				edge[i] = true
				key := noiseKey(lines[i])
				if count[key] == nil {
					count[key] = map[int]bool{}
				}
				count[key][p] = true
			}
		}
	}

	out := lines[:0:0]
	for i, l := range lines {
		noise := edge[i] && (pageNumber.MatchString(textnorm.Key(l.Text())) || len(pages) > 1 && len(count[noiseKey(l)]) > 1)
		if noise && (keep == nil || !keep(l)) {
			continue
		}
		out = append(out, l)
	}
	return out
}

package spatial

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/gustavoz65/finparser-lib/internal/pdf/matrix"
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

// Occupancy monta a matriz de ocupação das linhas: uma linha da matriz por
// linha visual, uma coluna por ponto do eixo X. Devolve nil sem linhas ou
// com coordenadas absurdas (PDF malicioso).
func Occupancy(lines []Line) *matrix.Grid {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, l := range lines {
		if len(l.Words) > 0 {
			lo = math.Min(lo, l.X0())
			hi = math.Max(hi, l.X1())
		}
	}
	g := matrix.New(len(lines), lo, hi, 1)
	if g == nil {
		return nil
	}
	for r, l := range lines {
		for _, w := range l.Words {
			g.Fill(r, w.X0, w.X1)
		}
	}
	return g
}

// Gutters encontra colunas pelo perfil de projeção (fallback do passo 4):
// projeta as faixas [X0, X1] de todas as linhas no eixo X; as "calhas" de
// espaço vazio que atravessam a maioria das linhas (≥ 90%) e têm pelo menos
// minGap × fonte de largura são fronteiras entre colunas.
func Gutters(lines []Line, minGap float64) []Band {
	g := Occupancy(lines)
	if g == nil {
		return nil
	}
	fonts := make([]float64, len(lines))
	for i, l := range lines {
		fonts[i] = l.FontSize
	}
	profile := g.Projection(0, g.Rows())
	width := int(math.Ceil(minGap * fontOr(median(fonts))))
	hi := g.X(0)
	for _, l := range lines {
		if len(l.Words) > 0 {
			hi = math.Max(hi, l.X1())
		}
	}

	var bands []Band
	start := 0
	for _, run := range matrix.Gutters(profile, int32(len(lines)/10), width) {
		if run.From > start { // calha na margem esquerda não separa nada
			bands = append(bands, Band{X0: g.X(start), X1: g.X(run.From)})
			start = run.To
		}
	}
	if start < g.Cols() {
		bands = append(bands, Band{X0: g.X(start), X1: hi})
	}
	return bands
}

// Boundaries transforma as células do cabeçalho em colunas que cobrem a
// largura inteira da tabela (passo 4, âncora pelo cabeçalho). A fronteira
// entre dois cabeçalhos vizinhos fica no vale da projeção das linhas do
// corpo: o trecho menos ocupado entre o fim de um e o começo do outro.
//
// A célula do cabeçalho sozinha é estreita demais: a descrição passa do
// fim de "Lançamento" e um valor alto começa antes de "Valor (R$)". O vale
// acompanha o dado de verdade, alinhado à esquerda, à direita ou centrado.
func Boundaries(header []Band, body []Line) []Band {
	if len(header) == 0 {
		return nil
	}
	out := make([]Band, len(header))
	lo, hi := header[0].X0, header[len(header)-1].X1
	for _, l := range body {
		if len(l.Words) > 0 {
			lo = math.Min(lo, l.X0())
			hi = math.Max(hi, l.X1())
		}
	}

	var profile []int32
	g := Occupancy(body)
	if g != nil {
		profile = g.Projection(0, g.Rows())
	}
	left := lo - 1
	for i := range header {
		right := hi + 1
		if i+1 < len(header) {
			a, b := header[i].X1, header[i+1].X0
			right = (a + b) / 2
			if g != nil && b > a {
				// Colunas inteiras dentro do vão entre os dois cabeçalhos.
				from, to := g.Col(a)+1, g.Col(b)
				if c, ok := matrix.Valley(profile, from, to); ok {
					right = g.X(0) + c
				}
			}
		}
		out[i] = Band{X0: left, X1: right}
		left = right
	}
	return out
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

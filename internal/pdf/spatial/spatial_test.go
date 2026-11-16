package spatial

import (
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/pdf/extract"
)

// glyphs gera um token por caractere, como o backend faz, com largura fixa
// de 5pt (fonte 10).
func glyphs(page int, x, y float64, s string) []extract.Token {
	var out []extract.Token
	for _, r := range s {
		out = append(out, extract.Token{Page: page, X: x, Y: y, W: 5, FontSize: 10, Text: string(r)})
		x += 5
	}
	return out
}

func texts(lines []Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.Text()
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWordsAndLines(t *testing.T) {
	var toks []extract.Token
	// Ordem do stream propositalmente embaralhada; Y cresce para cima.
	toks = append(toks, glyphs(1, 300, 700, "-45,00")...)
	toks = append(toks, glyphs(1, 50, 720, "Data")...)
	toks = append(toks, glyphs(1, 50, 700, "01/03")...)
	toks = append(toks, glyphs(1, 120, 700.8, "Padaria")...) // levemente desalinhado
	toks = append(toks, glyphs(1, 160, 700.8, "Sol")...)     // gap de 5pt = 0,5 fonte → palavra nova
	toks = append(toks, glyphs(2, 50, 720, "02/03")...)

	lines := Build(toks, 0.5)
	got := texts(lines)
	want := []string{"Data", "01/03 Padaria Sol -45,00", "02/03"}
	if !eq(got, want) {
		t.Fatalf("linhas = %q, want %q", got, want)
	}
	w := lines[1].Words[3]
	if w.X0 != 300 || w.X1 != 330 {
		t.Errorf("palavra -45,00 em [%v,%v]", w.X0, w.X1)
	}
	if lines[2].Page != 2 {
		t.Errorf("página = %d", lines[2].Page)
	}
}

func TestSplitSpaces(t *testing.T) {
	toks := []extract.Token{{Page: 1, X: 0, Y: 10, W: 60, FontSize: 10, Text: "PIX  ENVIADO"}}
	lines := Build(toks, 0)
	if got := lines[0].Text(); got != "PIX ENVIADO" {
		t.Errorf("texto = %q", got)
	}
	if n := len(lines[0].Words); n != 2 {
		t.Errorf("palavras = %d", n)
	}
}

func TestCellsAndAssign(t *testing.T) {
	var toks []extract.Token
	toks = append(toks, glyphs(1, 50, 700, "Data")...)
	toks = append(toks, glyphs(1, 120, 700, "Descrição")...)
	toks = append(toks, glyphs(1, 300, 700, "Valor")...)
	toks = append(toks, glyphs(1, 330, 700, "(R$)")...)
	header := Build(toks, 0)[0]
	cells := Cells(header, 1.0)
	if len(cells) != 3 || cells[2].Text != "Valor (R$)" {
		t.Fatalf("cells = %+v", cells)
	}

	bands := []Band{{cells[0].X0, cells[0].X1}, {cells[1].X0, cells[1].X1}, {cells[2].X0, cells[2].X1}}
	var row []extract.Token
	row = append(row, glyphs(1, 50, 680, "01/03")...)
	row = append(row, glyphs(1, 120, 680, "Mercado")...)
	row = append(row, glyphs(1, 165, 680, "Central")...)
	row = append(row, glyphs(1, 220, 680, "Ltda")...)
	row = append(row, glyphs(1, 310, 680, "1.234,56")...) // termina em 350, a borda direita do cabeçalho
	got := Assign(Build(row, 0)[0], bands)
	want := []string{"01/03", "Mercado Central Ltda", "1.234,56"}
	if !eq(got, want) {
		t.Errorf("Assign = %q, want %q", got, want)
	}
}

func TestGutters(t *testing.T) {
	var toks []extract.Token
	rows := []struct{ d, desc, v string }{
		{"01/03", "Padaria", "-45,00"},
		{"02/03", "Supermercado Central", "-120,00"},
		{"03/03", "PIX", "200,00"},
		{"04/03", "Tarifa mensal", "-10,00"},
	}
	for i, r := range rows {
		y := 700 - float64(i)*15
		toks = append(toks, glyphs(1, 50, y, r.d)...)
		toks = append(toks, glyphs(1, 120, y, r.desc)...)
		toks = append(toks, glyphs(1, 400-5*float64(len(r.v)), y, r.v)...)
	}
	lines := Build(toks, 0)
	bands := Gutters(lines, 1.0)
	if len(bands) != 3 {
		t.Fatalf("bands = %+v", bands)
	}
	got := Assign(lines[1], bands)
	if !eq(got, []string{"02/03", "Supermercado Central", "-120,00"}) {
		t.Errorf("Assign = %q", got)
	}
}

func TestRemoveNoise(t *testing.T) {
	var toks []extract.Token
	for p := 1; p <= 2; p++ {
		toks = append(toks, glyphs(p, 50, 800, "BANCO FICTICIO")...)
		toks = append(toks, glyphs(p, 50, 780, "Data Valor")...)
		d := string(rune('0' + p))
		toks = append(toks, glyphs(p, 50, 700, "0"+d+"/03 x 1,00")...)
		toks = append(toks, glyphs(p, 50, 690, "0"+d+"/03 y 2,00")...)
		toks = append(toks, glyphs(p, 50, 680, "0"+d+"/03 z 3,00")...)
		toks = append(toks, glyphs(p, 50, 670, "0"+d+"/03 w 4,00")...)
		toks = append(toks, glyphs(p, 50, 30, "Página "+string(rune('0'+p))+" de 2")...)
	}
	lines := Build(toks, 0)
	keep := func(l Line) bool { return l.Text() == "Data Valor" || strings.Contains(l.Text(), ",") }
	got := texts(RemoveNoise(lines, keep))
	want := []string{
		"Data Valor", "01/03 x 1,00", "01/03 y 2,00", "01/03 z 3,00", "01/03 w 4,00",
		"Data Valor", "02/03 x 1,00", "02/03 y 2,00", "02/03 z 3,00", "02/03 w 4,00",
	}
	if !eq(got, want) {
		t.Errorf("RemoveNoise = %q", got)
	}
}

func TestGuttersHugeCoordinates(t *testing.T) {
	toks := append(glyphs(1, 50, 700, "01/03 1,00"), extract.Token{Page: 1, X: 1e12, Y: 700, W: 5, FontSize: 10, Text: "x"})
	if b := Gutters(Build(toks, 0), 1.0); b != nil {
		t.Errorf("Gutters = %v, want nil", b)
	}
}

// guttersReference é a implementação original, célula a célula, mantida
// como referência para a versão em matriz.
func guttersReference(lines []Line, minGap float64) []Band {
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

// statement gera linhas de extrato: data, descrição de tamanho variado e
// um ou dois valores alinhados à direita.
func statement(n int, rng *rand.Rand) []Line {
	lines := make([]Line, n)
	for i := range lines {
		desc := 60 + rng.Float64()*220
		ws := []Word{{X0: 40, X1: 82, Text: "01/03/2026"}, {X0: 100, X1: 100 + desc, Text: "DESCRICAO"}}
		v := 20 + rng.Float64()*50
		ws = append(ws, Word{X0: 470 - v, X1: 470, Text: "1,00"})
		if rng.Intn(3) > 0 {
			ws = append(ws, Word{X0: 555 - v, X1: 555, Text: "2,00"})
		}
		lines[i] = Line{Page: 1 + i/50, Y: 800 - float64(i%50)*14, FontSize: 9, Words: ws}
	}
	return lines
}

func TestGuttersMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for _, n := range []int{1, 2, 9, 10, 11, 64, 300} {
		lines := statement(n, rng)
		got, want := Gutters(lines, 1), guttersReference(lines, 1)
		if len(got) != len(want) {
			t.Fatalf("n=%d: %v, want %v", n, got, want)
		}
		for i := range want {
			if math.Abs(got[i].X0-want[i].X0) > 1e-9 || math.Abs(got[i].X1-want[i].X1) > 1e-9 {
				t.Errorf("n=%d banda %d: %v, want %v", n, i, got[i], want[i])
			}
		}
	}
}

func TestBoundaries(t *testing.T) {
	// Cabeçalho: Data [40,60]  Lançamento [100,150]  Valor [440,470]  Saldo [525,555]
	head := []Band{{40, 60}, {100, 150}, {440, 470}, {525, 555}}
	body := []Line{
		{FontSize: 9, Words: []Word{{X0: 40, X1: 82}, {X0: 100, X1: 380}, {X0: 400, X1: 470}, {X0: 500, X1: 555}}},
		{FontSize: 9, Words: []Word{{X0: 40, X1: 62}, {X0: 100, X1: 160}, {X0: 450, X1: 470}}},
	}
	got := Boundaries(head, body)
	if len(got) != 4 {
		t.Fatalf("Boundaries = %v", got)
	}
	// As fronteiras caem nos vãos do dado: 82–100, 380–400 e 470–500.
	for i, span := range [][2]float64{{82, 100}, {380, 400}, {470, 500}} {
		if b := got[i].X1; b < span[0] || b > span[1] || got[i+1].X0 != b {
			t.Errorf("fronteira %d = %v, deveria estar em %v", i, b, span)
		}
	}
	if got[0].X0 >= 40 || got[3].X1 <= 555 {
		t.Errorf("bandas não cobrem a tabela: %v", got)
	}
	cells := Assign(body[0], got)
	if cells[1] == "" && len(cells) != 4 {
		t.Errorf("Assign = %q", cells)
	}

	// Sem corpo: ponto médio entre os cabeçalhos.
	got = Boundaries(head, nil)
	if got[0].X1 != 80 || got[1].X1 != 295 {
		t.Errorf("sem corpo: %v", got)
	}
	if Boundaries(nil, body) != nil {
		t.Error("sem cabeçalho deveria dar nil")
	}
}

func TestLinesManyWordsSameLine(t *testing.T) {
	// A mediana incremental tem de dar a mesma linha que a ordenação.
	var ws []Word
	for i := 0; i < 500; i++ {
		ws = append(ws, Word{Page: 1, X0: float64(i) * 10, X1: float64(i)*10 + 8, Y: 700 + float64(i%5)*0.3, FontSize: 8 + float64(i%3), Text: "x"})
	}
	ws = append(ws, Word{Page: 1, X0: 0, X1: 8, Y: 680, FontSize: 9, Text: "y"})
	lines := Lines(ws, 0.5)
	if len(lines) != 2 || len(lines[0].Words) != 500 || lines[0].FontSize != 9 {
		t.Fatalf("len = %d, primeira = %d palavras, fonte %v", len(lines), len(lines[0].Words), lines[0].FontSize)
	}
}

// Uma linha com centenas de milhares de palavras (PDF malicioso) não pode
// custar tempo quadrático.
func TestLinesHugeLineIsLinear(t *testing.T) {
	ws := make([]Word, 300000)
	for i := range ws {
		ws[i] = Word{Page: 1, X0: float64(i), X1: float64(i) + 0.5, Y: 700, FontSize: 9, Text: "x"}
	}
	start := time.Now()
	if lines := Lines(ws, 0.5); len(lines) != 1 {
		t.Fatalf("len = %d", len(lines))
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("Lines levou %v", d)
	}
}

func BenchmarkGutters(b *testing.B) {
	lines := statement(3000, rand.New(rand.NewSource(3))) // ~60 páginas
	b.Run("matriz", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			Gutters(lines, 1)
		}
	})
	b.Run("referencia", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			guttersReference(lines, 1)
		}
	})
}

package spatial

import (
	"strings"
	"testing"

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

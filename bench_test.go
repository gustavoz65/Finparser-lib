package finparser

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gustavoz65/finparser-lib/internal/pdf/extract"
	"github.com/gustavoz65/finparser-lib/internal/pdf/spatial"
	"github.com/gustavoz65/finparser-lib/internal/testgen"
)

// bigPDF gera um extrato de pages páginas com 50 lançamentos cada, cabeçalho
// só na primeira.
func bigPDF(tb testing.TB, pages int) []byte {
	tb.Helper()
	var doc testgen.Doc
	balance := 100000_00
	for p := 0; p < pages; p++ {
		var lines []testgen.Line
		y := 50.0
		if p == 0 {
			lines = append(lines,
				testgen.Line{Y: y, Cells: []testgen.Cell{{X: 40, Text: "Banco Fictício - Período: 01/01/2026 a 31/12/2026"}}},
				testgen.Line{Y: y + 20, Bold: true, Cells: []testgen.Cell{
					{X: 40, Text: "Data"}, {X: 100, Text: "Histórico"},
					{X: 470, Text: "Valor (R$)", Right: true}, {X: 555, Text: "Saldo (R$)", Right: true}}},
			)
			y += 40
		}
		for i := 0; i < 50; i++ {
			v := 100 + (p*50+i)*37%9000
			balance -= v
			lines = append(lines, testgen.Line{Y: y, Size: 8, Cells: []testgen.Cell{
				{X: 40, Text: fmt.Sprintf("%02d/%02d/2026", 1+i%28, 1+p%12)},
				{X: 100, Text: fmt.Sprintf("COMPRA CARTAO LOJA FICTICIA NUMERO %d", p*50+i)},
				{X: 470, Text: fmt.Sprintf("-%d,%02d", v/100, v%100), Right: true},
				{X: 555, Text: fmt.Sprintf("%d.%03d,%02d", balance/100000, balance/100%1000, balance%100), Right: true},
			}})
			y += 14
		}
		doc.Pages = append(doc.Pages, testgen.Page{Lines: lines})
	}
	b, err := testgen.PDF(doc)
	if err != nil {
		tb.Fatal(err)
	}
	return b
}

func TestBigPDFReconciles(t *testing.T) {
	st, err := ParseBytes(bigPDF(t, 20))
	if err != nil {
		t.Fatal(err)
	}
	rec := st.Reconcile()
	if len(st.Transactions) != 1000 || rec.Status != Verified || rec.Covered != 999 || len(st.Warnings) != 0 {
		t.Errorf("%d transações, Reconcile = %+v, %d avisos", len(st.Transactions), rec, len(st.Warnings))
	}
}

// BenchmarkParse mede cada fixture de ponta a ponta.
func BenchmarkParse(b *testing.B) {
	files, _ := filepath.Glob(filepath.Join("testdata", "synthetic", "*"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(filepath.Base(f), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Parse(bytes.NewReader(data), WithReferenceDate(ref)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkPDF separa as fases de um PDF de 20 páginas e 1000 lançamentos:
// extrair os tokens (backend), montar o espaço (motor espacial) e o Parse
// inteiro.
func BenchmarkPDF(b *testing.B) {
	data := bigPDF(b, 20)
	toks, err := extract.Default{}.Extract(data, "")
	if err != nil {
		b.Fatal(err)
	}
	b.Run("extract", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := (extract.Default{}).Extract(data, ""); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("spatial", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			lines := spatial.Build(toks, 0.5)
			spatial.Gutters(lines, 1)
		}
	})
	b.Run("parse", func(b *testing.B) {
		b.SetBytes(int64(len(data)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := ParseBytes(data); err != nil {
				b.Fatal(err)
			}
		}
	})
}

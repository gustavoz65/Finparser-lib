package extract

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/testgen"
)

func sample(t *testing.T, pw string) []byte {
	t.Helper()
	b, err := testgen.PDF(testgen.Doc{Password: pw, Pages: []testgen.Page{{Lines: []testgen.Line{
		{Y: 100, Cells: []testgen.Cell{{X: 50, Text: "01/03/2026"}, {X: 150, Text: "Padaria São João"}, {X: 500, Text: "-45,00", Right: true}}},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestExtractWidthsAndOrder(t *testing.T) {
	toks, err := Default{}.Extract(sample(t, ""), "")
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	prevEnd := 0.0
	for _, tk := range toks {
		sb.WriteString(tk.Text)
		if tk.W <= 0 {
			t.Fatalf("token sem largura: %+v", tk)
		}
		if tk.Page != 1 || math.Abs(tk.Y-742) > 1 {
			t.Fatalf("posição inesperada: %+v", tk)
		}
		if tk.Text == "P" && tk.X != 150 {
			t.Errorf("início da descrição em X=%v", tk.X)
		}
		if tk.X+0.01 < prevEnd && tk.X != 150 && tk.X < 400 {
			t.Errorf("glifos sobrepostos: %+v (fim anterior %.2f)", tk, prevEnd)
		}
		prevEnd = tk.X + tk.W
	}
	if got := sb.String(); got != "01/03/2026Padaria São João-45,00" {
		t.Errorf("texto = %q", got)
	}
	// Valor alinhado à direita termina em ~500.
	last := toks[len(toks)-1]
	if math.Abs(last.X+last.W-500) > 0.5 {
		t.Errorf("borda direita do valor = %.2f", last.X+last.W)
	}
}

func TestExtractEncrypted(t *testing.T) {
	data := sample(t, "segredo")
	if _, err := (Default{}).Extract(data, ""); !errors.Is(err, errs.ErrEncrypted) {
		t.Errorf("sem senha: err = %v", err)
	}
	if _, err := (Default{}).Extract(data, "errada"); !errors.Is(err, errs.ErrEncrypted) {
		t.Errorf("senha errada: err = %v", err)
	}
	// Com a senha certa o backend padrão pode não decifrar (limitação
	// conhecida do ledongthuc/pdf); o erro tem de ser ErrEncrypted, nunca
	// panic nem ErrMalformed.
	toks, err := Default{}.Extract(data, "segredo")
	if err != nil && !errors.Is(err, errs.ErrEncrypted) {
		t.Errorf("senha certa: err = %v", err)
	}
	if err == nil && len(toks) == 0 {
		t.Error("senha certa: nenhum token")
	}
}

func TestExtractRobust(t *testing.T) {
	full := sample(t, "")
	cases := map[string][]byte{
		"vazio":     nil,
		"cabeçalho": []byte("%PDF-1.4\n"),
		"truncado":  full[:len(full)/2],
		"lixo":      append([]byte("%PDF-1.4\n"), make([]byte, 300)...),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Default{}.Extract(data, "")
			if !errors.Is(err, errs.ErrMalformed) {
				t.Errorf("err = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestNoTextLayer(t *testing.T) {
	b, err := testgen.PDF(testgen.Doc{Pages: []testgen.Page{{}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Default{}).Extract(b, ""); !errors.Is(err, errs.ErrNoTextLayer) {
		t.Errorf("err = %v", err)
	}
}

func TestGarbledPages(t *testing.T) {
	toks := []Token{
		{Page: 1, Text: "Extrato de março"},
		{Page: 2, Text: "一二三ab"},
	}
	if got := GarbledPages(toks); len(got) != 1 || got[0] != 2 {
		t.Errorf("GarbledPages = %v", got)
	}
}

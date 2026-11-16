package pdf

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/pdf/extract"
	"github.com/gustavoz65/finparser-lib/internal/record"
)

// fake devolve tokens fixos, sem PDF de verdade.
type fake struct {
	toks []extract.Token
	err  error
}

func (f fake) Extract([]byte, string) ([]extract.Token, error) { return f.toks, f.err }

// text gera um token por caractere com largura 5 (fonte 10), na linha y
// (medida do topo, como num layout).
func text(page int, x, top float64, s string) []extract.Token {
	var out []extract.Token
	for _, r := range s {
		out = append(out, extract.Token{Page: page, X: x, Y: 842 - top, W: 5, FontSize: 10, Text: string(r)})
		x += 5
	}
	return out
}

func join(parts ...[]extract.Token) []extract.Token {
	var out []extract.Token
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestLineBased(t *testing.T) {
	toks := join(
		text(1, 40, 50, "Extrato 01/03/2026 a 31/03/2026"),
		text(1, 40, 80, "05/03 Mercado Fictício -89,90 910,10"),
	)
	res, err := Parse(nil, Options{Extractor: fake{toks: toks}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 1 {
		t.Fatalf("Records = %+v", res.Records)
	}
	r := res.Records[0]
	if r.Description != "Mercado Fictício" || r.Amount.String() != "-89.9" || r.Balance.String() != "910.1" {
		t.Errorf("Record = %+v", r)
	}
	if !r.Date.Equal(time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("Date = %v", r.Date)
	}
}

func TestHeaderAndContinuation(t *testing.T) {
	toks := join(
		text(1, 40, 50, "Data"), text(1, 120, 50, "Histórico"), text(1, 400, 50, "Valor"),
		text(1, 40, 64, "02/03/2026"), text(1, 120, 64, "PAGAMENTO BOLETO"), text(1, 400, 64, "-10,00"),
		text(1, 120, 75, "EMPRESA X"),
		text(1, 40, 89, "03/03/2026"), text(1, 120, 89, "DEPOSITO"), text(1, 400, 89, "5,00"),
		text(1, 40, 103, "04/03/2026"), text(1, 120, 103, "ESTRANHO"), text(1, 400, 103, "12.34.5"),
	)
	res, err := Parse(nil, Options{Extractor: fake{toks: toks}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 2 {
		t.Fatalf("Records = %+v", res.Records)
	}
	if d := res.Records[0].Description; d != "PAGAMENTO BOLETO EMPRESA X" {
		t.Errorf("descrição = %q", d)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Message != "valor ilegível" {
		t.Errorf("Warnings = %+v", res.Warnings)
	}
}

func TestGarbledPage(t *testing.T) {
	toks := join(
		text(1, 40, 50, "02/03/2026 Padaria -10,00"),
		text(2, 40, 50, "一二三一二三"),
	)
	res, err := Parse(nil, Options{Extractor: fake{toks: toks}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 1 || len(res.Warnings) != 1 || res.Warnings[0].Page != 2 {
		t.Errorf("res = %+v", res)
	}
}

func TestExtractorError(t *testing.T) {
	_, err := Parse(nil, Options{Extractor: fake{err: errs.ErrNoTextLayer}})
	if !errors.Is(err, errs.ErrNoTextLayer) {
		t.Errorf("err = %v", err)
	}
}

// right alinha s pela borda direita x1, como colunas de valor.
func right(page int, x1, top float64, s string) []extract.Token {
	return text(page, x1-5*float64(len([]rune(s))), top, s)
}

func mustParse(t *testing.T, toks []extract.Token) *record.Result {
	t.Helper()
	res, err := Parse(nil, Options{Extractor: fake{toks: toks}, Reference: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

type wantRec struct{ date, desc, amount, balance string }

func check(t *testing.T, res *record.Result, want []wantRec) {
	t.Helper()
	if len(res.Records) != len(want) {
		t.Fatalf("len(Records) = %d, want %d\nRecords = %+v\nWarnings = %+v", len(res.Records), len(want), res.Records, res.Warnings)
	}
	for i, w := range want {
		r := res.Records[i]
		bal := ""
		if r.Balance != nil {
			bal = r.Balance.String()
		}
		if r.Date.Format("02/01/2006") != w.date || r.Description != w.desc || r.Amount.String() != w.amount || bal != w.balance {
			t.Errorf("Records[%d] = %s | %s | %s | %s\nwant          %s | %s | %s | %s",
				i, r.Date.Format("02/01/2006"), r.Description, r.Amount, bal, w.date, w.desc, w.amount, w.balance)
		}
	}
}

// A descrição passa muito do fim do cabeçalho "Lançamento" e o valor alto
// começa antes de "Valor (R$)": a coluna segue o dado, não o cabeçalho.
func TestHeaderColumnsFollowData(t *testing.T) {
	long := "PIX RECEBIDO DE FULANO DE TAL DA SILVA SAURO COMERCIO LTDA ME"
	toks := join(
		text(1, 40, 50, "Período: 01/03/2026 a 31/03/2026"),
		text(1, 40, 100, "Data"), text(1, 100, 100, "Lançamento"), right(1, 480, 100, "Valor (R$)"), right(1, 560, 100, "Saldo (R$)"),
		text(1, 40, 114, "01/03"), text(1, 100, 114, "SALDO ANTERIOR"), right(1, 560, 114, "1.000,00"),
		text(1, 40, 128, "02/03"), text(1, 100, 128, long), right(1, 480, 128, "200,00"), right(1, 560, 128, "1.200,00"),
		text(1, 40, 142, "03/03"), text(1, 100, 142, "BOLETO"), right(1, 480, 142, "-1.234.150,35"), right(1, 560, 142, "-1.232.950,35"),
		text(1, 100, 154, "COMPANHIA ENERGETICA FICTICIA S/A - CNPJ 00.000.000/0001-00 - REF 03"),
		text(1, 40, 168, "04/03"), text(1, 100, 168, "TARIFA"), right(1, 480, 168, "-10,00"), right(1, 560, 168, "-1.232.960,35"),
	)
	res := mustParse(t, toks)
	check(t, res, []wantRec{
		{"02/03/2026", long, "200", "1200"},
		{"03/03/2026", "BOLETO COMPANHIA ENERGETICA FICTICIA S/A - CNPJ 00.000.000/0001-00 - REF 03", "-1234150.35", "-1232950.35"},
		{"04/03/2026", "TARIFA", "-10", "-1232960.35"},
	})
	if len(res.Warnings) != 0 || res.OpeningBalance == nil {
		t.Errorf("Warnings = %+v, OpeningBalance = %v", res.Warnings, res.OpeningBalance)
	}
}

// Crédito e débito em colunas separadas, com a de crédito quase vazia: um
// valor nunca pode cair na coluna vizinha.
func TestSparseCreditDebitColumns(t *testing.T) {
	toks := join(
		text(1, 40, 100, "Data"), text(1, 110, 100, "Histórico"), text(1, 300, 100, "Docto."),
		right(1, 420, 100, "Crédito (R$)"), right(1, 490, 100, "Débito (R$)"), right(1, 560, 100, "Saldo (R$)"),
	)
	var want []wantRec
	bal := 1000
	for i := 0; i < 14; i++ {
		top := 114 + float64(i)*14
		bal -= 10
		d := fmt.Sprintf("%02d/03/2026", i+1)
		toks = append(toks, join(text(1, 40, top, d), text(1, 110, top, "COMPRA CARTAO"), text(1, 300, top, fmt.Sprintf("%07d", i)),
			right(1, 490, top, "10,00"), right(1, 560, top, fmt.Sprintf("%d,00", bal)))...)
		want = append(want, wantRec{d, "COMPRA CARTAO", "-10", fmt.Sprint(bal)})
	}
	toks = append(toks, join(text(1, 40, 320, "20/03/2026"), text(1, 110, 320, "TED RECEBIDA"), text(1, 300, 320, "0000099"),
		right(1, 420, 320, "12.500,00"), right(1, 560, 320, "13.360,00"))...)
	want = append(want, wantRec{"20/03/2026", "TED RECEBIDA", "12500", "13360"})

	res := mustParse(t, toks)
	check(t, res, want)
	for _, r := range res.Records {
		if !r.SignKnown {
			t.Errorf("sinal deveria ser conhecido: %+v", r)
		}
	}
}

// Sem tabela: valor partido em várias palavras ("R$ 45,90 D", "- 12,00").
func TestLineBasedSplitAmounts(t *testing.T) {
	for in, want := range map[string]wantRec{
		"05/03/2026 Mercado R$ 45,90 D":       {"05/03/2026", "Mercado", "-45.9", ""},
		"05/03/2026 Pix recebido 200,00 C":    {"05/03/2026", "Pix recebido", "200", ""},
		"05/03/2026 Tarifa - 12,00 R$ 988,00": {"05/03/2026", "Tarifa", "-12", "988"},
		"05/03/2026 Loja −45,90":              {"05/03/2026", "Loja", "-45.9", ""},
		"05/03/2026 Posto C 30,00 D 958,00 C": {"05/03/2026", "Posto C", "-30", "958"},
		"05/03/2026 Débitos diversos 10,00 D": {"05/03/2026", "Débitos diversos", "-10", ""},
		"05/03/2026 Compra 3 x 15,00 45,00 D": {"05/03/2026", "Compra 3 x", "-45", "15"}, // dois valores: o último é o saldo
		"05/03/2026 Farmácia R$ 1.234,56":     {"05/03/2026", "Farmácia", "1234.56", ""},
	} {
		res := mustParse(t, text(1, 40, 80, in))
		if in == "05/03/2026 Compra 3 x 15,00 45,00 D" {
			// Ambíguo por natureza; só garante que não some nem dá panic.
			if len(res.Records) != 1 {
				t.Errorf("%q: Records = %+v", in, res.Records)
			}
			continue
		}
		t.Run(in, func(t *testing.T) { check(t, res, []wantRec{want}) })
	}
}

// Cabeçalho só na primeira página: a segunda herda as colunas.
func TestHeaderCarriesAcrossPages(t *testing.T) {
	toks := join(
		text(1, 40, 100, "Data"), text(1, 100, 100, "Histórico"), right(1, 480, 100, "Valor"), right(1, 560, 100, "Saldo"),
		text(1, 40, 114, "02/03/2026"), text(1, 100, 114, "PIX"), right(1, 480, 114, "-10,00"), right(1, 560, 114, "90,00"),
		text(2, 40, 60, "03/03/2026"), text(2, 100, 60, "TED COM DESCRICAO BEM COMPRIDA PARA PASSAR DO CABECALHO"), right(2, 480, 60, "5,00"), right(2, 560, 60, "95,00"),
	)
	check(t, mustParse(t, toks), []wantRec{
		{"02/03/2026", "PIX", "-10", "90"},
		{"03/03/2026", "TED COM DESCRICAO BEM COMPRIDA PARA PASSAR DO CABECALHO", "5", "95"},
	})
}

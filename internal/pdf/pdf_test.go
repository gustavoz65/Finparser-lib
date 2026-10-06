package pdf

import (
	"errors"
	"testing"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/pdf/extract"
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

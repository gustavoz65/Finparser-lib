package table

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/record"
)

func rows(lines ...string) []Row {
	out := make([]Row, len(lines))
	for i, l := range lines {
		out[i] = Row{Cells: strings.Split(l, ";"), Line: i + 1}
	}
	return out
}

func parse(t *testing.T, lines ...string) *record.Result {
	t.Helper()
	res, err := Parse(rows(lines...), Options{Reference: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestMultipleDescriptionColumns(t *testing.T) {
	res := parse(t,
		"Extrato Conta Corrente",
		"Período;01/03/2026 a 31/03/2026",
		"Data Lançamento;Histórico;Descrição;Valor;Saldo",
		"02/03/2026;Pix enviado;Fulano de Tal;-50,00;950,00",
		"03/03/2026;Pix recebido;Pix recebido;100,00;1.050,00",
		"04/03/2026;Tarifa;;-1,00;1.049,00",
	)
	want := []string{"Pix enviado - Fulano de Tal", "Pix recebido", "Tarifa"}
	for i, w := range want {
		if got := res.Records[i].Description; got != w {
			t.Errorf("Records[%d].Description = %q, want %q", i, got, w)
		}
	}
	if res.PeriodEnd.Format("2006-01-02") != "2026-03-31" {
		t.Errorf("PeriodEnd = %v", res.PeriodEnd)
	}
}

func TestSignColumn(t *testing.T) {
	res := parse(t,
		"Data;Histórico;Valor;Tipo",
		"02/03/2026;MERCADO;50,00;D",
		"03/03/2026;TRANSF;100,00;Crédito",
		"04/03/2026;AJUSTE;-3,00;C",
	)
	want := []string{"-50", "100", "3"}
	for i, w := range want {
		r := res.Records[i]
		if r.Amount.String() != w || !r.SignKnown {
			t.Errorf("Records[%d] = %s signed=%v, want %s", i, r.Amount, r.SignKnown, w)
		}
	}
}

// "Tipo" com PIX/TED não é coluna de sinal: os valores continuam sem sinal.
func TestTypeColumnIsNotSign(t *testing.T) {
	res := parse(t,
		"Data;Descrição;Valor;Tipo",
		"02/03/2026;MERCADO;50,00;PIX",
		"03/03/2026;TRANSF;100,00;C",
	)
	for _, r := range res.Records {
		if r.SignKnown || r.Amount.IsNegative() {
			t.Errorf("sinal inventado: %+v", r)
		}
	}
}

func TestCreditDebitWithZeros(t *testing.T) {
	res := parse(t,
		"Data Lançamento;Data Contábil;Título;Descrição;Entrada(R$);Saída(R$);Saldo do Dia(R$)",
		"02/03/2026;02/03/2026;Pix;Fulano;0.00;50.00;950.00",
		"03/03/2026;03/03/2026;Pix;Beltrano;100.00;0.00;1050.00",
	)
	if r := res.Records[0]; r.Amount.String() != "-50" || r.Description != "Pix - Fulano" || r.Balance.String() != "950" {
		t.Errorf("Records[0] = %+v", r)
	}
	if r := res.Records[1]; r.Amount.String() != "100" {
		t.Errorf("Records[1] = %+v", r)
	}
}

func TestSummaryRows(t *testing.T) {
	res := parse(t,
		"Data;Histórico;Valor;Saldo",
		"01/03/2026;SALDO ANTERIOR;;1000,00",
		"02/03/2026;LOJA;-50,00;",
		"02/03/2026;SALDO DO DIA;;950,00",
		"31/03/2026;TOTAL DE SAÍDAS;50,00;",
		"31/03/2026;SALDO FINAL;;950,00",
	)
	if len(res.Records) != 1 || res.Records[0].Balance.String() != "950" {
		t.Fatalf("Records = %+v", res.Records)
	}
	if res.OpeningBalance.String() != "1000" || res.ClosingBalance.String() != "950" {
		t.Errorf("abertura %v, fechamento %v", res.OpeningBalance, res.ClosingBalance)
	}
}

func TestWarnings(t *testing.T) {
	res := parse(t,
		"Data;Descrição;Valor",
		"xx/03/2026;A;-1,00",
		"02/03/2026;B;abc",
		"texto solto;;",
		"Data;Descrição;Valor",
		"03/03/2026;C;-2,00",
	)
	if len(res.Records) != 1 || len(res.Warnings) != 2 {
		t.Errorf("Records = %+v, Warnings = %+v", res.Records, res.Warnings)
	}
}

func TestMalformed(t *testing.T) {
	if _, err := Parse(nil, Options{}); !errors.Is(err, errs.ErrMalformed) {
		t.Errorf("vazio: %v", err)
	}
	if _, err := Parse(rows("a;b", "c;d"), Options{}); !errors.Is(err, errs.ErrMalformed) {
		t.Errorf("sem colunas: %v", err)
	}
}

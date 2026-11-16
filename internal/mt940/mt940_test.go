package mt940

import (
	"errors"
	"testing"

	"github.com/gustavoz65/finparser-lib/internal/errs"
)

const sample = "{1:F01FICTBRSPAXXX0000000000}{2:I940FICTBRSPXXXXN}{4:\r\n" +
	":20:EXTRATO0001\r\n" +
	":25:FICTBRSP/0001234567\r\n" +
	":28C:00012/001\r\n" +
	":60F:C260301BRL1000,00\r\n" +
	":61:2603020302C3500,00NTRFNONREF//BANK-REF-1\r\n" +
	"Folha de pagamento\r\n" +
	":86:Empresa Fictícia Ltda\r\n" +
	"salário de março\r\n" +
	":61:260303D45,9NMSCREF123\r\n" +
	":86:?00Kartenzahlung?20Padaria São?21 João?30FICTBRSP?31999?32Padaria SJ?33 Ltda\r\n" +
	":61:251231 0102RC10,NCHGNONREF\r\n" +
	":61:2603050305RD2,50NCHGNONREF\r\n" +
	":61:260399C1,00NTRFX\r\n" +
	":61:lixo\r\n" +
	":62F:C260331BRL4446,60\r\n" +
	":86:informação do extrato, não de lançamento\r\n" +
	":64:C260331BRL4446,60\r\n" +
	"-}\r\n"

func TestParse(t *testing.T) {
	res, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ date, desc, amount, id string }{
		{"2026-03-02", "Empresa Fictícia Ltda salário de março - Folha de pagamento", "3500", "BANK-REF-1"},
		{"2026-03-03", "Padaria SJ Ltda - Padaria São João", "-45.9", "REF123"},
		{"2026-03-05", "", "2.5", ""},
	}
	if len(res.Records) != len(want) {
		t.Fatalf("Records = %+v\nWarnings = %+v", res.Records, res.Warnings)
	}
	for i, w := range want {
		r := res.Records[i]
		if r.Date.Format("2006-01-02") != w.date || r.Description != w.desc || r.Amount.String() != w.amount || r.ID != w.id || !r.SignKnown {
			t.Errorf("Records[%d] = %+v, want %+v", i, r, w)
		}
	}
	// "251231 0102RC10," tem espaço: ilegível, como a data 260399 e o lixo.
	if len(res.Warnings) != 3 {
		t.Errorf("Warnings = %+v", res.Warnings)
	}
	if res.OpeningBalance.String() != "1000" || res.ClosingBalance.String() != "4446.6" || res.Currency != "BRL" {
		t.Errorf("saldos = %v, %v; moeda %q", res.OpeningBalance, res.ClosingBalance, res.Currency)
	}
	if res.Account.BankID != "FICTBRSP" || res.Account.Number != "0001234567" {
		t.Errorf("Account = %+v", res.Account)
	}
	if res.PeriodStart.Format("2006-01-02") != "2026-03-01" || res.PeriodEnd.Format("2006-01-02") != "2026-03-31" {
		t.Errorf("período = %v a %v", res.PeriodStart, res.PeriodEnd)
	}
}

func TestEntryDateAcrossYear(t *testing.T) {
	res, err := Parse(":20:X\n:61:2512310102RC10,NCHGNONREF\n:61:2601021231D1,NCHGNONREF\n")
	if err != nil || len(res.Records) != 2 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	// Valor em 31/12/2025, lançado em 02/01/2026; estorno de crédito é saída.
	if r := res.Records[0]; r.Date.Format("2006-01-02") != "2026-01-02" || r.Amount.String() != "-10" {
		t.Errorf("Records[0] = %+v", r)
	}
	if r := res.Records[1]; r.Date.Format("2006-01-02") != "2025-12-31" {
		t.Errorf("Records[1] = %+v", r)
	}
}

func TestMultipleStatementsDropBalances(t *testing.T) {
	res, err := Parse(":20:A\n:25:1\n:60F:C260301BRL1,00\n:61:260302C1,00NTRFA\n:62F:C260302BRL2,00\n:20:B\n:25:2\n:60F:C260301BRL9,00\n:61:260302D1,00NTRFB\n:62F:C260302BRL8,00\n")
	if err != nil || len(res.Records) != 2 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if res.OpeningBalance != nil || res.ClosingBalance != nil {
		t.Errorf("saldos de contas diferentes não se misturam: %v %v", res.OpeningBalance, res.ClosingBalance)
	}
}

func TestMalformed(t *testing.T) {
	for _, in := range []string{"", "texto qualquer\ncom linhas", ":86:solto"} {
		if _, err := Parse(in); !errors.Is(err, errs.ErrMalformed) {
			t.Errorf("Parse(%q): err = %v", in, err)
		}
	}
}

func FuzzParse(f *testing.F) {
	f.Add(sample)
	f.Add(":61:2603020302C3500,00NTRFNONREF//X\n:86:?20a?32b")
	f.Fuzz(func(t *testing.T, s string) {
		res, err := Parse(s)
		if err == nil && res == nil {
			t.Error("sem erro e sem resultado")
		}
	})
}

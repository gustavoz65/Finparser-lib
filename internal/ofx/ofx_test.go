package ofx

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/textenc"
)

func load(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := textenc.ToUTF8(b)
	return string(u)
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestParseSGML(t *testing.T) {
	res, err := Parse(load(t, "conta_sgml_1252.ofx"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Bank != "nubank" {
		t.Errorf("Bank = %q", res.Bank)
	}
	if !res.PeriodStart.Equal(day(2026, 3, 1)) || !res.PeriodEnd.Equal(day(2026, 3, 31)) {
		t.Errorf("período = %v a %v", res.PeriodStart, res.PeriodEnd)
	}
	if len(res.Records) != 3 {
		t.Fatalf("len(Records) = %d, want 3", len(res.Records))
	}
	want := []struct {
		date   time.Time
		amount string
		desc   string
		id     string
	}{
		{day(2026, 3, 2), "3500", "Transferência recebida - EMPRESA FICTÍCIA LTDA", "abc-001"},
		{day(2026, 3, 3), "-45.9", "Compra no débito - PADARIA SÃO JOÃO", "abc-002"},
		{day(2026, 3, 5), "-120", "PIX ENVIADO - Fulano de Tal", ""},
	}
	for i, w := range want {
		r := res.Records[i]
		if !r.Date.Equal(w.date) || r.Amount.String() != w.amount || r.Description != w.desc || r.ID != w.id || !r.SignKnown {
			t.Errorf("Records[%d] = %+v", i, r)
		}
	}
	// DEBIT positivo invertido + data inválida.
	if len(res.Warnings) != 2 {
		t.Fatalf("Warnings = %+v", res.Warnings)
	}
	if !strings.Contains(res.Warnings[1].Message, "data inválida") || res.Warnings[1].Line == 0 {
		t.Errorf("Warnings[1] = %+v", res.Warnings[1])
	}
}

func TestParseXMLCartao(t *testing.T) {
	res, err := Parse(load(t, "cartao_xml.ofx"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 2 {
		t.Fatalf("Records = %+v", res.Records)
	}
	if r := res.Records[0]; r.Description != "MERCADO & CIA" || r.Amount.String() != "-89.9" {
		t.Errorf("Records[0] = %+v", r)
	}
	if r := res.Records[1]; r.Description != "ESTORNO MERCADO" || r.Amount.String() != "89.9" {
		t.Errorf("Records[1] = %+v", r)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %+v", res.Warnings)
	}
}

func TestParseMalformed(t *testing.T) {
	cases := []string{
		"",
		"OFXHEADER:100\n",
		"<OFX><BANKMSGSRSV1><STMTTRNRS><STMTRS><CURDEF>BRL</STMTRS></STMTTRNRS></BANKMSGSRSV1></OFX>",
		"<OFX><BANKMSGSRSV1><STMTTRNRS><STMTRS><BANKTRANLIST><STMTTRN><TRNAMT>1",
	}
	for _, c := range cases[:3] {
		if _, err := Parse(c); !errors.Is(err, errs.ErrMalformed) {
			t.Errorf("Parse(%q) err = %v, want ErrMalformed", c, err)
		}
	}
	// Truncado no meio: não quebra, só não acha transação válida.
	res, err := Parse(cases[3])
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 0 || len(res.Warnings) != 1 {
		t.Errorf("truncado: %+v", res)
	}
}

func TestUnclosedSiblings(t *testing.T) {
	s := "<OFX><STMTRS><BANKTRANLIST><STMTTRN><DTPOSTED>20260301<TRNAMT>-1,00<STMTTRN><DTPOSTED>20260302<TRNAMT>2,00</BANKTRANLIST></STMTRS></OFX>"
	res, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 2 {
		t.Errorf("Records = %+v", res.Records)
	}
}

func FuzzTokenize(f *testing.F) {
	f.Add("<OFX><A>1<B>2</OFX>")
	f.Add("<!-- x --><A>")
	f.Add("<<>>")
	f.Fuzz(func(t *testing.T, s string) {
		build(tokenize(s))
		_, _ = Parse(s)
	})
}

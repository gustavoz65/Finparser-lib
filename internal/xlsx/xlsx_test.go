package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/testgen"
)

func day(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC) }

func TestParse(t *testing.T) {
	data, err := testgen.XLSX([][]any{
		{"Banco Fictício S.A."},
		{"Período: 01/03/2026 a 31/03/2026"},
		{},
		{"Data", "Histórico", "Descrição", "Valor", "Saldo"},
		{day(1), "SALDO ANTERIOR", nil, nil, 1000.0},
		{day(2), "Pix recebido", "Fulano & Cia <ME>", 1234.5600000000001, 2234.56},
		{day(3), "Compra", "Padaria São João", -45.9, 2188.66},
		{"04/03/2026", "Tarifa", nil, -1, 2187.66},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := Parse(data, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ date, desc, amount, balance string }{
		{"02/03/2026", "Pix recebido - Fulano & Cia <ME>", "1234.56", "2234.56"},
		{"03/03/2026", "Compra - Padaria São João", "-45.9", "2188.66"},
		{"04/03/2026", "Tarifa", "-1", "2187.66"},
	}
	if len(res.Records) != len(want) {
		t.Fatalf("Records = %+v\nWarnings = %+v", res.Records, res.Warnings)
	}
	for i, w := range want {
		r := res.Records[i]
		if r.Date.Format("02/01/2006") != w.date || r.Description != w.desc || r.Amount.String() != w.amount || r.Balance.String() != w.balance {
			t.Errorf("Records[%d] = %+v, want %+v", i, r, w)
		}
	}
	if res.Records[0].Line != 6 || res.OpeningBalance == nil || res.PeriodEnd.Day() != 31 {
		t.Errorf("Line = %d, OpeningBalance = %v, PeriodEnd = %v", res.Records[0].Line, res.OpeningBalance, res.PeriodEnd)
	}
}

func TestNumber(t *testing.T) {
	tests := []struct {
		in          string
		date, d1904 bool
		want        string
	}{
		{"1234.5600000000001", false, false, "1234.56"},
		{"0.1", false, false, "0.10"},
		{"1.234", false, false, "1.2340"}, // não pode virar 1234
		{"-45.9", false, false, "-45.90"},
		{"1000", false, false, "1000"},
		{"1E3", false, false, "1000"},
		{"46082", true, false, "01/03/2026"},
		{"46082.75", true, false, "01/03/2026"},
		{"44620", true, true, "01/03/2026"},
		{"abc", false, false, "abc"},
		{"-5", true, false, "-5"},
	}
	for _, tt := range tests {
		if got := number(tt.in, tt.date, tt.d1904); got != tt.want {
			t.Errorf("number(%q, %v, %v) = %q, want %q", tt.in, tt.date, tt.d1904, got, tt.want)
		}
	}
}

func TestHelpers(t *testing.T) {
	for ref, want := range map[string]int{"A1": 0, "Z9": 25, "AA10": 26, "bc3": 54, "12": -1, "": -1} {
		if got := column(ref); got != want {
			t.Errorf("column(%q) = %d, want %d", ref, got, want)
		}
	}
	for code, want := range map[string]bool{"dd/mm/yyyy": true, "0.00": false, `#,##0.00" dias"`: false, "[$-416]d-mmm-yy": true, "[Red]0.00": false, `0\d`: false} {
		if got := isDateFormat(code); got != want {
			t.Errorf("isDateFormat(%q) = %v", code, got)
		}
	}
}

// Planilha com texto embutido, célula sem referência e sem partes opcionais.
func TestMinimalWorkbook(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("xl/worksheets/sheet1.xml")
	_, _ = w.Write([]byte(`<worksheet><sheetData>
<row><c t="inlineStr"><is><t>Data</t></is></c><c t="inlineStr"><is><r><t>Va</t></r><r><t>lor</t></r></is></c><c t="str"><v>Descrição</v></c></row>
<row><c t="d"><v>2026-03-02T00:00:00</v></c><c><v>-10.5</v></c><c t="str"><v>Café</v></c></row>
</sheetData></worksheet>`))
	_ = zw.Close()
	res, err := Parse(buf.Bytes(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 1 || res.Records[0].Amount.String() != "-10.5" || res.Records[0].Description != "Café" {
		t.Errorf("Records = %+v", res.Records)
	}
}

func TestMalformed(t *testing.T) {
	var empty bytes.Buffer
	zw := zip.NewWriter(&empty)
	_, _ = zw.Create("outro.txt")
	_ = zw.Close()

	var broken bytes.Buffer
	zw = zip.NewWriter(&broken)
	w, _ := zw.Create("xl/worksheets/sheet1.xml")
	_, _ = w.Write([]byte(`<worksheet><sheetData><row><c>`))
	_ = zw.Close()

	blank, _ := testgen.XLSX(nil)
	for name, in := range map[string][]byte{"não é zip": []byte("PK\x03\x04lixo"), "zip sem planilha": empty.Bytes(), "xml cortado": broken.Bytes(), "sem linhas": blank} {
		if _, err := Parse(in, Options{}); !errors.Is(err, errs.ErrMalformed) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

package htmltable

import (
	"errors"
	"testing"

	"github.com/gustavoz65/finparser-lib/internal/errs"
)

const doc = `<html><head><style>td { color: red } /* <td>não é célula</td> */</style>
<script>var x = "<tr><td>01/01/2026</td><td>1,00</td></tr>";</script></head>
<body>
<table><tr><td colspan=2>Banco Fictício</td></tr>
<tr><td>Período: 01/03/2026 a 31/03/2026</td></tr></table>
<!-- <tr><td>comentário</td></tr> -->
<TABLE border="1">
<thead><tr><th>Data</th><th>Hist&oacute;rico</th><th>Valor (R$)</th><th>Saldo (R$)</th></tr></thead>
<tr><td>02/03/2026<td><b>PIX</b> RECEBIDO<br>FULANO &amp; CIA<td align="right">1.234,56<td>2.234,56
<tr class="x"><td>03/03/2026</td><td>COMPRA
   PADARIA</td><td><font color="red">-45,90</font></td><td>2.188,66</td></tr>
<tr><td></td><td></td></tr>
</TABLE></body></html>`

func TestParse(t *testing.T) {
	res, err := Parse(doc, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 2 {
		t.Fatalf("Records = %+v", res.Records)
	}
	if r := res.Records[0]; r.Description != "PIX RECEBIDO FULANO & CIA" || r.Amount.String() != "1234.56" || r.Balance.String() != "2234.56" || r.Line != 9 {
		t.Errorf("Records[0] = %+v", r)
	}
	if r := res.Records[1]; r.Description != "COMPRA PADARIA" || r.Amount.String() != "-45.9" {
		t.Errorf("Records[1] = %+v", r)
	}
	if res.PeriodStart.IsZero() {
		t.Error("período não lido do metadado")
	}
}

func TestRowsTolerant(t *testing.T) {
	for _, in := range []string{"", "<", "<table", "<td>a", "<tr><td>a<td>b</tr", "<script>", "<!-- sem fim", "<tr><td>x</td></tr><style>"} {
		Rows(in) // não pode dar panic
	}
	if rows := Rows("<td>solta</td><td>2</td>"); len(rows) != 1 || len(rows[0].Cells) != 2 {
		t.Errorf("célula sem <tr>: %+v", rows)
	}
	if _, err := Parse("<p>sem tabela</p>", Options{}); !errors.Is(err, errs.ErrMalformed) {
		t.Errorf("err = %v", err)
	}
}

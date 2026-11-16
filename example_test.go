package finparser_test

import (
	"fmt"
	"strings"

	finparser "github.com/gustavoz65/finparser-lib"
)

func ExampleParse() {
	ofx := `OFXHEADER:100
<OFX><BANKMSGSRSV1><STMTTRNRS><STMTRS>
<BANKACCTFROM><BANKID>0260<ACCTID>1</BANKACCTFROM>
<BANKTRANLIST>
<DTSTART>20260301<DTEND>20260331
<STMTTRN><TRNTYPE>CREDIT<DTPOSTED>20260302<TRNAMT>3500.00<FITID>1<MEMO>Salário</STMTTRN>
<STMTTRN><TRNTYPE>DEBIT<DTPOSTED>20260303<TRNAMT>-45,90<FITID>2<MEMO>Padaria</STMTTRN>
</BANKTRANLIST></STMTRS></STMTTRNRS></BANKMSGSRSV1></OFX>`

	st, err := finparser.Parse(strings.NewReader(ofx))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(st.Format, st.Bank)
	for _, t := range st.Transactions {
		fmt.Println(t.Date.Format("02/01/2006"), t.Description, t.Amount.StringFixed(2), t.Kind)
	}
	// Output:
	// ofx nubank
	// 02/03/2026 Salário 3500.00 income
	// 03/03/2026 Padaria -45.90 expense
}

// Reconcile prova, pelos saldos do próprio extrato, que nada faltou.
func ExampleStatement_Reconcile() {
	csv := `Data;Histórico;Valor;Saldo
01/03/2026;SALDO ANTERIOR;;1.000,00
02/03/2026;PIX RECEBIDO DE FULANO DE TAL;250,00;1.250,00
03/03/2026;COMPRA NO DEBITO - PADARIA;-45,90;1.204,10
`
	st, err := finparser.Parse(strings.NewReader(csv))
	if err != nil {
		fmt.Println(err)
		return
	}
	rec := st.Reconcile()
	fmt.Println(rec.Status, rec.Covered, "de", rec.Total)
	for _, t := range st.Transactions {
		fmt.Printf("%s %s %q\n", t.Kind, t.Method, t.Counterparty)
	}
	// Output:
	// verified 2 de 2
	// income pix "FULANO DE TAL"
	// expense card "PADARIA"
}

// Sem sinal no valor, a lib descobre pelo saldo o que entrou e o que saiu.
func ExampleParse_signFromBalance() {
	csv := `Data;Histórico;Valor;Saldo
01/03/2026;SALDO ANTERIOR;;1.000,00
02/03/2026;LOJA A;50,00;
02/03/2026;FULANO;200,00;
02/03/2026;SALDO DO DIA;;1.150,00
`
	st, err := finparser.Parse(strings.NewReader(csv))
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, t := range st.Transactions {
		fmt.Println(t.Description, t.Amount, t.Kind)
	}
	// Output:
	// LOJA A -50 expense
	// FULANO 200 income
}

// Qualquer formato de entrada vira OFX.
func ExampleStatement_WriteOFX() {
	st, err := finparser.Parse(strings.NewReader("Data;Descrição;Valor\n02/03/2026;Mercado;-10,50\n"))
	if err != nil {
		fmt.Println(err)
		return
	}
	var out strings.Builder
	if err := st.WriteOFX(&out); err != nil {
		fmt.Println(err)
		return
	}
	i := strings.Index(out.String(), "<STMTTRN>")
	j := strings.Index(out.String(), "<FITID>")
	fmt.Println(out.String()[i:j])
	// Output:
	// <STMTTRN><TRNTYPE>DEBIT<DTPOSTED>20260302<TRNAMT>-10.50
}

func ExampleStatement_Summary() {
	st, err := finparser.Parse(strings.NewReader("Data;Descrição;Valor\n02/03/2026;Salário;3.500,00\n03/03/2026;Aluguel;-1.200,00\n"))
	if err != nil {
		fmt.Println(err)
		return
	}
	sum := st.Summary()
	fmt.Println(sum.Income, sum.Expense, sum.Net)
	// Output:
	// 3500 -1200 2300
}

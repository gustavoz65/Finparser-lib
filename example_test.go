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

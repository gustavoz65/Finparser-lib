package finparser

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

// WriteJSON grava o extrato em JSON indentado.
func (s *Statement) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

// csvHeader são as colunas de WriteCSV. Os nomes estão no dicionário de
// cabeçalhos da própria lib: o arquivo gravado é lido de volta por Parse.
var csvHeader = []string{"date", "description", "amount", "balance", "kind", "method", "counterparty", "id"}

// WriteCSV grava o extrato como CSV (RFC 4180, UTF-8): datas em ISO 8601,
// valores com ponto decimal e sinal, uma transação por linha. Parse lê o
// resultado de volta com os mesmos IDs.
func (s *Statement) WriteCSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, t := range s.Transactions {
		balance := ""
		if t.Balance != nil {
			balance = plain(*t.Balance)
		}
		// Sem sinal conhecido o valor sai sem sinal, como veio.
		amount := plain(t.Amount)
		if t.Kind == Income {
			amount = "+" + amount
		}
		if err := cw.Write([]string{
			t.Date.Format("2006-01-02"), t.Description, amount, balance,
			t.Kind.String(), string(t.Method), t.Counterparty, t.ID,
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// plain formata d com ponto decimal e pelo menos duas casas. Três casas
// viram quatro, para "1.234" não ser lido como milhar por ninguém.
func plain(d decimal.Decimal) string {
	places := -d.Exponent()
	switch {
	case places <= 2:
		places = 2
	case places == 3:
		places = 4
	}
	return d.StringFixed(places)
}

// WriteOFX grava o extrato como OFX 1.02 (SGML, Windows-1252), o formato
// que programas de contabilidade e ERPs brasileiros importam. Com Parse,
// converte qualquer extrato suportado — inclusive PDF — em OFX.
//
// O FITID de cada transação é o ID estável da lib, então reimportar o mesmo
// extrato não duplica lançamentos. O OFX não tem como dizer "sinal
// desconhecido": lançamentos Unknown saem como TRNTYPE OTHER com o valor
// como está. Extrato de cartão (Account.Type
// "creditcard") sai como CCSTMTRS. Caracteres fora do Windows-1252 viram
// "?".
func (s *Statement) WriteOFX(w io.Writer) error {
	var b strings.Builder
	date := func(t time.Time) string { return t.Format("20060102") }
	asOf := s.PeriodEnd
	if asOf.IsZero() && len(s.Transactions) > 0 {
		asOf = s.Transactions[len(s.Transactions)-1].Date
	}
	currency := s.Currency
	if currency == "" {
		currency = "BRL"
	}
	acct := Account{}
	if s.Account != nil {
		acct = *s.Account
	}
	card := acct.Type == "creditcard"

	b.WriteString("OFXHEADER:100\r\nDATA:OFXSGML\r\nVERSION:102\r\nSECURITY:NONE\r\nENCODING:USASCII\r\nCHARSET:1252\r\nCOMPRESSION:NONE\r\nOLDFILEUID:NONE\r\nNEWFILEUID:NONE\r\n\r\n")
	b.WriteString("<OFX>\r\n<SIGNONMSGSRSV1><SONRS><STATUS><CODE>0<SEVERITY>INFO</STATUS>")
	fmt.Fprintf(&b, "<DTSERVER>%s<LANGUAGE>POR", date(asOf))
	if s.Bank != "" {
		fmt.Fprintf(&b, "<FI><ORG>%s</FI>", ofxText(s.Bank))
	}
	b.WriteString("</SONRS></SIGNONMSGSRSV1>\r\n")

	number := acct.Number
	if number == "" {
		number = "0"
	}
	if card {
		b.WriteString("<CREDITCARDMSGSRSV1><CCSTMTTRNRS><TRNUID>1<STATUS><CODE>0<SEVERITY>INFO</STATUS>\r\n")
		fmt.Fprintf(&b, "<CCSTMTRS><CURDEF>%s<CCACCTFROM><ACCTID>%s</CCACCTFROM>\r\n", ofxText(currency), ofxText(number))
	} else {
		b.WriteString("<BANKMSGSRSV1><STMTTRNRS><TRNUID>1<STATUS><CODE>0<SEVERITY>INFO</STATUS>\r\n")
		bank := acct.BankID
		if bank == "" {
			bank = "0"
		}
		fmt.Fprintf(&b, "<STMTRS><CURDEF>%s<BANKACCTFROM><BANKID>%s", ofxText(currency), ofxText(bank))
		if acct.Branch != "" {
			fmt.Fprintf(&b, "<BRANCHID>%s", ofxText(acct.Branch))
		}
		kind := "CHECKING"
		if acct.Type == "savings" {
			kind = "SAVINGS"
		}
		fmt.Fprintf(&b, "<ACCTID>%s<ACCTTYPE>%s</BANKACCTFROM>\r\n", ofxText(number), kind)
	}

	b.WriteString("<BANKTRANLIST>\r\n")
	if !s.PeriodStart.IsZero() && !s.PeriodEnd.IsZero() {
		fmt.Fprintf(&b, "<DTSTART>%s<DTEND>%s\r\n", date(s.PeriodStart), date(s.PeriodEnd))
	}
	for _, t := range s.Transactions {
		kind := "OTHER"
		switch t.Kind {
		case Income:
			kind = "CREDIT"
		case Expense:
			kind = "DEBIT"
		}
		fmt.Fprintf(&b, "<STMTTRN><TRNTYPE>%s<DTPOSTED>%s<TRNAMT>%s<FITID>%s<MEMO>%s</STMTTRN>\r\n",
			kind, date(t.Date), plain(t.Amount), ofxText(t.ID), ofxText(t.Description))
	}
	b.WriteString("</BANKTRANLIST>\r\n")
	if s.ClosingBalance != nil {
		fmt.Fprintf(&b, "<LEDGERBAL><BALAMT>%s<DTASOF>%s</LEDGERBAL>\r\n", plain(*s.ClosingBalance), date(asOf))
	}
	if card {
		b.WriteString("</CCSTMTRS></CCSTMTTRNRS></CREDITCARDMSGSRSV1>\r\n</OFX>\r\n")
	} else {
		b.WriteString("</STMTRS></STMTTRNRS></BANKMSGSRSV1>\r\n</OFX>\r\n")
	}

	out, err := encoding.ReplaceUnsupported(charmap.Windows1252.NewEncoder()).String(b.String())
	if err != nil {
		return fmt.Errorf("finparser: gravando OFX: %w", err)
	}
	// O encoder troca o que não existe em Windows-1252 por 0x1A (SUB).
	_, err = io.WriteString(w, strings.ReplaceAll(out, "\x1a", "?"))
	return err
}

// ofxText escapa o que o SGML do OFX não aceita cru e tira quebras de linha.
func ofxText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\r", " ", "\n", " ").Replace(s)
}

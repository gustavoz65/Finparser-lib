// Package camt lê extratos ISO 20022: camt.053 (extrato), camt.052
// (relatório intradia) e camt.054 (notificação de débito/crédito). É o
// padrão que substitui o MT940 e o formato de extrato de bancos europeus e
// de APIs de open banking.
package camt

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/date"
	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
	"github.com/shopspring/decimal"
)

type amount struct {
	Value string `xml:",chardata"`
	Ccy   string `xml:"Ccy,attr"`
}

type dateChoice struct {
	Dt   string `xml:"Dt"`
	DtTm string `xml:"DtTm"`
}

func (d dateChoice) text() string {
	if d.Dt != "" {
		return d.Dt
	}
	return d.DtTm
}

type party struct {
	Nm string `xml:"Nm"`
	// Versões novas (camt.053.001.08+) aninham o nome em <Pty>.
	PtyNm string `xml:"Pty>Nm"`
}

func (p party) name() string {
	if p.Nm != "" {
		return p.Nm
	}
	return p.PtyNm
}

type entry struct {
	Amt      amount     `xml:"Amt"`
	Ind      string     `xml:"CdtDbtInd"`
	Reversal string     `xml:"RvslInd"`
	BookgDt  dateChoice `xml:"BookgDt"`
	ValDt    dateChoice `xml:"ValDt"`
	Ref      string     `xml:"AcctSvcrRef"`
	Info     string     `xml:"AddtlNtryInf"`
	Details  []struct {
		EndToEnd string   `xml:"Refs>EndToEndId"`
		TxID     string   `xml:"Refs>TxId"`
		Debtor   party    `xml:"RltdPties>Dbtr"`
		Creditor party    `xml:"RltdPties>Cdtr"`
		Ustrd    []string `xml:"RmtInf>Ustrd"`
		Info     string   `xml:"AddtlTxInf"`
	} `xml:"NtryDtls>TxDtls"`
}

type statement struct {
	Acct struct {
		IBAN  string `xml:"Id>IBAN"`
		Other string `xml:"Id>Othr>Id"`
		Ccy   string `xml:"Ccy"`
		BIC   string `xml:"Svcr>FinInstnId>BIC"`
		BICFI string `xml:"Svcr>FinInstnId>BICFI"`
		Bank  string `xml:"Svcr>FinInstnId>Nm"`
	} `xml:"Acct"`
	Period struct {
		From string `xml:"FrDtTm"`
		To   string `xml:"ToDtTm"`
	} `xml:"FrToDt"`
	Balances []struct {
		Code string     `xml:"Tp>CdOrPrtry>Cd"`
		Amt  amount     `xml:"Amt"`
		Ind  string     `xml:"CdtDbtInd"`
		Dt   dateChoice `xml:"Dt"`
	} `xml:"Bal"`
	Entries []entry `xml:"Ntry"`
}

type document struct {
	Stmt  []statement `xml:"BkToCstmrStmt>Stmt"`
	Rpt   []statement `xml:"BkToCstmrAcctRpt>Rpt"`
	Ntfct []statement `xml:"BkToCstmrDbtCdtNtfctn>Ntfctn"`
}

// Parse lê um documento camt já em UTF-8.
func Parse(text string) (*record.Result, error) {
	var doc document
	if err := xml.NewDecoder(strings.NewReader(text)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: camt: %v", errs.ErrMalformed, err)
	}
	stmts := append(append(doc.Stmt, doc.Rpt...), doc.Ntfct...)
	if len(stmts) == 0 {
		return nil, fmt.Errorf("%w: camt sem Stmt, Rpt nem Ntfctn", errs.ErrMalformed)
	}

	res := &record.Result{}
	line := 0
	for _, st := range stmts {
		if res.Account.Number == "" {
			res.Account = record.Account{BankID: st.Acct.BIC + st.Acct.BICFI, Number: st.Acct.IBAN + st.Acct.Other}
			res.Bank = textnorm.Spaces(st.Acct.Bank)
		}
		if res.Currency == "" {
			res.Currency = st.Acct.Ccy
		}
		if c, err := date.Parse(st.Period.From); err == nil && c.HasYear && (res.PeriodStart.IsZero() || c.Time().Before(res.PeriodStart)) {
			res.PeriodStart = c.Time()
		}
		if c, err := date.Parse(st.Period.To); err == nil && c.HasYear && c.Time().After(res.PeriodEnd) {
			res.PeriodEnd = c.Time()
		}
		// Com várias contas no arquivo os saldos não são um só.
		if len(stmts) == 1 {
			for _, b := range st.Balances {
				v, err := signed(b.Amt.Value, b.Ind, "")
				if err != nil {
					continue
				}
				switch b.Code {
				case "OPBD", "PRCD":
					if res.OpeningBalance == nil {
						res.OpeningBalance = &v
					}
				case "CLBD":
					res.ClosingBalance = &v
				}
			}
		}
		for _, e := range st.Entries {
			line++
			if res.Currency == "" {
				res.Currency = e.Amt.Ccy
			}
			if rec, ok := parseEntry(e, line, res); ok {
				res.Records = append(res.Records, rec)
			}
		}
	}
	return res, nil
}

// signed aplica CdtDbtInd ao valor: DBIT é saída. Um estorno (RvslInd)
// inverte o sentido.
func signed(value, ind, reversal string) (decimal.Decimal, error) {
	v, err := decimal.NewFromString(strings.TrimSpace(value))
	if err != nil {
		return v, err
	}
	v = v.Abs()
	debit := strings.TrimSpace(ind) == "DBIT"
	if r := strings.TrimSpace(reversal); r == "true" || r == "1" {
		debit = !debit
	}
	if debit {
		v = v.Neg()
	}
	return v, nil
}

func parseEntry(e entry, n int, res *record.Result) (record.Record, bool) {
	raw := []string{"Amt=" + strings.TrimSpace(e.Amt.Value), "CdtDbtInd=" + strings.TrimSpace(e.Ind), "BookgDt=" + e.BookgDt.text()}
	rawLine := strings.Join(raw, "; ")

	dt := e.BookgDt.text()
	if dt == "" {
		dt = e.ValDt.text()
	}
	c, err := date.Parse(dt)
	if err != nil || !c.HasYear {
		res.Warn(0, n, rawLine, fmt.Sprintf("data inválida %q", dt))
		return record.Record{}, false
	}
	ind := strings.TrimSpace(e.Ind)
	if ind != "DBIT" && ind != "CRDT" {
		res.Warn(0, n, rawLine, fmt.Sprintf("CdtDbtInd inválido %q", e.Ind))
		return record.Record{}, false
	}
	amt, err := signed(e.Amt.Value, ind, e.Reversal)
	if err != nil {
		res.Warn(0, n, rawLine, fmt.Sprintf("valor inválido %q", e.Amt.Value))
		return record.Record{}, false
	}

	// Descrição: contraparte (quem recebeu, na saída; quem pagou, na
	// entrada), texto livre da remessa e informação adicional.
	var parts []string
	add := func(s string) {
		s = textnorm.Spaces(s)
		if s == "" {
			return
		}
		k := textnorm.Key(s)
		for _, p := range parts {
			if strings.Contains(textnorm.Key(p), k) {
				return
			}
		}
		parts = append(parts, s)
	}
	id := textnorm.Spaces(e.Ref)
	for _, d := range e.Details {
		if ind == "DBIT" {
			add(d.Creditor.name())
		} else {
			add(d.Debtor.name())
		}
		add(strings.Join(d.Ustrd, " "))
		add(d.Info)
		if id == "" {
			if id = textnorm.Spaces(d.TxID); id == "" && !strings.EqualFold(strings.TrimSpace(d.EndToEnd), "NOTPROVIDED") {
				id = textnorm.Spaces(d.EndToEnd)
			}
		}
	}
	add(e.Info)

	return record.Record{
		Date:        c.Time(),
		Description: strings.Join(parts, " - "),
		Amount:      amt,
		SignKnown:   true,
		ID:          id,
		Raw:         raw,
		Line:        n,
	}, true
}

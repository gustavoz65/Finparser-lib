package camt

import (
	"errors"
	"testing"

	"github.com/gustavoz65/finparser-lib/internal/errs"
)

const sample = `<?xml version="1.0" encoding="UTF-8"?>
<Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.02">
 <BkToCstmrStmt>
  <GrpHdr><MsgId>1</MsgId></GrpHdr>
  <Stmt>
   <Id>S1</Id>
   <FrToDt><FrDtTm>2026-03-01T00:00:00</FrDtTm><ToDtTm>2026-03-31T23:59:59</ToDtTm></FrToDt>
   <Acct><Id><IBAN>BR1800360305000010009795493C1</IBAN></Id><Ccy>BRL</Ccy><Svcr><FinInstnId><BIC>FICTBRSP</BIC><Nm>Banco Fictício</Nm></FinInstnId></Svcr></Acct>
   <Bal><Tp><CdOrPrtry><Cd>OPBD</Cd></CdOrPrtry></Tp><Amt Ccy="BRL">1000.00</Amt><CdtDbtInd>CRDT</CdtDbtInd><Dt><Dt>2026-03-01</Dt></Dt></Bal>
   <Bal><Tp><CdOrPrtry><Cd>CLBD</Cd></CdOrPrtry></Tp><Amt Ccy="BRL">100.10</Amt><CdtDbtInd>DBIT</CdtDbtInd><Dt><Dt>2026-03-31</Dt></Dt></Bal>
   <Ntry>
    <Amt Ccy="BRL">3500.00</Amt><CdtDbtInd>CRDT</CdtDbtInd><Sts>BOOK</Sts>
    <BookgDt><Dt>2026-03-02</Dt></BookgDt><AcctSvcrRef>REF-1</AcctSvcrRef>
    <NtryDtls><TxDtls><Refs><EndToEndId>E2E-1</EndToEndId></Refs>
     <RltdPties><Dbtr><Nm>Empresa Fictícia Ltda</Nm></Dbtr><Cdtr><Nm>Fulana de Tal</Nm></Cdtr></RltdPties>
     <RmtInf><Ustrd>Salário</Ustrd><Ustrd>março</Ustrd></RmtInf></TxDtls></NtryDtls>
   </Ntry>
   <Ntry>
    <Amt Ccy="BRL">1.234</Amt><CdtDbtInd>DBIT</CdtDbtInd>
    <BookgDt><DtTm>2026-03-03T10:30:00+01:00</DtTm></BookgDt>
    <NtryDtls><TxDtls><Refs><EndToEndId>NOTPROVIDED</EndToEndId></Refs>
     <RltdPties><Cdtr><Pty><Nm>Padaria São João</Nm></Pty></Cdtr></RltdPties></TxDtls></NtryDtls>
    <AddtlNtryInf>Compra no débito</AddtlNtryInf>
   </Ntry>
   <Ntry><Amt Ccy="BRL">50.00</Amt><CdtDbtInd>DBIT</CdtDbtInd><RvslInd>true</RvslInd><BookgDt><Dt>2026-03-04</Dt></BookgDt><AddtlNtryInf>Estorno de tarifa</AddtlNtryInf></Ntry>
   <Ntry><Amt Ccy="BRL">abc</Amt><CdtDbtInd>DBIT</CdtDbtInd><BookgDt><Dt>2026-03-05</Dt></BookgDt></Ntry>
   <Ntry><Amt Ccy="BRL">1.00</Amt><CdtDbtInd>DBIT</CdtDbtInd><BookgDt><Dt>ontem</Dt></BookgDt></Ntry>
   <Ntry><Amt Ccy="BRL">1.00</Amt><CdtDbtInd>????</CdtDbtInd><BookgDt><Dt>2026-03-05</Dt></BookgDt></Ntry>
  </Stmt>
 </BkToCstmrStmt>
</Document>`

func TestParse(t *testing.T) {
	res, err := Parse(sample)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ date, desc, amount, id string }{
		{"2026-03-02", "Empresa Fictícia Ltda - Salário março", "3500", "REF-1"},
		// "1.234" é um e duzentos e trinta e quatro milésimos, não 1234.
		{"2026-03-03", "Padaria São João - Compra no débito", "-1.234", ""},
		{"2026-03-04", "Estorno de tarifa", "50", ""},
	}
	if len(res.Records) != len(want) {
		t.Fatalf("Records = %+v", res.Records)
	}
	for i, w := range want {
		r := res.Records[i]
		if r.Date.Format("2006-01-02") != w.date || r.Description != w.desc || r.Amount.String() != w.amount || r.ID != w.id || !r.SignKnown {
			t.Errorf("Records[%d] = %+v, want %+v", i, r, w)
		}
	}
	if len(res.Warnings) != 3 {
		t.Errorf("Warnings = %+v", res.Warnings)
	}
	if res.Currency != "BRL" || res.Bank != "Banco Fictício" || res.Account.Number != "BR1800360305000010009795493C1" || res.Account.BankID != "FICTBRSP" {
		t.Errorf("Currency = %q, Bank = %q, Account = %+v", res.Currency, res.Bank, res.Account)
	}
	if res.OpeningBalance.String() != "1000" || res.ClosingBalance.String() != "-100.1" {
		t.Errorf("saldos = %v, %v", res.OpeningBalance, res.ClosingBalance)
	}
	if res.PeriodStart.Format("2006-01-02") != "2026-03-01" || res.PeriodEnd.Format("2006-01-02") != "2026-03-31" {
		t.Errorf("período = %v a %v", res.PeriodStart, res.PeriodEnd)
	}
}

func TestOtherMessages(t *testing.T) {
	for _, in := range []string{
		`<Document><BkToCstmrAcctRpt><Rpt><Acct><Id><Othr><Id>123</Id></Othr></Id></Acct><Ntry><Amt Ccy="EUR">5</Amt><CdtDbtInd>CRDT</CdtDbtInd><ValDt><Dt>2026-03-02</Dt></ValDt></Ntry></Rpt></BkToCstmrAcctRpt></Document>`,
		`<Document><BkToCstmrDbtCdtNtfctn><Ntfctn><Ntry><Amt Ccy="EUR">5</Amt><CdtDbtInd>CRDT</CdtDbtInd><BookgDt><Dt>2026-03-02</Dt></BookgDt></Ntry></Ntfctn></BkToCstmrDbtCdtNtfctn></Document>`,
	} {
		res, err := Parse(in)
		if err != nil || len(res.Records) != 1 || res.Records[0].Amount.String() != "5" || res.Currency != "EUR" {
			t.Errorf("res = %+v, err = %v", res, err)
		}
	}
}

func TestMalformed(t *testing.T) {
	for _, in := range []string{"", "<Document>", "<Document></Document>", "<a><b></a>"} {
		if _, err := Parse(in); !errors.Is(err, errs.ErrMalformed) {
			t.Errorf("Parse(%q): err = %v", in, err)
		}
	}
}

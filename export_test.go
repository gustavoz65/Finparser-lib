package finparser

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// same compara o que tem de sobreviver a uma conversão.
func same(t *testing.T, got, want *Statement, ids bool) {
	t.Helper()
	if len(got.Transactions) != len(want.Transactions) {
		t.Fatalf("%d transações, want %d", len(got.Transactions), len(want.Transactions))
	}
	for i, w := range want.Transactions {
		g := got.Transactions[i]
		// O OFX não guarda "sinal desconhecido": Unknown volta pelo sinal.
		kind := g.Kind == w.Kind || got.Format == FormatOFX && w.Kind == Unknown
		if !g.Date.Equal(w.Date) || g.Description != w.Description || !g.Amount.Equal(w.Amount) || !kind {
			t.Errorf("transação %d = %v | %s | %s | %s\nwant           %v | %s | %s | %s",
				i, g.Date, g.Description, g.Amount, g.Kind, w.Date, w.Description, w.Amount, w.Kind)
		}
		if ids && g.ID != w.ID {
			t.Errorf("transação %d: ID %q, want %q", i, g.ID, w.ID)
		}
	}
}

// Todo fixture, convertido para OFX e para CSV, é lido de volta igual.
func TestExportRoundTrip(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("testdata", "synthetic", "*"))
	for _, f := range files {
		name := filepath.Base(f)
		st := parseFile(t, name)
		t.Run(name+"/ofx", func(t *testing.T) {
			var buf bytes.Buffer
			if err := st.WriteOFX(&buf); err != nil {
				t.Fatal(err)
			}
			back, err := Parse(&buf)
			if err != nil {
				t.Fatalf("%v\n%s", err, buf.String())
			}
			if back.Format != FormatOFX {
				t.Errorf("Format = %q", back.Format)
			}
			same(t, back, st, true)
			if !back.PeriodStart.Equal(st.PeriodStart) || !back.PeriodEnd.Equal(st.PeriodEnd) {
				t.Errorf("período = %v a %v", back.PeriodStart, back.PeriodEnd)
			}
			if st.ClosingBalance != nil && (back.ClosingBalance == nil || !back.ClosingBalance.Equal(*st.ClosingBalance)) {
				t.Errorf("ClosingBalance = %v", back.ClosingBalance)
			}
		})
		t.Run(name+"/csv", func(t *testing.T) {
			var buf bytes.Buffer
			if err := st.WriteCSV(&buf); err != nil {
				t.Fatal(err)
			}
			back, err := Parse(&buf)
			if err != nil {
				t.Fatalf("%v\n%s", err, buf.String())
			}
			same(t, back, st, true)
			for i, w := range st.Transactions {
				g := back.Transactions[i]
				if (w.Balance == nil) != (g.Balance == nil) || w.Balance != nil && !w.Balance.Equal(*g.Balance) {
					t.Errorf("transação %d: saldo %v, want %v", i, g.Balance, w.Balance)
				}
			}
		})
	}
}

func TestWriteOFXDetails(t *testing.T) {
	card := parseFile(t, "cartao.ofx")
	var buf bytes.Buffer
	if err := card.WriteOFX(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"OFXHEADER:100", "CHARSET:1252", "<CCSTMTRS>", "<CCACCTFROM>", "</CREDITCARDMSGSRSV1>"} {
		if !strings.Contains(out, want) {
			t.Errorf("OFX de cartão sem %q", want)
		}
	}

	// Escapes, acento em Windows-1252, caractere sem equivalente e valor
	// sem sinal conhecido.
	st := &Statement{
		Bank: "banco <x>",
		Transactions: []Transaction{
			{Date: ref, Description: "Pão & Cia <loja>\nlinha 2 ☕", Amount: dec("1.234"), Kind: Unknown, ID: "a&b"},
		},
	}
	buf.Reset()
	if err := st.WriteOFX(&buf); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	if !bytes.Contains(raw, []byte("P\xe3o &amp; Cia &lt;loja&gt; linha 2 ?")) {
		t.Errorf("texto não escapado/codificado: %q", raw)
	}
	for _, want := range []string{"<TRNTYPE>OTHER", "<TRNAMT>1.2340", "<FITID>a&amp;b", "<ORG>banco &lt;x&gt;", "<CURDEF>BRL", "<ACCTID>0"} {
		if !bytes.Contains(raw, []byte(want)) {
			t.Errorf("OFX sem %q:\n%s", want, raw)
		}
	}
	back, err := Parse(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if tr := back.Transactions[0]; tr.Amount.String() != "1.234" || tr.Description != "Pão & Cia <loja> linha 2 ?" || tr.ID != "a&b" {
		t.Errorf("de volta: %+v", tr)
	}
}

func TestWriteCSVAndJSON(t *testing.T) {
	st := parseFile(t, "itau_conta.pdf")
	var buf bytes.Buffer
	if err := st.WriteCSV(&buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if lines[0] != "date,description,amount,balance,kind,method,counterparty,id" {
		t.Errorf("cabeçalho = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "2026-03-02,PIX TRANSF FULANO 02/03,+200.00,1200.00,income,pix,FULANO,") {
		t.Errorf("linha 1 = %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "2026-03-03,TAR PACOTE SERVICOS,-29.90,,expense,fee,,") {
		t.Errorf("linha 2 = %q", lines[2])
	}

	buf.Reset()
	if err := st.WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var back Statement
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	same(t, &back, st, true)
	if back.OpeningBalance == nil || !back.OpeningBalance.Equal(*st.OpeningBalance) {
		t.Errorf("OpeningBalance = %v", back.OpeningBalance)
	}
}

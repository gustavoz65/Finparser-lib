package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const dir = "../../testdata/synthetic/"

func TestOutputs(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{dir + "bradesco_conta.pdf"}, []string{"pdf · bradesco · 01/03/2026 a 31/03/2026", "TARIFA BANCARIA CESTA", "saldo confere", "6 de 6 lançamentos"}},
		{[]string{"-to", "json", dir + "extrato.mt940"}, []string{`"format": "mt940"`, `"closing_balance": "4424.2"`}},
		{[]string{"-to", "csv", dir + "inter_conta.xlsx"}, []string{"date,description,amount", "2026-03-02,Pix recebido - Fulano de Tal,+250.00"}},
		{[]string{"-to", "ofx", "-check", dir + "caixa_conta.pdf"}, []string{"OFXHEADER:100", "<TRNAMT>-2000.00"}},
		{[]string{dir + "cartao.ofx"}, []string{"sem saldos para conferir"}},
		// O mesmo arquivo duas vezes: Merge não duplica.
		{[]string{"-to", "csv", dir + "nubank_conta.csv", dir + "nubank_conta.csv"}, nil},
	}
	for _, tt := range tests {
		var out bytes.Buffer
		if err := run(tt.args, &out); err != nil {
			t.Fatalf("%v: %v", tt.args, err)
		}
		for _, w := range tt.want {
			if !strings.Contains(out.String(), w) {
				t.Errorf("%v: saída sem %q:\n%s", tt.args, w, out.String())
			}
		}
	}

	var once, twice bytes.Buffer
	_ = run([]string{"-to", "csv", dir + "nubank_conta.csv"}, &once)
	_ = run([]string{"-to", "csv", dir + "nubank_conta.csv", dir + "nubank_conta.csv"}, &twice)
	if once.String() != twice.String() || once.Len() == 0 {
		t.Errorf("Merge duplicou:\n%s\n---\n%s", once.String(), twice.String())
	}
}

func TestOutputFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saida.ofx")
	var out bytes.Buffer
	if err := run([]string{"-to", "ofx", "-o", path, dir + "itau_conta.pdf"}, &out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(b, []byte("<STMTTRN>")) || out.Len() != 0 {
		t.Errorf("arquivo: %v, stdout: %q", err, out.String())
	}
}

func TestFailures(t *testing.T) {
	var out bytes.Buffer
	for _, args := range [][]string{
		nil,
		{"-to", "xml", dir + "cartao.ofx"},
		{"nao-existe.pdf"},
		{"-check", dir + "cartao.ofx"}, // sem saldo não dá para afirmar que confere
		{"-nada"},
	} {
		if err := run(args, &out); err == nil {
			t.Errorf("%v deveria falhar", args)
		}
	}
}

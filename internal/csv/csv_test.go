package csv

import (
	"testing"
	"time"
)

func TestSniff(t *testing.T) {
	tests := map[string]rune{
		"a;b;c\n1;2,5;3\n":           ';',
		"a,b,c\n1,2.5,3\n":           ',',
		"a\tb\n1\t2\n":               '\t',
		"a|b|c\n1|2|3\n":             '|',
		"Nome: X\nData;Valor\n1;2\n": ';',
		"Data,Valor,Desc\n01/03/2026,-1.00,\"a, b\"\n": ',',
	}
	for in, want := range tests {
		if got := Sniff(in); got != want {
			t.Errorf("Sniff(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseCreditDebit(t *testing.T) {
	in := "Data;Histórico;Crédito;Débito;Saldo\n01/03/2026;Saldo anterior;;;100,00\n02/03/2026;Depósito;50,00;;150,00\n03/03/2026;Compra;;20,00;130,00\n"
	res, err := Parse(in, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 2 {
		t.Fatalf("Records = %+v", res.Records)
	}
	if r := res.Records[0]; r.Amount.String() != "50" || !r.SignKnown || r.Balance.String() != "150" {
		t.Errorf("Records[0] = %+v", r)
	}
	if r := res.Records[1]; r.Amount.String() != "-20" {
		t.Errorf("Records[1] = %+v", r)
	}
}

func TestUnsignedColumn(t *testing.T) {
	in := "Data;Descrição;Valor\n01/03/2026;Compra;10,00\n02/03/2026;Pix recebido;5,00\n"
	res, err := Parse(in, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res.Records {
		if r.SignKnown {
			t.Errorf("coluna sem sinal marcada como conhecida: %+v", r)
		}
	}
}

func TestYearFromReference(t *testing.T) {
	in := "Data;Descrição;Valor\n01/03;Compra;-10,00\n02/03;Café;-5,00\n"
	res, err := Parse(in, Options{Reference: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Records[0].Date.Year() != 2027 || len(res.Warnings) != 1 {
		t.Errorf("Records = %+v, Warnings = %+v", res.Records, res.Warnings)
	}
}

func TestOnlyHeader(t *testing.T) {
	res, err := Parse("Data;Descrição;Valor\n", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 0 {
		t.Errorf("Records = %+v", res.Records)
	}
}

func TestNoColumns(t *testing.T) {
	if _, err := Parse("só texto\nsem tabela\n", Options{}); err == nil {
		t.Error("deveria falhar")
	}
}

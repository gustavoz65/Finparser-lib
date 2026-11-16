package record

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestApplySummary(t *testing.T) {
	r := &Result{}
	ApplySummary(r, "SALDO ANTERIOR", "1.000,00")
	if r.OpeningBalance == nil || r.OpeningBalance.String() != "1000" {
		t.Fatalf("OpeningBalance = %v", r.OpeningBalance)
	}
	r.Records = append(r.Records, Record{Amount: decimal.NewFromInt(-10)})
	ApplySummary(r, "Total de saídas", "10,00")
	if r.Records[0].Balance != nil {
		t.Errorf("total virou saldo: %v", r.Records[0].Balance)
	}
	ApplySummary(r, "S A L D O", "990,00")
	if b := r.Records[0].Balance; b == nil || b.String() != "990" {
		t.Errorf("Balance = %v", b)
	}
	ApplySummary(r, "Saldo final do período", "990,00")
	if r.ClosingBalance == nil || r.ClosingBalance.String() != "990" {
		t.Errorf("ClosingBalance = %v", r.ClosingBalance)
	}
	ApplySummary(r, "Saldo anterior", "5,00") // depois da primeira transação: ignora
	ApplySummary(r, "Saldo do dia", "abc")
	if r.OpeningBalance.String() != "1000" || r.Records[0].Balance.String() != "990" {
		t.Errorf("resumo tardio alterou o resultado: %+v", r)
	}
}

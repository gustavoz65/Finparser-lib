package classify

import (
	"testing"

	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/shopspring/decimal"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func bal(s string) *decimal.Decimal { d := dec(s); return &d }

func TestSign(t *testing.T) {
	recs := []record.Record{
		{Amount: dec("10"), SignKnown: true},
		{Amount: dec("-10"), SignKnown: true},
		{Amount: dec("0"), SignKnown: true},
	}
	Classify(recs, nil)
	want := []record.Kind{record.Income, record.Expense, record.Unknown}
	for i, w := range want {
		if recs[i].Kind != w {
			t.Errorf("recs[%d].Kind = %v, want %v", i, recs[i].Kind, w)
		}
	}
}

func TestPatterns(t *testing.T) {
	recs := []record.Record{
		{Description: "PIX RECEBIDO - Fulano", Amount: dec("50")},
		{Description: "Pagamento recebido", Amount: dec("50")},
		{Description: "Compra no débito", Amount: dec("30")},
		{Description: "TARIFA PACOTE", Amount: dec("-12")},
		{Description: "Fulano", Amount: dec("7")},
	}
	Classify(recs, nil)
	want := []struct {
		kind record.Kind
		amt  string
	}{
		{record.Income, "50"}, {record.Income, "50"}, {record.Expense, "-30"},
		{record.Expense, "-12"}, {record.Unknown, "7"},
	}
	for i, w := range want {
		if recs[i].Kind != w.kind || recs[i].Amount.String() != w.amt {
			t.Errorf("recs[%d] = %v %s, want %v %s", i, recs[i].Kind, recs[i].Amount, w.kind, w.amt)
		}
	}
}

func TestBalanceAscending(t *testing.T) {
	recs := []record.Record{
		{Description: "abertura", Amount: dec("100"), Balance: bal("100"), SignKnown: true},
		{Description: "x", Amount: dec("30"), Balance: bal("70")},                  // sem sinal: descobre −30
		{Description: "y", Amount: dec("20"), Balance: bal("50"), SignKnown: true}, // sinal errado: inverte
		{Description: "z", Amount: dec("5"), Balance: bal("55"), SignKnown: true},  // confere
	}
	warns := Classify(recs, nil)
	want := []string{"100", "-30", "-20", "5"}
	for i, w := range want {
		if recs[i].Amount.String() != w {
			t.Errorf("recs[%d].Amount = %s, want %s", i, recs[i].Amount, w)
		}
	}
	if len(warns) != 1 {
		t.Errorf("warns = %+v", warns)
	}
	if recs[1].Kind != record.Expense {
		t.Errorf("recs[1].Kind = %v", recs[1].Kind)
	}
}

func TestBalanceDescending(t *testing.T) {
	// Mais recente primeiro.
	recs := []record.Record{
		{Description: "c", Amount: dec("5"), Balance: bal("55")},
		{Description: "b", Amount: dec("20"), Balance: bal("50")},
		{Description: "a", Amount: dec("30"), Balance: bal("70")},
		{Description: "abertura", Amount: dec("100"), Balance: bal("100")},
	}
	Classify(recs, nil)
	want := []string{"5", "-20", "-30", "100"}
	for i, w := range want[:3] {
		if recs[i].Amount.String() != w {
			t.Errorf("recs[%d].Amount = %s, want %s", i, recs[i].Amount, w)
		}
	}
	if recs[3].Kind != record.Unknown {
		t.Errorf("abertura sem sinal deveria ficar Unknown: %v", recs[3].Kind)
	}
}

func TestBalanceMismatch(t *testing.T) {
	recs := []record.Record{
		{Amount: dec("-10"), Balance: bal("90"), SignKnown: true},
		{Amount: dec("-10"), Balance: bal("50"), SignKnown: true},
	}
	if w := Classify(recs, nil); len(w) != 1 {
		t.Errorf("warns = %+v", w)
	}
}

func TestOpeningBalance(t *testing.T) {
	recs := []record.Record{
		{Description: "x", Amount: dec("30"), Balance: bal("70")},
		{Description: "y", Amount: dec("5"), Balance: bal("75")},
	}
	Classify(recs, bal("100"))
	if recs[0].Amount.String() != "-30" || recs[0].Kind != record.Expense {
		t.Errorf("recs[0] = %+v", recs[0])
	}
	if recs[1].Kind != record.Income {
		t.Errorf("recs[1] = %+v", recs[1])
	}
}

package classify

import (
	"strings"
	"testing"
	"time"

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

// Extrato que só mostra "SALDO DO DIA": vários lançamentos sem sinal entre
// dois saldos. A combinação de sinais que fecha a soma é única.
func TestBalanceGapSolved(t *testing.T) {
	recs := []record.Record{
		{Description: "loja a", Amount: dec("50")},
		{Description: "fulano", Amount: dec("200")},
		{Description: "loja b", Amount: dec("12.34"), Balance: bal("1137.66")}, // 1000 −50 +200 −12,34
	}
	warns := Classify(recs, bal("1000"))
	want := []string{"-50", "200", "-12.34"}
	for i, w := range want {
		if recs[i].Amount.String() != w || !recs[i].SignKnown {
			t.Errorf("recs[%d] = %s signed=%v, want %s", i, recs[i].Amount, recs[i].SignKnown, w)
		}
	}
	if len(warns) != 0 || recs[1].Kind != record.Income {
		t.Errorf("warns = %+v, Kind = %v", warns, recs[1].Kind)
	}
}

// Dois valores iguais com sinais opostos: há duas soluções, então esses dois
// ficam Unknown. O terceiro tem o mesmo sinal nas duas e é resolvido.
func TestBalanceGapAmbiguous(t *testing.T) {
	recs := []record.Record{
		{Description: "a", Amount: dec("10")},
		{Description: "b", Amount: dec("10")},
		{Description: "c", Amount: dec("7"), Balance: bal("93")}, // 100 ±10 ∓10 −7
	}
	Classify(recs, bal("100"))
	if recs[0].SignKnown || recs[1].SignKnown || recs[0].Kind != record.Unknown {
		t.Errorf("sinal ambíguo foi chutado: %+v %+v", recs[0], recs[1])
	}
	if recs[2].Amount.String() != "-7" || recs[2].Kind != record.Expense {
		t.Errorf("recs[2] = %+v", recs[2])
	}
}

func TestBalanceGapKnownSignsStayFixed(t *testing.T) {
	recs := []record.Record{
		{Amount: dec("-10"), SignKnown: true},
		{Amount: dec("30")},
		{Amount: dec("5"), Balance: bal("75")}, // 100 −10 −30 +5... não fecha; −10 −30 +15? só −10 −20 fecha
	}
	// 100 → 75 exige soma −25: −10 (fixo) ±30 ±5 nunca dá −15.
	warns := Classify(recs, bal("100"))
	if len(warns) != 1 || !strings.Contains(warns[0].Message, "3 lançamentos") {
		t.Errorf("warns = %+v", warns)
	}
	if recs[1].SignKnown {
		t.Errorf("trecho sem solução não pode definir sinal: %+v", recs[1])
	}
}

func TestBalanceGapDescending(t *testing.T) {
	// Mais recente primeiro; o saldo aparece só na última linha de cada dia.
	recs := []record.Record{
		{Description: "d3", Amount: dec("60"), Balance: bal("865")}, // 805 + 60
		{Description: "d2b", Amount: dec("100")},                    // 905 − 100
		{Description: "d2a", Amount: dec("5"), Balance: bal("905")},
		{Description: "d1", Amount: dec("200")},
	}
	Classify(recs, nil)
	if recs[0].Amount.String() != "60" || recs[1].Amount.String() != "-100" || !recs[1].SignKnown {
		t.Errorf("recs = %s, %s", recs[0].Amount, recs[1].Amount)
	}
	if recs[3].SignKnown {
		t.Errorf("fora de trecho não pode ganhar sinal: %+v", recs[3])
	}
}

func TestBalanceGapTooLarge(t *testing.T) {
	recs := make([]record.Record, 40)
	for i := range recs {
		recs[i] = record.Record{Amount: dec("1")}
	}
	recs[39].Balance = bal("0")
	if warns := Classify(recs, bal("0")); len(warns) != 0 || recs[0].SignKnown {
		t.Errorf("trecho grande deveria ser deixado em paz: %+v", warns)
	}
	// Precisão absurda também não entra na busca.
	weird := []record.Record{{Amount: dec("0.00000000001")}, {Amount: dec("1"), Balance: bal("1")}}
	Classify(weird, bal("0"))
}

// Milhares de trechos sem solução: o orçamento é do extrato inteiro, então
// o custo não cresce com o número de trechos.
func TestBalanceGapBudget(t *testing.T) {
	recs := make([]record.Record, 24*2000)
	for i := range recs {
		recs[i] = record.Record{Amount: dec("2")}
		if i%24 == 23 {
			recs[i].Balance = bal("1") // diferença 0 com 24 valores pares fecha; com saldo fixo, não
		}
	}
	recs[23].Balance = bal("0.5")
	start := time.Now()
	Classify(recs, bal("1"))
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("Classify levou %v", d)
	}
}

func BenchmarkBalanceGap(b *testing.B) {
	base := make([]record.Record, 20)
	sum := decimal.Zero
	for i := range base {
		v := decimal.New(int64(1000+i*137+i*i*11), -2)
		if i%3 == 0 {
			v = v.Neg()
		}
		sum = sum.Add(v)
		base[i] = record.Record{Amount: v.Abs()}
	}
	base[19].Balance = &sum
	zero := decimal.Zero
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		recs := append([]record.Record(nil), base...)
		Classify(recs, &zero)
	}
}

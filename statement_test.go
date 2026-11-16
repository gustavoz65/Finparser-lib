package finparser

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func bal(s string) *decimal.Decimal { d := dec(s); return &d }

func TestSummary(t *testing.T) {
	st := parseFile(t, "nubank_conta.pdf")
	sum := st.Summary()
	if sum.Count != 6 || sum.IncomeCount != 2 || sum.ExpenseCount != 4 || sum.UnknownCount != 0 {
		t.Errorf("contagens = %+v", sum)
	}
	if sum.Income.String() != "3500.87" || sum.Expense.String() != "-1321.7" || sum.Net.String() != "2179.17" {
		t.Errorf("totais = %s %s %s", sum.Income, sum.Expense, sum.Net)
	}

	// Sem sinal conhecido: fora dos totais.
	unsigned, err := Parse(strings.NewReader("Data;Descrição;Valor\n01/03/2026;Loja;10,00\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s := unsigned.Summary(); s.UnknownCount != 1 || !s.Net.IsZero() {
		t.Errorf("Summary = %+v", s)
	}
}

// Todo fixture que traz saldo tem de fechar: é a prova de que o parser não
// perdeu nem inventou lançamento.
func TestReconcileFixtures(t *testing.T) {
	want := map[string]struct {
		status  ReconcileStatus
		covered int
	}{
		"nubank_conta.pdf":      {Verified, 6},
		"itau_conta.pdf":        {Verified, 5},
		"bradesco_conta.pdf":    {Verified, 6},
		"caixa_conta.pdf":       {Verified, 5},
		"inter_conta.xlsx":      {Verified, 4},
		"inter_conta.csv":       {Verified, 2}, // sem saldo anterior: a primeira linha não tem com o que comparar
		"santander_conta.html":  {Verified, 3},
		"caixa_tipo_dc.csv":     {Verified, 2},
		"extrato_camt053.xml":   {Verified, 3},
		"extrato.mt940":         {Verified, 3},
		"bb_credito_debito.csv": {Verified, 4},
		"cartao.ofx":            {Unverifiable, 0},
		"conta.qif":             {Unverifiable, 0},
	}
	for name, w := range want {
		t.Run(name, func(t *testing.T) {
			rec := parseFile(t, name).Reconcile()
			if rec.Status != w.status || rec.Covered != w.covered || rec.Failed != 0 {
				t.Errorf("Reconcile = %+v", rec)
			}
		})
	}
}

func TestReconcileDetectsMissingTransaction(t *testing.T) {
	st := parseFile(t, "caixa_conta.pdf")
	// Some um lançamento do meio: a conta do trecho seguinte não fecha.
	st.Transactions = append(st.Transactions[:2], st.Transactions[3:]...)
	rec := st.Reconcile()
	if rec.Status != Mismatch || rec.Failed != 1 || len(rec.Mismatches) != 1 {
		t.Fatalf("Reconcile = %+v", rec)
	}
	m := rec.Mismatches[0]
	if m.Index != 2 || m.Expected.String() != "-2012.34" || m.Actual.String() != "-12.34" {
		t.Errorf("Mismatch = %+v", m)
	}
}

func TestReconcileTotalsAndDescending(t *testing.T) {
	tx := func(amount string, balance *decimal.Decimal) Transaction {
		return Transaction{Amount: dec(amount), Balance: balance}
	}
	// Só saldo inicial e final: uma conta, cobre tudo.
	st := &Statement{OpeningBalance: bal("100"), ClosingBalance: bal("75"), Transactions: []Transaction{tx("-30", nil), tx("5", nil)}}
	if rec := st.Reconcile(); rec.Status != Verified || rec.Checked != 1 || rec.Covered != 2 {
		t.Errorf("totais: %+v", rec)
	}
	st.ClosingBalance = bal("80")
	if rec := st.Reconcile(); rec.Status != Mismatch || rec.Mismatches[0].Index != -1 {
		t.Errorf("total errado: %+v", rec)
	}
	// Mais novo primeiro.
	desc := &Statement{Transactions: []Transaction{tx("5", bal("55")), tx("-20", bal("50")), tx("-30", bal("70"))}}
	if rec := desc.Reconcile(); rec.Status != Verified || rec.Checked != 2 || rec.Covered != 2 {
		t.Errorf("decrescente: %+v", rec)
	}
	if rec := (&Statement{}).Reconcile(); rec.Status != Unverifiable {
		t.Errorf("vazio: %+v", rec)
	}
}

func TestMerge(t *testing.T) {
	a, err := Parse(strings.NewReader("Data;Descrição;Valor\n01/03/2026;A;-1,00\n02/03/2026;B;-2,00\n02/03/2026;B;-2,00\n"))
	if err != nil {
		t.Fatal(err)
	}
	// Sobrepõe o dia 02 (duas compras iguais) e acrescenta o dia 03.
	b, err := Parse(strings.NewReader("Data;Descrição;Valor\n03/03/2026;C;-3,00\n02/03/2026;B;-2,00\n02/03/2026;B;-2,00\n"))
	if err != nil {
		t.Fatal(err)
	}
	m := Merge(a, nil, b)
	var got []string
	for _, tr := range m.Transactions {
		got = append(got, tr.Description)
	}
	if strings.Join(got, "") != "ABBC" {
		t.Errorf("Merge = %v", got)
	}
	if m.Format != FormatCSV || m.PeriodStart.Day() != 1 || m.PeriodEnd.Day() != 3 {
		t.Errorf("Format = %q, período %v a %v", m.Format, m.PeriodStart, m.PeriodEnd)
	}
	other := parseFile(t, "cartao.ofx")
	if mixed := Merge(a, other); mixed.Format != "" || mixed.Account != nil || len(mixed.Transactions) != len(a.Transactions)+len(other.Transactions) {
		t.Errorf("Merge de formatos diferentes = %+v", mixed)
	}
	if empty := Merge(); empty.Transactions == nil || len(empty.Transactions) != 0 {
		t.Errorf("Merge() = %+v", empty)
	}
}

func TestEntryPoints(t *testing.T) {
	path := filepath.Join("testdata", "synthetic", "inter_conta.xlsx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if f := DetectFormat(data); f != FormatXLSX {
		t.Errorf("DetectFormat = %q", f)
	}
	if f := DetectFormat([]byte{0, 1, 2}); f != "" {
		t.Errorf("DetectFormat(binário) = %q", f)
	}
	fromFile, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fromBytes, err := ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(fromFile.Transactions) != 4 || fromFile.Transactions[3].ID != fromBytes.Transactions[3].ID {
		t.Errorf("ParseFile e ParseBytes divergem")
	}
	if _, err := ParseFile(filepath.Join("testdata", "não-existe")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("arquivo ausente: %v", err)
	}
	if _, err := ParseBytes(data, WithMaxBytes(10)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("limite: %v", err)
	}
}

func TestOldExcelIsRejectedClearly(t *testing.T) {
	_, err := Parse(bytes.NewReader([]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0, 0, 0, 0}))
	if !errors.Is(err, ErrUnknownFormat) || !strings.Contains(err.Error(), ".xlsx") {
		t.Errorf("err = %v", err)
	}
}

func TestErrorMessageHasSinglePrefix(t *testing.T) {
	_, err := Parse(strings.NewReader("Data;Descrição;Valor\n"))
	if err == nil || strings.Count(err.Error(), "finparser:") != 1 {
		t.Errorf("err = %v", err)
	}
}

// Parse não tem estado global: roda em paralelo (o CI usa -race).
func TestConcurrentParse(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("testdata", "synthetic", "*"))
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		for _, f := range files {
			wg.Add(1)
			go func(f string) {
				defer wg.Done()
				if _, err := ParseFile(f, WithReferenceDate(ref)); err != nil {
					t.Errorf("%s: %v", f, err)
				}
			}(f)
		}
	}
	wg.Wait()
}

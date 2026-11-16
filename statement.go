package finparser

import (
	"sort"

	"github.com/shopspring/decimal"
)

// Summary são os totais de um extrato.
type Summary struct {
	Count        int `json:"count"`
	IncomeCount  int `json:"income_count"`
	ExpenseCount int `json:"expense_count"`
	// UnknownCount são os lançamentos em que nenhuma regra decidiu o sinal.
	// Eles ficam fora de Income, Expense e Net.
	UnknownCount int `json:"unknown_count"`
	// Income é a soma das receitas (positiva); Expense, a das despesas
	// (negativa); Net, a soma das duas.
	Income  decimal.Decimal `json:"income"`
	Expense decimal.Decimal `json:"expense"`
	Net     decimal.Decimal `json:"net"`
}

// Summary soma receitas e despesas.
func (s *Statement) Summary() Summary {
	sum := Summary{Count: len(s.Transactions)}
	for _, t := range s.Transactions {
		switch t.Kind {
		case Income:
			sum.IncomeCount++
			sum.Income = sum.Income.Add(t.Amount)
		case Expense:
			sum.ExpenseCount++
			sum.Expense = sum.Expense.Add(t.Amount)
		default:
			sum.UnknownCount++
		}
	}
	sum.Net = sum.Income.Add(sum.Expense)
	return sum
}

// ReconcileStatus é o veredito de Reconcile.
type ReconcileStatus string

// Vereditos da reconciliação.
const (
	// Verified: todas as contas possíveis fecharam.
	Verified ReconcileStatus = "verified"
	// Mismatch: alguma conta não fechou; veja Mismatches.
	Mismatch ReconcileStatus = "mismatch"
	// Unverifiable: o extrato não traz saldos para conferir.
	Unverifiable ReconcileStatus = "unverifiable"
)

// BalanceMismatch é uma conta que não fechou.
type BalanceMismatch struct {
	// Index é a transação que carrega o saldo conferido; −1 quando a conta
	// é a do extrato inteiro (saldo inicial + lançamentos = saldo final).
	Index int `json:"index"`
	// Expected é a variação de saldo declarada; Actual, a soma dos
	// lançamentos do trecho.
	Expected decimal.Decimal `json:"expected"`
	Actual   decimal.Decimal `json:"actual"`
}

// Reconciliation é o resultado de conferir o extrato com os saldos que ele
// mesmo declara.
type Reconciliation struct {
	Status ReconcileStatus `json:"status"`
	// Checked é quantas contas foram feitas: uma por trecho entre dois
	// saldos conhecidos, mais a do extrato inteiro quando há saldo inicial
	// e final. Failed é quantas não fecharam.
	Checked int `json:"checked"`
	Failed  int `json:"failed"`
	// Covered é quantos lançamentos estão dentro de alguma conta que
	// fechou; Total, quantos o extrato tem. Covered == Total com Status
	// Verified significa que cada lançamento foi conferido.
	Covered    int               `json:"covered"`
	Total      int               `json:"total"`
	Mismatches []BalanceMismatch `json:"mismatches,omitempty"`
}

// Reconcile confere o extrato contra os saldos que ele declara. Entre dois
// saldos conhecidos, a soma dos lançamentos tem de dar a diferença; com
// saldo inicial e final, a soma de tudo tem de levar de um ao outro.
//
// É a prova de que a leitura está completa: um lançamento perdido, repetido
// ou com valor errado faz alguma conta falhar. Funciona com o extrato em
// ordem crescente ou decrescente de data.
func (s *Statement) Reconcile() Reconciliation {
	rec := Reconciliation{Status: Unverifiable, Total: len(s.Transactions)}
	asc, desc := s.segments(false), s.segments(true)
	segs := asc
	if passing(desc) > passing(asc) {
		segs = desc
	}

	covered := make([]bool, len(s.Transactions))
	for _, sg := range segs {
		rec.Checked++
		if !sg.ok() {
			rec.Failed++
			rec.Mismatches = append(rec.Mismatches, BalanceMismatch{Index: sg.at, Expected: sg.expected, Actual: sg.actual})
			continue
		}
		for i := sg.from; i < sg.to; i++ {
			covered[i] = true
		}
	}

	if s.OpeningBalance != nil && s.ClosingBalance != nil {
		rec.Checked++
		total := decimal.Zero
		for _, t := range s.Transactions {
			total = total.Add(t.Amount)
		}
		if expected := s.ClosingBalance.Sub(*s.OpeningBalance); expected.Equal(total) {
			for i := range covered {
				covered[i] = true
			}
		} else {
			rec.Failed++
			rec.Mismatches = append(rec.Mismatches, BalanceMismatch{Index: -1, Expected: expected, Actual: total})
		}
	}

	for _, c := range covered {
		if c {
			rec.Covered++
		}
	}
	switch {
	case rec.Failed > 0:
		rec.Status = Mismatch
	case rec.Checked > 0:
		rec.Status = Verified
	}
	return rec
}

// span é um trecho [from, to) de transações entre dois saldos.
type span struct {
	from, to         int
	at               int
	expected, actual decimal.Decimal
}

func (sp span) ok() bool { return sp.expected.Equal(sp.actual) }

func passing(spans []span) int {
	n := 0
	for _, sp := range spans {
		if sp.ok() {
			n++
		}
	}
	return n
}

// segments monta os trechos entre saldos. Em ordem crescente o saldo de uma
// linha é o de depois dela; em ordem decrescente (mais novo primeiro)
// também, só que as linhas anteriores no tempo vêm depois no arquivo.
func (s *Statement) segments(descending bool) []span {
	ts := s.Transactions
	sum := func(from, to int) decimal.Decimal {
		total := decimal.Zero
		for _, t := range ts[from:to] {
			total = total.Add(t.Amount)
		}
		return total
	}
	var anchors []int
	for i, t := range ts {
		if t.Balance != nil {
			anchors = append(anchors, i)
		}
	}
	if len(anchors) == 0 {
		return nil
	}
	var out []span
	if !descending {
		if s.OpeningBalance != nil {
			a := anchors[0]
			out = append(out, span{0, a + 1, a, ts[a].Balance.Sub(*s.OpeningBalance), sum(0, a+1)})
		}
		for k := 1; k < len(anchors); k++ {
			p, c := anchors[k-1], anchors[k]
			out = append(out, span{p + 1, c + 1, c, ts[c].Balance.Sub(*ts[p].Balance), sum(p+1, c+1)})
		}
		return out
	}
	for k := 1; k < len(anchors); k++ {
		p, c := anchors[k-1], anchors[k]
		out = append(out, span{p, c, p, ts[p].Balance.Sub(*ts[c].Balance), sum(p, c)})
	}
	if s.OpeningBalance != nil {
		a := anchors[len(anchors)-1]
		out = append(out, span{a, len(ts), a, ts[a].Balance.Sub(*s.OpeningBalance), sum(a, len(ts))})
	}
	return out
}

// Merge junta extratos numa linha do tempo só, sem repetir lançamentos:
// duas exportações do mesmo período que se sobrepõem (janeiro–março e
// março–abril) geram os mesmos IDs para os lançamentos em comum, e cada um
// entra uma vez. O resultado sai em ordem de data.
//
// A deduplicação é por ID, então vale para arquivos do mesmo banco e
// formato; o mesmo lançamento num PDF e num OFX tem descrições diferentes e
// conta como dois. Saldos inicial e final não são propagados.
func Merge(statements ...*Statement) *Statement {
	out := &Statement{Transactions: []Transaction{}}
	seen := map[string]bool{}
	first := true
	for _, st := range statements {
		if st == nil {
			continue
		}
		if first {
			out.Format, out.Bank, out.Currency, out.Account = st.Format, st.Bank, st.Currency, st.Account
			out.PeriodStart, out.PeriodEnd = st.PeriodStart, st.PeriodEnd
			first = false
		} else {
			if out.Format != st.Format {
				out.Format = ""
			}
			if out.Bank != st.Bank {
				out.Bank = ""
			}
			if out.Currency != st.Currency {
				out.Currency = ""
			}
			if out.Account == nil || st.Account == nil || *out.Account != *st.Account {
				out.Account = nil
			}
			if !st.PeriodStart.IsZero() && (out.PeriodStart.IsZero() || st.PeriodStart.Before(out.PeriodStart)) {
				out.PeriodStart = st.PeriodStart
			}
			if st.PeriodEnd.After(out.PeriodEnd) {
				out.PeriodEnd = st.PeriodEnd
			}
		}
		for _, t := range st.Transactions {
			if t.ID != "" {
				if seen[t.ID] {
					continue
				}
				seen[t.ID] = true
			}
			out.Transactions = append(out.Transactions, t)
		}
		out.Warnings = append(out.Warnings, st.Warnings...)
	}
	sort.SliceStable(out.Transactions, func(i, j int) bool {
		return out.Transactions[i].Date.Before(out.Transactions[j].Date)
	})
	return out
}

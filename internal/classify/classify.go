// Package classify decide se cada lançamento é receita ou despesa.
//
// v1, nesta ordem: sinal do valor; padrões conhecidos na descrição quando a
// fonte não informa sinal; validação cruzada com o saldo, quando existe.
// Nada bateu → Unknown. Nunca chuta.
package classify

import (
	"fmt"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
	"github.com/shopspring/decimal"
)

var (
	incomePatterns = []string{
		"pix recebido", "transferencia recebida", "ted recebida", "doc recebido",
		"pagamento recebido", "salario", "rendimento", "deposito", "estorno",
		"reembolso", "credito em conta", "resgate",
	}
	expensePatterns = []string{
		"pix enviado", "transferencia enviada", "ted enviada", "doc enviado",
		"compra", "pagamento", "tarifa", "saque", "debito automatico", "iof",
		"juros", "anuidade", "aplicacao",
	}
)

// Classify ajusta sinal e Kind de recs no lugar e devolve avisos. opening é
// o saldo anterior à primeira transação, ou nil.
func Classify(recs []record.Record, opening *decimal.Decimal) []record.Warning {
	warns := checkBalance(recs, opening)
	for i := range recs {
		r := &recs[i]
		if !r.SignKnown {
			byPattern(r)
		}
		r.Kind = kindOf(*r)
	}
	return warns
}

func kindOf(r record.Record) record.Kind {
	if !r.SignKnown {
		return record.Unknown
	}
	switch r.Amount.Sign() {
	case 1:
		return record.Income
	case -1:
		return record.Expense
	}
	return record.Unknown
}

func byPattern(r *record.Record) {
	k := textnorm.Key(r.Description)
	abs := r.Amount.Abs()
	for _, p := range incomePatterns {
		if strings.Contains(k, p) {
			r.Amount, r.SignKnown = abs, true
			return
		}
	}
	for _, p := range expensePatterns {
		if strings.Contains(k, p) {
			r.Amount, r.SignKnown = abs.Neg(), true
			return
		}
	}
}

// checkBalance usa saldo[i] − saldo[i−1] == valor[i] para confirmar ou
// descobrir o sinal. Detecta se o extrato está em ordem crescente ou
// decrescente de data pela direção que explica mais linhas.
func checkBalance(recs []record.Record, opening *decimal.Decimal) []record.Warning {
	type pair struct{ prev, cur int } // cur é a linha cujo valor explica a diferença
	var asc, desc []pair
	// O saldo anterior entra como uma linha virtual de índice −1 (ordem
	// crescente) ou len(recs) (ordem decrescente).
	balanceAt := func(i int) *decimal.Decimal {
		if i < 0 || i >= len(recs) {
			return opening
		}
		return recs[i].Balance
	}
	last := -1
	for i := range recs {
		if recs[i].Balance == nil {
			continue
		}
		if last >= 0 {
			asc = append(asc, pair{last, i})
			desc = append(desc, pair{i, last})
		}
		last = i
	}
	if opening != nil && len(recs) > 0 {
		if recs[0].Balance != nil {
			asc = append([]pair{{-1, 0}}, asc...)
		}
		if n := len(recs); recs[n-1].Balance != nil {
			desc = append(desc, pair{n, n - 1})
		}
	}
	if len(asc) == 0 && len(desc) == 0 {
		return nil
	}

	matches := func(ps []pair) int {
		n := 0
		for _, p := range ps {
			diff := recs[p.cur].Balance.Sub(*balanceAt(p.prev))
			if diff.Abs().Equal(recs[p.cur].Amount.Abs()) {
				n++
			}
		}
		return n
	}
	pairs := asc
	if matches(desc) > matches(asc) {
		pairs = desc
	}

	var warns []record.Warning
	for _, p := range pairs {
		r := &recs[p.cur]
		// Só pares adjacentes: com linha sem saldo no meio a soma não fecha.
		if gap := p.cur - p.prev; gap != 1 && gap != -1 {
			continue
		}
		diff := r.Balance.Sub(*balanceAt(p.prev))
		switch {
		case diff.Equal(r.Amount):
			r.SignKnown = true
		case diff.Equal(r.Amount.Neg()) && !r.Amount.IsZero():
			if r.SignKnown {
				warns = append(warns, warn(*r, fmt.Sprintf("sinal invertido pela validação de saldo (%s → %s)", r.Amount, diff)))
			}
			r.Amount, r.SignKnown = diff, true
		default:
			warns = append(warns, warn(*r, fmt.Sprintf("saldo não confere: diferença %s, valor %s", diff, r.Amount)))
		}
	}
	return warns
}

func warn(r record.Record, msg string) record.Warning {
	return record.Warning{Page: r.Page, Line: r.Line, Message: msg, Raw: strings.Join(r.Raw, " | ")}
}

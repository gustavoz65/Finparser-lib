// Package classify decide se cada lançamento é receita ou despesa.
//
// v1, nesta ordem: sinal do valor; padrões conhecidos na descrição quando a
// fonte não informa sinal; validação cruzada com o saldo, quando existe.
// Nada bateu → Unknown. Nunca chuta.
package classify

import (
	"fmt"
	"sort"
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

// segment é um trecho entre dois saldos conhecidos: a soma dos valores de
// items tem de dar diff.
type segment struct {
	items []int
	diff  decimal.Decimal
	at    int // lançamento que carrega o saldo, para pendurar avisos
}

// segments monta os trechos na ordem crescente de data (o saldo de uma
// linha é o saldo depois dela e das anteriores) ou decrescente (extrato com
// o lançamento mais novo primeiro). O saldo anterior entra como âncora
// antes da primeira linha (crescente) ou depois da última (decrescente).
func segments(recs []record.Record, opening *decimal.Decimal, descending bool) []segment {
	var anchors []int
	for i := range recs {
		if recs[i].Balance != nil {
			anchors = append(anchors, i)
		}
	}
	if len(anchors) == 0 {
		return nil
	}
	span := func(from, to int) []int {
		out := make([]int, 0, to-from)
		for i := from; i < to; i++ {
			out = append(out, i)
		}
		return out
	}
	var segs []segment
	if !descending {
		if opening != nil {
			a := anchors[0]
			segs = append(segs, segment{span(0, a+1), recs[a].Balance.Sub(*opening), a})
		}
		for k := 1; k < len(anchors); k++ {
			p, c := anchors[k-1], anchors[k]
			segs = append(segs, segment{span(p+1, c+1), recs[c].Balance.Sub(*recs[p].Balance), c})
		}
		return segs
	}
	for k := 1; k < len(anchors); k++ {
		p, c := anchors[k-1], anchors[k]
		segs = append(segs, segment{span(p, c), recs[p].Balance.Sub(*recs[c].Balance), p})
	}
	if opening != nil {
		a := anchors[len(anchors)-1]
		segs = append(segs, segment{span(a, len(recs)), recs[a].Balance.Sub(*opening), a})
	}
	return segs
}

// checkBalance usa a diferença entre saldos para confirmar ou descobrir o
// sinal dos valores. Com um lançamento entre dois saldos, a conta é direta:
// saldo[i] − saldo[i−1] == valor[i]. Com vários (extrato que só mostra
// "SALDO DO DIA"), procura a combinação de sinais que fecha a soma e só
// aceita o sinal de um lançamento se ele for o mesmo em todas as
// combinações possíveis.
//
// Detecta se o extrato está em ordem crescente ou decrescente de data pela
// direção que explica mais trechos.
func checkBalance(recs []record.Record, opening *decimal.Decimal) []record.Warning {
	asc, desc := segments(recs, opening, false), segments(recs, opening, true)
	// Um orçamento só para o extrato inteiro: muitos trechos difíceis não
	// podem somar segundos de busca.
	budget := maxNodes
	segs := asc
	if explained(recs, desc, &budget) > explained(recs, asc, &budget) {
		segs = desc
	}

	var warns []record.Warning
	for _, sg := range segs {
		if len(sg.items) == 1 {
			r := &recs[sg.items[0]]
			switch {
			case sg.diff.Equal(r.Amount):
				r.SignKnown = true
			case sg.diff.Equal(r.Amount.Neg()) && !r.Amount.IsZero():
				if r.SignKnown {
					warns = append(warns, warn(*r, fmt.Sprintf("sinal invertido pela validação de saldo (%s → %s)", r.Amount, sg.diff)))
				}
				r.Amount, r.SignKnown = sg.diff, true
			default:
				warns = append(warns, warn(*r, fmt.Sprintf("saldo não confere: diferença %s, valor %s", sg.diff, r.Amount)))
			}
			continue
		}

		signs, state := solve(recs, sg, false, &budget)
		switch state {
		case solved:
			for k, i := range sg.items {
				r := &recs[i]
				if r.SignKnown || signs[k] == 0 {
					continue
				}
				if signs[k] < 0 {
					r.Amount = r.Amount.Abs().Neg()
				} else {
					r.Amount = r.Amount.Abs()
				}
				r.SignKnown = true
			}
		case impossible:
			warns = append(warns, warn(recs[sg.at], fmt.Sprintf("saldo não confere: diferença %s em %d lançamentos", sg.diff, len(sg.items))))
		}
	}
	return warns
}

// explained conta os trechos que fecham com alguma combinação de sinais.
func explained(recs []record.Record, segs []segment, budget *int) int {
	n := 0
	for _, sg := range segs {
		if _, state := solve(recs, sg, true, budget); state == solved {
			n++
		}
	}
	return n
}

type solveState int

const (
	impossible solveState = iota // nenhuma combinação fecha a soma
	solved
	gaveUp // trecho grande demais ou valores fora do alcance de int64
)

const (
	maxFree   = 24      // lançamentos sem sinal por trecho
	maxNodes  = 1 << 22 // orçamento da busca, por extrato
	maxScale  = 6       // casas decimais
	maxAmount = 1 << 52 // em unidades mínimas
)

// solve procura os sinais dos lançamentos de sg que fazem a soma dar
// sg.diff. Lançamentos com sinal conhecido ficam fixos, a menos que allFree.
// Devolve, por item, +1 ou −1 quando o sinal é o mesmo em todas as soluções
// e 0 quando varia ou já era fixo.
//
// É soma de subconjuntos com sinal: busca em profundidade sobre inteiros
// (centavos), maiores valores primeiro, cortando o ramo quando o que falta
// é maior que a soma do que resta. budget é o número de nós que ainda pode
// visitar; quando acaba, desiste sem mexer em nada.
func solve(recs []record.Record, sg segment, allFree bool, budget *int) ([]int8, solveState) {
	scale := int32(0)
	for _, i := range sg.items {
		scale = max(scale, -recs[i].Amount.Exponent())
	}
	scale = max(scale, -sg.diff.Exponent())
	if scale > maxScale {
		return nil, gaveUp
	}
	target, ok := units(sg.diff, scale)
	if !ok {
		return nil, gaveUp
	}

	type item struct {
		v int64
		k int // posição em sg.items
	}
	var free []item
	for k, i := range sg.items {
		v, ok := units(recs[i].Amount, scale)
		if !ok {
			return nil, gaveUp
		}
		switch {
		case v == 0:
		case recs[i].SignKnown && !allFree:
			target -= v
		default:
			free = append(free, item{abs(v), k})
		}
	}
	if len(free) > maxFree {
		return nil, gaveUp
	}
	sort.Slice(free, func(a, b int) bool { return free[a].v > free[b].v })
	rest := make([]int64, len(free)+1) // rest[i] = soma de free[i:]
	for i := len(free) - 1; i >= 0; i-- {
		rest[i] = rest[i+1] + free[i].v
	}

	var (
		cur      = make([]int8, len(free))
		pos, neg = make([]bool, len(free)), make([]bool, len(free))
		found    bool
	)
	var walk func(i int, left int64) bool
	walk = func(i int, left int64) bool {
		if *budget--; *budget < 0 {
			return false
		}
		if abs(left) > rest[i] {
			return true
		}
		if i == len(free) {
			found = true
			for j, s := range cur {
				if s > 0 {
					pos[j] = true
				} else {
					neg[j] = true
				}
			}
			return true
		}
		cur[i] = 1
		if !walk(i+1, left-free[i].v) {
			return false
		}
		cur[i] = -1
		return walk(i+1, left+free[i].v)
	}
	if !walk(0, target) {
		return nil, gaveUp
	}
	if !found {
		return nil, impossible
	}
	signs := make([]int8, len(sg.items))
	for j, it := range free {
		switch {
		case pos[j] && !neg[j]:
			signs[it.k] = 1
		case neg[j] && !pos[j]:
			signs[it.k] = -1
		}
	}
	return signs, solved
}

// units converte d para inteiro em unidades de 10^-scale.
func units(d decimal.Decimal, scale int32) (int64, bool) {
	s := d.Shift(scale)
	if !s.IsInteger() || s.Abs().GreaterThan(decimal.New(maxAmount, 0)) {
		return 0, false
	}
	return s.IntPart(), true
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func warn(r record.Record, msg string) record.Warning {
	return record.Warning{Page: r.Page, Line: r.Line, Message: msg, Raw: strings.Join(r.Raw, " | ")}
}

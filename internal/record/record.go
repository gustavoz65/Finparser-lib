// Package record define a representação intermediária de transações que os
// parsers internos produzem antes de virarem tipos públicos.
package record

import (
	"time"

	"github.com/shopspring/decimal"
)

// Kind espelha finparser.Kind.
type Kind int

// Tipos de lançamento.
const (
	Unknown Kind = iota
	Income
	Expense
)

// Record é uma transação já interpretada.
type Record struct {
	Date        time.Time
	Description string
	Amount      decimal.Decimal
	Balance     *decimal.Decimal
	// SignKnown indica que a fonte informou o sinal (valor com -, D/C,
	// colunas separadas de crédito e débito, OFX).
	SignKnown bool
	Kind      Kind
	ID        string // ID vindo da fonte (FITID); vazio se não houver
	Raw       []string
	Page      int
	Line      int
}

// Warning registra algo que o parser não conseguiu interpretar.
type Warning struct {
	Page    int
	Line    int
	Message string
	Raw     string
}

// Result é a saída de um parser de formato.
type Result struct {
	Bank        string
	PeriodStart time.Time
	PeriodEnd   time.Time
	// OpeningBalance é o saldo antes da primeira transação ("SALDO
	// ANTERIOR"), quando o extrato informa.
	OpeningBalance *decimal.Decimal
	Records        []Record
	Warnings       []Warning
}

// Warn acrescenta um aviso.
func (r *Result) Warn(page, line int, raw, msg string) {
	r.Warnings = append(r.Warnings, Warning{Page: page, Line: line, Message: msg, Raw: raw})
}

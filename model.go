package finparser

import (
	"time"

	"github.com/shopspring/decimal"
)

// Format é o formato de origem do extrato.
type Format string

// Formatos suportados.
const (
	FormatOFX Format = "ofx"
	FormatQIF Format = "qif"
	FormatCSV Format = "csv"
	FormatPDF Format = "pdf"
)

// Kind diz se o lançamento é receita ou despesa.
type Kind int

// Tipos de lançamento. Unknown significa que nenhuma regra conseguiu
// decidir; a lib nunca chuta.
const (
	Unknown Kind = iota
	Income       // receita
	Expense      // despesa
)

// String devolve "unknown", "income" ou "expense".
func (k Kind) String() string {
	switch k {
	case Income:
		return "income"
	case Expense:
		return "expense"
	}
	return "unknown"
}

// MarshalText faz Kind sair como texto em JSON.
func (k Kind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// UnmarshalText aceita o texto produzido por MarshalText.
func (k *Kind) UnmarshalText(b []byte) error {
	switch string(b) {
	case "income":
		*k = Income
	case "expense":
		*k = Expense
	default:
		*k = Unknown
	}
	return nil
}

// Location aponta de onde uma transação ou aviso veio. Page é 0 em formatos
// sem página (OFX, CSV); Line começa em 1.
type Location struct {
	Page int `json:"page,omitempty"`
	Line int `json:"line,omitempty"`
}

// Transaction é um lançamento padronizado.
type Transaction struct {
	// Date é a data civil: meia-noite UTC, sem fuso.
	Date time.Time `json:"date"`
	// Description vem com espaços normalizados.
	Description string `json:"description"`
	// Amount é negativo para saída de dinheiro.
	Amount decimal.Decimal `json:"amount"`
	// Balance é o saldo após o lançamento; nil quando o extrato não traz.
	Balance *decimal.Decimal `json:"balance,omitempty"`
	Kind    Kind             `json:"kind"`
	// ID é o FITID do OFX quando único, senão um hash estável de data,
	// valor, descrição e ordinal. O mesmo arquivo sempre gera os mesmos IDs.
	ID string `json:"id"`
	// Raw guarda as colunas brutas, para depuração.
	Raw    []string `json:"raw,omitempty"`
	Source Location `json:"source"`
}

// Warning registra uma linha que parecia transação mas não foi interpretada,
// ou um ajuste feito pelo parser (ex.: sinal invertido pelo saldo).
type Warning struct {
	Source  Location `json:"source"`
	Message string   `json:"message"`
	Raw     string   `json:"raw,omitempty"`
}

// Statement é o extrato interpretado.
type Statement struct {
	Format Format `json:"format"`
	// Bank é "nubank", "itau"... ou "" se não identificado.
	Bank         string        `json:"bank,omitempty"`
	PeriodStart  time.Time     `json:"period_start"`
	PeriodEnd    time.Time     `json:"period_end"`
	Transactions []Transaction `json:"transactions"`
	Warnings     []Warning     `json:"warnings,omitempty"`
}

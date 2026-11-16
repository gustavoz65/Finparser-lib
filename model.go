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
	// FormatXLSX é a planilha Excel (.xlsx).
	FormatXLSX Format = "xlsx"
	// FormatHTML é o extrato exportado como tabela HTML (muitos ".xls" de
	// banco são isso por dentro).
	FormatHTML Format = "html"
	// FormatCAMT é o extrato ISO 20022 (camt.052, camt.053, camt.054).
	FormatCAMT Format = "camt053"
	// FormatMT940 é o extrato SWIFT MT940/MT942.
	FormatMT940 Format = "mt940"
)

// Method é o meio de pagamento de um lançamento, lido da descrição.
type Method string

// Meios de pagamento. A string vazia significa que a descrição não deixou
// claro.
const (
	MethodPix         Method = "pix"
	MethodTED         Method = "ted"
	MethodDOC         Method = "doc"
	MethodBoleto      Method = "boleto"
	MethodCard        Method = "card"
	MethodWithdrawal  Method = "withdrawal"
	MethodDeposit     Method = "deposit"
	MethodCheck       Method = "check"
	MethodDirectDebit Method = "direct_debit"
	MethodFee         Method = "fee"
	MethodTax         Method = "tax"
	MethodInterest    Method = "interest"
	MethodInvestment  Method = "investment"
	MethodTransfer    Method = "transfer"
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
	// Method é o meio de pagamento (Pix, TED, boleto, cartão...), quando a
	// descrição deixa claro.
	Method Method `json:"method,omitempty"`
	// Counterparty é quem pagou ou recebeu, quando a descrição segue um
	// padrão conhecido ("PIX RECEBIDO DE FULANO"). Vazio na dúvida.
	Counterparty string `json:"counterparty,omitempty"`
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

// Account identifica a conta do extrato. Só os campos que a fonte informa
// vêm preenchidos.
type Account struct {
	// BankID é o código do banco (COMPE, no OFX brasileiro) ou o BIC.
	BankID string `json:"bank_id,omitempty"`
	// Branch é a agência.
	Branch string `json:"branch,omitempty"`
	// Number é o número da conta, do cartão ou o IBAN.
	Number string `json:"number,omitempty"`
	// Type é "checking", "savings", "creditcard"...
	Type string `json:"type,omitempty"`
}

// Statement é o extrato interpretado.
type Statement struct {
	Format Format `json:"format"`
	// Bank é "nubank", "itau"... ou "" se não identificado.
	Bank string `json:"bank,omitempty"`
	// Account é nil quando a fonte não identifica a conta.
	Account *Account `json:"account,omitempty"`
	// Currency é o código ISO 4217 ("BRL"), quando a fonte informa.
	Currency    string    `json:"currency,omitempty"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	// OpeningBalance é o saldo antes da primeira transação ("SALDO
	// ANTERIOR"); ClosingBalance, o saldo final declarado. nil quando o
	// extrato não traz. Reconcile usa os dois para provar que nada faltou.
	OpeningBalance *decimal.Decimal `json:"opening_balance,omitempty"`
	ClosingBalance *decimal.Decimal `json:"closing_balance,omitempty"`
	Transactions   []Transaction    `json:"transactions"`
	Warnings       []Warning        `json:"warnings,omitempty"`
}

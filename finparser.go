// Package finparser lê extratos bancários (OFX, QIF, CSV, XLSX, HTML, PDF
// com camada de texto, ISO 20022 camt.053 e SWIFT MT940) e devolve
// transações padronizadas: data civil, descrição limpa, valor decimal
// exato, saldo opcional, tipo (receita/despesa), meio de pagamento e
// contraparte. O foco são os extratos de bancos brasileiros.
//
// Além de ler, a lib confere: Statement.Reconcile prova, pelos saldos que o
// próprio extrato declara, que nenhum lançamento faltou ou veio com o valor
// errado. E converte: WriteOFX, WriteCSV e WriteJSON gravam o extrato
// padronizado, então um PDF vira OFX em duas chamadas.
//
// Parse é seguro para uso concorrente: a lib não tem estado global.
//
// Uso básico:
//
//	f, _ := os.Open("extrato.ofx")
//	st, err := finparser.Parse(f)
//	if err != nil {
//		// errors.Is(err, finparser.ErrUnknownFormat) etc.
//	}
//	for _, t := range st.Transactions {
//		fmt.Println(t.Date.Format("02/01/2006"), t.Description, t.Amount, t.Kind)
//	}
//
// Valores monetários usam github.com/shopspring/decimal; nunca float64.
package finparser

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/camt"
	"github.com/gustavoz65/finparser-lib/internal/classify"
	"github.com/gustavoz65/finparser-lib/internal/csv"
	"github.com/gustavoz65/finparser-lib/internal/detect"
	"github.com/gustavoz65/finparser-lib/internal/enrich"
	"github.com/gustavoz65/finparser-lib/internal/htmltable"
	"github.com/gustavoz65/finparser-lib/internal/mt940"
	"github.com/gustavoz65/finparser-lib/internal/ofx"
	"github.com/gustavoz65/finparser-lib/internal/pdf"
	"github.com/gustavoz65/finparser-lib/internal/profile"
	"github.com/gustavoz65/finparser-lib/internal/qif"
	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/table"
	"github.com/gustavoz65/finparser-lib/internal/textenc"
	"github.com/gustavoz65/finparser-lib/internal/xlsx"
)

// Parse lê um extrato de r. O formato é detectado pelos bytes, a menos que
// WithFormat seja usado. Lê no máximo WithMaxBytes bytes (padrão 20 MB).
func Parse(r io.Reader, opts ...Option) (*Statement, error) {
	cfg := newConfig(opts)

	data, err := io.ReadAll(io.LimitReader(r, cfg.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("finparser: lendo entrada: %w", err)
	}
	if int64(len(data)) > cfg.maxBytes {
		return nil, fmt.Errorf("%w (%d bytes)", ErrTooLarge, cfg.maxBytes)
	}
	return parseBytes(data, cfg)
}

// ParseBytes é Parse para quem já tem o arquivo em memória. O limite de
// WithMaxBytes vale do mesmo jeito.
func ParseBytes(data []byte, opts ...Option) (*Statement, error) {
	cfg := newConfig(opts)
	if int64(len(data)) > cfg.maxBytes {
		return nil, fmt.Errorf("%w (%d bytes)", ErrTooLarge, cfg.maxBytes)
	}
	return parseBytes(data, cfg)
}

// ParseFile abre e lê o arquivo em path. O formato vem dos bytes, nunca da
// extensão.
func ParseFile(path string, opts ...Option) (*Statement, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("finparser: %w", err)
	}
	defer func() { _ = f.Close() }()
	return Parse(f, opts...)
}

// DetectFormat diz o formato de data olhando os bytes, sem interpretar o
// arquivo. Devolve "" quando não reconhece.
func DetectFormat(data []byte) Format {
	switch detect.Detect(data) {
	case detect.PDF:
		return FormatPDF
	case detect.OFX:
		return FormatOFX
	case detect.QIF:
		return FormatQIF
	case detect.CSV:
		return FormatCSV
	case detect.XLSX:
		return FormatXLSX
	case detect.HTML:
		return FormatHTML
	case detect.CAMT:
		return FormatCAMT
	case detect.MT940:
		return FormatMT940
	}
	return ""
}

func parseBytes(data []byte, cfg config) (*Statement, error) {
	format := cfg.format
	if format == "" {
		format = DetectFormat(data)
		if format == "" {
			if detect.Detect(data) == detect.XLS {
				return nil, fmt.Errorf("%w: Excel binário antigo (.xls) não é suportado; exporte como .xlsx ou CSV", ErrUnknownFormat)
			}
			return nil, ErrUnknownFormat
		}
	}
	if cfg.logger != nil {
		cfg.logger.Debug("finparser: formato", "format", format, "bytes", len(data))
	}

	var (
		res *record.Result
		err error
	)
	tab := table.Options{Reference: cfg.reference}
	if p, ok := profile.Lookup(cfg.bank); ok {
		tab.Profile = p
	}
	switch format {
	case FormatOFX:
		text, _ := textenc.ToUTF8(data)
		res, err = ofx.Parse(string(text))
	case FormatCSV:
		text, _ := textenc.ToUTF8(data)
		res, err = csv.Parse(string(text), tab)
	case FormatXLSX:
		res, err = xlsx.Parse(data, tab)
	case FormatHTML:
		text, _ := textenc.ToUTF8(data)
		res, err = htmltable.Parse(string(text), tab)
	case FormatCAMT:
		text, _ := textenc.ToUTF8(data)
		res, err = camt.Parse(string(text))
	case FormatMT940:
		text, _ := textenc.ToUTF8(data)
		res, err = mt940.Parse(string(text))
	case FormatPDF:
		opt := pdf.Options{
			Password:     cfg.password,
			RowTolerance: cfg.rowTolerance,
			Reference:    cfg.reference,
		}
		if p, ok := profile.Lookup(cfg.bank); ok {
			opt.Profile = p
		}
		res, err = pdf.Parse(data, opt)
	case FormatQIF:
		text, _ := textenc.ToUTF8(data)
		res, err = qif.Parse(string(text))
	default:
		return nil, fmt.Errorf("%w: formato %q não suportado", ErrUnknownFormat, format)
	}
	if err != nil {
		return nil, &ParseError{Format: format, Err: err}
	}

	if cfg.bank != "" {
		if p, ok := profile.Lookup(cfg.bank); ok {
			res.Bank = p.Name()
		} else {
			res.Bank = cfg.bank
		}
	}
	return finish(format, res)
}

// finish classifica, gera IDs estáveis e converte para os tipos públicos.
func finish(format Format, res *record.Result) (*Statement, error) {
	if len(res.Records) == 0 {
		err := error(ErrNoTransactions)
		if len(res.Warnings) > 0 {
			w := res.Warnings[0]
			err = fmt.Errorf("%w (%d linhas ignoradas; primeira: %s)", ErrNoTransactions, len(res.Warnings), w.Message)
		}
		return nil, &ParseError{Format: format, Err: err}
	}

	warns := classify.Classify(res.Records, res.OpeningBalance)
	res.Warnings = append(res.Warnings, warns...)

	st := &Statement{
		Format:         format,
		Bank:           res.Bank,
		Currency:       res.Currency,
		PeriodStart:    res.PeriodStart,
		PeriodEnd:      res.PeriodEnd,
		OpeningBalance: res.OpeningBalance,
		ClosingBalance: res.ClosingBalance,
		Transactions:   make([]Transaction, len(res.Records)),
	}
	if res.Account != (record.Account{}) {
		st.Account = &Account{BankID: res.Account.BankID, Branch: res.Account.Branch, Number: res.Account.Number, Type: res.Account.Type}
	}
	ids := assignIDs(res.Records)
	for i, r := range res.Records {
		st.Transactions[i] = Transaction{
			Date:         r.Date,
			Description:  r.Description,
			Amount:       r.Amount,
			Balance:      r.Balance,
			Kind:         Kind(r.Kind),
			Method:       Method(enrich.Method(r.Description)),
			Counterparty: enrich.Counterparty(r.Description),
			ID:           ids[i],
			Raw:          r.Raw,
			Source:       Location{Page: r.Page, Line: r.Line},
		}
		// Sem período declarado na fonte: usa a menor e a maior data.
		if res.PeriodStart.IsZero() && (st.PeriodStart.IsZero() || r.Date.Before(st.PeriodStart)) {
			st.PeriodStart = r.Date
		}
		if res.PeriodEnd.IsZero() && r.Date.After(st.PeriodEnd) {
			st.PeriodEnd = r.Date
		}
	}
	for _, w := range res.Warnings {
		st.Warnings = append(st.Warnings, Warning{
			Source:  Location{Page: w.Page, Line: w.Line},
			Message: w.Message,
			Raw:     w.Raw,
		})
	}
	return st, nil
}

// assignIDs usa o ID da fonte quando ele é único no arquivo; senão gera um
// hash estável de data + valor + descrição + ordinal (quantas transações
// idênticas apareceram antes).
func assignIDs(recs []record.Record) []string {
	count := map[string]int{}
	for _, r := range recs {
		if r.ID != "" {
			count[r.ID]++
		}
	}
	seen := map[string]int{}
	ids := make([]string, len(recs))
	for i, r := range recs {
		if r.ID != "" && count[r.ID] == 1 {
			ids[i] = r.ID
			continue
		}
		key := strings.Join([]string{r.Date.Format("2006-01-02"), r.Amount.String(), r.Description}, "\x1f")
		ord := seen[key]
		seen[key]++
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x1f%d", key, ord)))
		ids[i] = hex.EncodeToString(sum[:12])
	}
	return ids
}

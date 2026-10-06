// Package csv lê extratos em CSV: descobre o delimitador, a linha de
// cabeçalho (o que vem antes é metadado) e o papel de cada coluna.
package csv

import (
	stdcsv "encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/date"
	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/header"
	"github.com/gustavoz65/finparser-lib/internal/money"
	"github.com/gustavoz65/finparser-lib/internal/profile"
	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
	"github.com/shopspring/decimal"
)

// Options ajusta o parser.
type Options struct {
	// Reference completa o ano de datas sem ano quando o período é
	// desconhecido.
	Reference time.Time
	// Profile, se não nil, força o perfil de banco.
	Profile profile.Profile
}

var delimiters = []rune{';', ',', '\t', '|'}

// Sniff escolhe o delimitador que gera o número de campos mais consistente
// (e > 1) nas primeiras 20 linhas não vazias. Brasil usa muito ';' porque
// ',' é separador decimal.
func Sniff(text string) rune {
	var sample []string
	for _, ln := range strings.Split(text, "\n") {
		if strings.TrimSpace(ln) != "" {
			sample = append(sample, ln)
			if len(sample) == 20 {
				break
			}
		}
	}
	joined := strings.Join(sample, "\n")

	best, bestScore := ';', -1
	for _, d := range delimiters {
		r := newReader(joined, d)
		freq := map[int]int{}
		for {
			rec, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				continue
			}
			freq[len(rec)]++
		}
		mode, n := 0, 0
		for fields, c := range freq {
			if c > n || c == n && fields > mode {
				mode, n = fields, c
			}
		}
		if mode > 1 && n > bestScore {
			best, bestScore = d, n
		}
	}
	return best
}

func newReader(text string, delim rune) *stdcsv.Reader {
	r := stdcsv.NewReader(strings.NewReader(text))
	r.Comma = delim
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	r.ReuseRecord = false
	return r
}

type row struct {
	cells []string
	line  int
}

// Parse lê um CSV já em UTF-8.
func Parse(text string, opt Options) (*record.Result, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	res := &record.Result{}

	r := newReader(text, Sniff(text))
	var rows []row
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		line, _ := r.FieldPos(0)
		if err != nil {
			var pe *stdcsv.ParseError
			if errors.As(err, &pe) {
				line = pe.StartLine
			}
			res.Warn(0, line, "", fmt.Sprintf("linha CSV inválida: %v", err))
			continue
		}
		for i := range rec {
			rec[i] = textnorm.Spaces(rec[i])
		}
		if strings.Join(rec, "") == "" {
			continue
		}
		rows = append(rows, row{cells: rec, line: line})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: CSV vazio", errs.ErrMalformed)
	}

	// Cabeçalho: primeira linha com ≥ 2 palavras-chave. Antes dela, metadado.
	headerAt := -1
	var m header.Mapping
	for i, rw := range rows {
		if hm, ok := header.FromHeader(rw.cells); ok {
			headerAt, m = i, hm
			break
		}
	}
	var meta []string
	data := rows
	if headerAt >= 0 {
		for _, rw := range rows[:headerAt+1] {
			meta = append(meta, strings.Join(rw.cells, " "))
		}
		data = rows[headerAt+1:]
	} else {
		cells := make([][]string, len(rows))
		for i, rw := range rows {
			cells[i] = rw.cells
		}
		hm, ok := header.Infer(cells)
		if !ok {
			return nil, fmt.Errorf("%w: CSV sem cabeçalho reconhecível nem colunas de data e valor", errs.ErrMalformed)
		}
		m = hm
	}
	metaText := strings.Join(meta, "\n")

	if s, e, ok := date.FindPeriod(metaText); ok {
		res.PeriodStart, res.PeriodEnd = s, e
	}
	prof := opt.Profile
	if prof == nil {
		prof = profile.Detect(metaText)
	}
	var skip []*regexp.Regexp
	if prof != nil {
		res.Bank = prof.Name()
		skip = prof.Hints().Skip
	}

	signed := m.Has(header.Credit) || m.Has(header.Debit) || columnSigned(data, m[header.Amount])

	cell := func(rw row, role header.Role) string {
		i, ok := m[role]
		if !ok || i >= len(rw.cells) {
			return ""
		}
		return rw.cells[i]
	}

	guessedYear := false
	for _, rw := range data {
		raw := strings.Join(rw.cells, " | ")
		desc := cell(rw, header.Description)
		if header.IsSummary(desc) {
			record.ApplySummary(res, desc, cell(rw, header.Balance))
			continue
		}
		if matchAny(skip, desc) || matchAny(skip, raw) {
			continue
		}
		// Cabeçalho repetido no meio do arquivo.
		if _, ok := header.FromHeader(rw.cells); ok {
			continue
		}

		dc, err := date.Parse(cell(rw, header.Date))
		if err != nil {
			if looksTransactional(rw.cells) {
				res.Warn(0, rw.line, raw, fmt.Sprintf("data inválida %q", cell(rw, header.Date)))
			}
			continue
		}

		amt, ok := amount(rw, m, cell)
		if !ok {
			res.Warn(0, rw.line, raw, "valor ausente ou inválido")
			continue
		}

		when, guessed := dc.Resolve(res.PeriodEnd, opt.Reference)
		if guessed && !guessedYear {
			guessedYear = true
			res.Warn(0, rw.line, raw, fmt.Sprintf("data sem ano e período desconhecido; assumido %d", when.Year()))
		}

		rec := record.Record{
			Date:        when,
			Description: desc,
			Amount:      amt,
			SignKnown:   signed,
			Raw:         rw.cells,
			Line:        rw.line,
		}
		if id := cell(rw, header.Document); len(id) >= 16 {
			// Identificador longo (UUID do Nubank) é estável; número de
			// documento curto se repete entre extratos e não serve de ID.
			rec.ID = id
		}
		if b := cell(rw, header.Balance); b != "" {
			if v, err := money.Parse(b); err == nil {
				rec.Balance = &v
			}
		}
		res.Records = append(res.Records, rec)
	}
	return res, nil
}

// amount lê o valor da linha: coluna única ou crédito − débito.
func amount(rw row, m header.Mapping, cell func(row, header.Role) string) (decimal.Decimal, bool) {
	if m.Has(header.Amount) {
		if v, err := money.Parse(cell(rw, header.Amount)); err == nil {
			return v, true
		}
		if !m.Has(header.Credit) && !m.Has(header.Debit) {
			return decimal.Decimal{}, false
		}
	}
	cr, dr := cell(rw, header.Credit), cell(rw, header.Debit)
	if cr == "" && dr == "" {
		return decimal.Decimal{}, false
	}
	total := decimal.Zero
	if cr != "" {
		v, err := money.Parse(cr)
		if err != nil {
			return decimal.Decimal{}, false
		}
		total = total.Add(v.Abs())
	}
	if dr != "" {
		v, err := money.Parse(dr)
		if err != nil {
			return decimal.Decimal{}, false
		}
		total = total.Sub(v.Abs())
	}
	return total, true
}

// columnSigned informa se algum valor da coluna traz sinal explícito. Uma
// coluna em que nenhum valor tem sinal não diz se é entrada ou saída.
func columnSigned(rows []row, col int) bool {
	for _, rw := range rows {
		if col < len(rw.cells) {
			if v, err := money.ParseValue(rw.cells[col]); err == nil && v.Signed {
				return true
			}
		}
	}
	return false
}

// looksTransactional: a linha tem algum valor monetário, então parecia
// transação e merece aviso em vez de ser ignorada em silêncio.
func looksTransactional(cells []string) bool {
	for _, c := range cells {
		if money.LooksLike(c) {
			return true
		}
	}
	return false
}

func matchAny(res []*regexp.Regexp, s string) bool {
	if len(res) == 0 {
		return false
	}
	k := textnorm.Key(s)
	for _, re := range res {
		if re.MatchString(k) {
			return true
		}
	}
	return false
}

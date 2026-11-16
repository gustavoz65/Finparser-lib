// Package table transforma linhas tabulares (CSV, planilha) em transações:
// acha a linha de cabeçalho (o que vem antes é metadado), descobre o papel
// de cada coluna e interpreta data, valor, sinal e saldo.
package table

import (
	"fmt"
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

// Row é uma linha da tabela com sua posição na fonte.
type Row struct {
	Cells []string
	Line  int
}

// Parse interpreta as linhas. Células devem vir com espaços normalizados.
func Parse(rows []Row, opt Options) (*record.Result, error) {
	res := &record.Result{}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: tabela vazia", errs.ErrMalformed)
	}

	// Cabeçalho: primeira linha com ≥ 2 palavras-chave. Antes dela, metadado.
	headerAt := -1
	var m header.Mapping
	var descCols []int
	idColumn := false // a coluna de documento é um identificador único
	for i, rw := range rows {
		if hm, ok := header.FromHeader(rw.Cells); ok {
			headerAt, m = i, hm
			descCols = header.DescriptionColumns(rw.Cells)
			if c, ok := hm[header.Document]; ok {
				idColumn = header.IsID(rw.Cells[c])
			}
			break
		}
	}
	var meta []string
	data := rows
	if headerAt >= 0 {
		for _, rw := range rows[:headerAt+1] {
			meta = append(meta, strings.Join(rw.Cells, " "))
		}
		data = rows[headerAt+1:]
	} else {
		cells := make([][]string, len(rows))
		for i, rw := range rows {
			cells[i] = rw.Cells
		}
		hm, ok := header.Infer(cells)
		if !ok {
			return nil, fmt.Errorf("%w: sem cabeçalho reconhecível nem colunas de data e valor", errs.ErrMalformed)
		}
		m = hm
		if i, ok := m[header.Description]; ok {
			descCols = []int{i}
		}
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

	cell := func(rw Row, role header.Role) string {
		i, ok := m[role]
		if !ok || i >= len(rw.Cells) {
			return ""
		}
		return rw.Cells[i]
	}

	// A coluna "Tipo" só vale como sinal se as células forem D/C; em muitos
	// bancos ela traz "PIX", "TED"...
	signCol := m.Has(header.Sign) && signColumn(data, m[header.Sign])
	signed := signCol || m.Has(header.Credit) || m.Has(header.Debit) || columnSigned(data, m, header.Amount)

	guessedYear := false
	for _, rw := range data {
		raw := strings.Join(rw.Cells, " | ")
		desc := description(rw, descCols)
		if header.IsSummary(desc) {
			bal := cell(rw, header.Balance)
			if bal == "" {
				bal = cell(rw, header.Amount)
			}
			record.ApplySummary(res, desc, bal)
			continue
		}
		if matchAny(skip, desc) || matchAny(skip, raw) {
			continue
		}
		// Cabeçalho repetido no meio do arquivo.
		if _, ok := header.FromHeader(rw.Cells); ok {
			continue
		}

		dc, err := date.Parse(cell(rw, header.Date))
		if err != nil {
			if looksTransactional(rw.Cells) {
				res.Warn(0, rw.Line, raw, fmt.Sprintf("data inválida %q", cell(rw, header.Date)))
			}
			continue
		}

		amt, ok := amount(rw, m, cell)
		if !ok {
			res.Warn(0, rw.Line, raw, "valor ausente ou inválido")
			continue
		}
		rowSigned := signed
		if signCol {
			switch header.SignOf(cell(rw, header.Sign)) {
			case -1:
				amt = amt.Abs().Neg()
			case 1:
				amt = amt.Abs()
			default:
				rowSigned = columnSigned([]Row{rw}, m, header.Amount)
			}
		}

		when, guessed := dc.Resolve(res.PeriodEnd, opt.Reference)
		if guessed && !guessedYear {
			guessedYear = true
			res.Warn(0, rw.Line, raw, fmt.Sprintf("data sem ano e período desconhecido; assumido %d", when.Year()))
		}

		rec := record.Record{
			Date:        when,
			Description: desc,
			Amount:      amt,
			SignKnown:   rowSigned,
			Raw:         rw.Cells,
			Line:        rw.Line,
		}
		if id := cell(rw, header.Document); id != "" && (idColumn || len(id) >= 16) {
			// Coluna "ID" ou identificador longo (UUID) é estável; número
			// de documento curto se repete entre extratos e não serve.
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

// description junta as colunas de descrição sem repetir texto.
func description(rw Row, cols []int) string {
	out, key := "", ""
	for _, i := range cols {
		if i >= len(rw.Cells) || rw.Cells[i] == "" {
			continue
		}
		c := rw.Cells[i]
		k := textnorm.Key(c)
		switch {
		case out == "":
			out, key = c, k
		case strings.Contains(key, k):
		case strings.Contains(k, key):
			out, key = c, k
		default:
			out, key = out+" - "+c, key+" - "+k
		}
	}
	return out
}

// amount lê o valor da linha: coluna única ou crédito − débito.
func amount(rw Row, m header.Mapping, cell func(Row, header.Role) string) (decimal.Decimal, bool) {
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
func columnSigned(rows []Row, m header.Mapping, role header.Role) bool {
	col, ok := m[role]
	if !ok {
		return false
	}
	for _, rw := range rows {
		if col < len(rw.Cells) {
			if v, err := money.ParseValue(rw.Cells[col]); err == nil && v.Signed {
				return true
			}
		}
	}
	return false
}

// signColumn: pelo menos uma célula preenchida e todas elas são D/C.
func signColumn(rows []Row, col int) bool {
	n := 0
	for _, rw := range rows {
		if col >= len(rw.Cells) || rw.Cells[col] == "" {
			continue
		}
		if !header.IsSignCell(rw.Cells[col]) {
			return false
		}
		n++
	}
	return n > 0
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

// Package pdf transforma um PDF com camada de texto em transações:
// extração de tokens → motor espacial → colunas → linhas de transação.
package pdf

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/date"
	"github.com/gustavoz65/finparser-lib/internal/header"
	"github.com/gustavoz65/finparser-lib/internal/money"
	"github.com/gustavoz65/finparser-lib/internal/pdf/extract"
	"github.com/gustavoz65/finparser-lib/internal/pdf/spatial"
	"github.com/gustavoz65/finparser-lib/internal/profile"
	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
	"github.com/shopspring/decimal"
)

// Options ajusta o parser.
type Options struct {
	Password     string
	RowTolerance float64 // múltiplo da fonte; padrão 0,5
	Reference    time.Time
	Profile      profile.Profile // força o perfil; nil detecta
	Extractor    extract.Extractor
}

// headerGap: dentro do cabeçalho, palavras a até 1 × fonte formam a mesma
// célula ("Valor (R$)").
const headerGap = 1.0

// Parse lê o PDF.
func Parse(data []byte, opt Options) (*record.Result, error) {
	ex := opt.Extractor
	if ex == nil {
		ex = extract.Default{}
	}
	toks, err := ex.Extract(data, opt.Password)
	if err != nil {
		return nil, err
	}
	res := &record.Result{}

	if bad := extract.GarbledPages(toks); len(bad) > 0 {
		skip := map[int]bool{}
		for _, p := range bad {
			skip[p] = true
			res.Warn(p, 0, "", "página ilegível (fonte sem mapa de caracteres); ignorada")
		}
		kept := toks[:0:0]
		for _, t := range toks {
			if !skip[t.Page] {
				kept = append(kept, t)
			}
		}
		toks = kept
	}

	lines := spatial.Build(toks, opt.RowTolerance)
	if len(lines) == 0 {
		return res, nil
	}

	// Começo do documento: identifica banco e período antes da tabela.
	head := headText(lines)
	prof := opt.Profile
	if prof == nil {
		prof = profile.Detect(head)
	}
	var skip []*regexp.Regexp
	if prof != nil {
		res.Bank = prof.Name()
		skip = prof.Hints().Skip
	}
	// O período fica no cabeçalho do documento: basta a primeira página.
	if s, e, ok := date.FindPeriod(firstPageText(lines)); ok {
		res.PeriodStart, res.PeriodEnd = s, e
	}

	lines = spatial.RemoveNoise(lines, func(l spatial.Line) bool {
		_, isHeader := headerOf(l)
		return isHeader || hasMoney(l)
	})

	p := &parser{res: res, opt: opt, skip: skip}
	p.run(lines)
	return res, nil
}

// headText devolve o texto das 15 primeiras e 5 últimas linhas da primeira
// página: cabeçalho e rodapé, onde o banco se identifica. O miolo fica de
// fora porque descrições citam outros bancos.
func headText(lines []spatial.Line) string {
	n := 0
	for n < len(lines) && lines[n].Page == lines[0].Page {
		n++
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		if i < 15 || i >= n-5 {
			sb.WriteString(lines[i].Text())
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func firstPageText(lines []spatial.Line) string {
	var sb strings.Builder
	for _, l := range lines {
		if l.Page != lines[0].Page {
			break
		}
		sb.WriteString(l.Text())
		sb.WriteByte('\n')
	}
	return sb.String()
}

// headerOf tenta ler a linha como cabeçalho de tabela.
func headerOf(l spatial.Line) (layout, bool) {
	cells := spatial.Cells(l, headerGap)
	texts := make([]string, len(cells))
	for i, c := range cells {
		texts[i] = c.Text
	}
	m, ok := header.FromHeader(texts)
	if !ok {
		return layout{}, false
	}
	bands := make([]spatial.Band, len(cells))
	for i, c := range cells {
		bands[i] = spatial.Band{X0: c.X0, X1: c.X1}
	}
	return layout{bands: bands, m: m}, true
}

func hasMoney(l spatial.Line) bool {
	for _, w := range l.Words {
		if money.LooksLike(w.Text) {
			return true
		}
	}
	return false
}

// layout é o mapeamento de colunas em uso: faixas X e papel de cada uma.
type layout struct {
	bands []spatial.Band
	m     header.Mapping
}

func (lay layout) valid() bool { return len(lay.bands) > 0 }

type parser struct {
	res  *record.Result
	opt  Options
	skip []*regexp.Regexp

	lastDate    time.Time
	hasDate     bool
	sectionSign int // +1 depois de "Total de entradas", −1 depois de "Total de saídas"
	guessedYear bool

	line int // linha visual atual dentro da página (1 = topo)

	// última transação aberta, para continuação de descrição
	open     bool
	openY    float64
	openPage int
}

// sectionRe reconhece linhas que abrem um grupo com sinal implícito
// (Nubank: "Total de entradas + 1.000,00", "Total de saídas - 45,90").
var sectionRe = regexp.MustCompile(`^(total de )?(entradas|saidas|creditos|debitos)\b`)

func (p *parser) run(lines []spatial.Line) {
	// Lê cada linha como cabeçalho uma vez só; o resultado serve à busca do
	// primeiro cabeçalho, ao corpo de cada tabela e ao laço principal.
	heads := make([]*layout, len(lines))
	hasHeader := false
	for i, l := range lines {
		if hl, ok := headerOf(l); ok {
			heads[i] = &hl
			hasHeader = true
		}
	}
	var lay layout
	if !hasHeader {
		// Sem cabeçalho: perfil de projeção sobre as linhas com data e
		// valor; se nem isso, leitura linha a linha.
		lay = p.projection(lines)
	}

	page, n := 0, 0
	for i, l := range lines {
		if l.Page != page {
			page, n = l.Page, 0
		}
		n++
		p.line = n

		if hl := heads[i]; hl != nil {
			// Cabeçalho (repetido ou não) redefine as colunas a partir daqui;
			// páginas que não o repetem herdam o anterior. As fronteiras
			// saem da projeção das linhas da tabela, não da largura do
			// texto do cabeçalho.
			end := i + 1
			for end < len(lines) && heads[end] == nil {
				end++
			}
			lay = layout{bands: spatial.Boundaries(hl.bands, tableBody(lines[i+1:end])), m: hl.m}
			p.open = false
			continue
		}
		switch {
		case lay.valid():
			p.row(l, lay)
		case hasHeader:
			// Antes do primeiro cabeçalho é metadado; só o saldo anterior
			// interessa.
			if header.IsSummary(l.Text()) {
				if m := lastMoney(l); m != "" {
					p.summaryBalance(l.Text(), m)
				}
			}
		default:
			p.lineBased(l)
		}
	}
}

// tableBody devolve as linhas de lançamento (com data ou valor) de lines,
// que é o trecho entre um cabeçalho e o próximo.
func tableBody(lines []spatial.Line) []spatial.Line {
	var out []spatial.Line
	for _, l := range lines {
		if hasMoney(l) || leadingDate(l) {
			out = append(out, l)
		}
	}
	return out
}

func lastMoney(l spatial.Line) string {
	for i := len(l.Words) - 1; i >= 0; i-- {
		if money.LooksLike(l.Words[i].Text) {
			return l.Words[i].Text
		}
	}
	return ""
}

// projection monta colunas pelas calhas de espaço vazio e descobre os papéis
// pelo conteúdo.
func (p *parser) projection(lines []spatial.Line) layout {
	var cand []spatial.Line
	for _, l := range lines {
		if hasMoney(l) && leadingDate(l) {
			cand = append(cand, l)
		}
	}
	if len(cand) < 2 {
		return layout{}
	}
	bands := spatial.Gutters(cand, 1.0)
	rows := make([][]string, len(cand))
	for i, l := range cand {
		rows[i] = spatial.Assign(l, bands)
	}
	m, ok := header.Infer(rows)
	if !ok {
		return layout{}
	}
	return layout{bands: bands, m: m}
}

func leadingDate(l spatial.Line) bool {
	_, n := dateAt(l.Words)
	return n > 0
}

// dateAt tenta ler uma data nas primeiras 1 a 3 palavras ("01/03/2026",
// "02 MAR", "02 MAR 2026"). Devolve quantas palavras usou.
func dateAt(ws []spatial.Word) (date.Civil, int) {
	for n := min(3, len(ws)); n >= 1; n-- {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = ws[i].Text
		}
		if c, err := date.Parse(strings.Join(parts, " ")); err == nil {
			return c, n
		}
	}
	return date.Civil{}, 0
}

// row interpreta uma linha com colunas conhecidas (passo 5).
func (p *parser) row(l spatial.Line, lay layout) {
	cells := spatial.Assign(l, lay.bands)
	get := func(r header.Role) string {
		if i, ok := lay.m[r]; ok && i < len(cells) {
			return cells[i]
		}
		return ""
	}
	desc := textnorm.Spaces(get(header.Description))
	dateText := get(header.Date)
	amountText := get(header.Amount)
	creditText, debitText := get(header.Credit), get(header.Debit)
	balanceText := get(header.Balance)

	c, err := date.Parse(dateText)
	hasDate := err == nil

	p.handle(l, c, hasDate, desc, amountText, creditText, debitText, balanceText, cells)
}

// lineBased interpreta uma linha sem colunas: data no começo, valores no
// fim (o último é saldo quando há dois), descrição no meio.
func (p *parser) lineBased(l spatial.Line) {
	ws := l.Words
	c, n := dateAt(ws)
	ws = ws[n:]
	var moneys []string
	for {
		text, used := trailingMoney(ws)
		if used == 0 {
			break
		}
		moneys = append([]string{text}, moneys...)
		ws = ws[:len(ws)-used]
	}
	parts := make([]string, len(ws))
	for i, w := range ws {
		parts[i] = w.Text
	}
	amountText, balanceText := "", ""
	switch len(moneys) {
	case 0:
	case 1:
		amountText = moneys[0]
	default:
		amountText, balanceText = moneys[len(moneys)-2], moneys[len(moneys)-1]
	}
	p.handle(l, c, n > 0, strings.Join(parts, " "), amountText, "", "", balanceText, []string{l.Text()})
}

// trailingMoney lê um valor no fim de ws e diz quantas palavras ele ocupa.
// O valor pode vir partido: "R$ 45,90", "- 45,90", "45,90 D".
func trailingMoney(ws []spatial.Word) (string, int) {
	n := len(ws)
	if n == 0 {
		return "", 0
	}
	text, used := ws[n-1].Text, 1
	if !money.LooksLike(text) {
		// Marcador de débito/crédito solto depois do valor.
		if n < 2 || header.SignOf(text) == 0 || !money.LooksLike(ws[n-2].Text+" "+text) {
			return "", 0
		}
		text, used = ws[n-2].Text+" "+text, 2
	}
	// Sinal ou moeda soltos antes do valor.
	for used < n && used < 4 {
		prev := ws[n-used-1].Text
		if !isMoneyPrefix(prev) || !money.LooksLike(prev+" "+text) {
			break
		}
		text = prev + " " + text
		used++
	}
	return text, used
}

func isMoneyPrefix(s string) bool {
	switch strings.ToUpper(s) {
	case "-", "+", "\u2212", "\u2013", "R$", "US$", "$", "BRL":
		return true
	}
	return false
}

func (p *parser) handle(l spatial.Line, c date.Civil, hasDate bool, desc, amountText, creditText, debitText, balanceText string, raw []string) {
	text := l.Text()
	key := textnorm.Key(text)

	if hasDate {
		when, guessed := c.Resolve(p.res.PeriodEnd, p.opt.Reference)
		if guessed && !p.guessedYear {
			p.guessedYear = true
			p.res.Warn(l.Page, p.line, text, fmt.Sprintf("data sem ano e período desconhecido; assumido %d", when.Year()))
		}
		p.lastDate, p.hasDate = when, true
	}

	if matchAny(p.skip, key) || matchAny(p.skip, textnorm.Key(desc)) {
		p.open = false
		return
	}

	// Grupo com sinal implícito. Sem o "total de", só vale se a linha não
	// traz valor: "Débitos diversos 10,00" é lançamento.
	if k := textnorm.Key(desc); isSection(k, amountText+creditText+debitText) {
		switch {
		case strings.Contains(k, "entrada"), strings.Contains(k, "credito"):
			p.sectionSign = 1
		default:
			p.sectionSign = -1
		}
		p.open = false
		return
	}

	// Resumo: vira saldo ou é ignorado.
	if header.IsSummary(desc) || header.IsSummary(text) {
		bt := balanceText
		if bt == "" {
			bt = amountText
		}
		label := desc
		if label == "" {
			label = text
		}
		p.summaryBalance(label, bt)
		p.open = false
		return
	}

	amt, signed, hasAmount := parseAmount(amountText, creditText, debitText)

	switch {
	case hasAmount:
		if !p.hasDate {
			p.res.Warn(l.Page, p.line, text, "lançamento sem data")
			p.open = false
			return
		}
		if !signed && p.sectionSign != 0 {
			if p.sectionSign < 0 {
				amt = amt.Abs().Neg()
			} else {
				amt = amt.Abs()
			}
			signed = true
		}
		rec := record.Record{
			Date:        p.lastDate,
			Description: desc,
			Amount:      amt,
			SignKnown:   signed,
			Raw:         raw,
			Page:        l.Page,
			Line:        p.line,
		}
		if balanceText != "" {
			if v, err := money.Parse(balanceText); err == nil {
				rec.Balance = &v
			}
		}
		p.res.Records = append(p.res.Records, rec)
		p.open, p.openY, p.openPage = true, l.Y, l.Page
	case !hasDate && p.open && l.Page == p.openPage && p.openY-l.Y <= 2.5*l.FontSize && !hasMoney(l):
		// Descrição quebrada em duas linhas. Vale o texto inteiro: a
		// continuação pode passar da largura da coluna de descrição.
		r := &p.res.Records[len(p.res.Records)-1]
		r.Description = textnorm.Spaces(r.Description + " " + text)
		r.Raw = append(r.Raw, raw...)
		p.openY = l.Y
	case hasDate && hasDigit(amountText+creditText+debitText):
		// Linha com data e um número que não é dinheiro na coluna de valor.
		p.res.Warn(l.Page, p.line, text, "valor ilegível")
		p.open = false
	default:
		// Data sozinha (grupo de dia) ou texto solto: não é transação.
		if hasDate {
			p.open = false
		}
	}
}

func isSection(key, amounts string) bool {
	m := sectionRe.FindStringSubmatch(key)
	return m != nil && (m[1] != "" || amounts == "")
}

func (p *parser) summaryBalance(label, text string) {
	record.ApplySummary(p.res, label, text)
}

func parseAmount(amountText, creditText, debitText string) (decimal.Decimal, bool, bool) {
	if amountText != "" {
		if v, err := money.ParseValue(amountText); err == nil {
			return v.Amount, v.Signed, true
		}
	}
	if creditText == "" && debitText == "" {
		return decimal.Decimal{}, false, false
	}
	total := decimal.Zero
	if creditText != "" {
		v, err := money.Parse(creditText)
		if err != nil {
			return decimal.Decimal{}, false, false
		}
		total = total.Add(v.Abs())
	}
	if debitText != "" {
		v, err := money.Parse(debitText)
		if err != nil {
			return decimal.Decimal{}, false, false
		}
		total = total.Sub(v.Abs())
	}
	return total, true, true
}

func matchAny(res []*regexp.Regexp, k string) bool {
	for _, re := range res {
		if re.MatchString(k) {
			return true
		}
	}
	return false
}

func hasDigit(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0
}

// Columns devolve as faixas de coluna que o parser usaria para as linhas
// dadas: as do primeiro cabeçalho, ou as do perfil de projeção. Usado pela
// CLI de debug para desenhar as fronteiras.
func Columns(lines []spatial.Line) []spatial.Band {
	for i, l := range lines {
		if lay, ok := headerOf(l); ok {
			end := i + 1
			for end < len(lines) {
				if _, next := headerOf(lines[end]); next {
					break
				}
				end++
			}
			return spatial.Boundaries(lay.bands, tableBody(lines[i+1:end]))
		}
	}
	p := &parser{res: &record.Result{}}
	return p.projection(lines).bands
}

// Package header descobre qual coluna de uma tabela de extrato é data,
// descrição, valor, crédito, débito, saldo ou documento.
package header

import (
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/date"
	"github.com/gustavoz65/finparser-lib/internal/money"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
	"github.com/shopspring/decimal"
)

// Role é o papel de uma coluna.
type Role int

// Papéis reconhecidos.
const (
	None Role = iota
	Date
	Description
	Amount
	Credit
	Debit
	Balance
	Document
)

var roleNames = [...]string{"none", "date", "description", "amount", "credit", "debit", "balance", "document"}

func (r Role) String() string {
	if int(r) < len(roleNames) {
		return roleNames[r]
	}
	return "none"
}

// dictionary: forma normalizada → papel. Comparação sem acento e sem caixa.
var dictionary = buildDictionary()

func buildDictionary() map[string]Role {
	d := map[string]Role{}
	add := func(r Role, keys ...string) {
		for _, k := range keys {
			d[k] = r
		}
	}
	add(Date, "data", "dt", "data lancamento", "data de lancamento", "data mov", "data movimento",
		"data movimentacao", "data da transacao", "data transacao", "date", "dia")
	add(Description, "descricao", "historico", "lancamento", "lancamentos", "detalhes", "estabelecimento",
		"description", "title", "titulo", "memo", "transacao", "descricao do lancamento", "historico descricao")
	add(Amount, "valor", "quantia", "montante", "amount", "value", "valor da transacao", "valor lancamento")
	add(Credit, "credito", "creditos", "entradas", "entrada", "credit", "valor credito")
	add(Debit, "debito", "debitos", "saidas", "saida", "debit", "valor debito")
	add(Balance, "saldo", "saldos", "balance", "saldo apos lancamento", "saldo do dia", "saldo final")
	add(Document, "documento", "doc", "nr doc", "n doc", "no doc", "numero do documento", "identificador", "id")
	return d
}

// clean normaliza um cabeçalho: sem acento, minúsculo, sem "(r$)" nem
// pontuação nas pontas.
func clean(s string) string {
	k := textnorm.Key(s)
	k = strings.NewReplacer("(r$)", " ", "r$", " ", "(", " ", ")", " ", ":", " ", "*", " ", "/", " ", "º", " ", ".", " ").Replace(k)
	return textnorm.Spaces(k)
}

// Match devolve o papel de uma célula de cabeçalho, ou None.
func Match(cell string) Role {
	k := clean(cell)
	if k == "" {
		return None
	}
	if r, ok := dictionary[k]; ok {
		return r
	}
	words := strings.Fields(k)
	has := func(ws ...string) bool {
		for _, w := range words {
			for _, x := range ws {
				if w == x {
					return true
				}
			}
		}
		return false
	}
	switch {
	case words[0] == "data" || words[0] == "dt":
		return Date
	case has("saldo", "saldos"):
		return Balance
	case has("credito", "creditos", "entradas"):
		return Credit
	case has("debito", "debitos", "saidas"):
		return Debit
	case has("valor", "quantia", "montante"):
		return Amount
	case has("descricao", "historico", "detalhes", "estabelecimento"):
		return Description
	case has("documento"):
		return Document
	}
	return None
}

// Mapping associa papel → índice da coluna.
type Mapping map[Role]int

// Has informa se o papel foi mapeado.
func (m Mapping) Has(r Role) bool { _, ok := m[r]; return ok }

// Valid exige data e alguma coluna de valor (valor, crédito ou débito).
func (m Mapping) Valid() bool {
	return m.Has(Date) && (m.Has(Amount) || m.Has(Credit) || m.Has(Debit))
}

// FromHeader tenta ler cells como linha de cabeçalho: precisa casar ≥ 2
// palavras-chave e formar um mapeamento válido.
func FromHeader(cells []string) (Mapping, bool) {
	m := Mapping{}
	for i, c := range cells {
		r := Match(c)
		if r == None || m.Has(r) {
			continue
		}
		m[r] = i
	}
	if len(m) < 2 || !m.Valid() {
		return nil, false
	}
	return m, true
}

// Infer descobre as colunas pelo conteúdo, quando não há cabeçalho: a
// coluna em que ≥ 80% das células são data é a data; entre as monetárias,
// a que muda de forma cumulativa em relação a outra é saldo; a mais
// variada é valor; a de texto mais longo é descrição.
func Infer(rows [][]string) (Mapping, bool) {
	ncols := 0
	for _, r := range rows {
		ncols = max(ncols, len(r))
	}
	if ncols == 0 {
		return nil, false
	}
	type stat struct {
		nonEmpty, dates, moneys, letters, length int
		values                                   []*decimal.Decimal
		signs                                    map[int]bool
	}
	stats := make([]stat, ncols)
	for c := range stats {
		stats[c].values = make([]*decimal.Decimal, len(rows))
		stats[c].signs = map[int]bool{}
	}
	for i, r := range rows {
		for c, cell := range r {
			cell = strings.TrimSpace(cell)
			if cell == "" {
				continue
			}
			s := &stats[c]
			s.nonEmpty++
			if _, err := date.Parse(cell); err == nil {
				s.dates++
				continue
			}
			if money.LooksLike(cell) {
				if d, err := money.Parse(cell); err == nil {
					s.moneys++
					s.values[i] = &d
					s.signs[d.Sign()] = true
					continue
				}
			}
			if strings.IndexFunc(cell, isLetter) >= 0 {
				s.letters++
				s.length += len(cell)
			}
		}
	}

	frac := func(n, d int) bool { return d > 0 && n*5 >= d*4 } // ≥ 80%
	m := Mapping{}
	best := -1
	for c, s := range stats {
		if frac(s.dates, s.nonEmpty) && (best < 0 || s.dates > stats[best].dates) {
			best = c
		}
	}
	if best < 0 {
		return nil, false
	}
	m[Date] = best

	var moneyCols []int
	for c, s := range stats {
		if c != best && frac(s.moneys, s.nonEmpty) {
			moneyCols = append(moneyCols, c)
		}
	}
	if len(moneyCols) == 0 {
		return nil, false
	}

	// Saldo: coluna b tal que b[i] − b[i−1] = ±a[i] (ou na ordem inversa)
	// para a maioria das linhas.
	bestScore, amt, bal := 0, -1, -1
	for _, a := range moneyCols {
		for _, b := range moneyCols {
			if a == b {
				continue
			}
			if sc := cumulative(stats[a].values, stats[b].values); sc > bestScore {
				bestScore, amt, bal = sc, a, b
			}
		}
	}
	if amt >= 0 && bestScore*2 >= len(rows)-1 {
		m[Amount], m[Balance] = amt, bal
	} else {
		amt = moneyCols[0]
		for _, c := range moneyCols[1:] {
			if len(stats[c].signs) > len(stats[amt].signs) {
				amt = c
			}
		}
		m[Amount] = amt
	}

	desc, bestLen := -1, 0
	for c, s := range stats {
		if c == m[Date] || c == m[Amount] || (m.Has(Balance) && c == m[Balance]) {
			continue
		}
		if s.letters > 0 && s.length/s.letters > bestLen {
			desc, bestLen = c, s.length/s.letters
		}
	}
	if desc >= 0 {
		m[Description] = desc
	}
	return m, true
}

func cumulative(amt, bal []*decimal.Decimal) int {
	asc, desc := 0, 0
	for i := 1; i < len(bal); i++ {
		if bal[i] == nil || bal[i-1] == nil {
			continue
		}
		d := bal[i].Sub(*bal[i-1]).Abs()
		if amt[i] != nil && d.Equal(amt[i].Abs()) {
			asc++
		}
		if amt[i-1] != nil && d.Equal(amt[i-1].Abs()) {
			desc++
		}
	}
	return max(asc, desc)
}

func isLetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 127
}

var summaryPrefixes = []string{
	"saldo anterior", "saldo do dia", "saldo final", "saldo inicial", "saldo em",
	"saldo disponivel", "saldo total", "saldo bloqueado", "s a l d o", "saldo",
	"total de entradas", "total de saidas", "total", "resumo",
}

// IsSummary informa se a descrição é de uma linha de resumo ("SALDO DO DIA",
// "SALDO ANTERIOR", "TOTAL DE ENTRADAS"), que não é transação.
func IsSummary(desc string) bool {
	k := textnorm.Key(desc)
	for _, p := range summaryPrefixes {
		if k == p || strings.HasPrefix(k, p+" ") {
			return true
		}
	}
	return false
}

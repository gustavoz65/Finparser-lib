// Package mt940 lê extratos SWIFT MT940 (e MT942): o formato de texto com
// campos ":61:" e ":86:" que bancos entregam para conciliação de empresas.
package mt940

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
	"github.com/shopspring/decimal"
)

// field é um campo ":tag:valor"; o valor pode ocupar várias linhas.
type field struct {
	tag   string
	value string
	line  int
}

var tagRe = regexp.MustCompile(`^:(\d{2}[A-Z]?):`)

func fields(text string) []field {
	var out []field
	for i, ln := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		// Envelope SWIFT: "{1:F01...}{4:" abre o bloco de texto, "-}" fecha.
		if j := strings.LastIndex(ln, "{4:"); j >= 0 {
			ln = ln[j+3:]
		}
		if strings.HasPrefix(ln, "-}") || strings.TrimSpace(ln) == "-" {
			continue
		}
		if m := tagRe.FindStringSubmatch(ln); m != nil {
			out = append(out, field{tag: m[1], value: ln[len(m[0]):], line: i + 1})
		} else if len(out) > 0 && strings.TrimSpace(ln) != "" {
			out[len(out)-1].value += "\n" + ln
		}
	}
	return out
}

// lineRe lê o começo do campo :61: — data valor (AAMMDD), data de
// lançamento opcional (MMDD), marca D/C (com R de estorno), terceira letra
// da moeda opcional, valor com vírgula, tipo (N + 3 caracteres) e
// referências.
var lineRe = regexp.MustCompile(`^(\d{6})(\d{4})?(R?[CD])([A-Z])?(\d[\d,]*)([A-Z][A-Z0-9]{3})([^/\n]*)(?://([^\n]*))?`)

// balanceRe lê :60F:/:62F:/:60M:/:62M: — marca D/C, data, moeda e valor.
var balanceRe = regexp.MustCompile(`^([CD])(\d{6})([A-Z]{3})(\d[\d,]*)`)

// Parse lê um ou mais extratos MT940/MT942 já em UTF-8.
func Parse(text string) (*record.Result, error) {
	fs := fields(text)
	res := &record.Result{}
	seen := false
	statements := 0
	var last *record.Record

	for _, f := range fs {
		switch f.tag {
		case "20":
			statements++
			last = nil
		case "25":
			if res.Account.Number == "" {
				// "BIC/conta" ou só a conta.
				if bank, acct, ok := strings.Cut(strings.TrimSpace(f.value), "/"); ok {
					res.Account = record.Account{BankID: bank, Number: acct}
				} else {
					res.Account.Number = strings.TrimSpace(f.value)
				}
			}
		case "60F", "60M", "62F", "62M":
			m := balanceRe.FindStringSubmatch(strings.TrimSpace(f.value))
			if m == nil {
				res.Warn(0, f.line, f.value, "saldo ilegível")
				continue
			}
			v, err := amount(m[4], m[1] == "D")
			if err != nil {
				res.Warn(0, f.line, f.value, "saldo ilegível")
				continue
			}
			if res.Currency == "" {
				res.Currency = m[3]
			}
			when, okDate := yymmdd(m[2])
			// Só o primeiro saldo inicial e o último final valem para o
			// arquivo; ":60M:"/":62M:" são saldos intermediários de página.
			if f.tag == "60F" || f.tag == "60M" {
				if res.OpeningBalance == nil && len(res.Records) == 0 {
					res.OpeningBalance = &v
				}
				if okDate && (res.PeriodStart.IsZero() || when.Before(res.PeriodStart)) {
					res.PeriodStart = when
				}
			} else {
				res.ClosingBalance = &v
				if okDate && when.After(res.PeriodEnd) {
					res.PeriodEnd = when
				}
			}
			last = nil
		case "61":
			seen = true
			last = nil
			rec, err := statementLine(f)
			if err != nil {
				res.Warn(0, f.line, f.value, err.Error())
				continue
			}
			res.Records = append(res.Records, rec)
			last = &res.Records[len(res.Records)-1]
		case "86":
			if last == nil {
				continue // :86: depois de saldo é informação do extrato
			}
			info := information(f.value)
			last.Raw = append(last.Raw, ":86:"+strings.ReplaceAll(f.value, "\n", " "))
			switch {
			case info == "":
			case last.Description == "":
				last.Description = info
			default:
				last.Description = info + " - " + last.Description
			}
			last = nil
		}
	}
	if statements == 0 && !seen {
		return nil, fmt.Errorf("%w: MT940 sem campo :20: nem :61:", errs.ErrMalformed)
	}
	// Vários extratos no mesmo arquivo podem ser contas diferentes; os
	// saldos só fazem sentido quando há um.
	if statements > 1 {
		res.OpeningBalance, res.ClosingBalance = nil, nil
	}
	return res, nil
}

func statementLine(f field) (record.Record, error) {
	m := lineRe.FindStringSubmatch(f.value)
	if m == nil {
		return record.Record{}, fmt.Errorf("lançamento :61: ilegível")
	}
	valueDate, ok := yymmdd(m[1])
	if !ok {
		return record.Record{}, fmt.Errorf("data inválida %q", m[1])
	}
	when := valueDate
	if m[2] != "" {
		// Data de lançamento (MMDD): o ano é o da data valor, ajustado na
		// virada (valor em 31/12, lançamento em 02/01).
		if booked, ok := mmdd(m[2], valueDate); ok {
			when = booked
		}
	}
	// C entra, D sai; RC é estorno de crédito (sai), RD estorno de débito.
	debit := m[3] == "D" || m[3] == "RC"
	amt, err := amount(m[5], debit)
	if err != nil {
		return record.Record{}, fmt.Errorf("valor inválido %q", m[5])
	}

	ref := strings.TrimSpace(m[7])
	bankRef := strings.TrimSpace(m[8])
	id := bankRef
	if id == "" && !strings.EqualFold(ref, "NONREF") {
		id = ref
	}
	// Linha suplementar do :61: (até 34 caracteres de texto livre).
	extra := ""
	if _, rest, ok := strings.Cut(f.value, "\n"); ok {
		extra = textnorm.Spaces(rest)
	}
	return record.Record{
		Date:        when,
		Description: extra,
		Amount:      amt,
		SignKnown:   true,
		ID:          id,
		Raw:         []string{":61:" + strings.ReplaceAll(f.value, "\n", " ")},
		Line:        f.line,
	}, nil
}

// subfieldRe casa os códigos "?20".."?63" do :86: estruturado (padrão
// alemão/europeu): ?00 tipo, ?20–?29 e ?60–?63 finalidade, ?30 banco,
// ?31 conta, ?32–?33 nome da contraparte.
var subfieldRe = regexp.MustCompile(`\?(\d{2})`)

// information transforma o :86: em descrição. No formato estruturado junta
// nome da contraparte e finalidade; no livre, só normaliza os espaços.
func information(v string) string {
	flat := strings.ReplaceAll(v, "\n", "")
	loc := subfieldRe.FindAllStringSubmatchIndex(flat, -1)
	if len(loc) == 0 {
		return textnorm.Spaces(strings.ReplaceAll(v, "\n", " "))
	}
	var name, purpose, kind []string
	for i, l := range loc {
		end := len(flat)
		if i+1 < len(loc) {
			end = loc[i+1][0]
		}
		text := strings.TrimSpace(flat[l[1]:end])
		if text == "" {
			continue
		}
		switch code := flat[l[2]:l[3]]; {
		case code == "00":
			kind = append(kind, text)
		case code >= "20" && code <= "29", code >= "60" && code <= "63":
			purpose = append(purpose, text)
		case code == "32" || code == "33":
			name = append(name, text)
		}
	}
	var parts []string
	for _, p := range [][]string{name, purpose} {
		if s := textnorm.Spaces(strings.Join(p, " ")); s != "" {
			parts = append(parts, s)
		}
	}
	if len(parts) == 0 {
		return textnorm.Spaces(strings.Join(kind, " "))
	}
	return strings.Join(parts, " - ")
}

// amount lê o valor SWIFT: só dígitos e uma vírgula decimal ("1234,56",
// "100,").
func amount(s string, debit bool) (decimal.Decimal, error) {
	if strings.Count(s, ",") > 1 {
		return decimal.Decimal{}, fmt.Errorf("valor inválido")
	}
	s = strings.TrimSuffix(strings.Replace(s, ",", ".", 1), ".")
	v, err := decimal.NewFromString(s)
	if err != nil {
		return v, err
	}
	if debit {
		v = v.Neg()
	}
	return v, nil
}

func yymmdd(s string) (time.Time, bool) {
	var y, m, d int
	if _, err := fmt.Sscanf(s, "%2d%2d%2d", &y, &m, &d); err != nil {
		return time.Time{}, false
	}
	// Janela SWIFT: 80–99 é 19xx.
	if y >= 80 {
		y += 1900
	} else {
		y += 2000
	}
	return civil(y, m, d)
}

func mmdd(s string, near time.Time) (time.Time, bool) {
	var m, d int
	if _, err := fmt.Sscanf(s, "%2d%2d", &m, &d); err != nil {
		return time.Time{}, false
	}
	t, ok := civil(near.Year(), m, d)
	if !ok {
		return t, false
	}
	// Mais de meio ano de distância: está do outro lado da virada.
	switch diff := t.Sub(near).Hours() / 24; {
	case diff > 183:
		return civil(near.Year()-1, m, d)
	case diff < -183:
		return civil(near.Year()+1, m, d)
	}
	return t, true
}

func civil(y, m, d int) (time.Time, bool) {
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if m < 1 || m > 12 || t.Day() != d {
		return time.Time{}, false
	}
	return t, true
}

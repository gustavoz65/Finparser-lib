// Package money converte valores monetários escritos em extratos brasileiros
// ("R$ 1.234,56", "1.234,56 D", "(45,00)") para decimal exato.
package money

import (
	"errors"
	"strings"
	"unicode"

	"github.com/shopspring/decimal"
)

// ErrInvalid indica que o texto não representa um valor monetário.
var ErrInvalid = errors.New("money: valor inválido")

// Value é o resultado de uma conversão.
type Value struct {
	Amount decimal.Decimal
	// Signed indica que o texto trazia sinal explícito (-, +, D, C ou
	// parênteses). Um valor sem sinal pode ser receita ou despesa.
	Signed bool
}

// Parse converte s em decimal. Atalho para ParseValue quando o sinal
// explícito não importa.
func Parse(s string) (decimal.Decimal, error) {
	v, err := ParseValue(s)
	return v.Amount, err
}

// ParseValue converte s em decimal e informa se havia sinal explícito.
//
// Formatos aceitos: "R$ 1.234,56", "-R$ 45,00", "R$ -45,00", "1.234,56 D",
// "1.234,56 C", "100,00 CR", "100,00 DB", "1.234,56-", "(1.234,56)",
// "+ 100,00", "−45,90" (menos tipográfico), "1234.56", "0,01".
func ParseValue(s string) (Value, error) {
	num, neg, signed, err := scan(s)
	if err != nil {
		return Value{}, err
	}
	d, err := decimal.NewFromString(num)
	if err != nil {
		return Value{}, ErrInvalid
	}
	if neg {
		d = d.Neg()
	}
	return Value{Amount: d, Signed: signed}, nil
}

// currencies são prefixos de moeda ignorados, já em minúsculas.
var currencies = []string{"r$", "us$", "brl", "usd", "eur", "$", "€"}

// suffixes são marcadores de débito/crédito no fim do valor.
var suffixes = []struct {
	text string
	neg  bool
}{{"db", true}, {"dr", true}, {"cr", false}, {"d", true}, {"c", false}}

// scan valida s e devolve o número no formato de decimal.NewFromString,
// o sinal e se ele era explícito. Não cria decimal.
func scan(s string) (num string, neg, signed bool, err error) {
	s = compact(s)
	if s == "" {
		return "", false, false, ErrInvalid
	}

	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = s[1 : len(s)-1]
		neg, signed = true, true
	}

	// Sufixo D/C, DB/CR (débito/crédito), sempre depois de um dígito.
	for _, suf := range suffixes {
		n := len(s) - len(suf.text)
		if n > 0 && isDigit(s[n-1]) && strings.EqualFold(s[n:], suf.text) {
			s = s[:n]
			neg, signed = neg != suf.neg, true
			break
		}
	}

	// Sinais e símbolo de moeda podem aparecer em qualquer ordem no começo
	// ("-R$45,00", "R$-45,00") e o sinal também pode vir no fim ("45,00-").
strip:
	for {
		for _, c := range currencies {
			if len(s) >= len(c) && strings.EqualFold(s[:len(c)], c) {
				s = s[len(c):]
				continue strip
			}
		}
		switch {
		case strings.HasPrefix(s, "-"):
			s = s[1:]
			neg, signed = !neg, true
		case strings.HasPrefix(s, "+"):
			s = s[1:]
			signed = true
		case strings.HasSuffix(s, "-"):
			s = s[:len(s)-1]
			neg, signed = !neg, true
		case strings.HasSuffix(s, "+"):
			s = s[:len(s)-1]
			signed = true
		default:
			break strip
		}
	}

	num, err = normalize(s)
	return num, neg, signed, err
}

// LooksLike informa, sem criar decimal, se s tem forma de valor monetário
// com casas decimais (ex.: "45,00", "R$ 1.234,56"). Inteiros puros como
// "2026" não contam, para não confundir com ano ou número de documento.
func LooksLike(s string) bool {
	num, _, _, err := scan(s)
	if err != nil {
		return false
	}
	i := strings.IndexByte(num, '.')
	return i >= 0 && len(num)-i-1 == 2
}

// compact remove espaços (inclusive o não-quebrável) de s e troca os
// traços tipográficos usados como menos em PDF ("−", "–") por "-".
func compact(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r) || r == '\u00a0':
			return -1
		case r == '\u2212' || r == '\u2013' || r == '\ufe63' || r == '\uff0d':
			return '-'
		}
		return r
	}, s)
}

// normalize recebe só dígitos e separadores e devolve o número no formato
// aceito por decimal.NewFromString.
func normalize(s string) (string, error) {
	if s == "" {
		return "", ErrInvalid
	}
	digits := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case isDigit(c):
			digits++
		case c == '.' || c == ',':
		default:
			return "", ErrInvalid
		}
	}
	if digits == 0 {
		return "", ErrInvalid
	}

	last := strings.LastIndexAny(s, ".,")
	if last < 0 {
		return s, nil
	}
	sep := s[last]
	frac := s[last+1:]
	intPart := s[:last]

	decimalSep := true
	switch {
	case strings.ContainsAny(frac, ".,"):
		return "", ErrInvalid
	case len(frac) == 0:
		return "", ErrInvalid
	case len(frac) == 3:
		// "1.234" ou "1,234": separador de milhar, a menos que o outro
		// separador apareça antes ("1,234.567" seria estranho, mas decimal).
		other := byte(',')
		if sep == ',' {
			other = '.'
		}
		// "0.500" não é agrupamento de milhar: é decimal.
		decimalSep = strings.IndexByte(intPart, other) >= 0 || strings.HasPrefix(intPart, "0")
	}

	if !decimalSep {
		return validGroups(s, sep)
	}

	// O separador que não é o decimal é milhar: some com ele, mas ele não
	// pode ser o mesmo caractere do decimal ("1.234.56" é inválido).
	if strings.IndexByte(intPart, sep) >= 0 {
		return "", ErrInvalid
	}
	thousand := byte(',')
	if sep == ',' {
		thousand = '.'
	}
	ip, err := validGroups(intPart, thousand)
	if err != nil {
		return "", err
	}
	if ip == "" {
		ip = "0"
	}
	return ip + "." + frac, nil
}

// validGroups remove o separador de milhar sep de s, conferindo que os
// grupos depois do primeiro têm exatamente 3 dígitos.
func validGroups(s string, sep byte) (string, error) {
	if strings.IndexByte(s, sep) < 0 {
		return s, nil
	}
	parts := strings.Split(s, string(sep))
	if len(parts[0]) == 0 || len(parts[0]) > 3 {
		return "", ErrInvalid
	}
	for _, p := range parts[1:] {
		if len(p) != 3 {
			return "", ErrInvalid
		}
	}
	return strings.Join(parts, ""), nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

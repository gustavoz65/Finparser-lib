// Package date interpreta datas de extratos brasileiros ("02/03/2026",
// "02 MAR", "20260301120000[-3:BRT]") como datas civis.
//
// Data civil: meia-noite UTC, sem fuso. Um lançamento de 01/03 nunca vira
// 28/02 por causa de conversão de horário.
package date

import (
	"errors"
	"strings"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

// ErrInvalid indica que o texto não é uma data reconhecida.
var ErrInvalid = errors.New("date: data inválida")

// Civil é uma data que pode estar sem ano ("02 MAR").
type Civil struct {
	Year    int // 0 quando o texto não trazia ano
	Month   time.Month
	Day     int
	HasYear bool
}

// Time devolve a data como meia-noite UTC. Só faz sentido com HasYear.
func (c Civil) Time() time.Time {
	return time.Date(c.Year, c.Month, c.Day, 0, 0, 0, 0, time.UTC)
}

// Resolve completa o ano de uma data sem ano usando o fim do período do
// extrato. Se o mês da transação for maior que o mês final do período, ela
// pertence ao ano anterior (extrato de dezembro a janeiro). Sem período
// (periodEnd zero), usa o ano de ref e devolve guessed = true.
func (c Civil) Resolve(periodEnd, ref time.Time) (t time.Time, guessed bool) {
	if c.HasYear {
		return c.Time(), false
	}
	if periodEnd.IsZero() {
		return time.Date(ref.Year(), c.Month, c.Day, 0, 0, 0, 0, time.UTC), true
	}
	y := periodEnd.Year()
	if c.Month > periodEnd.Month() {
		y--
	}
	return time.Date(y, c.Month, c.Day, 0, 0, 0, 0, time.UTC), false
}

var months = map[string]time.Month{
	"jan": time.January, "janeiro": time.January,
	"fev": time.February, "fevereiro": time.February,
	"mar": time.March, "marco": time.March,
	"abr": time.April, "abril": time.April,
	"mai": time.May, "maio": time.May,
	"jun": time.June, "junho": time.June,
	"jul": time.July, "julho": time.July,
	"ago": time.August, "agosto": time.August,
	"set": time.September, "setembro": time.September,
	"out": time.October, "outubro": time.October,
	"nov": time.November, "novembro": time.November,
	"dez": time.December, "dezembro": time.December,
	// Inglês, comum em CSV exportado por fintechs.
	"feb": time.February, "apr": time.April, "may": time.May,
	"aug": time.August, "sep": time.September, "oct": time.October,
	"dec": time.December,
}

// Parse reconhece: 02/01/2006, 02/01/06, 02/01, 02-01-2006, 2006-01-02,
// 02 MAR, 02 MAR 2026, 02/mar, 02 de março de 2026, 02.01.2006.
// Um horário depois da data ("02/01/2006 10:30") é ignorado.
func Parse(s string) (Civil, error) {
	s = textnorm.Key(s)
	if s == "" {
		return Civil{}, ErrInvalid
	}
	// Descarta horário: "2026-03-01t10:00:00", "01/03/2026 10:30".
	if i := strings.IndexByte(s, ':'); i >= 0 {
		j := strings.LastIndexAny(s[:i], " t")
		if j <= 0 {
			return Civil{}, ErrInvalid
		}
		s = strings.TrimSpace(s[:j])
	}

	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '/' || r == '-' || r == '.' || r == ' ' || r == ' '
	})
	// "02 de março de 2026"
	filtered := parts[:0]
	for _, p := range parts {
		if p != "de" {
			filtered = append(filtered, p)
		}
	}
	parts = filtered
	if len(parts) < 2 || len(parts) > 3 {
		return Civil{}, ErrInvalid
	}

	// ISO: 2006-01-02
	if len(parts) == 3 && len(parts[0]) == 4 {
		y, ok1 := atoi(parts[0])
		m, ok2 := atoi(parts[1])
		d, ok3 := atoi(parts[2])
		if !ok1 || !ok2 || !ok3 {
			return Civil{}, ErrInvalid
		}
		return build(y, m, d, true)
	}

	d, ok := atoi(parts[0])
	if !ok || len(parts[0]) > 2 {
		return Civil{}, ErrInvalid
	}
	var m int
	if mm, ok := months[parts[1]]; ok {
		m = int(mm)
	} else if n, ok := atoi(parts[1]); ok && len(parts[1]) <= 2 {
		m = n
	} else {
		return Civil{}, ErrInvalid
	}
	if len(parts) == 2 {
		return build(0, m, d, false)
	}
	y, ok := atoi(parts[2])
	if !ok {
		return Civil{}, ErrInvalid
	}
	switch len(parts[2]) {
	case 2:
		y += 2000
	case 4:
	default:
		return Civil{}, ErrInvalid
	}
	return build(y, m, d, true)
}

// ParseCompact lê datas OFX: "20260301", "20260301120000",
// "20260301120000[-3:BRT]". Só os 8 primeiros dígitos importam.
func ParseCompact(s string) (Civil, error) {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return Civil{}, ErrInvalid
	}
	y, ok1 := atoi(s[0:4])
	m, ok2 := atoi(s[4:6])
	d, ok3 := atoi(s[6:8])
	if !ok1 || !ok2 || !ok3 {
		return Civil{}, ErrInvalid
	}
	return build(y, m, d, true)
}

func build(y, m, d int, hasYear bool) (Civil, error) {
	if m < 1 || m > 12 || d < 1 || d > 31 {
		return Civil{}, ErrInvalid
	}
	// Ano bissexto como referência para validar 29/02 sem ano.
	ry := y
	if !hasYear {
		ry = 2000
	}
	t := time.Date(ry, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if t.Day() != d || int(t.Month()) != m {
		return Civil{}, ErrInvalid
	}
	if hasYear && (y < 1900 || y > 2200) {
		return Civil{}, ErrInvalid
	}
	return Civil{Year: y, Month: time.Month(m), Day: d, HasYear: hasYear}, nil
}

func atoi(s string) (int, bool) {
	if s == "" || len(s) > 9 {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

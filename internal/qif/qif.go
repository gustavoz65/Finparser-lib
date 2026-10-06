// Package qif lê arquivos QIF (Quicken Interchange Format), tratados como
// um OFX simplificado: um campo por linha e "^" fechando cada transação.
package qif

import (
	"fmt"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/date"
	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/money"
	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

// Parse lê um QIF já em UTF-8. Datas são interpretadas como dia/mês/ano.
func Parse(text string) (*record.Result, error) {
	res := &record.Result{}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	var (
		fields []string
		start  int
		d, t   string
		payee  string
		memo   string
		inType bool
	)
	flush := func(line int) {
		defer func() { fields, d, t, payee, memo = nil, "", "", "", "" }()
		if len(fields) == 0 {
			return
		}
		raw := strings.Join(fields, "; ")
		c, err := date.Parse(strings.ReplaceAll(d, "'", "/"))
		if err != nil || !c.HasYear {
			res.Warn(0, start, raw, fmt.Sprintf("data inválida %q", d))
			return
		}
		v, err := money.ParseValue(t)
		if err != nil {
			res.Warn(0, start, raw, fmt.Sprintf("valor inválido %q", t))
			return
		}
		desc := textnorm.Spaces(payee)
		if m := textnorm.Spaces(memo); m != "" && m != desc {
			if desc == "" {
				desc = m
			} else {
				desc += " - " + m
			}
		}
		res.Records = append(res.Records, record.Record{
			Date: c.Time(), Description: desc, Amount: v.Amount,
			SignKnown: true, Raw: fields, Line: start,
		})
	}

	for i, ln := range lines {
		ln = strings.TrimRight(ln, " \t")
		if ln == "" {
			continue
		}
		if strings.HasPrefix(ln, "!") {
			inType = strings.HasPrefix(strings.ToLower(ln), "!type:")
			continue
		}
		if !inType {
			continue
		}
		if ln[0] == '^' {
			flush(i + 1)
			continue
		}
		if len(fields) == 0 {
			start = i + 1
		}
		fields = append(fields, ln)
		val := strings.TrimSpace(ln[1:])
		switch ln[0] {
		case 'D':
			d = val
		case 'T', 'U':
			if t == "" {
				t = val
			}
		case 'P':
			payee = val
		case 'M':
			memo = val
		}
	}
	flush(len(lines))

	if len(res.Records) == 0 && len(res.Warnings) == 0 {
		return nil, fmt.Errorf("%w: QIF sem transações", errs.ErrMalformed)
	}
	return res, nil
}

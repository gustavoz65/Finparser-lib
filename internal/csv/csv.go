// Package csv lê extratos em CSV: descobre o delimitador, a linha de
// cabeçalho (o que vem antes é metadado) e o papel de cada coluna.
package csv

import (
	stdcsv "encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/table"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

// Options ajusta o parser.
type Options = table.Options

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

// Parse lê um CSV já em UTF-8.
func Parse(text string, opt Options) (*record.Result, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n") // Mac clássico

	// Diretiva do Excel: "sep=;" na primeira linha.
	delim := rune(0)
	if first, rest, ok := strings.Cut(text, "\n"); ok {
		if f := strings.TrimSpace(first); len(f) == 5 && strings.EqualFold(f[:4], "sep=") {
			delim, text = rune(f[4]), "\n"+rest // mantém a numeração das linhas
		}
	}
	if delim == 0 {
		delim = Sniff(text)
	}

	var warns []record.Warning
	r := newReader(text, delim)
	var rows []table.Row
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			line := 0
			var pe *stdcsv.ParseError
			if errors.As(err, &pe) {
				line = pe.StartLine
			}
			warns = append(warns, record.Warning{Line: line, Message: fmt.Sprintf("linha CSV inválida: %v", err)})
			continue
		}
		line, _ := r.FieldPos(0)
		for i := range rec {
			rec[i] = textnorm.Spaces(rec[i])
		}
		if strings.Join(rec, "") == "" {
			continue
		}
		rows = append(rows, table.Row{Cells: rec, Line: line})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: CSV vazio", errs.ErrMalformed)
	}
	res, err := table.Parse(rows, opt)
	if err != nil {
		return nil, err
	}
	res.Warnings = append(warns, res.Warnings...)
	return res, nil
}

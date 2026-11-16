// Package htmltable lê extratos exportados como tabela HTML. Vários bancos
// entregam um ".xls" que, por dentro, é só uma página com <table>: o Excel
// abre, mas não é planilha. As linhas da tabela vão para o motor tabular.
package htmltable

import (
	"fmt"
	"html"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/table"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

// Options ajusta o parser.
type Options = table.Options

// Parse lê todas as tabelas do documento, já em UTF-8.
func Parse(text string, opt Options) (*record.Result, error) {
	rows := Rows(text)
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: HTML sem tabela", errs.ErrMalformed)
	}
	return table.Parse(rows, opt)
}

// Rows devolve as linhas (<tr>) não vazias de todas as tabelas, em ordem.
// Tolerante: tags sem fechamento, atributos e marcação dentro da célula não
// atrapalham. Nunca falha.
func Rows(s string) []table.Row {
	var (
		rows   []table.Row
		cells  []string
		cell   strings.Builder
		inCell bool
		inRow  bool
		rowAt  int
		line   = 1
	)
	endCell := func() {
		if inCell {
			cells = append(cells, textnorm.Spaces(html.UnescapeString(cell.String())))
			cell.Reset()
			inCell = false
		}
	}
	endRow := func() {
		endCell()
		if inRow && strings.Join(cells, "") != "" {
			rows = append(rows, table.Row{Cells: cells, Line: rowAt})
		}
		cells, inRow = nil, false
	}

	for i := 0; i < len(s); {
		lt := strings.IndexByte(s[i:], '<')
		if lt < 0 {
			lt = len(s) - i
		}
		if inCell {
			cell.WriteString(s[i : i+lt])
		}
		line += strings.Count(s[i:i+lt], "\n")
		i += lt
		if i >= len(s) {
			break
		}

		if strings.HasPrefix(s[i:], "<!--") {
			end := strings.Index(s[i:], "-->")
			if end < 0 {
				break
			}
			line += strings.Count(s[i:i+end], "\n")
			i += end + 3
			continue
		}
		gt := strings.IndexByte(s[i:], '>')
		if gt < 0 {
			break
		}
		tag := s[i+1 : i+gt]
		line += strings.Count(tag, "\n")
		i += gt + 1

		closing := strings.HasPrefix(tag, "/")
		name := strings.ToLower(strings.TrimPrefix(tag, "/"))
		if j := strings.IndexAny(name, " \t\r\n/"); j >= 0 {
			name = name[:j]
		}
		switch name {
		case "script", "style":
			if !closing {
				// O conteúdo não é texto da página.
				end := strings.Index(strings.ToLower(s[i:]), "</"+name)
				if end < 0 {
					i = len(s)
					break
				}
				line += strings.Count(s[i:i+end], "\n")
				i += end
			}
		case "tr":
			endRow()
			if !closing {
				inRow, rowAt = true, line
			}
		case "td", "th":
			endCell()
			if !closing {
				if !inRow {
					inRow, rowAt = true, line
				}
				inCell = true
			}
		case "table", "thead", "tbody", "tfoot":
			endRow()
		case "br", "p", "div":
			if inCell {
				cell.WriteByte(' ')
			}
		}
	}
	endRow()
	return rows
}

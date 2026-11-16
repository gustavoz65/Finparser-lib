// Package xlsx lê planilhas Excel (.xlsx) só com a biblioteca padrão: o
// arquivo é um ZIP de XMLs. Lê a primeira planilha, resolve textos
// compartilhados, converte datas seriais e entrega as linhas ao motor
// tabular, o mesmo do CSV.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/table"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

// Limites contra arquivo malicioso (zip bomb, planilha gigante).
const (
	maxPart = 64 << 20 // bytes descomprimidos por parte
	maxRows = 200000
	maxCols = 256
)

// Options ajusta o parser.
type Options = table.Options

// Parse lê a primeira planilha do arquivo.
func Parse(data []byte, opt Options) (*record.Result, error) {
	rows, err := Rows(data)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: planilha vazia", errs.ErrMalformed)
	}
	return table.Parse(rows, opt)
}

// Rows devolve as linhas não vazias da primeira planilha, com as células já
// como texto: datas em dd/mm/aaaa e números sem ruído de ponto flutuante.
func Rows(data []byte) ([]table.Row, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%w: xlsx: %v", errs.ErrMalformed, err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[strings.TrimPrefix(f.Name, "/")] = f
	}

	wb, err := readWorkbook(files)
	if err != nil {
		return nil, err
	}
	sheet := files[wb.sheet]
	if sheet == nil {
		return nil, fmt.Errorf("%w: xlsx sem planilha", errs.ErrMalformed)
	}
	shared, err := readShared(files["xl/sharedStrings.xml"])
	if err != nil {
		return nil, err
	}
	dates, err := readDateStyles(files["xl/styles.xml"])
	if err != nil {
		return nil, err
	}
	return readSheet(sheet, shared, dates, wb.date1904)
}

func open(f *zip.File) (io.ReadCloser, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: xlsx: %v", errs.ErrMalformed, err)
	}
	return struct {
		io.Reader
		io.Closer
	}{io.LimitReader(rc, maxPart), rc}, nil
}

func decode(f *zip.File, v any) error {
	rc, err := open(f)
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	if err := xml.NewDecoder(rc).Decode(v); err != nil {
		return fmt.Errorf("%w: xlsx %s: %v", errs.ErrMalformed, f.Name, err)
	}
	return nil
}

type workbook struct {
	sheet    string // caminho da primeira planilha dentro do ZIP
	date1904 bool
}

func readWorkbook(files map[string]*zip.File) (workbook, error) {
	wb := workbook{sheet: "xl/worksheets/sheet1.xml"}
	f := files["xl/workbook.xml"]
	if f == nil {
		if files[wb.sheet] == nil {
			return wb, fmt.Errorf("%w: não é um xlsx (sem xl/workbook.xml)", errs.ErrMalformed)
		}
		return wb, nil
	}
	var doc struct {
		Pr struct {
			Date1904 string `xml:"date1904,attr"`
		} `xml:"workbookPr"`
		Sheets []struct {
			RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := decode(f, &doc); err != nil {
		return wb, err
	}
	wb.date1904 = doc.Pr.Date1904 == "1" || doc.Pr.Date1904 == "true"
	if len(doc.Sheets) == 0 {
		return wb, nil
	}
	if rels := files["xl/_rels/workbook.xml.rels"]; rels != nil {
		var r struct {
			Rel []struct {
				ID     string `xml:"Id,attr"`
				Target string `xml:"Target,attr"`
			} `xml:"Relationship"`
		}
		if err := decode(rels, &r); err != nil {
			return wb, err
		}
		for _, rel := range r.Rel {
			if rel.ID == doc.Sheets[0].RID {
				if strings.HasPrefix(rel.Target, "/") {
					wb.sheet = strings.TrimPrefix(rel.Target, "/")
				} else {
					wb.sheet = path.Join("xl", rel.Target)
				}
			}
		}
	}
	return wb, nil
}

// readShared lê a tabela de textos compartilhados. Texto com formatação vem
// partido em vários <r><t>; os pedaços são concatenados.
func readShared(f *zip.File) ([]string, error) {
	if f == nil {
		return nil, nil
	}
	var doc struct {
		SI []struct {
			T string   `xml:"t"`
			R []string `xml:"r>t"`
		} `xml:"si"`
	}
	if err := decode(f, &doc); err != nil {
		return nil, err
	}
	out := make([]string, len(doc.SI))
	for i, si := range doc.SI {
		out[i] = si.T + strings.Join(si.R, "")
	}
	return out, nil
}

// readDateStyles devolve, por índice de estilo de célula, se o formato
// numérico é de data. No xlsx data é só um número com formato de data.
func readDateStyles(f *zip.File) ([]bool, error) {
	if f == nil {
		return nil, nil
	}
	var doc struct {
		NumFmts []struct {
			ID   int    `xml:"numFmtId,attr"`
			Code string `xml:"formatCode,attr"`
		} `xml:"numFmts>numFmt"`
		Xfs []struct {
			NumFmtID int `xml:"numFmtId,attr"`
		} `xml:"cellXfs>xf"`
	}
	if err := decode(f, &doc); err != nil {
		return nil, err
	}
	custom := map[int]bool{}
	for _, nf := range doc.NumFmts {
		custom[nf.ID] = isDateFormat(nf.Code)
	}
	out := make([]bool, len(doc.Xfs))
	for i, xf := range doc.Xfs {
		id := xf.NumFmtID
		// Formatos embutidos de data e hora.
		out[i] = custom[id] || id >= 14 && id <= 22 || id >= 27 && id <= 36 || id >= 45 && id <= 47 || id >= 50 && id <= 58
	}
	return out, nil
}

// isDateFormat: o código tem d, m, y ou h fora de aspas e colchetes.
func isDateFormat(code string) bool {
	quoted, bracket := false, false
	for i := 0; i < len(code); i++ {
		switch c := code[i]; {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '\\':
			i++
		case c == '[':
			bracket = true
		case c == ']':
			bracket = false
		case bracket:
		case strings.IndexByte("dmyhDMYH", c) >= 0:
			return true
		}
	}
	return false
}

func readSheet(f *zip.File, shared []string, dates []bool, date1904 bool) ([]table.Row, error) {
	rc, err := open(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	type cell struct {
		Ref   string `xml:"r,attr"`
		Type  string `xml:"t,attr"`
		Style int    `xml:"s,attr"`
		V     string `xml:"v"`
		IS    struct {
			T string   `xml:"t"`
			R []string `xml:"r>t"`
		} `xml:"is"`
	}
	type row struct {
		R     int    `xml:"r,attr"`
		Cells []cell `xml:"c"`
	}

	var out []table.Row
	dec := xml.NewDecoder(rc)
	n := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: xlsx %s: %v", errs.ErrMalformed, f.Name, err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "row" {
			continue
		}
		var r row
		if err := dec.DecodeElement(&r, &se); err != nil {
			return nil, fmt.Errorf("%w: xlsx %s: %v", errs.ErrMalformed, f.Name, err)
		}
		if n++; n > maxRows {
			return nil, fmt.Errorf("%w: planilha com mais de %d linhas", errs.ErrTooLarge, maxRows)
		}
		if r.R == 0 {
			r.R = n
		}
		var cells []string
		empty := true
		for i, c := range r.Cells {
			col := column(c.Ref)
			if col < 0 {
				col = i
			}
			if col >= maxCols {
				continue
			}
			var text string
			switch c.Type {
			case "s":
				if k, err := strconv.Atoi(strings.TrimSpace(c.V)); err == nil && k >= 0 && k < len(shared) {
					text = shared[k]
				}
			case "inlineStr":
				text = c.IS.T + strings.Join(c.IS.R, "")
			case "str", "e":
				text = c.V
			case "b":
				text = c.V
			case "d":
				text = isoDate(c.V)
			default:
				text = number(c.V, c.Style < len(dates) && dates[c.Style], date1904)
			}
			text = textnorm.Spaces(text)
			if text == "" {
				continue
			}
			for len(cells) <= col {
				cells = append(cells, "")
			}
			cells[col] = text
			empty = false
		}
		if !empty {
			out = append(out, table.Row{Cells: cells, Line: r.R})
		}
	}
	return out, nil
}

// column converte a referência "BC12" no índice da coluna (A = 0), ou −1.
func column(ref string) int {
	n := 0
	i := 0
	for ; i < len(ref); i++ {
		c := ref[i] | 0x20
		if c < 'a' || c > 'z' {
			break
		}
		n = n*26 + int(c-'a') + 1
		if n > 1<<20 {
			return -1
		}
	}
	if i == 0 {
		return -1
	}
	return n - 1
}

// number converte o valor numérico de uma célula em texto. O xlsx guarda
// double; 15 dígitos significativos é a precisão que o Excel mostra, e
// tira o ruído ("1234.5600000000001" → "1234.56").
func number(v string, isDate, date1904 bool) string {
	v = strings.TrimSpace(v)
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return v
	}
	if isDate {
		return serialDate(f, date1904)
	}
	s := strconv.FormatFloat(f, 'e', 14, 64) // 15 significativos
	f, _ = strconv.ParseFloat(s, 64)
	s = strconv.FormatFloat(f, 'f', -1, 64)
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return s // inteiro: número de documento, quantidade
	}
	// Sempre 2 casas (ou 4+) para o valor não ser confundido com milhar:
	// "1.5" → "1.50", "1.234" → "1.2340".
	switch decimals := len(s) - i - 1; {
	case decimals == 1:
		s += "0"
	case decimals == 3:
		s += "0"
	}
	return s
}

// serialDate converte o número serial do Excel (dias desde 30/12/1899, ou
// desde 01/01/1904) em dd/mm/aaaa. A parte fracionária é o horário.
func serialDate(f float64, date1904 bool) string {
	days := math.Floor(f)
	if days < 0 || days > 200000 {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	if date1904 {
		epoch = time.Date(1904, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return epoch.AddDate(0, 0, int(days)).Format("02/01/2006")
}

func isoDate(v string) string {
	if len(v) >= 10 {
		if t, err := time.Parse("2006-01-02", v[:10]); err == nil {
			return t.Format("02/01/2006")
		}
	}
	return v
}

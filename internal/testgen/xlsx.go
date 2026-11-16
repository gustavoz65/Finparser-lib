package testgen

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// XLSX monta uma planilha mínima e determinística. Cada célula pode ser
// string (texto compartilhado), float64 ou int (número), time.Time (data
// serial com formato de data) ou nil (vazia).
func XLSX(rows [][]any) ([]byte, error) {
	var (
		shared []string
		index  = map[string]int{}
		sheet  strings.Builder
	)
	esc := func(s string) string {
		var b bytes.Buffer
		_ = xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for r, row := range rows {
		fmt.Fprintf(&sheet, `<row r="%d">`, r+1)
		for c, v := range row {
			ref := fmt.Sprintf("%c%d", 'A'+c, r+1)
			switch x := v.(type) {
			case nil:
			case string:
				i, ok := index[x]
				if !ok {
					i = len(shared)
					index[x] = i
					shared = append(shared, x)
				}
				fmt.Fprintf(&sheet, `<c r="%s" t="s"><v>%d</v></c>`, ref, i)
			case time.Time:
				serial := x.Sub(time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)).Hours() / 24
				fmt.Fprintf(&sheet, `<c r="%s" s="1"><v>%s</v></c>`, ref, strconv.FormatFloat(serial, 'f', -1, 64))
			case float64:
				fmt.Fprintf(&sheet, `<c r="%s"><v>%s</v></c>`, ref, strconv.FormatFloat(x, 'g', 17, 64))
			case int:
				fmt.Fprintf(&sheet, `<c r="%s"><v>%d</v></c>`, ref, x)
			default:
				return nil, fmt.Errorf("testgen: célula %s de tipo %T", ref, v)
			}
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)

	var sst strings.Builder
	sst.WriteString(`<?xml version="1.0" encoding="UTF-8"?><sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	for _, s := range shared {
		sst.WriteString(`<si><t>` + esc(s) + `</t></si>`)
	}
	sst.WriteString(`</sst>`)

	parts := []struct{ name, body string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/extrato.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Extrato" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/extrato.xml"/></Relationships>`},
		{"xl/styles.xml", `<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cellXfs count="2"><xf numFmtId="0"/><xf numFmtId="14"/></cellXfs></styleSheet>`},
		{"xl/sharedStrings.xml", sst.String()},
		{"xl/worksheets/extrato.xml", sheet.String()},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, p := range parts {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: p.name, Method: zip.Deflate})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(p.body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

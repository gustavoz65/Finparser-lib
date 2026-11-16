// Package detect descobre o formato de um extrato olhando os bytes, nunca a
// extensão do arquivo.
package detect

import (
	"bytes"
	"regexp"
	"unicode"
	"unicode/utf8"

	"github.com/gustavoz65/finparser-lib/internal/textenc"
)

// Format é o formato detectado.
type Format string

// Formatos reconhecidos. Unknown é a string vazia.
const (
	Unknown Format = ""
	PDF     Format = "pdf"
	OFX     Format = "ofx"
	QIF     Format = "qif"
	CSV     Format = "csv"
	XLSX    Format = "xlsx"
	HTML    Format = "html"
	CAMT    Format = "camt053"
	MT940   Format = "mt940"
	// XLS é o Excel binário antigo (BIFF): reconhecido para dar um erro
	// claro, mas não lido.
	XLS Format = "xls"
)

const window = 1024

var (
	zipMagic = []byte("PK\x03\x04")
	oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	mt940Tag = regexp.MustCompile(`(?m)^:(20|25|60[FM]|61):`)
)

// Detect aplica, nos primeiros ~1 KB, a ordem: %PDF- → PDF; ZIP com
// planilha → XLSX; OFXHEADER: ou <OFX> → OFX; !Type: → QIF; <Document> ISO
// 20022 → CAMT; campos ":20:"/":61:" → MT940; <table>/<html> → HTML; texto
// válido → CSV; senão Unknown.
func Detect(b []byte) Format {
	head := b
	if len(head) > window {
		head = head[:window]
	}
	switch {
	case bytes.HasPrefix(b, zipMagic):
		// Os nomes das partes ficam sem compressão dentro do ZIP.
		if bytes.Contains(b, []byte("xl/worksheets/")) || bytes.Contains(b, []byte("xl/workbook.xml")) {
			return XLSX
		}
		return Unknown
	case bytes.HasPrefix(b, oleMagic):
		return XLS
	}
	// O padrão PDF tolera lixo antes do cabeçalho dentro do primeiro KB.
	if bytes.Contains(head, []byte("%PDF-")) {
		return PDF
	}

	text, _ := textenc.ToUTF8(head)
	trimmed := bytes.TrimLeftFunc(text, unicode.IsSpace)
	upper := bytes.ToUpper(trimmed)

	switch {
	case bytes.HasPrefix(upper, []byte("OFXHEADER:")), bytes.Contains(upper, []byte("<OFX>")):
		return OFX
	case bytes.HasPrefix(upper, []byte("!TYPE:")):
		return QIF
	case bytes.Contains(upper, []byte("<DOCUMENT")) && (bytes.Contains(upper, []byte("CAMT.0")) || bytes.Contains(upper, []byte("<BKTOCSTMR"))),
		bytes.Contains(upper, []byte("<BKTOCSTMR")):
		return CAMT
	case bytes.HasPrefix(trimmed, []byte("{1:")) && bytes.Contains(head, []byte("{4:")), mt940Tag.Match(trimmed) && bytes.HasPrefix(trimmed, []byte(":")):
		return MT940
	case bytes.Contains(upper, []byte("<TABLE")), bytes.Contains(upper, []byte("<HTML")), bytes.HasPrefix(upper, []byte("<!DOCTYPE HTML")):
		return HTML
	case len(trimmed) > 0 && isText(text):
		return CSV
	}
	return Unknown
}

// isText informa se b parece texto: sem NUL e com no máximo 5% de runes de
// controle ou inválidos. O último rune pode estar cortado pela janela.
func isText(b []byte) bool {
	total, bad := 0, 0
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		b = b[size:]
		if r == utf8.RuneError && size <= 1 && len(b) > 0 {
			bad++
		} else if r == 0 {
			return false
		} else if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			bad++
		}
		total++
	}
	return total > 0 && bad*20 <= total
}

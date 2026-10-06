// Package detect descobre o formato de um extrato olhando os bytes, nunca a
// extensão do arquivo.
package detect

import (
	"bytes"
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
)

const window = 1024

// Detect aplica, nos primeiros ~1 KB, a ordem: %PDF- → PDF; OFXHEADER: ou
// <OFX> → OFX; !Type: → QIF; texto válido → CSV; senão Unknown.
func Detect(b []byte) Format {
	head := b
	if len(head) > window {
		head = head[:window]
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

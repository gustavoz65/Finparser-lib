// Package textenc converte bytes de extrato para UTF-8.
//
// Bancos brasileiros exportam muito em Windows-1252 (superconjunto prático
// do Latin-1). Regra: remove BOM; se já é UTF-8 válido, devolve como está;
// senão decodifica como Windows-1252.
package textenc

import (
	"bytes"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// Encoding identifica a codificação detectada.
type Encoding string

// Codificações reconhecidas.
const (
	UTF8        Encoding = "utf-8"
	UTF16       Encoding = "utf-16"
	Windows1252 Encoding = "windows-1252"
)

// ToUTF8 devolve b em UTF-8 sem BOM e a codificação de origem.
func ToUTF8(b []byte) ([]byte, Encoding) {
	switch {
	case bytes.HasPrefix(b, bomUTF8):
		b = b[len(bomUTF8):]
	case bytes.HasPrefix(b, bomUTF16LE), bytes.HasPrefix(b, bomUTF16BE):
		dec := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder()
		if out, err := dec.Bytes(b); err == nil {
			return out, UTF16
		}
	}
	if utf8.Valid(b) {
		return b, UTF8
	}
	out, err := charmap.Windows1252.NewDecoder().Bytes(b)
	if err != nil {
		// Windows-1252 mapeia todos os bytes; não deveria acontecer.
		return bytes.ToValidUTF8(b, []byte("�")), UTF8
	}
	return out, Windows1252
}

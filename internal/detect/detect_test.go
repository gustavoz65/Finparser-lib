package detect

import (
	"bytes"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want Format
	}{
		{"pdf", []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n1 0 obj"), PDF},
		{"pdf com lixo antes", append([]byte("\x00\x00junk"), "%PDF-1.7"...), PDF},
		{"ofx sgml", []byte("OFXHEADER:100\nDATA:OFXSGML\n<OFX>"), OFX},
		{"ofx com bom", []byte("\xEF\xBB\xBF  \r\nOFXHEADER:100"), OFX},
		{"ofx xml", []byte(`<?xml version="1.0"?><?OFX OFXHEADER="200"?><OFX><SIGNONMSGSRSV1>`), OFX},
		{"ofx minúsculo", []byte("<ofx><bankmsgsrsv1>"), OFX},
		{"qif", []byte("!Type:Bank\nD03/01/2026\nT-45.00\n^"), QIF},
		{"csv", []byte("Data;Descrição;Valor\n01/03/2026;Padaria;-45,00\n"), CSV},
		{"csv cp1252", []byte("Data;Descri\xe7\xe3o;Valor\n"), CSV},
		{"binário", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x00}, Unknown},
		{"zip", append([]byte("PK\x03\x04\x14\x00\x00\x00"), bytes.Repeat([]byte{0x00, 0x8f}, 50)...), Unknown},
		{"vazio", nil, Unknown},
		{"só espaços", []byte("   \n\t"), Unknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Detect(tt.in); got != tt.want {
				t.Errorf("Detect = %q, want %q", got, tt.want)
			}
		})
	}
}

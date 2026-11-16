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
		{"xlsx", []byte("PK\x03\x04\x14\x00\x08\x00[Content_Types].xml....PK\x03\x04xl/workbook.xml"), XLSX},
		{"xls antigo", []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1, 0, 0}, XLS},
		{"camt.053", []byte(`<?xml version="1.0"?><Document xmlns="urn:iso:std:iso:20022:tech:xsd:camt.053.001.02"><BkToCstmrStmt>`), CAMT},
		{"camt sem namespace", []byte("<Document>\n <BkToCstmrAcctRpt>"), CAMT},
		{"mt940", []byte(":20:REF\r\n:25:123\r\n:60F:C260301BRL1,00\r\n"), MT940},
		{"mt940 com envelope", []byte("{1:F01BANKBRSP0000}{2:I940X}{4:\n:20:REF\n"), MT940},
		{"html", []byte("<html><body><table><tr><td>Data</td>"), HTML},
		{"html só tabela", []byte("\n<TABLE border=1><tr><th>Data"), HTML},
		{"csv com dois-pontos", []byte("Conta: 123\nData;Valor\n01/03/2026;1,00\n"), CSV},
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

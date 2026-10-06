package textenc

import "testing"

func TestToUTF8(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
		enc  Encoding
	}{
		{"utf8", []byte("Padaria São João"), "Padaria São João", UTF8},
		{"bom", append([]byte{0xEF, 0xBB, 0xBF}, "Descrição"...), "Descrição", UTF8},
		{"cp1252", []byte{'S', 0xE3, 'o', ' ', 'J', 'o', 0xE3, 'o'}, "São João", Windows1252},
		{"cp1252 aspas", []byte{0x93, 'x', 0x94}, "“x”", Windows1252},
		{"utf16le", []byte{0xFF, 0xFE, 'O', 0, 'F', 0, 'X', 0}, "OFX", UTF16},
		{"vazio", nil, "", UTF8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, enc := ToUTF8(tt.in)
			if string(got) != tt.want || enc != tt.enc {
				t.Errorf("ToUTF8 = %q (%s), want %q (%s)", got, enc, tt.want, tt.enc)
			}
		})
	}
}

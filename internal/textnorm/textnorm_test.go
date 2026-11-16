package textnorm

import "testing"

func TestKey(t *testing.T) {
	tests := map[string]string{
		"Descrição":          "descricao",
		"  Data Lançamento ": "data lancamento",
		"SALDO   DO DIA":     "saldo do dia",
		"Histórico\t":        "historico",
		"PIX\u00a0 ENVIADO":  "pix enviado",
		"São João":           "sao joao",
		"":                   "",
	}
	for in, want := range tests {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFoldMatchesSlow(t *testing.T) {
	for r := rune(0xA0); r <= 0xFF; r++ {
		in := "x" + string(r) + "y"
		if got, want := Fold(in), foldSlow(in); got != want {
			t.Errorf("Fold(%q) = %q, lento = %q", in, got, want)
		}
	}
	if got := Fold("e\u0301 ł"); got != "e ł" {
		t.Errorf("Fold com marca combinante = %q", got)
	}
}

func BenchmarkKey(b *testing.B) {
	in := []string{"PIX TRANSF FULANO 02/03", "Saldo do dia", "Descrição", "1.234,56"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Key(in[i%len(in)])
	}
}

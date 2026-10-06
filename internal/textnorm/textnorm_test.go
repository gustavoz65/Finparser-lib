package textnorm

import "testing"

func TestKey(t *testing.T) {
	tests := map[string]string{
		"Descrição":          "descricao",
		"  Data Lançamento ": "data lancamento",
		"SALDO   DO DIA":     "saldo do dia",
		"Histórico\t":        "historico",
	}
	for in, want := range tests {
		if got := Key(in); got != want {
			t.Errorf("Key(%q) = %q, want %q", in, got, want)
		}
	}
}

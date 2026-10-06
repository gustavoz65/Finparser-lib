package profile

import "testing"

func TestDetect(t *testing.T) {
	tests := map[string]string{
		"Nu Pagamentos S.A. - Instituição de Pagamento": "nubank",
		"Data,Valor,Identificador,Descrição":            "nubank",
		"ITAÚ UNIBANCO S.A.\nExtrato conta corrente":    "itau",
		"Banco do Brasil S.A.":                          "bb",
		"Extrato qualquer":                              "",
	}
	for in, want := range tests {
		got := ""
		if p := Detect(in); p != nil {
			got = p.Name()
		}
		if got != want {
			t.Errorf("Detect(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestByCode(t *testing.T) {
	for code, want := range map[string]string{"260": "nubank", "0341": "itau", "077": "inter", "001": "bb", "999": ""} {
		if got := ByCode(code); got != want {
			t.Errorf("ByCode(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestLookup(t *testing.T) {
	if p, ok := Lookup("Nubank"); !ok || p.Name() != "nubank" {
		t.Errorf("Lookup(Nubank) = %v, %v", p, ok)
	}
	if _, ok := Lookup("banco-x"); ok {
		t.Error("Lookup(banco-x) deveria falhar")
	}
}

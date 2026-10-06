package money

import (
	"testing"
)

func TestParseValue(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		signed bool
	}{
		{"R$ 1.234,56", "1234.56", false},
		{"-R$ 45,00", "-45", true},
		{"R$ -45,00", "-45", true},
		{"1.234,56 D", "-1234.56", true},
		{"1.234,56 C", "1234.56", true},
		{"1.234,56-", "-1234.56", true},
		{"(1.234,56)", "-1234.56", true},
		{"+ 100,00", "100", true},
		{"1234.56", "1234.56", false},
		{"0,01", "0.01", false},
		{"R$ 1.234,56", "1234.56", false},
		{"1,234.56", "1234.56", false},
		{"-45.00", "-45", true},
		{"-45,5", "-45.5", true},
		{"1.234", "1234", false},
		{"1.234.567,89", "1234567.89", false},
		{"12", "12", false},
		{",50", "0.5", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseValue(tt.in)
			if err != nil {
				t.Fatalf("ParseValue(%q): %v", tt.in, err)
			}
			if got.Amount.String() != tt.want {
				t.Errorf("ParseValue(%q) = %s, want %s", tt.in, got.Amount, tt.want)
			}
			if got.Signed != tt.signed {
				t.Errorf("ParseValue(%q).Signed = %v, want %v", tt.in, got.Signed, tt.signed)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"", "R$", "abc", "1.23.4", "1.234.56", "12/03", "-", "()", "1,2,3,4", "1..2", "D"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) deveria falhar", in)
		}
	}
}

func TestLooksLike(t *testing.T) {
	yes := []string{"45,00", "R$ 1.234,56", "-45,00", "1.234,56 D", "0,01"}
	no := []string{"2026", "12/03", "PIX", "", "1.234"}
	for _, s := range yes {
		if !LooksLike(s) {
			t.Errorf("LooksLike(%q) = false", s)
		}
	}
	for _, s := range no {
		if LooksLike(s) {
			t.Errorf("LooksLike(%q) = true", s)
		}
	}
}

func FuzzParseMoney(f *testing.F) {
	for _, s := range []string{"R$ 1.234,56", "(1,00)", "1.234,56 D", "-", "1..2", "+ 100,00"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v, err := ParseValue(s)
		if err != nil {
			return
		}
		// Sem sinal explícito o valor nunca pode sair negativo.
		if !v.Signed && v.Amount.IsNegative() {
			t.Errorf("ParseValue(%q) = %s sem sinal explícito", s, v.Amount)
		}
	})
}

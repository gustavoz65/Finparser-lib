package date

import "testing"

func TestFindPeriod(t *testing.T) {
	tests := map[string][2]string{
		"Período: 01/03/2026 a 31/03/2026":                       {"2026-03-01", "2026-03-31"},
		"Extrato de 01 de março de 2026 até 31 de março de 2026": {"2026-03-01", "2026-03-31"},
		"01 MAR 2026 - 31 MAR 2026":                              {"2026-03-01", "2026-03-31"},
		"Movimentação de 15/12/2025 a 14/01/2026":                {"2025-12-15", "2026-01-14"},
	}
	for in, want := range tests {
		s, e, ok := FindPeriod(in)
		if !ok || s.Format("2006-01-02") != want[0] || e.Format("2006-01-02") != want[1] {
			t.Errorf("FindPeriod(%q) = %v %v %v", in, s, e, ok)
		}
	}
	for _, in := range []string{"sem período", "31/03/2026 a 01/03/2026", "01/03 a 31/03"} {
		if _, _, ok := FindPeriod(in); ok {
			t.Errorf("FindPeriod(%q) deveria falhar", in)
		}
	}
}

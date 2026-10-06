package date

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in      string
		y, m, d int
		hasYear bool
	}{
		{"02/01/2006", 2006, 1, 2, true},
		{"02/01/06", 2006, 1, 2, true},
		{"02/01", 0, 1, 2, false},
		{"02-01-2006", 2006, 1, 2, true},
		{"2006-01-02", 2006, 1, 2, true},
		{"02 MAR", 0, 3, 2, false},
		{"02 MAR 2026", 2026, 3, 2, true},
		{"02/mar", 0, 3, 2, false},
		{"2 de março de 2026", 2026, 3, 2, true},
		{"15 dezembro", 0, 12, 15, false},
		{"02.03.2026", 2026, 3, 2, true},
		{"01/03/2026 10:30", 2026, 3, 1, true},
		{"2026-03-01T10:00:00", 2026, 3, 1, true},
		{"29/02", 0, 2, 29, false},
		{" 05 Out 2026 ", 2026, 10, 5, true},
		{"10 OCT 2026", 2026, 10, 10, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			c, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.in, err)
			}
			if c.Year != tt.y || int(c.Month) != tt.m || c.Day != tt.d || c.HasYear != tt.hasYear {
				t.Errorf("Parse(%q) = %+v", tt.in, c)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"", "32/01/2026", "31/02/2026", "29/02/2025", "02/13", "PIX", "1234,56", "02 XYZ", "1/2/3/4", "02/01/123", "10:30"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) deveria falhar", in)
		}
	}
}

func TestParseCompact(t *testing.T) {
	for _, in := range []string{"20260301", "20260301120000", "20260301120000[-3:BRT]", "20260301000000.000[-03:EST]"} {
		c, err := ParseCompact(in)
		if err != nil {
			t.Fatalf("ParseCompact(%q): %v", in, err)
		}
		if !c.Time().Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("ParseCompact(%q) = %v", in, c.Time())
		}
	}
	for _, in := range []string{"", "2026", "2026AB01", "20261301"} {
		if _, err := ParseCompact(in); err == nil {
			t.Errorf("ParseCompact(%q) deveria falhar", in)
		}
	}
}

func TestResolve(t *testing.T) {
	ref := time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC)
	jan := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	// Extrato dezembro→janeiro: dezembro é do ano anterior.
	got, guessed := Civil{Month: time.December, Day: 20}.Resolve(jan, ref)
	if guessed || !got.Equal(time.Date(2025, 12, 20, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("dezembro: %v %v", got, guessed)
	}
	got, _ = Civil{Month: time.January, Day: 5}.Resolve(jan, ref)
	if !got.Equal(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("janeiro: %v", got)
	}
	// Sem período: ano de referência e aviso.
	got, guessed = Civil{Month: time.March, Day: 2}.Resolve(time.Time{}, ref)
	if !guessed || got.Year() != 2030 {
		t.Errorf("sem período: %v %v", got, guessed)
	}
	// Com ano: nada a resolver.
	got, guessed = Civil{Year: 2024, Month: time.March, Day: 2, HasYear: true}.Resolve(jan, ref)
	if guessed || got.Year() != 2024 {
		t.Errorf("com ano: %v %v", got, guessed)
	}
}

func FuzzParseDate(f *testing.F) {
	for _, s := range []string{"02/01/2006", "02 MAR", "2006-01-02", "01/03/2026 10:30", ":", "t:"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		c, err := Parse(s)
		if err != nil {
			return
		}
		if c.Month < 1 || c.Month > 12 || c.Day < 1 || c.Day > 31 {
			t.Errorf("Parse(%q) = %+v fora do intervalo", s, c)
		}
	})
}

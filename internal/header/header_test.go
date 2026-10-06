package header

import "testing"

func TestMatch(t *testing.T) {
	tests := map[string]Role{
		"Data":             Date,
		"DATA LANÇAMENTO":  Date,
		"Dt. Movimento":    Date,
		"Descrição":        Description,
		"Histórico":        Description,
		"Lançamento":       Description,
		"Estabelecimento":  Description,
		"Valor (R$)":       Amount,
		"Valor":            Amount,
		"Crédito (R$)":     Credit,
		"Entradas":         Credit,
		"Débito":           Debit,
		"Saídas":           Debit,
		"Saldo (R$)":       Balance,
		"Saldo após lanç.": Balance,
		"Nr. Doc":          Document,
		"Identificador":    Document,
		"Agência":          None,
		"":                 None,
	}
	for in, want := range tests {
		if got := Match(in); got != want {
			t.Errorf("Match(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestFromHeader(t *testing.T) {
	m, ok := FromHeader([]string{"Data", "Valor", "Identificador", "Descrição"})
	if !ok || m[Date] != 0 || m[Amount] != 1 || m[Document] != 2 || m[Description] != 3 {
		t.Errorf("Nubank: %v %v", m, ok)
	}
	m, ok = FromHeader([]string{"Data", "Histórico", "Docto.", "Crédito (R$)", "Débito (R$)", "Saldo (R$)"})
	if !ok || m[Credit] != 3 || m[Debit] != 4 || m[Balance] != 5 {
		t.Errorf("crédito/débito: %v %v", m, ok)
	}
	if _, ok := FromHeader([]string{"Nome", "Agência", "Conta"}); ok {
		t.Error("metadado não é cabeçalho")
	}
	if _, ok := FromHeader([]string{"Descrição", "Saldo"}); ok {
		t.Error("sem data/valor não é válido")
	}
}

func TestInfer(t *testing.T) {
	rows := [][]string{
		{"01/03/2026", "Saldo anterior", "", "1.000,00"},
		{"02/03/2026", "Padaria São João", "-45,90", "954,10"},
		{"03/03/2026", "PIX recebido de Fulano", "200,00", "1.154,10"},
		{"04/03/2026", "Tarifa", "-10,00", "1.144,10"},
		{"05/03/2026", "Mercado", "-44,10", "1.100,00"},
	}
	m, ok := Infer(rows)
	if !ok || m[Date] != 0 || m[Description] != 1 || m[Amount] != 2 || m[Balance] != 3 {
		t.Errorf("Infer = %v %v", m, ok)
	}

	// Sem saldo.
	m, ok = Infer([][]string{{"Padaria", "01/03", "45,00"}, {"Mercado", "02/03", "-10,00"}})
	if !ok || m[Date] != 1 || m[Amount] != 2 || m[Description] != 0 || m.Has(Balance) {
		t.Errorf("Infer sem saldo = %v %v", m, ok)
	}

	if _, ok := Infer([][]string{{"a", "b"}, {"c", "d"}}); ok {
		t.Error("sem datas não deveria inferir")
	}
}

func TestIsSummary(t *testing.T) {
	for _, s := range []string{"SALDO DO DIA", "Saldo anterior", "TOTAL DE ENTRADAS", "Total de saídas", "S A L D O"} {
		if !IsSummary(s) {
			t.Errorf("IsSummary(%q) = false", s)
		}
	}
	for _, s := range []string{"PIX RECEBIDO", "Saldão das Tintas", "Totalmente Café"} {
		if IsSummary(s) {
			t.Errorf("IsSummary(%q) = true", s)
		}
	}
}

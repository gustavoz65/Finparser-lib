package testgen

import "fmt"

// Todos os dados abaixo são fictícios.

// NubankConta imita o extrato de conta do Nubank: sem cabeçalho de tabela,
// data só na linha que abre o dia, valores sem sinal agrupados por "Total
// de entradas" / "Total de saídas", descrição em duas linhas e "Saldo do
// dia" fechando cada grupo.
func NubankConta() Doc {
	const (
		xDate  = 40.0
		xDesc  = 120.0
		xRight = 555.0
	)
	day := func(y float64, d, label, total string) Line {
		return Line{Y: y, Bold: true, Cells: []Cell{{X: xDate, Text: d}, {X: xDesc, Text: label}, {X: xRight, Text: total, Right: true}}}
	}
	section := func(y float64, label, total string) Line {
		return Line{Y: y, Bold: true, Cells: []Cell{{X: xDesc, Text: label}, {X: xRight, Text: total, Right: true}}}
	}
	item := func(y float64, desc, v string) Line {
		cells := []Cell{{X: xDesc, Text: desc}}
		if v != "" {
			cells = append(cells, Cell{X: xRight, Text: v, Right: true})
		}
		return Line{Y: y, Cells: cells}
	}
	footer := func(p, n int) []Line {
		return []Line{
			{Y: 800, Size: 7, Cells: []Cell{{X: xDate, Text: "Nu Pagamentos S.A. - Instituição de Pagamento"}, {X: xRight, Text: fmt.Sprintf("%d de %d", p, n), Right: true}}},
			{Y: 810, Size: 7, Cells: []Cell{{X: xDate, Text: "CNPJ 00.000.000/0001-00"}}},
		}
	}

	p1 := []Line{
		{Y: 50, Size: 12, Bold: true, Cells: []Cell{{X: xDate, Text: "Fulana de Tal"}}},
		{Y: 66, Cells: []Cell{{X: xDate, Text: "CPF •••.123.456-••   Agência 0001   Conta 1234567-8"}}},
		{Y: 90, Cells: []Cell{{X: xDate, Text: "01 DE MARÇO DE 2026 a 31 DE MARÇO DE 2026"}, {X: xRight, Text: "VALORES EM R$", Right: true}}},
		{Y: 110, Cells: []Cell{{X: xDate, Text: "Saldo inicial"}, {X: xRight, Text: "1.000,00", Right: true}}},
		{Y: 124, Cells: []Cell{{X: xDate, Text: "Saldo final do período"}, {X: xRight, Text: "3.179,17", Right: true}}},
		{Y: 150, Size: 11, Bold: true, Cells: []Cell{{X: xDate, Text: "Movimentações"}}},

		day(180, "01 MAR 2026", "Total de entradas", "+ 3.500,00"),
		item(194, "Transferência recebida pelo Pix", "3.500,00"),
		item(205, "EMPRESA FICTÍCIA LTDA - 00.000.000/0001-00", ""),
		section(222, "Saldo do dia", "4.500,00"),

		day(250, "02 MAR 2026", "Total de saídas", "- 91,80"),
		item(264, "Compra no débito", "45,90"),
		item(275, "PADARIA SÃO JOÃO", ""),
		item(292, "Compra no débito", "45,90"),
		item(303, "PADARIA SÃO JOÃO", ""),
		section(320, "Saldo do dia", "4.408,20"),

		day(348, "05 MAR 2026", "Total de entradas", "+ 0,87"),
		item(362, "Rendimento da conta", "0,87"),
		section(380, "Total de saídas", "- 1.200,00"),
		item(394, "Transferência enviada pelo Pix", "1.200,00"),
		item(405, "FULANO DE TAL - •••.000.000-•• - BANCO FICTÍCIO", ""),
		section(422, "Saldo do dia", "3.209,07"),
	}
	p2 := []Line{
		day(60, "10 MAR 2026", "Total de saídas", "- 29,90"),
		item(74, "Pagamento de fatura", "29,90"),
		section(92, "Saldo do dia", "3.179,17"),
		{Y: 130, Size: 8, Cells: []Cell{{X: xDate, Text: "Tem alguma dúvida? Fale com a gente pelo app."}}},
	}
	return Doc{Pages: []Page{{Lines: append(p1, footer(1, 2)...)}, {Lines: append(p2, footer(2, 2)...)}}}
}

// ItauConta imita um extrato de conta com tabela de cabeçalho explícito
// (Data | Lançamento | Valor | Saldo), datas sem ano, "SALDO DO DIA",
// descrição quebrada, cabeçalho repetido e "Página X de Y" no rodapé.
func ItauConta() Doc {
	const (
		xDate   = 40.0
		xDesc   = 100.0
		xValue  = 470.0
		xSaldo  = 555.0
		pageTop = 50.0
	)
	top := []Line{
		{Y: pageTop, Size: 11, Bold: true, Cells: []Cell{{X: xDate, Text: "Itaú Unibanco S.A."}}},
		{Y: pageTop + 14, Cells: []Cell{{X: xDate, Text: "Extrato de conta corrente"}}},
		{Y: pageTop + 28, Cells: []Cell{{X: xDate, Text: "Agência 0000   Conta 00000-0"}}},
	}
	head := Line{Y: pageTop + 70, Bold: true, Cells: []Cell{
		{X: xDate, Text: "Data"}, {X: xDesc, Text: "Lançamento"},
		{X: xValue, Text: "Valor (R$)", Right: true}, {X: xSaldo, Text: "Saldo (R$)", Right: true},
	}}
	row := func(y float64, d, desc, v, saldo string) Line {
		cells := []Cell{{X: xDesc, Text: desc}}
		if d != "" {
			cells = append(cells, Cell{X: xDate, Text: d})
		}
		if v != "" {
			cells = append(cells, Cell{X: xValue, Text: v, Right: true})
		}
		if saldo != "" {
			cells = append(cells, Cell{X: xSaldo, Text: saldo, Right: true})
		}
		return Line{Y: y, Cells: cells}
	}
	footer := func(p int) Line {
		return Line{Y: 810, Size: 7, Cells: []Cell{{X: xSaldo, Text: fmt.Sprintf("Página %d de 2", p), Right: true}}}
	}

	p1 := append(append([]Line{}, top...),
		Line{Y: pageTop + 42, Cells: []Cell{{X: xDate, Text: "Período: 01/03/2026 a 31/03/2026"}}},
		head,
		row(140, "01/03", "SALDO ANTERIOR", "", "1.000,00"),
		row(154, "02/03", "PIX TRANSF FULANO 02/03", "200,00", ""),
		row(168, "02/03", "SALDO DO DIA", "", "1.200,00"),
		row(182, "03/03", "TAR PACOTE SERVICOS", "-29,90", ""),
		row(196, "04/03", "PAG BOLETO ENERGIA ELETRICA", "-150,35", ""),
		row(207, "", "DISTRIBUIDORA FICTICIA S/A", "", ""),
		row(221, "04/03", "SALDO DO DIA", "", "1.019,75"),
		footer(1),
	)
	p2 := append(append([]Line{}, top...),
		head,
		row(140, "05/03", "COMPRA CARTAO MERCADO", "-89,90", ""),
		row(154, "06/03", "RENDIMENTO POUPANCA", "1,23", ""),
		row(168, "06/03", "SALDO DO DIA", "", "931,08"),
		footer(2),
	)
	return Doc{Pages: []Page{{Lines: p1}, {Lines: p2}}}
}

// Scanned imita um PDF escaneado: páginas sem nenhum texto.
func Scanned() Doc { return Doc{Pages: []Page{{}, {}}} }

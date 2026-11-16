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

// BradescoConta imita um extrato com crédito e débito em colunas separadas
// (Data | Histórico | Docto. | Crédito | Débito | Saldo), descrição longa
// que passa do cabeçalho, valor alto que começa antes dele, saldo só em
// algumas linhas e a tabela continuando na segunda página sem repetir o
// cabeçalho.
func BradescoConta() Doc {
	const (
		xDate   = 40.0
		xDesc   = 95.0
		xDoc    = 330.0
		xCredit = 430.0
		xDebit  = 495.0
		xSaldo  = 560.0
	)
	row := func(y float64, d, desc, doc, credit, debit, saldo string) Line {
		var cells []Cell
		add := func(x float64, text string, right bool) {
			if text != "" {
				cells = append(cells, Cell{X: x, Text: text, Right: right})
			}
		}
		add(xDate, d, false)
		add(xDesc, desc, false)
		add(xDoc, doc, false)
		add(xCredit, credit, true)
		add(xDebit, debit, true)
		add(xSaldo, saldo, true)
		return Line{Y: y, Size: 8, Cells: cells}
	}
	p1 := []Line{
		{Y: 50, Size: 11, Bold: true, Cells: []Cell{{X: xDate, Text: "Banco Bradesco S.A."}}},
		{Y: 64, Cells: []Cell{{X: xDate, Text: "Extrato de: Ag: 0000 | CC: 0000000-0 | Entre 01/03/2026 e 31/03/2026"}}},
		{Y: 78, Cells: []Cell{{X: xDate, Text: "Período: 01/03/2026 a 31/03/2026"}}},
		{Y: 110, Size: 8, Bold: true, Cells: []Cell{
			{X: xDate, Text: "Data"}, {X: xDesc, Text: "Histórico"}, {X: xDoc, Text: "Docto."},
			{X: xCredit, Text: "Crédito (R$)", Right: true}, {X: xDebit, Text: "Débito (R$)", Right: true}, {X: xSaldo, Text: "Saldo (R$)", Right: true},
		}},
		row(124, "01/03/26", "SALDO ANTERIOR", "", "", "", "2.500,00"),
		row(136, "02/03/26", "TRANSF PIX REM: EMPRESA FICTICIA COMERCIO LTDA", "0001234", "1.250.000,00", "", "1.252.500,00"),
		row(148, "03/03/26", "PAGTO ELETRON COBRANCA", "0001235", "", "389,90", ""),
		row(158, "", "CONDOMINIO EDIFICIO FICTICIO BLOCO B APTO 000", "", "", "", ""),
		row(170, "03/03/26", "TARIFA BANCARIA CESTA", "0001236", "", "32,50", "1.252.077,60"),
		row(182, "05/03/26", "PIX QR CODE DINAMICO DES: POSTO FICTICIO", "0001237", "", "250,00", "1.251.827,60"),
		{Y: 810, Size: 7, Cells: []Cell{{X: xSaldo, Text: "Folha 1/2", Right: true}}},
	}
	p2 := []Line{
		row(60, "06/03/26", "RESGATE INVEST FACIL", "0001238", "100,00", "", "1.251.927,60"),
		row(72, "09/03/26", "TED DIF.TITUL.CC H.BANK DEST. FULANO DE TAL", "0001239", "", "1.000.000,00", "251.927,60"),
		row(96, "", "Total", "", "1.250.100,00", "1.000.672,40", "251.927,60"),
		{Y: 810, Size: 7, Cells: []Cell{{X: xSaldo, Text: "Folha 2/2", Right: true}}},
	}
	return Doc{Pages: []Page{{Lines: p1}, {Lines: p2}}}
}

// CaixaConta imita um extrato sem linha de cabeçalho reconhecível, com o
// sinal num "D" ou "C" depois do valor e saldo em todas as linhas.
func CaixaConta() Doc {
	row := func(y float64, d, doc, desc, v, saldo string) Line {
		return Line{Y: y, Cells: []Cell{
			{X: 40, Text: d}, {X: 110, Text: doc}, {X: 170, Text: desc},
			{X: 460, Text: v, Right: true}, {X: 555, Text: saldo, Right: true},
		}}
	}
	return Doc{Pages: []Page{{Lines: []Line{
		{Y: 50, Size: 11, Bold: true, Cells: []Cell{{X: 40, Text: "Caixa Econômica Federal"}}},
		{Y: 64, Cells: []Cell{{X: 40, Text: "Extrato por período - 01/03/2026 a 31/03/2026"}}},
		{Y: 90, Cells: []Cell{{X: 40, Text: "SALDO ANTERIOR"}, {X: 555, Text: "800,00 C", Right: true}}},
		row(110, "02/03/2026", "000001", "CRED PIX CHAVE", "1.200,00 C", "2.000,00 C"),
		row(124, "03/03/2026", "000002", "SAQUE ATM", "300,00 D", "1.700,00 C"),
		row(138, "04/03/2026", "000003", "PAG AGUA", "2.000,00 D", "300,00 D"),
		row(152, "05/03/2026", "000004", "DEB JUROS", "12,34 D", "312,34 D"),
		row(166, "06/03/2026", "000005", "CRED TED", "500,00 C", "187,66 C"),
	}}}}
}

// Scanned imita um PDF escaneado: páginas sem nenhum texto.
func Scanned() Doc { return Doc{Pages: []Page{{}, {}}} }

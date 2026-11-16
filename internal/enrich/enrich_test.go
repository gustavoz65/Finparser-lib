package enrich

import "testing"

func TestMethod(t *testing.T) {
	tests := map[string]string{
		"Transferência recebida pelo Pix EMPRESA FICTÍCIA LTDA": Pix,
		"PIX TRANSF FULANO 02/03":                               Pix,
		"TED RECEBIDA 033 0001 EMPRESA X":                       TED,
		"DOC ENVIADO":                                           DOC,
		"PAG BOLETO ENERGIA ELETRICA":                           Boleto,
		"PAGAMENTO DE TITULO":                                   Boleto,
		"Compra no débito - PADARIA SÃO JOÃO":                   Card,
		"COMPRA CARTAO MERCADO":                                 Card,
		"SAQUE 24H":                                             Withdrawal,
		"DEPÓSITO EM DINHEIRO":                                  Deposit,
		"CHEQUE COMPENSADO 000123":                              Check,
		"DEBITO AUTOMATICO CIA DE AGUA":                         DirectDebit,
		"TAR PACOTE SERVICOS":                                   Fee,
		"IOF":                                                   Tax,
		"RENDIMENTO POUPANCA":                                   Interest,
		"APLICACAO CDB":                                         Investment,
		"TRANSFERENCIA ENTRE CONTAS":                            Transfer,
		// Palavra inteira: "documento" não é DOC, "tarde" não é tarifa.
		"DOCUMENTO 123": "",
		"PADARIA TARDE": "",
		"Salário":       "",
		"":              "",
	}
	for in, want := range tests {
		if got := Method(in); got != want {
			t.Errorf("Method(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCounterparty(t *testing.T) {
	tests := map[string]string{
		"Transferência recebida pelo Pix EMPRESA FICTÍCIA LTDA - 00.000.000/0001-00":     "EMPRESA FICTÍCIA LTDA",
		"Transferência enviada pelo Pix FULANO DE TAL - •••.000.000-•• - BANCO FICTÍCIO": "FULANO DE TAL",
		"PIX TRANSF FULANO 02/03":                               "FULANO",
		"PIX RECEBIDO DE MARIA SILVA":                           "MARIA SILVA",
		"Pix enviado - Fulano de Tal":                           "Fulano de Tal",
		"Compra no débito PADARIA SÃO JOÃO":                     "PADARIA SÃO JOÃO",
		"Compra no débito - PADARIA SÃO JOÃO":                   "PADARIA SÃO JOÃO",
		"TED RECEBIDA 033 0001 EMPRESA X":                       "EMPRESA X",
		"COMPRA CARTAO MERCADO":                                 "", // sem conectivo, não dá para saber
		"COMPRA CARTAO - MERCADO":                               "MERCADO",
		"COMPRA CARTAO DEB MC FARMACIA":                         "",
		"PIX QR CODE DINAMICO DES: POSTO FICTICIO":              "POSTO FICTICIO",
		"TED DIF.TITUL.CC H.BANK DEST. FULANO DE TAL":           "FULANO DE TAL",
		"TRANSFERENCIA PIX REM: EMPRESA FICTICIA COMERCIO LTDA": "EMPRESA FICTICIA COMERCIO LTDA",
		"PIX ENVIADO H.BANK FULANO":                             "",
		"PIX PARA BELTRANO":                                     "BELTRANO",
		"PIX ENVIADO":                                           "",
		"Pix - Recebido":                                        "",
		"Pix enviado - Pix - Fulano":                            "",
		"PAG BOLETO ENERGIA ELETRICA":                           "",
		"PIX 02/03 123":                                         "",
		"TAR PACOTE SERVICOS":                                   "",
		"Pagamento de fatura":                                   "",
		"RENDIMENTO POUPANCA":                                   "",
		"":                                                      "",
	}
	for in, want := range tests {
		if got := Counterparty(in); got != want {
			t.Errorf("Counterparty(%q) = %q, want %q", in, got, want)
		}
	}
}

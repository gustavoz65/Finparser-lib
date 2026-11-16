// Package enrich tira da descrição de um lançamento o que ela diz além do
// texto: o meio de pagamento (Pix, TED, boleto, cartão...) e a contraparte.
// Só responde quando o padrão é claro; na dúvida devolve vazio.
package enrich

import (
	"strings"
	"unicode"

	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

// Meios de pagamento reconhecidos.
const (
	Pix         = "pix"
	TED         = "ted"
	DOC         = "doc"
	Boleto      = "boleto"
	Card        = "card"
	Withdrawal  = "withdrawal"
	Deposit     = "deposit"
	Check       = "check"
	DirectDebit = "direct_debit"
	Fee         = "fee"
	Tax         = "tax"
	Interest    = "interest"
	Investment  = "investment"
	Transfer    = "transfer"
)

// rules vai do mais específico para o mais genérico: o trilho do dinheiro
// (Pix, TED, cartão) ganha de "transferência". word casa palavra inteira;
// text casa trecho.
var rules = []struct {
	method string
	words  []string
	texts  []string
}{
	{Pix, []string{"pix"}, nil},
	{TED, []string{"ted"}, nil},
	{DOC, []string{"doc"}, nil},
	{Boleto, []string{"boleto", "boletos"}, []string{"pagamento de titulo", "pagto titulo", "pag titulo", "pagamento de conta"}},
	{DirectDebit, nil, []string{"debito automatico", "deb automatico", "deb aut"}},
	{Card, []string{"compra", "compras", "cartao", "visa", "mastercard", "maestro"}, []string{"no debito", "no credito"}},
	{Withdrawal, []string{"saque", "saques"}, nil},
	{Deposit, []string{"deposito", "dep"}, nil},
	{Check, []string{"cheque", "chq"}, nil},
	{Tax, []string{"iof", "irrf", "imposto", "darf"}, nil},
	{Fee, []string{"tarifa", "tarifas", "tar", "anuidade", "cesta"}, []string{"pacote de servicos", "taxa de"}},
	{Interest, []string{"juros", "encargos", "rendimento", "rendimentos", "rend"}, nil},
	{Investment, []string{"aplicacao", "resgate", "cdb", "poupanca", "tesouro"}, nil},
	{Transfer, []string{"transferencia", "transf", "tev"}, nil},
}

// Method devolve o meio de pagamento, ou "".
func Method(desc string) string {
	key := textnorm.Key(desc)
	if key == "" {
		return ""
	}
	words := map[string]bool{}
	for _, w := range strings.FieldsFunc(key, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		words[w] = true
	}
	for _, r := range rules {
		for _, w := range r.words {
			if words[w] {
				return r.method
			}
		}
		for _, t := range r.texts {
			if strings.Contains(key, t) {
				return r.method
			}
		}
	}
	return ""
}

// prefixes são as aberturas de descrição depois das quais vem o nome da
// contraparte. Mais longas primeiro. As de bare exigem um conectivo ("DE",
// "PARA", "-") antes do nome: depois de um "PIX" ou "COMPRA" solto pode vir
// qualquer coisa ("PIX QR CODE DINAMICO", "COMPRA CARTAO DEB MC").
var prefixes = []struct {
	words []string
	bare  bool
}{
	{strings.Fields("transferencia recebida pelo pix"), false},
	{strings.Fields("transferencia enviada pelo pix"), false},
	{strings.Fields("transferencia recebida"), false},
	{strings.Fields("transferencia enviada"), false},
	{strings.Fields("pix recebido"), false},
	{strings.Fields("pix enviado"), false},
	{strings.Fields("pix transf"), false},
	{strings.Fields("ted recebida"), false},
	{strings.Fields("ted enviada"), false},
	{strings.Fields("doc recebido"), false},
	{strings.Fields("doc enviado"), false},
	{strings.Fields("compra no debito"), false},
	{strings.Fields("compra no credito"), false},
	{strings.Fields("compra com cartao"), true},
	{strings.Fields("compra cartao"), true},
	{strings.Fields("compra"), true},
	{strings.Fields("pix"), true},
	{strings.Fields("ted"), true},
}

// markers anunciam o nome em qualquer ponto da descrição: "REM: FULANO",
// "DEST. FULANO", "FAV: FULANO".
var markers = map[string]bool{
	"rem": true, "remet": true, "remetente": true, "des": true, "dest": true, "destino": true,
	"destinatario": true, "fav": true, "favorecido": true, "pagador": true, "beneficiario": true,
}

// generic são palavras que sobram depois do prefixo e não são nome de
// ninguém ("Pix - Recebido").
var generic = map[string]bool{
	"recebido": true, "recebida": true, "enviado": true, "enviada": true, "devolvido": true,
	"devolvida": true, "estorno": true, "estornado": true, "agendado": true, "agendada": true,
	"transferencia": true, "pagamento": true, "credito": true, "debito": true, "pix": true,
	"qr": true, "code": true, "automatico": true, "parcelada": true, "parcelado": true,
	"internacional": true, "nacional": true, "online": true, "saque": true, "troco": true,
}

// Counterparty devolve o nome de quem pagou ou recebeu, ou "". Exemplos:
//
//	"Transferência enviada pelo Pix FULANO DE TAL - •••.000.000-•• - BANCO X" → "FULANO DE TAL"
//	"PIX RECEBIDO DE MARIA SILVA"                                             → "MARIA SILVA"
//	"Compra no débito - PADARIA SÃO JOÃO"                                     → "PADARIA SÃO JOÃO"
//	"TED DIF.TITUL.CC DEST. FULANO DE TAL"                                    → "FULANO DE TAL"
func Counterparty(desc string) string {
	words := strings.Fields(desc)
	keys := make([]string, len(words))
	for i, w := range words {
		keys[i] = textnorm.Key(w)
	}

	// Marcador explícito ganha de tudo.
	for i, k := range keys {
		if n := len(k); n > 1 && (k[n-1] == ':' || k[n-1] == '.') && markers[k[:n-1]] {
			return name(words[i+1:], keys[i+1:])
		}
	}

	for _, p := range prefixes {
		if !hasPrefix(keys, p.words) {
			continue
		}
		rest, restKeys := words[len(p.words):], keys[len(p.words):]
		if p.bare && (len(rest) == 0 || !isConnector(restKeys[0])) {
			return ""
		}
		return name(rest, restKeys)
	}
	return ""
}

// name limpa o que vem depois do prefixo ou marcador e devolve o nome, ou
// "" se não sobrar um.
func name(words, keys []string) string {
	// Conectivos e números soltos antes do nome ("DE", "-", "033 0001").
	for len(words) > 0 && (isConnector(keys[0]) || !hasLetter(words[0])) {
		words, keys = words[1:], keys[1:]
	}
	// O nome vai até o primeiro " - " (depois vêm documento e banco).
	for i, w := range words {
		if w == "-" || w == "|" {
			words = words[:i]
			break
		}
	}
	// Data ou número pendurado no fim ("FULANO 02/03").
	for len(words) > 0 && !hasLetter(words[len(words)-1]) {
		words = words[:len(words)-1]
	}
	named := false
	for _, w := range words {
		k := textnorm.Key(w)
		// Abreviação ou rótulo no meio ("DIF.TITUL.CC", "DES:") não é nome.
		if strings.ContainsAny(strings.TrimRight(w, ".,"), ".:") {
			return ""
		}
		if hasLetter(w) && !generic[k] {
			named = true
		}
	}
	if !named {
		return ""
	}
	return strings.Join(words, " ")
}

func hasPrefix(keys, prefix []string) bool {
	if len(keys) < len(prefix) {
		return false
	}
	for i, p := range prefix {
		if keys[i] != p {
			return false
		}
	}
	return true
}

func isConnector(k string) bool {
	switch k {
	case "de", "para", "a", "-", ":", "em", "p/", "|":
		return true
	}
	return false
}

func hasLetter(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0
}

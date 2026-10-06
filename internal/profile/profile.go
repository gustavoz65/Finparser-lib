// Package profile guarda dicas por banco. O motor genérico roda sempre; um
// perfil só ajusta parâmetros (nome do banco, linhas a ignorar).
package profile

import (
	"regexp"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

// Hints são ajustes que um perfil aplica ao motor genérico.
type Hints struct {
	// Skip lista padrões de linhas (texto normalizado com textnorm.Key) que
	// não são transação nem cabeçalho e devem ser ignoradas.
	Skip []*regexp.Regexp
}

// Profile identifica um banco pelo texto do começo do documento.
type Profile interface {
	Name() string
	Match(headText string) bool
	Hints() Hints
}

type bank struct {
	name    string
	code    string   // código COMPE sem zeros à esquerda
	markers []string // trechos normalizados (textnorm.Key)
	hints   Hints
}

func (b bank) Name() string { return b.name }
func (b bank) Hints() Hints { return b.hints }
func (b bank) Match(s string) bool {
	k := textnorm.Key(s)
	for _, m := range b.markers {
		if strings.Contains(k, m) {
			return true
		}
	}
	return false
}

var banks = []bank{
	{
		name: "nubank", code: "260",
		markers: []string{"nu pagamentos", "nubank", "data,valor,identificador,descricao"},
		hints: Hints{Skip: []*regexp.Regexp{
			regexp.MustCompile(`^(saldo inicial|saldo final do periodo|rendimento liquido)\b`),
			regexp.MustCompile(`^(tem alguma duvida|caso a solucao|ouvidoria|extrato gerado)`),
		}},
	},
	{name: "itau", code: "341", markers: []string{"itau unibanco", "banco itau"}},
	{name: "inter", code: "77", markers: []string{"banco inter", "inter&co"}},
	{name: "bb", code: "1", markers: []string{"banco do brasil"}},
	{name: "bradesco", code: "237", markers: []string{"bradesco"}},
	{name: "santander", code: "33", markers: []string{"santander"}},
	{name: "caixa", code: "104", markers: []string{"caixa economica federal"}},
	{name: "c6", code: "336", markers: []string{"c6 bank", "banco c6"}},
}

// Lookup devolve o perfil pelo nome ("nubank", "itau"...).
func Lookup(name string) (Profile, bool) {
	name = textnorm.Key(name)
	for _, b := range banks {
		if b.name == name {
			return b, true
		}
	}
	return nil, false
}

// Detect devolve o primeiro perfil cujo marcador aparece em headText, ou
// nil. Passe só o começo do documento: descrições de transações citam
// outros bancos ("PIX para ITAU UNIBANCO").
func Detect(headText string) Profile {
	for _, b := range banks {
		if b.Match(headText) {
			return b
		}
	}
	return nil
}

// ByCode devolve o nome do banco pelo código COMPE ("260", "0341").
func ByCode(code string) string {
	code = strings.TrimLeft(strings.TrimSpace(code), "0")
	for _, b := range banks {
		if b.code == code {
			return b.name
		}
	}
	return ""
}

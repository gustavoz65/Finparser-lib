package ofx

import (
	"fmt"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/date"
	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/gustavoz65/finparser-lib/internal/money"
	"github.com/gustavoz65/finparser-lib/internal/profile"
	"github.com/gustavoz65/finparser-lib/internal/record"
	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

type node struct {
	name     string
	value    string
	line     int
	children []*node
}

// child devolve o primeiro filho direto com o nome dado.
func (n *node) child(name string) *node {
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

// value devolve o valor do filho direto, ou "".
func (n *node) get(name string) string {
	if c := n.child(name); c != nil {
		return c.value
	}
	return ""
}

// find devolve todos os descendentes com o nome dado, em ordem.
func (n *node) find(name string) []*node {
	var out []*node
	var walk func(*node)
	walk = func(x *node) {
		for _, c := range x.children {
			if c.name == name {
				out = append(out, c)
				continue
			}
			walk(c)
		}
	}
	walk(n)
	return out
}

// build monta a árvore a partir dos tokens. Um fechamento sem abertura
// correspondente (comum em XML: </NAME> depois de uma folha) é ignorado; um
// fechamento que pula níveis fecha tudo até a abertura correspondente.
//
// Uma tag vazia cujo nome nunca é fechado no arquivo (<FITID> sem valor no
// SGML) e não é agregado conhecido vira folha vazia: senão engoliria as
// tags seguintes.
func build(toks []token) *node {
	closed := map[string]bool{}
	for _, t := range toks {
		if t.kind == tokClose {
			closed[t.name] = true
		}
	}

	root := &node{name: "#root"}
	stack := []*node{root}
	for _, t := range toks {
		top := stack[len(stack)-1]
		switch t.kind {
		case tokOpen:
			if !closed[t.name] && !aggregates[t.name] {
				top.children = append(top.children, &node{name: t.name, line: t.line})
				continue
			}
			// <STMTTRN> sem </STMTTRN> antes do próximo: são irmãos.
			if top.name == t.name && len(stack) > 1 {
				stack = stack[:len(stack)-1]
				top = stack[len(stack)-1]
			}
			n := &node{name: t.name, line: t.line}
			top.children = append(top.children, n)
			stack = append(stack, n)
		case tokLeaf:
			top.children = append(top.children, &node{name: t.name, value: t.value, line: t.line})
		case tokClose:
			for i := len(stack) - 1; i > 0; i-- {
				if stack[i].name == t.name {
					stack = stack[:i]
					break
				}
			}
		}
	}
	return root
}

// aggregates são os agregados OFX que importam para a extração; contam como
// agregado mesmo quando o arquivo vem truncado sem o fechamento.
var aggregates = map[string]bool{
	"OFX": true, "SIGNONMSGSRSV1": true, "SONRS": true, "STATUS": true, "FI": true,
	"BANKMSGSRSV1": true, "STMTTRNRS": true, "STMTRS": true, "BANKACCTFROM": true,
	"BANKTRANLIST": true, "STMTTRN": true, "LEDGERBAL": true, "AVAILBAL": true,
	"CREDITCARDMSGSRSV1": true, "CCSTMTTRNRS": true, "CCSTMTRS": true, "CCACCTFROM": true,
	"PAYEE": true, "BANKACCTTO": true, "CCACCTTO": true, "CURRENCY": true, "ORIGCURRENCY": true,
}

// Parse lê um OFX já em UTF-8. Suporta extrato de conta (STMTRS) e de
// cartão (CCSTMTRS).
func Parse(text string) (*record.Result, error) {
	root := build(tokenize(text))

	stmts := append(root.find("STMTRS"), root.find("CCSTMTRS")...)
	if len(stmts) == 0 {
		return nil, fmt.Errorf("%w: OFX sem STMTRS/CCSTMTRS", errs.ErrMalformed)
	}

	res := &record.Result{}
	if fi := root.find("FI"); len(fi) > 0 {
		res.Bank = bankFromOrg(fi[0].get("ORG"))
	}

	found := false
	for _, st := range stmts {
		if acct := st.child("BANKACCTFROM"); acct != nil && res.Bank == "" {
			res.Bank = profile.ByCode(acct.get("BANKID"))
		}
		lists := st.find("BANKTRANLIST")
		if len(lists) == 0 {
			continue
		}
		found = true
		for _, list := range lists {
			start, _ := date.ParseCompact(list.get("DTSTART"))
			end, _ := date.ParseCompact(list.get("DTEND"))
			if start.HasYear && (res.PeriodStart.IsZero() || start.Time().Before(res.PeriodStart)) {
				res.PeriodStart = start.Time()
			}
			if end.HasYear && end.Time().After(res.PeriodEnd) {
				res.PeriodEnd = end.Time()
			}
			for _, trn := range list.find("STMTTRN") {
				if rec, ok := parseTrn(trn, res); ok {
					res.Records = append(res.Records, rec)
				}
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("%w: OFX sem BANKTRANLIST", errs.ErrMalformed)
	}
	return res, nil
}

var debitTypes = map[string]bool{
	"DEBIT": true, "PAYMENT": true, "FEE": true, "SRVCHG": true,
	"ATM": true, "POS": true, "CHECK": true, "CASH": true,
}

func parseTrn(n *node, res *record.Result) (record.Record, bool) {
	raw := make([]string, 0, len(n.children))
	for _, c := range n.children {
		if c.value != "" {
			raw = append(raw, c.name+"="+c.value)
		}
	}
	rawLine := strings.Join(raw, "; ")

	dt := n.get("DTPOSTED")
	if dt == "" {
		dt = n.get("DTUSER")
	}
	d, err := date.ParseCompact(dt)
	if err != nil {
		res.Warn(0, n.line, rawLine, fmt.Sprintf("data inválida %q", dt))
		return record.Record{}, false
	}
	amt, err := money.Parse(n.get("TRNAMT"))
	if err != nil {
		res.Warn(0, n.line, rawLine, fmt.Sprintf("valor inválido %q", n.get("TRNAMT")))
		return record.Record{}, false
	}

	// Alguns bancos mandam TRNTYPE=DEBIT com valor positivo.
	trnType := strings.ToUpper(n.get("TRNTYPE"))
	if debitTypes[trnType] && amt.IsPositive() {
		amt = amt.Neg()
		res.Warn(0, n.line, rawLine, "TRNTYPE de débito com valor positivo; sinal invertido")
	}

	name := n.get("NAME")
	if payee := n.child("PAYEE"); payee != nil && name == "" {
		name = payee.get("NAME")
	}
	return record.Record{
		Date:        d.Time(),
		Description: description(name, n.get("MEMO")),
		Amount:      amt,
		SignKnown:   true,
		ID:          n.get("FITID"),
		Raw:         raw,
		Line:        n.line,
	}, true
}

// description combina NAME e MEMO sem repetir texto.
func description(name, memo string) string {
	name, memo = textnorm.Spaces(name), textnorm.Spaces(memo)
	switch {
	case memo == "":
		return name
	case name == "":
		return memo
	}
	kn, km := textnorm.Key(name), textnorm.Key(memo)
	switch {
	case strings.Contains(km, kn):
		return memo
	case strings.Contains(kn, km):
		return name
	}
	return name + " - " + memo
}

func bankFromOrg(org string) string {
	if org == "" {
		return ""
	}
	if p := profile.Detect(org); p != nil {
		return p.Name()
	}
	return ""
}

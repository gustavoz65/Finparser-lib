// Package ofx é um parser OFX tolerante, escrito para os arquivos que bancos
// brasileiros realmente exportam: v1 SGML com tags folha sem fechamento,
// cabeçalho chave:valor, valores com vírgula e FITID vazio ou repetido.
//
// encoding/xml não serve aqui porque falha nas tags sem fechamento.
package ofx

import (
	"html"
	"strings"
)

type tokenKind int

const (
	tokOpen tokenKind = iota
	tokClose
	tokLeaf
)

type token struct {
	kind  tokenKind
	name  string
	value string
	line  int
}

// tokenize emite open(tag), close(tag) e leaf(tag, valor). Tudo antes do
// primeiro '<' (cabeçalho SGML) é descartado. Nunca falha: entrada truncada
// só produz menos tokens.
func tokenize(s string) []token {
	var toks []token
	line := 1
	i := strings.IndexByte(s, '<')
	if i < 0 {
		return nil
	}
	line += strings.Count(s[:i], "\n")

	for i < len(s) {
		// s[i] == '<'
		if strings.HasPrefix(s[i:], "<!--") {
			c := strings.Index(s[i:], "-->")
			if c < 0 {
				break
			}
			line += strings.Count(s[i:i+c], "\n")
			i += c + 3
			i = skipTo(s, i, &line)
			continue
		}

		end := strings.IndexByte(s[i:], '>')
		if end < 0 {
			break
		}
		tag := s[i+1 : i+end]
		tagLine := line
		line += strings.Count(tag, "\n")
		i += end + 1

		// Texto até o próximo '<'.
		text := s[i:]
		if next := strings.IndexByte(text, '<'); next >= 0 {
			text = text[:next]
		}

		switch {
		case strings.HasPrefix(tag, "?"), strings.HasPrefix(tag, "!"), strings.HasSuffix(tag, "/"):
			// Instrução de processamento, DOCTYPE ou elemento vazio.
		case strings.HasPrefix(tag, "/"):
			if name := tagName(tag[1:]); name != "" {
				toks = append(toks, token{kind: tokClose, name: name, line: tagLine})
			}
		default:
			name := tagName(tag)
			if name == "" {
				break
			}
			value := strings.TrimSpace(html.UnescapeString(text))
			if value != "" {
				toks = append(toks, token{kind: tokLeaf, name: name, value: value, line: tagLine})
			} else {
				toks = append(toks, token{kind: tokOpen, name: name, line: tagLine})
			}
		}

		line += strings.Count(text, "\n")
		i += len(text)
	}
	return toks
}

// skipTo avança i até o próximo '<' (ou o fim), contando linhas.
func skipTo(s string, i int, line *int) int {
	k := strings.IndexByte(s[i:], '<')
	if k < 0 {
		return len(s)
	}
	*line += strings.Count(s[i:i+k], "\n")
	return i + k
}

// tagName devolve o nome da tag em maiúsculas, sem atributos.
func tagName(tag string) string {
	f := strings.Fields(tag)
	if len(f) == 0 {
		return ""
	}
	return strings.ToUpper(f[0])
}

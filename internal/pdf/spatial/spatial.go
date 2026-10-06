// Package spatial reconstrói a leitura humana de uma página de PDF a partir
// da nuvem de tokens: glifos → palavras → linhas → colunas.
//
// Três eixos: Page separa contexto, Y agrupa linhas, X separa colunas. Y do
// PDF cresce de baixo para cima, então ler de cima para baixo é ordenar por
// Y decrescente. Toda tolerância é relativa ao tamanho da fonte.
package spatial

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/gustavoz65/finparser-lib/internal/pdf/extract"
)

// Word é uma palavra com extensão horizontal [X0, X1].
type Word struct {
	Page     int
	X0, X1   float64
	Y        float64
	FontSize float64
	Text     string
}

// Line é uma linha visual: palavras ordenadas por X0.
type Line struct {
	Page     int
	Y        float64 // mediana do Y das palavras
	FontSize float64 // mediana do tamanho de fonte
	Words    []Word
}

// Text junta as palavras com espaço.
func (l Line) Text() string {
	parts := make([]string, len(l.Words))
	for i, w := range l.Words {
		parts[i] = w.Text
	}
	return strings.Join(parts, " ")
}

// X0 é a borda esquerda da linha.
func (l Line) X0() float64 { return l.Words[0].X0 }

// X1 é a borda direita da linha.
func (l Line) X1() float64 { return l.Words[len(l.Words)-1].X1 }

// Words junta glifos em palavras (passo 1). Na mesma página e com Y
// praticamente igual (|ΔY| < 0,2 × fonte), ordenados por X: se o espaço
// entre o fim do anterior e o início do atual for < 0,25 × fonte, concatena;
// senão começa palavra nova. Tokens de espaço sempre separam.
func Words(toks []extract.Token) []Word {
	toks = splitSpaces(toks)
	sort.SliceStable(toks, func(i, j int) bool {
		a, b := toks[i], toks[j]
		if a.Page != b.Page {
			return a.Page < b.Page
		}
		if a.Y != b.Y {
			return a.Y > b.Y
		}
		return a.X < b.X
	})

	var words []Word
	for start := 0; start < len(toks); {
		// Linha de base: tokens com Y próximo do primeiro.
		first := toks[start]
		tol := 0.2 * fontOr(first.FontSize)
		end := start + 1
		for end < len(toks) && toks[end].Page == first.Page && math.Abs(toks[end].Y-first.Y) < tol {
			end++
		}
		row := append([]extract.Token(nil), toks[start:end]...)
		sort.SliceStable(row, func(i, j int) bool { return row[i].X < row[j].X })
		words = append(words, mergeRow(row)...)
		start = end
	}
	return words
}

func mergeRow(row []extract.Token) []Word {
	var (
		out []Word
		cur *Word
		sb  strings.Builder
		fs  []float64
	)
	flush := func() {
		if cur != nil {
			cur.Text = sb.String()
			cur.FontSize = median(fs)
			out = append(out, *cur)
		}
		cur, fs = nil, fs[:0]
		sb.Reset()
	}
	for _, t := range row {
		if strings.TrimFunc(t.Text, isSpace) == "" {
			flush()
			continue
		}
		font := fontOr(t.FontSize)
		if cur != nil {
			gap := t.X - cur.X1
			if gap >= 0.25*font || gap < -0.5*font {
				flush()
			}
		}
		if cur == nil {
			cur = &Word{Page: t.Page, X0: t.X, X1: t.X + t.W, Y: t.Y}
		}
		sb.WriteString(t.Text)
		cur.X1 = math.Max(cur.X1, t.X+t.W)
		fs = append(fs, font)
	}
	flush()
	return out
}

// splitSpaces quebra tokens que já vêm com espaços dentro (backends que
// devolvem frases inteiras), distribuindo a largura proporcionalmente.
func splitSpaces(toks []extract.Token) []extract.Token {
	out := make([]extract.Token, 0, len(toks))
	for _, t := range toks {
		runes := []rune(t.Text)
		if len(runes) <= 1 || strings.IndexFunc(t.Text, isSpace) < 0 {
			out = append(out, t)
			continue
		}
		per := t.W / float64(len(runes))
		start := -1
		for i := 0; i <= len(runes); i++ {
			if i < len(runes) && !isSpace(runes[i]) {
				if start < 0 {
					start = i
				}
				continue
			}
			if start >= 0 {
				p := t
				p.X = t.X + float64(start)*per
				p.W = float64(i-start) * per
				p.Text = string(runes[start:i])
				out = append(out, p)
				start = -1
			}
		}
	}
	return out
}

// Lines agrupa palavras em linhas (passo 2). Duas palavras estão na mesma
// linha se |ΔY| ≤ tolerance × min(fonte A, fonte B), comparando com a
// mediana do Y da linha atual (não com o último elemento, senão a linha
// "escorrega" em texto inclinado). tolerance padrão: 0,5.
func Lines(words []Word, tolerance float64) []Line {
	if tolerance <= 0 {
		tolerance = 0.5
	}
	ws := append([]Word(nil), words...)
	sort.SliceStable(ws, func(i, j int) bool {
		if ws[i].Page != ws[j].Page {
			return ws[i].Page < ws[j].Page
		}
		return ws[i].Y > ws[j].Y
	})

	var (
		lines []Line
		cur   []Word
		ys    []float64
		fonts []float64
	)
	flush := func() {
		if len(cur) == 0 {
			return
		}
		sort.SliceStable(cur, func(i, j int) bool { return cur[i].X0 < cur[j].X0 })
		lines = append(lines, Line{Page: cur[0].Page, Y: median(ys), FontSize: median(fonts), Words: cur})
		cur, ys, fonts = nil, nil, nil
	}
	for _, w := range ws {
		if len(cur) > 0 {
			my := median(ys)
			lineFont := median(fonts)
			if w.Page != cur[0].Page || math.Abs(w.Y-my) > tolerance*math.Min(fontOr(w.FontSize), lineFont) {
				flush()
			}
		}
		cur = append(cur, w)
		ys = append(ys, w.Y)
		fonts = append(fonts, fontOr(w.FontSize))
	}
	flush()
	return lines
}

// Build é o atalho tokens → palavras → linhas.
func Build(toks []extract.Token, tolerance float64) []Line {
	return Lines(Words(toks), tolerance)
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func fontOr(f float64) float64 {
	if f <= 0 {
		return 10
	}
	return f
}

func isSpace(r rune) bool { return unicode.IsSpace(r) || r == ' ' }

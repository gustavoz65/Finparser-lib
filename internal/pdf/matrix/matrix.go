// Package matrix guarda o espaço do documento como um tensor de ocupação
// Página × Linha × X e faz as contas do motor espacial sobre ele.
//
// O tensor é uma matriz de bits densa num único []uint64, em ordem de linha
// (row-major): cada linha visual do PDF é uma linha da matriz, cada coluna é
// uma faixa de Step pontos do eixo X, e o bit diz "há tinta aqui". O eixo
// das páginas é uma fatia de linhas consecutivas (Slice), já que as linhas
// chegam ordenadas por página.
//
// Tudo o que o motor precisa vira aritmética de palavra:
//
//   - preencher um intervalo [a, b] são duas máscaras e um laço de OR;
//   - a projeção no eixo X (quantas linhas ocupam cada coluna) é uma soma
//     vertical feita com contadores bit-sliced: cada operação soma 64
//     colunas de uma vez, sem olhar bit a bit;
//   - calhas (colunas vazias) e vales (ponto menos ocupado entre dois
//     cabeçalhos) saem do vetor de projeção.
//
// Sem alocação por linha nem por palavra: uma alocação para a matriz e uma
// para cada projeção pedida.
package matrix

import (
	"math"
	"math/bits"
	"strings"
)

// MaxWords limita o tamanho da matriz (em uint64): 32 MB. PDFs reais ficam
// milhares de vezes abaixo; o teto existe para entrada maliciosa.
const MaxWords = 1 << 22

// MaxCols limita a largura analisada. Uma página A4 tem ~600 pontos.
const MaxCols = 20000

// Grid é a matriz de ocupação.
type Grid struct {
	x0     float64
	step   float64
	cols   int
	stride int // uint64 por linha
	rows   int
	bits   []uint64
}

// New cria uma matriz de rows linhas cobrindo [x0, x1] com células de step
// pontos. Devolve nil se as dimensões forem inválidas ou absurdas.
func New(rows int, x0, x1, step float64) *Grid {
	span := (x1 - x0) / step
	if rows <= 0 || step <= 0 || !(span >= 0 && span <= MaxCols) {
		return nil
	}
	cols := int(math.Ceil(span)) + 1
	stride := (cols + 63) / 64
	if rows > MaxWords/stride {
		return nil
	}
	return &Grid{x0: x0, step: step, cols: cols, stride: stride, rows: rows, bits: make([]uint64, rows*stride)}
}

// Rows devolve o número de linhas.
func (g *Grid) Rows() int { return g.rows }

// Cols devolve o número de colunas.
func (g *Grid) Cols() int { return g.cols }

// Col devolve a coluna que contém a coordenada x, limitada à matriz.
func (g *Grid) Col(x float64) int {
	c := int(math.Floor((x - g.x0) / g.step))
	return min(max(c, 0), g.cols-1)
}

// X devolve a coordenada da borda esquerda da coluna c.
func (g *Grid) X(c int) float64 { return g.x0 + float64(c)*g.step }

// Fill marca como ocupado o intervalo [a, b] da linha row: da célula que
// contém a até a que contém b, inclusive.
func (g *Grid) Fill(row int, a, b float64) {
	if row < 0 || row >= g.rows || !(b >= a) {
		return
	}
	lo := int(math.Floor((a - g.x0) / g.step))
	hi := int(math.Ceil((b - g.x0) / g.step))
	if hi < 0 || lo >= g.cols {
		return
	}
	lo, hi = max(lo, 0), min(hi, g.cols-1)

	w := g.bits[row*g.stride : (row+1)*g.stride]
	first, last := lo>>6, hi>>6
	head := ^uint64(0) << (uint(lo) & 63)
	tail := ^uint64(0) >> (63 - uint(hi)&63)
	if first == last {
		w[first] |= head & tail
		return
	}
	w[first] |= head
	for i := first + 1; i < last; i++ {
		w[i] = ^uint64(0)
	}
	w[last] |= tail
}

// Get informa se a célula (row, col) está ocupada.
func (g *Grid) Get(row, col int) bool {
	if row < 0 || row >= g.rows || col < 0 || col >= g.cols {
		return false
	}
	return g.bits[row*g.stride+col>>6]>>(uint(col)&63)&1 == 1
}

// Projection soma as linhas [from, to) no eixo X: out[c] é quantas delas
// ocupam a coluna c. É a "sombra" da tabela no eixo horizontal.
//
// A soma usa contadores bit-sliced: planes[p] guarda o bit p do contador de
// cada coluna. Somar uma linha é propagar o vai-um pelos planos com AND e
// XOR — 64 colunas por operação.
func (g *Grid) Projection(from, to int) []int32 {
	out := make([]int32, g.cols)
	from, to = max(from, 0), min(to, g.rows)
	if from >= to {
		return out
	}
	nplanes := bits.Len(uint(to - from))
	planes := make([]uint64, nplanes*g.stride)
	carry := make([]uint64, g.stride)

	for r := from; r < to; r++ {
		copy(carry, g.bits[r*g.stride:(r+1)*g.stride])
		for p := 0; p < nplanes; p++ {
			plane := planes[p*g.stride : (p+1)*g.stride]
			var any uint64
			for i, c := range carry {
				t := plane[i] & c
				plane[i] ^= c
				carry[i] = t
				any |= t
			}
			if any == 0 {
				break
			}
		}
	}

	// Planos → inteiros: só percorre os bits ligados.
	for p := 0; p < nplanes; p++ {
		plane := planes[p*g.stride : (p+1)*g.stride]
		for i, w := range plane {
			for w != 0 {
				c := i<<6 + bits.TrailingZeros64(w)
				out[c] += 1 << uint(p)
				w &= w - 1
			}
		}
	}
	return out
}

// Run é um intervalo de colunas [From, To).
type Run struct{ From, To int }

// Gutters devolve os trechos de profile com ocupação ≤ limit e pelo menos
// minWidth colunas: as calhas entre colunas da tabela.
func Gutters(profile []int32, limit int32, minWidth int) []Run {
	var out []Run
	for c := 0; c < len(profile); {
		if profile[c] > limit {
			c++
			continue
		}
		e := c
		for e < len(profile) && profile[e] <= limit {
			e++
		}
		if e-c >= minWidth {
			out = append(out, Run{c, e})
		}
		c = e
	}
	return out
}

// Valley devolve o centro do trecho menos ocupado de profile em [from, to):
// entre os trechos com o valor mínimo, o mais largo. É onde passa a
// fronteira entre duas colunas vizinhas. ok é false se o intervalo é vazio.
func Valley(profile []int32, from, to int) (center float64, ok bool) {
	from, to = max(from, 0), min(to, len(profile))
	if from >= to {
		return 0, false
	}
	lowest := profile[from]
	for _, v := range profile[from:to] {
		lowest = min(lowest, v)
	}
	best := Run{}
	for c := from; c < to; {
		if profile[c] != lowest {
			c++
			continue
		}
		e := c
		for e < to && profile[e] == lowest {
			e++
		}
		if e-c > best.To-best.From {
			best = Run{c, e}
		}
		c = e
	}
	return float64(best.From+best.To) / 2, true
}

// String desenha a matriz: uma linha de texto por linha, '#' ocupado e '.'
// vazio. Serve para depuração.
func (g *Grid) String() string {
	var sb strings.Builder
	sb.Grow(g.rows * (g.cols + 1))
	for r := 0; r < g.rows; r++ {
		for c := 0; c < g.cols; c++ {
			if g.Get(r, c) {
				sb.WriteByte('#')
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

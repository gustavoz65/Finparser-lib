package matrix

import (
	"math/rand"
	"testing"
)

func TestFillAndGet(t *testing.T) {
	g := New(2, 10, 210, 1)
	g.Fill(0, 12.3, 14.2) // células 2..5
	g.Fill(1, 70, 140)    // atravessa palavras de 64 bits
	g.Fill(1, -50, 10.5)  // cortado à esquerda
	g.Fill(1, 205, 9999)  // cortado à direita
	g.Fill(0, 500, 600)   // fora
	g.Fill(0, 20, 19)     // invertido
	g.Fill(5, 0, 1)       // linha inexistente
	want0 := map[int]bool{2: true, 3: true, 4: true, 5: true}
	for c := 0; c < g.Cols(); c++ {
		if g.Get(0, c) != want0[c] {
			t.Errorf("linha 0, coluna %d = %v", c, g.Get(0, c))
		}
		want1 := c <= 1 || c >= 60 && c <= 130 || c >= 195
		if g.Get(1, c) != want1 {
			t.Errorf("linha 1, coluna %d = %v", c, g.Get(1, c))
		}
	}
	if g.Get(0, -1) || g.Get(9, 0) || g.Get(0, g.Cols()) {
		t.Error("Get fora da matriz deveria ser false")
	}
}

func TestNewRejectsAbsurdSizes(t *testing.T) {
	for _, g := range []*Grid{
		New(0, 0, 100, 1), New(1, 0, 100, 0), New(1, 100, 0, 1),
		New(1, 0, 1e9, 1), New(MaxWords, 0, 600, 1),
	} {
		if g != nil {
			t.Errorf("deveria ser nil: %d×%d", g.Rows(), g.Cols())
		}
	}
	if g := New(1, 5, 5, 1); g == nil || g.Cols() != 1 {
		t.Error("largura zero é válida: uma coluna")
	}
}

// naive é a projeção de referência, bit a bit.
func naive(g *Grid, from, to int) []int32 {
	out := make([]int32, g.Cols())
	for r := from; r < to; r++ {
		for c := 0; c < g.Cols(); c++ {
			if g.Get(r, c) {
				out[c]++
			}
		}
	}
	return out
}

func random(rows int, rng *rand.Rand) *Grid {
	g := New(rows, 30, 565, 1)
	for r := 0; r < rows; r++ {
		for k := rng.Intn(8); k > 0; k-- {
			a := 30 + rng.Float64()*535
			g.Fill(r, a, a+rng.Float64()*80)
		}
	}
	return g
}

func TestProjectionMatchesNaive(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, rows := range []int{1, 2, 3, 63, 64, 65, 200, 1023} {
		g := random(rows, rng)
		for _, span := range [][2]int{{0, rows}, {rows / 3, rows}, {0, rows / 2}, {-5, rows + 5}} {
			got := g.Projection(span[0], span[1])
			want := naive(g, max(span[0], 0), min(span[1], rows))
			for c := range want {
				if got[c] != want[c] {
					t.Fatalf("rows=%d span=%v coluna %d: %d, want %d", rows, span, c, got[c], want[c])
				}
			}
		}
	}
	if p := random(4, rng).Projection(3, 3); len(p) == 0 || p[0] != 0 {
		t.Error("intervalo vazio deveria dar zeros")
	}
}

func TestGutters(t *testing.T) {
	p := []int32{0, 0, 5, 5, 1, 1, 1, 5, 0, 5, 0, 0, 0}
	got := Gutters(p, 1, 2)
	want := []Run{{0, 2}, {4, 7}, {10, 13}}
	if len(got) != len(want) {
		t.Fatalf("Gutters = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Gutters[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestValley(t *testing.T) {
	//            0  1  2  3  4  5  6  7  8  9
	p := []int32{9, 3, 0, 3, 0, 0, 0, 2, 9, 0}
	if c, ok := Valley(p, 0, 9); !ok || c != 5.5 { // trecho mais largo de zeros: [4,7)
		t.Errorf("Valley = %v %v", c, ok)
	}
	if c, ok := Valley(p, 7, 9); !ok || c != 7.5 { // sem zero: o mínimo é 2
		t.Errorf("Valley sem vazio = %v %v", c, ok)
	}
	if _, ok := Valley(p, 5, 5); ok {
		t.Error("intervalo vazio deveria falhar")
	}
	if c, ok := Valley(p, -3, 99); !ok || c != 5.5 {
		t.Errorf("Valley com limites fora = %v %v", c, ok)
	}
}

func TestString(t *testing.T) {
	g := New(2, 0, 4, 1)
	g.Fill(0, 1, 2)
	g.Fill(1, 4, 4)
	if got := g.String(); got != ".##..\n....#\n" {
		t.Errorf("String = %q", got)
	}
}

func BenchmarkProjection(b *testing.B) {
	g := random(3000, rand.New(rand.NewSource(2))) // ~50 páginas de extrato
	b.Run("bitsliced", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			g.Projection(0, g.Rows())
		}
	})
	b.Run("naive", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			naive(g, 0, g.Rows())
		}
	})
}

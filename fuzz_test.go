package finparser

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// FuzzParse garante que nenhuma entrada derruba a lib: erro limpo, nunca
// panic.
func FuzzParse(f *testing.F) {
	files, _ := filepath.Glob(filepath.Join("testdata", "synthetic", "*"))
	for _, name := range files {
		if b, err := os.ReadFile(name); err == nil && len(b) < 64<<10 {
			f.Add(b)
		}
	}
	f.Add([]byte("Data;Valor\n01/03;1,00\n"))
	f.Fuzz(func(t *testing.T, b []byte) {
		st, err := Parse(bytes.NewReader(b))
		if err == nil && len(st.Transactions) == 0 {
			t.Error("sucesso sem transações")
		}
	})
}

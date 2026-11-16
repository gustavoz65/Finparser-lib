package finparser

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
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
	f.Add([]byte(":20:X\n:60F:C260301BRL1,00\n:61:2603020302C1,00NTRFNONREF//A\n:86:?20a?32b\n:62F:C260302BRL2,00\n"))
	f.Add([]byte("<table><tr><th>Data<th>Valor<tr><td>01/03/2026<td>-1,00</table>"))
	f.Add([]byte(`<Document><BkToCstmrStmt><Stmt><Ntry><Amt Ccy="BRL">1</Amt><CdtDbtInd>DBIT</CdtDbtInd><BookgDt><Dt>2026-03-01</Dt></BookgDt></Ntry></Stmt></BkToCstmrStmt></Document>`))
	f.Fuzz(func(t *testing.T, b []byte) {
		start := time.Now()
		st, err := Parse(bytes.NewReader(b))
		// Entrada pequena não pode custar segundos: seria negação de
		// serviço para quem expõe a lib num servidor.
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("Parse levou %v para %d bytes", d, len(b))
		}
		if err != nil {
			return
		}
		if len(st.Transactions) == 0 {
			t.Error("sucesso sem transações")
		}
		// O que foi lido tem de sobreviver às contas e à exportação.
		st.Summary()
		st.Reconcile()
		var out bytes.Buffer
		if err := st.WriteOFX(&out); err != nil {
			t.Errorf("WriteOFX: %v", err)
		}
		if err := st.WriteCSV(&out); err != nil {
			t.Errorf("WriteCSV: %v", err)
		}
	})
}

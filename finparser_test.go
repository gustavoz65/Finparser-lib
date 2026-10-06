package finparser

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "regrava os golden files em testdata/golden")

var ref = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

// golden compara st com testdata/golden/<name>.json.
func golden(t *testing.T, name string, st *Statement) {
	t.Helper()
	got, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "golden", name+".json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (rode go test -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("saída difere de %s (rode go test -update e revise o diff)\n%s", path, got)
	}
}

func parseFile(t *testing.T, name string, opts ...Option) *Statement {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "synthetic", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	st, err := Parse(f, append([]Option{WithReferenceDate(ref)}, opts...)...)
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return st
}

func TestGolden(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "synthetic", "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			st := parseFile(t, name)
			golden(t, strings.ReplaceAll(name, ".", "_"), st)
		})
	}
}

func TestStableIDs(t *testing.T) {
	a := parseFile(t, "nubank_conta.ofx")
	b := parseFile(t, "nubank_conta.ofx")
	seen := map[string]bool{}
	for i := range a.Transactions {
		if a.Transactions[i].ID != b.Transactions[i].ID {
			t.Errorf("ID instável na transação %d", i)
		}
		if seen[a.Transactions[i].ID] {
			t.Errorf("ID repetido: %s", a.Transactions[i].ID)
		}
		seen[a.Transactions[i].ID] = true
	}
}

func TestDuplicateFITID(t *testing.T) {
	ofx := `<OFX><STMTRS><BANKTRANLIST>
<STMTTRN><DTPOSTED>20260301<TRNAMT>-1.00<FITID>X<MEMO>CAFE</STMTTRN>
<STMTTRN><DTPOSTED>20260301<TRNAMT>-1.00<FITID>X<MEMO>CAFE</STMTTRN>
</BANKTRANLIST></STMTRS></OFX>`
	st, err := Parse(strings.NewReader(ofx))
	if err != nil {
		t.Fatal(err)
	}
	if st.Transactions[0].ID == st.Transactions[1].ID || st.Transactions[0].ID == "X" {
		t.Errorf("IDs = %q, %q", st.Transactions[0].ID, st.Transactions[1].ID)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		opts []Option
		want error
	}{
		{"binário", []byte{0, 1, 2, 3, 0xff}, nil, ErrUnknownFormat},
		{"vazio", nil, nil, ErrUnknownFormat},
		{"ofx sem lista", []byte("<OFX><STMTRS></STMTRS></OFX>"), nil, ErrMalformed},
		{"ofx sem transações", []byte("<OFX><STMTRS><BANKTRANLIST></BANKTRANLIST></STMTRS></OFX>"), nil, ErrNoTransactions},
		{"grande demais", bytes.Repeat([]byte("a"), 100), []Option{WithMaxBytes(10)}, ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(bytes.NewReader(tt.in), tt.opts...)
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestParseErrorContext(t *testing.T) {
	_, err := Parse(strings.NewReader("<OFX><STMTRS></STMTRS></OFX>"))
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Format != FormatOFX {
		t.Fatalf("err = %#v", err)
	}
}

func TestWithBank(t *testing.T) {
	st := parseFile(t, "cartao.ofx", WithBank("Inter"))
	if st.Bank != "inter" {
		t.Errorf("Bank = %q", st.Bank)
	}
}

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const pdfFile = "../../testdata/synthetic/itau_conta.pdf"

func TestCommands(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"tokens", pdfFile}, "FontSize"},
		{[]string{"lines", pdfFile}, "PAG BOLETO ENERGIA ELETRICA"},
		{[]string{"parse", pdfFile}, `"bank": "itau"`},
		{[]string{"svg", "-out", dir, pdfFile}, "itau_conta-p2.svg"},
	}
	for _, tt := range tests {
		var out bytes.Buffer
		if err := run(tt.args, &out); err != nil {
			t.Fatalf("%v: %v", tt.args, err)
		}
		if !strings.Contains(out.String(), tt.want) {
			t.Errorf("%v: saída sem %q", tt.args, tt.want)
		}
	}
	svg, err := os.ReadFile(filepath.Join(dir, "itau_conta-p1.svg"))
	if err != nil || !bytes.Contains(svg, []byte("<svg")) {
		t.Errorf("svg: %v", err)
	}
}

func TestBadArgs(t *testing.T) {
	var out bytes.Buffer
	for _, args := range [][]string{nil, {"tokens"}, {"voar", pdfFile}, {"tokens", "nao-existe.pdf"}} {
		if err := run(args, &out); err == nil {
			t.Errorf("%v deveria falhar", args)
		}
	}
}

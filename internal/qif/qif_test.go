package qif

import (
	"errors"
	"testing"

	"github.com/gustavoz65/finparser-lib/internal/errs"
)

func TestParse(t *testing.T) {
	in := "!Type:Bank\r\nD02/03/2026\r\nT-45,90\r\nPPADARIA\r\nMcafé\r\n^\r\nD05/03'26\r\nU1.000,00\r\nT1.000,00\r\nPSALARIO\r\n^\r\nDxx\r\nT1\r\n^\r\nD06/03/2026\r\nT2,00"
	res, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 3 {
		t.Fatalf("Records = %+v", res.Records)
	}
	if r := res.Records[0]; r.Description != "PADARIA - café" || r.Amount.String() != "-45.9" || r.Date.Day() != 2 {
		t.Errorf("Records[0] = %+v", r)
	}
	if r := res.Records[1]; r.Amount.String() != "1000" || r.Date.Year() != 2026 {
		t.Errorf("Records[1] = %+v", r)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Line != 12 {
		t.Errorf("Warnings = %+v", res.Warnings)
	}
}

func TestEmpty(t *testing.T) {
	if _, err := Parse("!Type:Bank\n"); !errors.Is(err, errs.ErrMalformed) {
		t.Errorf("err = %v", err)
	}
}

func TestSkipsNonTransactionLists(t *testing.T) {
	res, err := Parse("!Type:Cat\nNAlimentacao\nE\n^\n!Account\nNConta\n^\n!Type:Bank\nD02/03/2026\nT-10,00\nPPadaria\n^\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Records) != 1 || len(res.Warnings) != 0 {
		t.Errorf("Records = %+v, Warnings = %+v", res.Records, res.Warnings)
	}
}

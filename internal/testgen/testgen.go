// Package testgen gera PDFs sintéticos que imitam o layout de extratos,
// para testes sem extrato real. Só é importado por testes.
package testgen

import (
	"bytes"
	"time"

	"github.com/go-pdf/fpdf"
)

// Cell é um texto numa linha. X é a borda esquerda, ou a direita se Right.
type Cell struct {
	X     float64
	Text  string
	Right bool
}

// Line é uma linha de texto. Y é a linha de base medida do topo da página,
// em pontos (A4: 595 × 842).
type Line struct {
	Y     float64
	Size  float64 // padrão 9
	Bold  bool
	Cells []Cell
}

// Page é uma página.
type Page struct{ Lines []Line }

// Doc é o documento.
type Doc struct {
	Pages    []Page
	Password string // se não vazio, protege o PDF (RC4)
}

// PDF renderiza d. A saída é determinística (data de criação fixa).
func PDF(d Doc) ([]byte, error) {
	p := fpdf.New("P", "pt", "A4", "")
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p.SetCreationDate(fixed)
	p.SetModificationDate(fixed)
	p.SetCompression(d.Password != "")
	if d.Password != "" {
		p.SetProtection(fpdf.CnProtectPrint, d.Password, "owner-"+d.Password)
	}
	tr := p.UnicodeTranslatorFromDescriptor("") // cp1252
	for _, pg := range d.Pages {
		p.AddPage()
		for _, ln := range pg.Lines {
			size := ln.Size
			if size == 0 {
				size = 9
			}
			style := ""
			if ln.Bold {
				style = "B"
			}
			p.SetFont("Helvetica", style, size)
			for _, c := range ln.Cells {
				txt := tr(c.Text)
				x := c.X
				if c.Right {
					x -= p.GetStringWidth(txt)
				}
				p.Text(x, ln.Y, txt)
			}
		}
	}
	var b bytes.Buffer
	if err := p.Output(&b); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

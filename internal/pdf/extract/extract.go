// Package extract tira do PDF cada pedaço de texto com sua posição:
// a nuvem de Token{Page, X, Y, W, FontSize, Text} que o motor espacial lê.
//
// O backend padrão é github.com/ledongthuc/pdf (Go puro, BSD-3). Ele herda
// do rsc/pdf o hábito de dar panic em PDF malformado; toda chamada fica
// atrás de recover e vira errs.ErrMalformed.
package extract

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/errs"
	"github.com/ledongthuc/pdf"
)

// Token é um pedaço de texto posicionado. Coordenadas em pontos, origem no
// canto inferior esquerdo da página: Y cresce de baixo para cima.
type Token struct {
	Page     int
	X, Y     float64 // canto inferior esquerdo
	W        float64
	FontSize float64
	Font     string
	Text     string
}

// Extractor é a interface do backend, para trocar de motor sem tocar no
// resto (ex.: PDFium via cgo, atrás de build tag).
type Extractor interface {
	Extract(data []byte, password string) ([]Token, error)
}

// Default é o backend Go puro.
type Default struct{}

// Extract devolve os tokens de todas as páginas, na ordem do stream.
// Erros: errs.ErrEncrypted, errs.ErrMalformed, errs.ErrNoTextLayer.
func (Default) Extract(data []byte, password string) (toks []Token, err error) {
	encrypted := false
	defer func() {
		if r := recover(); r != nil {
			toks = nil
			// O suporte do backend a criptografia é limitado: abrir com a
			// senha certa e falhar ao decifrar o conteúdo é erro de senha/
			// cifra, não arquivo corrompido.
			if encrypted {
				err = fmt.Errorf("%w: backend não decifrou o conteúdo: %v", errs.ErrEncrypted, r)
			} else {
				err = fmt.Errorf("%w: %v", errs.ErrMalformed, r)
			}
		}
	}()

	var r *pdf.Reader
	ra, size := bytes.NewReader(data), int64(len(data))
	if password == "" {
		r, err = pdf.NewReader(ra, size)
	} else {
		tried := false
		r, err = pdf.NewReaderEncrypted(ra, size, func() string {
			if tried {
				return ""
			}
			tried = true
			return password
		})
	}
	if err != nil {
		if errors.Is(err, pdf.ErrInvalidPassword) || strings.Contains(err.Error(), "encrypt") {
			return nil, fmt.Errorf("%w: %v", errs.ErrEncrypted, err)
		}
		return nil, fmt.Errorf("%w: %v", errs.ErrMalformed, err)
	}
	encrypted = r.Trailer().Key("Encrypt").Kind() != pdf.Null

	for n := 1; n <= r.NumPage(); n++ {
		p := r.Page(n)
		if p.V.IsNull() {
			continue
		}
		toks = append(toks, fixWidths(n, p.Content().Text)...)
	}
	if !hasText(toks) {
		return nil, errs.ErrNoTextLayer
	}
	return toks, nil
}

func hasText(toks []Token) bool {
	for _, t := range toks {
		if strings.TrimSpace(t.Text) != "" {
			return true
		}
	}
	return false
}

// fixWidths converte para Token e corrige glifos sem largura. Fontes padrão
// (Helvetica, Times, Courier) podem vir sem /Widths: o backend devolve W = 0
// e não avança o X, empilhando todos os glifos de um Tj no mesmo ponto. Aqui
// a largura é estimada pelas métricas AFM e o X é reconstruído.
func fixWidths(page int, texts []pdf.Text) []Token {
	out := make([]Token, 0, len(texts))
	var (
		inRun    bool
		runY     float64
		prevRawX float64
		prevEst  float64
		offset   float64
	)
	for _, t := range texts {
		tok := Token{Page: page, X: t.X, Y: t.Y, W: t.W, FontSize: t.FontSize, Font: t.Font, Text: t.S}
		if t.W != 0 || t.S == "" {
			inRun = false
			out = append(out, tok)
			continue
		}
		est := textWidth(t.Font, t.S) * t.FontSize / 1000
		// Continua a sequência se está na mesma linha e o X bruto avançou
		// menos que a largura real do glifo anterior.
		if inRun && t.Y == runY && t.X >= prevRawX && t.X-prevRawX < prevEst {
			offset += prevEst // o X bruto já inclui Tc/Tw; falta só a largura
		} else {
			inRun, runY, offset = true, t.Y, 0
		}
		prevRawX, prevEst = t.X, est
		tok.X += offset
		tok.W = est
		out = append(out, tok)
	}
	return out
}

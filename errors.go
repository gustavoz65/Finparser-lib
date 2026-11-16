package finparser

import (
	"fmt"
	"strings"

	"github.com/gustavoz65/finparser-lib/internal/errs"
)

// Erros sentinela. Todo erro devolvido por Parse envolve um deles; use
// errors.Is para testar.
var (
	// ErrUnknownFormat: os bytes não são de nenhum formato suportado.
	ErrUnknownFormat = errs.ErrUnknownFormat
	// ErrNoTextLayer: PDF sem texto extraível (provavelmente escaneado).
	ErrNoTextLayer = errs.ErrNoTextLayer
	// ErrEncrypted: PDF protegido e a senha está ausente ou errada.
	ErrEncrypted = errs.ErrEncrypted
	// ErrMalformed: arquivo corrompido ou fora da estrutura esperada.
	ErrMalformed = errs.ErrMalformed
	// ErrNoTransactions: o arquivo foi lido, mas nenhuma transação apareceu.
	ErrNoTransactions = errs.ErrNoTransactions
	// ErrTooLarge: a entrada passou do limite de WithMaxBytes.
	ErrTooLarge = errs.ErrTooLarge
)

// ParseError dá contexto (formato, página, linha) a um erro de parse.
type ParseError struct {
	Format Format
	Page   int
	Line   int
	Err    error
}

func (e *ParseError) Error() string {
	// Os sentinelas já começam com "finparser: "; não repete o prefixo.
	msg := strings.TrimPrefix(e.Err.Error(), "finparser: ")
	switch {
	case e.Page > 0 && e.Line > 0:
		return fmt.Sprintf("finparser: %s página %d linha %d: %v", e.Format, e.Page, e.Line, msg)
	case e.Page > 0:
		return fmt.Sprintf("finparser: %s página %d: %v", e.Format, e.Page, msg)
	case e.Line > 0:
		return fmt.Sprintf("finparser: %s linha %d: %v", e.Format, e.Line, msg)
	case e.Format != "":
		return fmt.Sprintf("finparser: %s: %v", e.Format, msg)
	}
	return e.Err.Error()
}

// Unwrap permite errors.Is(err, ErrMalformed) através do ParseError.
func (e *ParseError) Unwrap() error { return e.Err }

// Package errs guarda os erros sentinela compartilhados entre o pacote
// público e os parsers internos. O pacote raiz os reexporta, então
// errors.Is funciona dos dois lados.
package errs

import "errors"

// Erros sentinela.
var (
	ErrUnknownFormat  = errors.New("finparser: formato não reconhecido")
	ErrNoTextLayer    = errors.New("finparser: PDF sem camada de texto (escaneado?)")
	ErrEncrypted      = errors.New("finparser: PDF protegido por senha")
	ErrMalformed      = errors.New("finparser: arquivo malformado")
	ErrNoTransactions = errors.New("finparser: nenhuma transação encontrada")
	ErrTooLarge       = errors.New("finparser: arquivo maior que o limite configurado")
)

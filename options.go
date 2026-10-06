package finparser

import (
	"log/slog"
	"time"
)

// DefaultMaxBytes é o limite padrão de leitura: 20 MB.
const DefaultMaxBytes int64 = 20 << 20

// Option configura Parse.
type Option func(*config)

type config struct {
	format       Format
	bank         string
	password     string
	maxBytes     int64
	rowTolerance float64
	reference    time.Time
	logger       *slog.Logger
}

func newConfig(opts []Option) config {
	c := config{maxBytes: DefaultMaxBytes, rowTolerance: 0.5}
	for _, o := range opts {
		o(&c)
	}
	if c.reference.IsZero() {
		c.reference = time.Now()
	}
	return c
}

// WithFormat pula a detecção automática de formato.
func WithFormat(f Format) Option { return func(c *config) { c.format = f } }

// WithBank força um perfil de banco ("nubank", "itau", "inter", "bb",
// "bradesco", "santander", "caixa", "c6").
func WithBank(name string) Option { return func(c *config) { c.bank = name } }

// WithPassword informa a senha de um PDF protegido.
func WithPassword(pw string) Option { return func(c *config) { c.password = pw } }

// WithMaxBytes limita quantos bytes Parse lê. Padrão: DefaultMaxBytes.
// Valores ≤ 0 mantêm o padrão.
func WithMaxBytes(n int64) Option {
	return func(c *config) {
		if n > 0 {
			c.maxBytes = n
		}
	}
}

// WithRowTolerance define, no PDF, a distância vertical máxima para duas
// palavras ficarem na mesma linha, como múltiplo do tamanho da fonte.
// Padrão: 0.5. Valores ≤ 0 mantêm o padrão.
func WithRowTolerance(factor float64) Option {
	return func(c *config) {
		if factor > 0 {
			c.rowTolerance = factor
		}
	}
}

// WithReferenceDate define o "hoje" usado para completar datas sem ano
// quando o extrato não informa o período. Padrão: time.Now().
func WithReferenceDate(t time.Time) Option { return func(c *config) { c.reference = t } }

// WithLogger recebe um logger para mensagens de depuração. Sem ele a lib
// não registra nada.
func WithLogger(l *slog.Logger) Option { return func(c *config) { c.logger = l } }

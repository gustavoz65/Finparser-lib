# finparser

[![CI](https://github.com/gustavoz65/finparser-lib/actions/workflows/ci.yml/badge.svg)](https://github.com/gustavoz65/finparser-lib/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/gustavoz65/finparser-lib.svg)](https://pkg.go.dev/github.com/gustavoz65/finparser-lib)

Biblioteca Go para ler **extratos bancários brasileiros** (OFX, QIF, CSV e PDF com camada de texto) e devolver transações padronizadas: data, descrição limpa, valor decimal exato, saldo opcional e tipo (receita/despesa).

```
go get github.com/gustavoz65/finparser-lib
```

- Go puro: compila com `CGO_ENABLED=0`, sem dependência de C.
- Dinheiro em [`decimal.Decimal`](https://github.com/shopspring/decimal), nunca `float64`.
- Datas civis (meia-noite UTC): 01/03 não vira 28/02 por causa de fuso.
- Nunca chuta: o que não deu para interpretar aparece em `Warnings`, e tipo indefinido fica `Unknown`.
- Só dependências com licença permissiva (MIT/BSD).

## Uso

```go
package main

import (
	"fmt"
	"log"
	"os"

	finparser "github.com/gustavoz65/finparser-lib"
)

func main() {
	f, err := os.Open("extrato.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	st, err := finparser.Parse(f)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(st.Format, st.Bank, st.PeriodStart.Format("02/01/2006"), "a", st.PeriodEnd.Format("02/01/2006"))
	for _, t := range st.Transactions {
		fmt.Println(t.Date.Format("02/01/2006"), t.Description, t.Amount.StringFixed(2), t.Kind)
	}
	for _, w := range st.Warnings {
		fmt.Println("aviso:", w.Source, w.Message, w.Raw)
	}
}
```

### Opções

```go
finparser.Parse(r,
	finparser.WithFormat(finparser.FormatCSV), // pula a detecção
	finparser.WithBank("nubank"),               // força um perfil de banco
	finparser.WithPassword("1234"),             // PDF protegido
	finparser.WithMaxBytes(5<<20),              // padrão: 20 MB
	finparser.WithRowTolerance(0.6),            // PDF: tolerância de linha (× fonte)
	finparser.WithReferenceDate(time.Now()),    // ano de datas sem ano, se o período for desconhecido
	finparser.WithLogger(slog.Default()),       // logs de depuração; sem isso, silêncio
)
```

### Erros

Todo erro envolve um sentinela; use `errors.Is`:

| Erro | Quando |
|------|--------|
| `ErrUnknownFormat` | os bytes não são PDF, OFX, QIF nem texto |
| `ErrNoTextLayer` | PDF sem texto (escaneado) — não há OCR |
| `ErrEncrypted` | PDF protegido, senha ausente ou errada |
| `ErrMalformed` | arquivo corrompido ou fora da estrutura esperada |
| `ErrNoTransactions` | arquivo lido, nenhuma transação encontrada |
| `ErrTooLarge` | passou de `WithMaxBytes` |

`*ParseError` traz formato, página e linha quando disponíveis (`errors.As`).

## Modelo

```go
type Transaction struct {
	Date        time.Time        // data civil, meia-noite UTC
	Description string           // espaços normalizados
	Amount      decimal.Decimal  // negativo = saída
	Balance     *decimal.Decimal // nil quando o extrato não traz
	Kind        Kind             // Unknown, Income, Expense
	ID          string           // FITID/identificador único, ou hash estável
	Raw         []string         // colunas brutas, para depuração
	Source      Location         // página/linha de origem
}
```

`ID` é estável: o mesmo arquivo parseado duas vezes gera os mesmos IDs (hash de data + valor + descrição + ordinal quando a fonte não traz identificador único). Serve para deduplicar importações.

## O que é suportado

| Formato | Situação |
|---------|----------|
| OFX v1 (SGML) e v2 (XML), conta e cartão | ✅ parser próprio e tolerante (tags sem fechamento, `TRNAMT` com vírgula, FITID vazio/repetido, Windows-1252) |
| QIF | ✅ |
| CSV | ✅ delimitador (`;` `,` tab `\|`), cabeçalho após linhas de metadado, colunas por nome ou por conteúdo, crédito/débito separados |
| PDF com texto | ✅ motor espacial genérico (veja abaixo) |
| PDF escaneado | ❌ erro `ErrNoTextLayer` (sem OCR) |

| Banco | Testado com |
|-------|-------------|
| Nubank | OFX, CSV e PDF sintéticos que imitam o layout real |
| Itaú | CSV e PDF sintéticos |
| Banco do Brasil | CSV sintético (crédito/débito separados) |
| Inter, Bradesco, Santander, Caixa, C6 | só identificação do banco; layout passa pelo motor genérico |

Os fixtures do repositório são **sintéticos**: nenhum extrato real é commitado. Se o seu extrato não sai certo, abra uma issue com a saída do `finparser-debug` (sem dados pessoais).

### Receita ou despesa

1. Sinal do valor (`-`, `D`/`C`, parênteses, colunas de crédito/débito, grupos "Total de entradas/saídas").
2. Se a fonte não tem sinal: padrões na descrição (`PIX RECEBIDO`, `SALARIO`, `COMPRA`, `TARIFA`...).
3. Validação cruzada com o saldo: `saldo[i] − saldo[i−1] == valor[i]` confirma o sinal; se der o contrário, inverte e registra aviso.
4. Nada bateu → `Unknown`.

## Como o PDF é lido

Um PDF guarda comandos de desenho, não texto em ordem de leitura. A finparser monta uma "tela" em memória com cada pedaço de texto na sua coordenada — `Token{Page, X, Y, W, FontSize, Text}` — e reconstrói a página como um humano vê (mesma ideia do Tabula e do Camelot):

1. **glifos → palavras**: mesmo Y, espaço horizontal < 0,25 × fonte;
2. **palavras → linhas**: |ΔY| ≤ 0,5 × fonte, comparando com a mediana da linha;
3. **ruído de página**: cabeçalho/rodapé repetido entre páginas ("Página 2 de 5") sai;
4. **linhas → colunas**: pela linha de cabeçalho da tabela; sem ela, pelas "calhas" vazias do perfil de projeção; sem isso, data no começo e valores no fim;
5. **linhas → transações**: linha sem data herda a do grupo, linha sem data nem valor continua a descrição, "SALDO DO DIA" vira saldo.

Toda tolerância é relativa ao tamanho da fonte, e o Y do PDF cresce de baixo para cima.

## CLI de debug

```
go install github.com/gustavoz65/finparser-lib/cmd/finparser-debug@latest

finparser-debug tokens extrato.pdf     # tabela Page, X, Y, W, FontSize, Text
finparser-debug lines  extrato.pdf     # linhas reconstruídas
finparser-debug svg -out /tmp extrato.pdf  # um SVG por página: tokens, linhas e colunas
finparser-debug parse  extrato.ofx     # JSON do Statement (qualquer formato)
```

Quando uma coluna sai errada, o SVG mostra o porquê.

## Desenvolvimento

```
go test ./...                 # inclui golden files em testdata/golden
go test . -update             # regenera PDFs sintéticos e golden files (revise o diff!)
go test ./internal/money -run '^$' -fuzz FuzzParseMoney
```

- Extratos reais ficam em `testdata/private/` (ignorado pelo git) — nunca commite um, nem "anonimizado à mão".
- O documento de construção, com decisões e armadilhas, está em [docs/BUILD.md](docs/BUILD.md).
- Versionamento semântico; enquanto `v0.x`, a API pública pode mudar. Veja o [CHANGELOG](CHANGELOG.md).

## Licença

[MIT](LICENSE).

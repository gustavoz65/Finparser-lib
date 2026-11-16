# finparser

[![CI](https://github.com/gustavoz65/finparser-lib/actions/workflows/ci.yml/badge.svg)](https://github.com/gustavoz65/finparser-lib/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/gustavoz65/finparser-lib.svg)](https://pkg.go.dev/github.com/gustavoz65/finparser-lib)

Biblioteca Go que lê **extratos bancários** em qualquer formato que o banco entregar — PDF, OFX, CSV, Excel, HTML, QIF, ISO 20022 e MT940 — e devolve transações padronizadas. O foco são os bancos brasileiros.

```
go get github.com/gustavoz65/finparser-lib
```

O que ela faz além de ler:

- **Prova que leu certo.** `Reconcile` confere o extrato contra os saldos que ele mesmo declara; um lançamento perdido ou com valor errado faz a conta falhar.
- **Descobre o sinal pelo saldo.** Extrato sem `-` nem D/C? A lib resolve quais valores entraram e quais saíram pela diferença entre saldos — e só aceita a resposta quando ela é única.
- **Converte.** PDF → OFX, XLSX → CSV, qualquer entrada → qualquer saída, com IDs estáveis para reimportar sem duplicar.
- **Lê PDF como matriz.** A página vira um tensor de bits Página × Linha × X; as colunas saem de aritmética de palavra, não de heurística de texto.
- **Nunca chuta.** O que não deu para interpretar vai para `Warnings`; sinal indefinido fica `Unknown`.

Go puro (`CGO_ENABLED=0`), dinheiro em [`decimal.Decimal`](https://github.com/shopspring/decimal), sem estado global, só dependências MIT/BSD.

## Uso

```go
st, err := finparser.ParseFile("extrato.pdf")
if err != nil {
	log.Fatal(err)
}

fmt.Println(st.Format, st.Bank, st.PeriodStart.Format("02/01/2006"), "a", st.PeriodEnd.Format("02/01/2006"))
for _, t := range st.Transactions {
	fmt.Println(t.Date.Format("02/01/2006"), t.Description, t.Amount.StringFixed(2), t.Kind, t.Method, t.Counterparty)
}

// A leitura está completa?
if rec := st.Reconcile(); rec.Status != finparser.Verified {
	log.Printf("saldo não confere: %+v", rec.Mismatches)
}

// PDF vira OFX.
out, _ := os.Create("extrato.ofx")
defer out.Close()
st.WriteOFX(out)
```

`Parse(io.Reader)`, `ParseBytes([]byte)` e `ParseFile(path)` fazem a mesma coisa; o formato vem dos bytes, nunca da extensão. `Parse` é seguro para uso concorrente.

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
| `ErrUnknownFormat` | os bytes não são de nenhum formato suportado (inclui o `.xls` binário antigo) |
| `ErrNoTextLayer` | PDF sem texto (escaneado) — não há OCR |
| `ErrEncrypted` | PDF protegido, senha ausente ou errada |
| `ErrMalformed` | arquivo corrompido ou fora da estrutura esperada |
| `ErrNoTransactions` | arquivo lido, nenhuma transação encontrada |
| `ErrTooLarge` | passou de `WithMaxBytes` |

`*ParseError` traz formato, página e linha quando disponíveis (`errors.As`).

## Modelo

```go
type Statement struct {
	Format         Format           // "pdf", "ofx", "csv", "xlsx", "html", "qif", "camt053", "mt940"
	Bank           string           // "nubank", "itau"... ou ""
	Account        *Account         // banco, agência, conta, tipo — quando a fonte informa
	Currency       string           // "BRL"
	PeriodStart    time.Time
	PeriodEnd      time.Time
	OpeningBalance *decimal.Decimal // "SALDO ANTERIOR"
	ClosingBalance *decimal.Decimal // saldo final declarado
	Transactions   []Transaction
	Warnings       []Warning
}

type Transaction struct {
	Date         time.Time        // data civil, meia-noite UTC
	Description  string           // espaços normalizados
	Amount       decimal.Decimal  // negativo = saída
	Balance      *decimal.Decimal // nil quando o extrato não traz
	Kind         Kind             // Unknown, Income, Expense
	Method       Method           // pix, ted, doc, boleto, card, withdrawal, fee, tax...
	Counterparty string           // "FULANO DE TAL", quando a descrição deixa claro
	ID           string           // FITID/identificador único, ou hash estável
	Raw          []string         // colunas brutas, para depuração
	Source       Location         // página/linha de origem
}
```

`ID` é estável: o mesmo arquivo parseado duas vezes gera os mesmos IDs (hash de data + valor + descrição + ordinal quando a fonte não traz identificador único). `Merge` usa isso para juntar exportações que se sobrepõem sem repetir lançamento.

## Conferência de saldo

```go
rec := st.Reconcile()
// rec.Status   verified | mismatch | unverifiable
// rec.Checked  contas feitas (uma por trecho entre dois saldos, mais a do extrato inteiro)
// rec.Covered  lançamentos dentro de contas que fecharam
```

Entre dois saldos conhecidos, a soma dos lançamentos tem de dar a diferença. Com saldo inicial e final, a soma de tudo tem de levar de um ao outro. `Verified` com `Covered == Total` significa que **cada lançamento foi conferido** — vale para extrato em ordem crescente ou decrescente de data. Todos os fixtures do repositório que trazem saldo passam nesse teste, inclusive um PDF de 1000 lançamentos.

### Receita ou despesa

1. Sinal do valor (`-`, `−`, `D`/`C`, `DB`/`CR`, parênteses, colunas de crédito/débito, coluna "Tipo", grupos "Total de entradas/saídas").
2. Se a fonte não tem sinal: padrões na descrição (`PIX RECEBIDO`, `SALARIO`, `COMPRA`, `TARIFA`...).
3. Saldo: com um lançamento entre dois saldos, `saldo[i] − saldo[i−1] == valor[i]` confirma o sinal (se der o contrário, inverte e avisa). Com vários — extrato que só mostra "SALDO DO DIA" — procura a combinação de sinais que fecha a soma, e só define o sinal de um lançamento se ele for o mesmo em **todas** as combinações possíveis.
4. Nada bateu → `Unknown`.

## O que é suportado

| Formato | Situação |
|---------|----------|
| OFX v1 (SGML) e v2 (XML), conta e cartão | ✅ parser próprio e tolerante (tags sem fechamento, CDATA, `TRNAMT` com vírgula, FITID vazio/repetido, Windows-1252) |
| CSV | ✅ delimitador (`;` `,` tab `\|`, `sep=`), cabeçalho após metadados, colunas por nome ou por conteúdo, crédito/débito separados, coluna de sinal D/C |
| XLSX | ✅ só biblioteca padrão; datas seriais, textos compartilhados, ruído de ponto flutuante removido |
| HTML | ✅ o "`.xls`" de banco que por dentro é uma `<table>` |
| PDF com texto | ✅ motor espacial em matriz (veja abaixo) |
| QIF | ✅ |
| ISO 20022 camt.052 / 053 / 054 | ✅ |
| SWIFT MT940 / MT942 | ✅ com `:86:` livre ou estruturado |
| XLS binário (BIFF) | ❌ erro claro pedindo `.xlsx` ou CSV |
| PDF escaneado | ❌ `ErrNoTextLayer` (sem OCR) |

| Banco | Testado com |
|-------|-------------|
| Nubank | OFX, CSV e PDF sintéticos que imitam o layout real |
| Itaú | CSV e PDF |
| Bradesco | PDF (crédito e débito em colunas separadas) |
| Caixa | PDF (sufixo D/C) e CSV (coluna Deb./Cred.) |
| Inter | XLSX e CSV |
| Santander | HTML |
| Banco do Brasil | CSV |
| C6 | só identificação do banco; layout passa pelo motor genérico |

Os fixtures do repositório são **sintéticos**: nenhum extrato real é commitado. Se o seu extrato não sai certo, abra uma issue com a saída do `finparser-debug` (sem dados pessoais).

## Converter e juntar

```go
st.WriteOFX(w)  // OFX 1.02 SGML, Windows-1252: o que ERPs e programas de contabilidade importam
st.WriteCSV(w)  // RFC 4180, datas ISO, ponto decimal
st.WriteJSON(w)

all := finparser.Merge(janeiro, fevereiro, marco) // uma linha do tempo, sem duplicar
sum := all.Summary()                              // Income, Expense, Net, contagens
```

O que `WriteOFX` e `WriteCSV` gravam é lido de volta por `Parse` com as mesmas datas, valores, descrições e IDs — testado para todos os fixtures.

## Como o PDF é lido

Um PDF guarda comandos de desenho, não texto em ordem de leitura. A finparser monta o espaço do documento em três eixos — página, Y e X — e reconstrói a tabela como um humano a vê:

1. **glifos → palavras**: mesmo Y, espaço horizontal < 0,25 × fonte;
2. **palavras → linhas**: |ΔY| ≤ 0,5 × fonte, comparando com a mediana da linha;
3. **ruído de página**: cabeçalho/rodapé repetido entre páginas sai;
4. **linhas → colunas**, pela **matriz de ocupação**;
5. **linhas → transações**: linha sem data herda a do grupo, linha sem data nem valor continua a descrição, "SALDO DO DIA" vira saldo.

### A matriz

Cada linha visual é uma linha de uma matriz de bits; cada coluna é um ponto do eixo X; o bit diz "há tinta aqui". O eixo das páginas é uma fatia de linhas. Tudo fica num único `[]uint64`:

```
$ finparser-debug grid -scale 7 extrato.pdf
####...#######...........................#####...########.########..########   ← cabeçalho
######.############..................................................#######
######.#################################.######..########..........#########
######.###################...............######..............######.........
######.#################.................######...............####.#########
9999883999999999888776665544444444433322165555400222222220111344442345666666   ← projeção
```

- A **projeção** (quantas linhas ocupam cada coluna) é uma soma vertical com contadores *bit-sliced*: cada operação soma 64 colunas de uma vez.
- Sem cabeçalho, as colunas são as **calhas** — faixas de zeros da projeção.
- Com cabeçalho, a fronteira entre duas colunas fica no **vale** da projeção entre os dois títulos. A coluna segue o dado, não a largura do texto do cabeçalho: descrição comprida e valor de sete dígitos caem no lugar certo, com a coluna alinhada à esquerda, à direita ou centrada.

Toda tolerância é relativa ao tamanho da fonte, e o Y do PDF cresce de baixo para cima.

### Desempenho

Medido com `go test -bench` (4 vCPUs, Go 1.24):

| O quê | Tempo |
|-------|-------|
| Colunas de 3000 linhas pela matriz | 0,62 ms e 13 alocações (a versão célula a célula: 2,3 ms e 3020) |
| PDF de 20 páginas, 1000 lançamentos, ponta a ponta | ~100 ms, mais da metade no backend de extração |
| OFX | ~50 µs |
| XLSX | ~0,5 ms |

Entrada maliciosa não custa caro: matriz com teto de memória, busca de sinais com orçamento por extrato, limites no XLSX (zip bomb) e fuzz de ponta a ponta no CI.

## CLI

```
go install github.com/gustavoz65/finparser-lib/cmd/finparser@latest

finparser extrato.pdf                          # tabela, totais e conferência de saldo
finparser -to ofx -o extrato.ofx extrato.pdf   # converte
finparser -to csv jan.ofx fev.ofx mar.ofx      # junta sem duplicar
finparser -check extrato.xlsx                  # sai com erro se o saldo não fechar
```

```
pdf · caixa · 01/03/2026 a 31/03/2026

DATA        DESCRIÇÃO       VALOR     SALDO    TIPO     MEIO
02/03/2026  CRED PIX CHAVE  1200.00   2000.00  income   pix
03/03/2026  SAQUE ATM       -300.00   1700.00  expense  withdrawal
...

5 lançamentos · entradas 1700.00 · saídas -2312.34 · líquido -612.34
saldo confere: 5 contas, 5 de 5 lançamentos cobertos
```

Para investigar um PDF que sai errado:

```
go install github.com/gustavoz65/finparser-lib/cmd/finparser-debug@latest

finparser-debug tokens extrato.pdf         # tabela Page, X, Y, W, FontSize, Text
finparser-debug lines  extrato.pdf         # linhas reconstruídas
finparser-debug grid   extrato.pdf         # a matriz de ocupação e a projeção
finparser-debug svg -out /tmp extrato.pdf  # um SVG por página: tokens, linhas e colunas
finparser-debug parse  extrato.ofx         # JSON do Statement (qualquer formato)
```

## Desenvolvimento

```
go test ./...                 # inclui golden files em testdata/golden
go test . -update             # regenera fixtures sintéticos e golden files (revise o diff!)
go test ./... -run '^$' -bench .
go test . -run '^$' -fuzz FuzzParse
```

- Extratos reais ficam em `testdata/private/` (ignorado pelo git) — nunca commite um, nem "anonimizado à mão".
- O documento de construção, com decisões e armadilhas, está em [docs/BUILD.md](docs/BUILD.md).
- Versionamento semântico; enquanto `v0.x`, a API pública pode mudar. Veja o [CHANGELOG](CHANGELOG.md).

## Licença

[MIT](LICENSE).

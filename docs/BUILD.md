# finparser — README de criação

> Documento de construção da lib. Não é o README público final: é o mapa de como ela nasce, quais decisões já foram tomadas e quais armadilhas evitar. Serve para você e para qualquer agente de IA (Claude, Copilot etc.) que for implementar partes dela.

```
go get github.com/gustavoz65/finparser
```

---

## 0. Origem da ideia

A lib nasceu dentro do **Fy / FiNext** com uma necessidade simples:

> ler qualquer tipo de extrato bancário, limpar os dados e identificar se cada lançamento é despesa ou receita.

A sacada central foi a **leitura espacial**. Um PDF não guarda texto em ordem de leitura; guarda comandos de desenho ("escreva `01/03` na posição x=300, y=200"). Ler o stream bruto em ordem mistura tudo. Em vez disso, a lib monta uma "tela" em memória com cada pedaço de texto na sua coordenada e reconstrói linhas e colunas do jeito que um humano enxerga o documento. É a mesma abordagem de ferramentas como Tabula e Camelot.

**Sobre o "3D":** na prática o espaço tem três eixos, e é isso que torna a ideia precisa:

| Eixo | Significado | Uso |
|------|-------------|-----|
| `Page` (z) | número da página | separa contexto, remove cabeçalho/rodapé repetido |
| `Y` | posição vertical | agrupa tokens em **linhas** |
| `X` | posição horizontal | separa tokens em **colunas** |

Cada ponto desse espaço não é um número, é um objeto: texto, fonte, tamanho, largura. O documento vira uma nuvem de `Token{Page, X, Y, W, FontSize, Text}`.

Decisões herdadas da conversa original, que continuam valendo:

1. **Ordem invertida de construção:** OFX → CSV → PDF. OFX é estruturado e determinístico; PDF é o problema mais difícil e fica por último.
2. **PDF começa com um banco só** (sugestão: Nubank) e só generaliza depois de funcionar.
3. **Spike antes de se comprometer com a lib de PDF**, usando extratos reais de pelo menos 3 bancos.
4. **Classificação simples na v1:** sinal do valor + padrões conhecidos. Nada de ML/NLP agora.
5. **YAGNI:** o módulo espacial só nasce quando o trabalho de PDF começar.

---

## 1. Objetivo e não-objetivos

**Objetivo:** dado um `io.Reader` com um extrato (OFX, CSV ou PDF com camada de texto), devolver uma lista padronizada de transações com data, descrição, valor exato, saldo opcional e tipo (receita/despesa), além de avisos sobre o que não deu para interpretar.

**Norte de longo prazo:** "qualquer extrato". **Realidade da v1:** um motor genérico + lista explícita de bancos testados. O README público sempre declara quais bancos e formatos são suportados.

**Fora de escopo (por enquanto):**
- OCR de PDF escaneado (sem camada de texto → erro claro, não chute)
- Faturas de cartão com parcelamento complexo (vem depois do extrato de conta)
- Categorização (alimentação, transporte...) — fica para v0.6+
- Conexão com bancos / Open Finance

---

## 2. Decisões de stack

### 2.1 Go puro por padrão, cgo só como opção

A lib **compila com `CGO_ENABLED=0`**. Isso não é negociável para o caminho padrão, porque:

- `go get` precisa funcionar em qualquer máquina sem compilador C instalado
- cross-compile (`GOOS=linux GOARCH=arm64`) e imagens `scratch`/`distroless` quebram com cgo
- o Fy e qualquer outro consumidor herdam a dependência de cgo se ela estiver no caminho padrão

cgo **pode** entrar para coisas mais profundas (extração de PDF mais robusta), mas só como **backend opcional atrás de build tag**:

```go
//go:build finparser_pdfium
```

Quem quiser o backend nativo compila com `go build -tags finparser_pdfium`. Quem não quiser nem percebe que ele existe.

**Atenção à licença do backend em C.** Aqui é onde a coisa morde:

| Motor C | Licença | Pode usar numa lib MIT? |
|---------|---------|-------------------------|
| PDFium (Google) | BSD-3 / Apache-2.0 | ✅ Sim — **escolha recomendada** para o backend cgo |
| MuPDF (via `go-fitz`) | AGPL-3.0 | ❌ Contamina quem usar |
| Poppler | GPL | ❌ Contamina quem usar |

O backend cgo é fase posterior (v0.7+). Não comece por ele.

### 2.2 Dependências

| Pacote | Para quê | Licença | Observação |
|--------|----------|---------|------------|
| `github.com/ledongthuc/pdf` | extrair texto com X, Y, W, fonte | BSD-3 | fork do `rsc/pdf`; sem tags semânticas (usa pseudo-versão) |
| `github.com/shopspring/decimal` | dinheiro exato | MIT | **nunca** `float64` para valor |
| `golang.org/x/text/encoding/charmap` | Windows-1252 / Latin-1 → UTF-8 | BSD-3 | essencial para CSV e OFX brasileiros |
| `github.com/go-pdf/fpdf` (só testes) | gerar PDFs sintéticos para fixtures | MIT | evita commitar extrato real |

**Não usar `github.com/aclindsa/ofxgo`.** Na conversa original ele foi sugerido como base, mas a licença dele é **GPL-2.0**: importar isso torna a finparser efetivamente GPL. Além disso, OFX de banco brasileiro costuma vir malformado e um parser tolerante próprio é mais simples do que contornar um parser estrito. Ele continua útil como **referência de leitura**, não como dependência.

### 2.3 Licença da finparser

**MIT.** Arquivo `LICENSE` na raiz desde o primeiro commit — sem ele o pkg.go.dev não exibe a documentação.

---

## 3. Publicação para `go get`

1. Repositório **público** em `github.com/gustavoz65/finparser`.
2. O caminho do módulo **tem que ser idêntico** à URL:
   ```
   go mod init github.com/gustavoz65/finparser
   ```
3. Diretiva mínima no `go.mod`: `go 1.22` (não suba sem motivo; versão alta exclui consumidores).
4. Versão = tag git anotada no formato `vMAJOR.MINOR.PATCH`:
   ```
   git tag -a v0.1.0 -m "v0.1.0: OFX reader"
   git push origin v0.1.0
   ```
5. Forçar indexação no proxy e no pkg.go.dev:
   ```
   GOPROXY=https://proxy.golang.org go list -m github.com/gustavoz65/finparser@v0.1.0
   ```
6. **Tags são imutáveis no proxy.** Publicou errado? **Não apague nem refaça a tag.** Lance `v0.1.1` e marque a ruim com `retract` no `go.mod`:
   ```
   retract v0.1.0 // parser de data quebrado
   ```
7. Fique em `v0.x` enquanto a API pública mudar. `v1.0.0` é a promessa de não quebrar.
8. Se um dia for `v2`, o caminho do módulo muda para `github.com/gustavoz65/finparser/v2`.
9. Comentário de doc em **todo identificador exportado** e um `example_test.go` — os `Example*` aparecem no pkg.go.dev e contam como documentação viva.

---

## 4. Estrutura do repositório

API pública **mínima** na raiz; todo o resto em `internal/`. Assim dá para refatorar o motor sem quebrar quem importa.

```
finparser/
├── go.mod
├── LICENSE
├── README.md                  # público (o que você está lendo vira docs/BUILD.md)
├── finparser.go               # Parse(), opções
├── model.go                   # Statement, Transaction, Kind, Format
├── errors.go                  # erros sentinela + ParseError
├── example_test.go
│
├── internal/
│   ├── detect/                # descobre o formato pelos bytes
│   ├── money/                 # "R$ 1.234,56" → decimal (+ fuzz tests)
│   ├── date/                  # "02 MAR", "02/03/2026" → data civil
│   ├── textenc/               # detecta e converte encoding para UTF-8
│   ├── ofx/                   # parser SGML/XML tolerante (próprio)
│   ├── csv/                   # sniff de delimitador, cabeçalho, colunas
│   ├── pdf/
│   │   ├── extract/           # backend padrão (ledongthuc) → []Token
│   │   ├── extract_pdfium/    # backend cgo, build tag finparser_pdfium
│   │   └── spatial/           # tokens → linhas → colunas → tabela  (o "3D")
│   ├── header/                # descobre qual coluna é data/descrição/valor/saldo
│   ├── classify/              # receita x despesa
│   └── profile/               # dicas por banco (nubank, itau, inter, bb...)
│
├── cmd/
│   └── finparser-debug/       # CLI: despeja tokens e gera SVG com as caixas
│
└── testdata/
    ├── synthetic/             # gerados por código, commitados
    ├── golden/                # saída esperada em JSON
    └── private/               # extratos reais — NO .gitignore, nunca commitar
```

---

## 5. Modelo de dados público

```go
type Format string // "ofx", "csv", "pdf"

type Kind int // Unknown, Income (receita), Expense (despesa)

type Transaction struct {
    Date        time.Time        // data civil: meia-noite UTC, sem fuso
    Description string           // limpa (espaços normalizados)
    Amount      decimal.Decimal  // negativo = saída
    Balance     *decimal.Decimal // nil quando o extrato não traz
    Kind        Kind
    ID          string           // FITID do OFX ou hash estável
    Raw         []string         // colunas brutas, para debug
    Source      Location         // página/linha de origem
}

type Statement struct {
    Format       Format
    Bank         string // "nubank", "itau"... ou "" se não identificado
    PeriodStart  time.Time
    PeriodEnd    time.Time
    Transactions []Transaction
    Warnings     []Warning // linhas ignoradas e por quê
}
```

Regras do modelo:
- **Datas são civis.** Nada de `time.Local`; um extrato do dia 01/03 não pode virar 28/02 por causa de fuso.
- **`ID` é estável:** o mesmo arquivo parseado duas vezes gera os mesmos IDs (hash de data + valor + descrição + ordinal). O Fy usa isso para deduplicar importações.
- **`Warnings` em vez de silêncio:** se uma linha parecia transação e não foi interpretada, ela aparece lá.

---

## 6. API pública

```go
func Parse(r io.Reader, opts ...Option) (*Statement, error)

func WithFormat(f Format) Option          // pula a detecção
func WithBank(name string) Option         // força um perfil
func WithPassword(pw string) Option       // PDF protegido
func WithMaxBytes(n int64) Option         // padrão: 20 MB
func WithRowTolerance(factor float64) Option // multiplicador do tamanho da fonte
```

Detalhes que evitam dor:
- A entrada é `io.Reader`, mas o leitor de PDF exige `io.ReaderAt` + tamanho. Leia tudo para `[]byte` (respeitando `MaxBytes` via `io.LimitReader`) e use `bytes.NewReader`.
- Nada de estado global, nada de `init()` com efeito colateral, nada de log direto. Se precisar de log, opção recebendo `*slog.Logger`.
- Functional options em vez da struct `Options` da ideia original: dá para adicionar opção nova sem quebrar compatibilidade.

### Erros

```go
var (
    ErrUnknownFormat = errors.New("finparser: formato não reconhecido")
    ErrNoTextLayer   = errors.New("finparser: PDF sem camada de texto (escaneado?)")
    ErrEncrypted     = errors.New("finparser: PDF protegido por senha")
    ErrMalformed     = errors.New("finparser: arquivo malformado")
    ErrNoTransactions = errors.New("finparser: nenhuma transação encontrada")
)

type ParseError struct {
    Format Format
    Page   int
    Line   int
    Err    error
}
```

Sempre envolva com `%w` para o consumidor usar `errors.Is`.

---

## 7. Pipeline

```
io.Reader
   │
   ▼
[detect] ──► OFX ──► [textenc] ──► [ofx] ─────────────────────┐
   │                                                          │
   ├──────► CSV ──► [textenc] ──► [csv] ──► [header] ─────────┤
   │                                                          │
   └──────► PDF ──► [pdf/extract] ──► []Token                 │
                         │                                    │
                         ▼                                    │
                    [spatial] palavras → linhas → colunas     │
                         │                                    │
                         ▼                                    │
                    [header] ──► linhas tabulares ────────────┤
                                                              ▼
                                      [money] [date] → Transaction
                                                              │
                                                              ▼
                                                        [classify]
                                                              │
                                                              ▼
                                                         Statement
```

### 7.1 detect

Ordem de checagem sobre os primeiros ~1 KB:
1. `%PDF-` → PDF
2. `OFXHEADER:` ou `<OFX>` (ignorando BOM e espaços) → OFX
3. `!Type:` → QIF (tratar como OFX simplificado, v0.2+)
4. Senão, se for texto válido → CSV
5. Senão → `ErrUnknownFormat`

Nunca confiar só na extensão do arquivo.

### 7.2 textenc

- Remova BOM UTF-8 (`EF BB BF`).
- Se `utf8.Valid(b)` → já é UTF-8.
- Senão → decodifique como **Windows-1252** (superconjunto prático do Latin-1 que os bancos brasileiros usam).
- No OFX, o cabeçalho `CHARSET:1252` confirma; mas o cabeçalho mente às vezes, então valide os bytes mesmo assim.

### 7.3 OFX (parser próprio, tolerante)

OFX brasileiro quase sempre é **v1 SGML**, não XML. Peculiaridades reais:

- Cabeçalho `chave:valor` antes do `<OFX>` — descarte até o primeiro `<`.
- **Tags folha sem fechamento:** `<TRNAMT>-45.00` seguido direto de `<FITID>...`. O valor vai até o próximo `<`.
- Datas como `20260301120000[-3:BRT]` ou `20260301` — pegue os 8 primeiros dígitos.
- `TRNAMT` às vezes com vírgula (`-45,00`). Passe pelo `money`.
- `FITID` vazio ou duplicado em alguns bancos → gerar ID por hash.
- `MEMO` com acentos em Latin-1 → `textenc` antes.
- Extrato de conta: `BANKTRANLIST` em `STMTRS`; cartão: em `CCSTMTRS`. Suporte aos dois.

Estratégia: tokenizador simples que emite `open(tag)`, `close(tag)`, `leaf(tag, valor)`. Monta-se uma árvore e busca-se `STMTTRN`. Não use `encoding/xml` direto — ele falha nas tags sem fechamento.

### 7.4 CSV

1. **Delimitador:** teste `;`, `,`, `\t`, `|` nas primeiras 20 linhas não vazias; vence o que gera número de campos mais consistente e > 1. Brasil usa muito `;` justamente porque `,` é separador decimal.
2. `csv.Reader` com `LazyQuotes = true` e `FieldsPerRecord = -1`.
3. **Linha de cabeçalho:** a primeira linha que bate ≥ 2 palavras-chave do dicionário (seção 7.7). Linhas antes dela são metadados (nome, agência, período) — aproveite o período se encontrar.
4. Mapeamento de colunas via `header`.
5. Alguns bancos separam **Crédito** e **Débito** em colunas distintas → `Amount = crédito − débito`.

### 7.5 PDF — extração (o "3D")

Backend padrão: `ledongthuc/pdf`, `Reader.Page(n).Content().Text` → `[]Text{Font, FontSize, X, Y, W, S}`.

**Armadilhas conhecidas desse backend:**

- **Y cresce de baixo para cima** (sistema de coordenadas do PDF). Para ler de cima para baixo, ordene por **Y decrescente**. O pseudocódigo original ordenava crescente — isso inverte a página.
- O texto pode vir **por glifo** (uma letra por `Text`), não por palavra. É preciso juntar (seção 7.6, passo 1).
- O pacote herda do `rsc/pdf` o hábito de **`panic`** em PDF malformado. Toda chamada ao backend fica dentro de uma função com `defer recover()` que converte em `ErrMalformed`. Sem isso, um PDF ruim derruba o servidor do Fy.
- Suporte a criptografia é fraco. Tente `NewReaderEncrypted` com a senha; falhou → `ErrEncrypted`.
- Fontes com CMap ausente geram texto lixo (caracteres trocados). Heurística: se > 30% dos runes de uma página estão fora de letras/dígitos/pontuação comum, marque a página como ilegível e emita `Warning`. É aqui que o backend PDFium (cgo) paga o custo dele.
- Página sem nenhum `Text` em todas as páginas → `ErrNoTextLayer`.

O backend é uma interface interna, para trocar sem tocar no resto:

```go
type Extractor interface {
    Extract(data []byte, password string) ([]Token, error)
}

type Token struct {
    Page     int
    X, Y     float64 // canto inferior esquerdo, em pontos
    W        float64
    FontSize float64
    Font     string
    Text     string
}
```

### 7.6 PDF — motor espacial

**Passo 1 — glifos → palavras.** Na mesma página, com Y praticamente igual (|ΔY| < 0,2 × fonte), ordenados por X: se o espaço entre o fim do anterior (`X + W`) e o início do atual for < 0,25 × fonte, concatena; se for maior, começa palavra nova. Guarde `X0` (início) e `X1` (fim) de cada palavra.

**Passo 2 — palavras → linhas.** Tolerância **relativa à fonte**, não 5 px fixo: duas palavras estão na mesma linha se |ΔY| ≤ 0,5 × `min(fontA, fontB)`. Compare com a **mediana** do Y da linha atual, não com o último elemento (senão a linha "escorrega" em texto levemente inclinado).

**Passo 3 — remover ruído de página.** Linhas que se repetem idênticas no topo/rodapé de várias páginas ("Página 2 de 5", nome do banco, CPF mascarado) são cabeçalho/rodapé → descarte.

**Passo 4 — linhas → colunas.** Duas estratégias, nesta ordem:
1. **Âncora pelo cabeçalho:** ache a linha de cabeçalho da tabela (seção 7.7); cada palavra-chave define uma faixa [X0, X1]. Cada palavra das linhas seguintes cai na coluna com maior sobreposição horizontal.
2. **Perfil de projeção (fallback):** projete as faixas [X0, X1] de todas as linhas candidatas no eixo X; as "calhas" de espaço vazio que atravessam a maioria das linhas são as fronteiras entre colunas.

Valores monetários costumam ser **alinhados à direita** — para tokens numéricos use `X1` (borda direita) como referência, não `X0`.

**Passo 5 — linhas → transações.**
- Linha com data e valor → transação nova.
- Linha **sem data nem valor**, logo abaixo de uma transação → continuação da descrição (descrição quebrada em duas linhas). Junte com espaço.
- Linha com valor mas **sem data** → herda a última data vista. Alguns bancos (verifique no Nubank) agrupam lançamentos por dia com a data aparecendo uma vez só.
- Linhas de resumo ("SALDO DO DIA", "SALDO ANTERIOR", "TOTAL DE ENTRADAS") → não são transação; viram saldo ou são ignoradas.

**Passo 6 — tabela continua entre páginas.** O mapeamento de colunas descoberto na página 1 é reaproveitado nas seguintes se elas não repetirem o cabeçalho.

### 7.7 header — dicionário de colunas

Comparação sem acento e sem caixa (normalize com `golang.org/x/text/unicode/norm` + remover marcas):

| Papel | Palavras-chave |
|-------|----------------|
| Data | data, dt, data lancamento, data mov |
| Descrição | descricao, historico, lancamento, detalhes, estabelecimento |
| Valor | valor, valor (r$), quantia, montante |
| Crédito | credito, entradas |
| Débito | debito, saidas |
| Saldo | saldo, saldo (r$) |
| Documento | documento, doc, nr doc |

Sem cabeçalho detectável → inferência por conteúdo: a coluna em que ≥ 80% das células parseiam como data é a data; a que parseia como dinheiro e mais varia é valor; a monetária que muda de forma cumulativa é saldo.

### 7.8 money

Entradas reais a suportar (escreva **um teste por linha**):

```
R$ 1.234,56      → 1234.56
-R$ 45,00        → -45.00
R$ -45,00        → -45.00
1.234,56 D       → -1234.56
1.234,56 C       → 1234.56
1.234,56-        → -1234.56
(1.234,56)       → -1234.56
+ 100,00         → 100.00
1234.56          → 1234.56   (OFX)
0,01             → 0.01
```

Algoritmo: remover `R$` e espaços (inclusive `\u00a0`, espaço não-quebrável, muito comum em PDF) → extrair sinal (`-` antes/depois, `D`/`C`, parênteses) → decidir separador decimal: **o último `,` ou `.` seguido de exatamente 2 dígitos no fim** é o decimal; o outro é milhar → `decimal.NewFromString`.

Escreva um **fuzz test** (`go test -fuzz=FuzzParseMoney`) garantindo que nunca dá panic.

### 7.9 date

Formatos: `02/01/2006`, `02/01/06`, `02/01`, `02-01-2006`, `2006-01-02`, `02 MAR`, `02 MAR 2026`, `02/mar`.
Meses em português: JAN FEV MAR ABR MAI JUN JUL AGO SET OUT NOV DEZ (aceitar também por extenso).

**Data sem ano:** use o período do extrato (extraído do cabeçalho do documento). Extrato de dezembro a janeiro: se o mês da transação é maior que o mês final do período, ela é do ano anterior. Sem período conhecido → ano atual + `Warning`.

### 7.10 classify

v1, nesta ordem:
1. `Amount > 0` → `Income`; `Amount < 0` → `Expense`.
2. Fonte não informa sinal (coluna de valor sem D/C nem `-`, sem colunas separadas) → padrões na descrição: "PIX RECEBIDO", "TED RECEBIDA", "TRANSFERENCIA RECEBIDA", "SALARIO", "RENDIMENTO" → `Income`; "PIX ENVIADO", "COMPRA", "PAGAMENTO", "TARIFA", "SAQUE" → `Expense`.
3. Validação cruzada com saldo, quando existir: `saldo[i] − saldo[i−1] == valor[i]` confirma o sinal; se der `-valor`, inverte e emite `Warning`. Essa checagem é a melhor rede de segurança do parser de PDF inteiro.
4. Nada bateu → `Unknown`. Nunca chute.

### 7.11 profile (perfis de banco)

Opcional, só para o que o motor genérico não resolve:

```go
type Profile interface {
    Name() string
    Match(firstPageText string) bool      // ex.: contém "Nu Pagamentos S.A."
    Hints() Hints                          // palavras-chave de cabeçalho, regex de linhas a ignorar, convenção de sinal
}
```

O motor genérico roda sempre; o perfil só **ajusta** parâmetros. Se você se pegar escrevendo um parser inteiro dentro de um perfil, o motor genérico está faltando algo.

---

## 8. Testes

- **Nunca commite extrato real.** Nem "anonimizado à mão" — é fácil esquecer um CPF ou nome. Extratos reais ficam em `testdata/private/` (no `.gitignore`) e rodam só localmente com `go test -tags private`.
- **Fixtures sintéticos:** gere PDFs que imitam o layout de cada banco com `go-pdf/fpdf` (coordenadas, fontes, quebra de página, descrição em duas linhas) e CSV/OFX escritos à mão com dados falsos.
- **Golden files:** saída esperada em `testdata/golden/*.json`; flag `-update` para regenerar conscientemente.
- **Table-driven tests** para `money`, `date`, `detect`, `header`.
- **Fuzz** para `money`, `date` e o tokenizador OFX.
- Teste de **robustez:** PDF truncado, PDF vazio, CSV só com cabeçalho, OFX sem `BANKTRANLIST` → erro limpo, nunca panic.

---

## 9. CLI de debug (`cmd/finparser-debug`)

A ferramenta mais importante para trabalhar no PDF:

- `finparser-debug tokens arquivo.pdf` → tabela com Page, X, Y, W, FontSize, Text
- `finparser-debug svg arquivo.pdf` → um SVG por página desenhando a caixa de cada token, linhas detectadas e fronteiras de coluna em cores
- `finparser-debug parse arquivo.pdf` → JSON do `Statement`

É literalmente "ver o espaço" que a lib enxerga. Quando uma coluna sair errada, o SVG mostra o porquê em segundos.

---

## 10. CI (GitHub Actions)

Em todo push e PR:
- `go vet ./...`
- `go test -race ./...` em `stable` e `oldstable`
- `CGO_ENABLED=0 go build ./...` — garante que o caminho padrão continua Go puro
- `golangci-lint run`
- `govulncheck ./...`
- Job separado (opcional, só quando o backend existir): build com `-tags finparser_pdfium`

---

## 11. Roadmap

| Versão | Entrega | Pronto quando |
|--------|---------|---------------|
| v0.1.0 | `model`, `errors`, `money`, `date`, `textenc`, `detect`, OFX | Fy importa via `go get` e lê o seu OFX real |
| v0.2.0 | CSV + `header` | 2 bancos em CSV com golden files |
| v0.3.0 | Extração PDF + CLI de debug | **spike:** tokens legíveis de Nubank, Itaú e BB; decisão registrada sobre o backend |
| v0.4.0 | Motor espacial, Nubank | Nubank em PDF com saldo batendo linha a linha |
| v0.5.0 | Generalização + `profile` | 3+ bancos em PDF |
| v0.6.0 | `classify` completo + validação por saldo | `Unknown` < 5% nos fixtures |
| v0.7.0 | Backend PDFium (cgo, build tag) | mesmos golden files passando com as duas engines |
| v1.0.0 | API congelada | 1 consumidor externo além do Fy |

---

## 12. Regras para quem for implementar (humano ou IA)

Leia isto antes de escrever código. Cada item é um erro que já foi previsto:

1. **Nunca `float64` para dinheiro.** `decimal.Decimal` do começo ao fim.
2. **Nunca importar `aclindsa/ofxgo`** nem nenhuma dependência GPL/AGPL (MuPDF, Poppler, `go-fitz`).
3. **Nunca cgo no caminho padrão.** Código com `import "C"` só em arquivos com `//go:build finparser_pdfium`.
4. **Y do PDF cresce para cima.** Ordenar por Y decrescente para ler de cima para baixo.
5. **Tolerâncias relativas ao `FontSize`**, nunca pixels fixos.
6. **`recover()` em volta de toda chamada ao backend de PDF.**
7. **`io.Reader` → `[]byte` → `bytes.NewReader`** para obter `ReaderAt`; respeitar `MaxBytes`.
8. **Encoding antes de qualquer parse de texto.** BOM fora, Windows-1252 → UTF-8.
9. **Espaço não-quebrável (`\u00a0`)** tratado como espaço em todo lugar.
10. **Datas civis em UTC**, sem `time.Local`.
11. **Não engolir erro.** Linha não interpretada vira `Warning`; arquivo não interpretável vira erro sentinela com `%w`.
12. **Nada de estado global, `init()` ou `fmt.Println`** dentro da lib.
13. **Tudo que não é API fica em `internal/`.**
14. **Nenhum extrato real no repositório.**
15. **Um pacote por vez, com teste.** Não implementar o pipeline inteiro de uma vez; seguir o roadmap.
16. **Não reetiquetar tag publicada.** Errou → nova versão + `retract`.
17. **Ao travar no PDF, rodar a CLI de debug antes de mexer em heurística.**

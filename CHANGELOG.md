# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/); o projeto segue [versionamento semântico](https://semver.org/lang/pt-BR/).

## [Não lançado]

### Adicionado
- Formatos: XLSX (só biblioteca padrão), tabela HTML (o falso `.xls`), ISO 20022 camt.052/053/054 e SWIFT MT940/MT942.
- `ParseFile`, `ParseBytes` e `DetectFormat`.
- `Statement.Reconcile`: confere o extrato contra os saldos que ele declara.
- `Statement.Summary`, `Merge` (junta extratos sem duplicar) e `WriteOFX`, `WriteCSV`, `WriteJSON`.
- `Statement.Account`, `Currency`, `OpeningBalance` e `ClosingBalance`; `Transaction.Method` e `Counterparty`.
- Sinal resolvido pelo saldo quando há vários lançamentos entre dois saldos (soma de subconjuntos, só com solução única).
- Motor espacial em matriz de bits (`internal/pdf/matrix`): projeção bit-sliced, calhas e vales.
- CSV: coluna de sinal D/C, várias colunas de descrição, fim de linha CR e diretiva `sep=`.
- CLI `finparser` (mostra, confere, junta, converte) e comando `grid` no `finparser-debug`.
- Fixtures sintéticos de Bradesco e Caixa (PDF), Inter (XLSX, CSV) e Santander (HTML); benchmarks de ponta a ponta.

### Corrigido
- PDF com cabeçalho perdia lançamentos quando a descrição passava da largura do título da coluna ou o valor começava antes dele.
- PDF: valor partido em palavras (`R$ 45,90`, `- 45,90`, `45,90 D`) na leitura linha a linha.
- Valores com menos tipográfico (`−45,90`), sufixos `CR`/`DB` e prefixo de moeda; `0.500` não é mais lido como 500.
- `10.12` não é mais lido como data; data sem ano que cairia no futuro vai para o ano anterior.
- OFX: `CDATA` no `MEMO`, `TRNTYPE` `POS`/`ATM` seguindo o sinal do valor, `12.345` lido como decimal.
- QIF: listas de categoria e classe não geram mais avisos.
- "TOTAL DE ..." não é mais tomado por saldo.
- Mensagem de erro sem o prefixo `finparser:` repetido.

### Desempenho
- Colunas pela matriz: 3,7× mais rápido e 230× menos alocações que a versão célula a célula.
- PDF de 1000 lançamentos: de 148 ms para ~100 ms.
- Normalização de texto 4× mais rápida em texto acentuado.

### Base
- `Parse` com detecção de formato pelos bytes e opções funcionais.
- OFX v1/v2 (conta e cartão) com parser próprio e tolerante.
- QIF.
- CSV com detecção de delimitador, cabeçalho, período e colunas por conteúdo.
- PDF com camada de texto: extração posicionada, motor espacial, colunas por cabeçalho ou projeção.
- Classificação receita/despesa por sinal, padrões e validação de saldo.
- Perfis de banco (Nubank, Itaú, Inter, BB, Bradesco, Santander, Caixa, C6).
- CLI `finparser-debug` (tokens, linhas, SVG, parse).

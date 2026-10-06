# Changelog

Formato baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/); o projeto segue [versionamento semântico](https://semver.org/lang/pt-BR/).

## [Não lançado]

### Adicionado
- `Parse` com detecção de formato pelos bytes e opções funcionais.
- OFX v1/v2 (conta e cartão) com parser próprio e tolerante.
- QIF.
- CSV com detecção de delimitador, cabeçalho, período e colunas por conteúdo.
- PDF com camada de texto: extração posicionada, motor espacial, colunas por cabeçalho ou projeção.
- Classificação receita/despesa por sinal, padrões e validação de saldo.
- Perfis de banco (Nubank, Itaú, Inter, BB, Bradesco, Santander, Caixa, C6).
- CLI `finparser-debug` (tokens, linhas, SVG, parse).

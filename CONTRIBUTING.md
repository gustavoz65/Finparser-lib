# Contribuindo

1. Abra uma issue descrevendo o banco/formato e o que saiu errado. Anexe a saída de `finparser-debug lines` ou o SVG **sem dados pessoais**.
2. Nunca envie extrato real. Para reproduzir um layout, crie um fixture sintético em `internal/testgen` (PDF) ou em `testdata/synthetic` (OFX/CSV/QIF) com dados falsos.
3. Rode antes do PR:
   ```
   go vet ./...
   go test -race ./...
   CGO_ENABLED=0 go build ./...
   golangci-lint run
   ```
4. Mudou a saída de algum fixture? `go test . -update` e revise o diff dos golden files.

Regras do projeto (detalhes em [docs/BUILD.md](docs/BUILD.md)): nada de `float64` para dinheiro, nada de dependência GPL/AGPL, nada de cgo fora de build tag, tolerâncias do PDF sempre relativas ao tamanho da fonte, e tudo que não é API pública fica em `internal/`.

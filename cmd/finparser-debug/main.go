// Comando finparser-debug mostra o que a finparser enxerga num extrato.
//
//	finparser-debug tokens arquivo.pdf   tabela Page, X, Y, W, FontSize, Text
//	finparser-debug lines  arquivo.pdf   linhas reconstruídas pelo motor espacial
//	finparser-debug svg    arquivo.pdf   um SVG por página (tokens, linhas, colunas)
//	finparser-debug grid   arquivo.pdf   a matriz de ocupação e a projeção no eixo X
//	finparser-debug parse  arquivo       JSON do Statement (qualquer formato)
//
// Flags: -password, -bank, -out (diretório dos SVGs), -tolerance, -scale
// (pontos por caractere no grid).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	finparser "github.com/gustavoz65/finparser-lib"
	"github.com/gustavoz65/finparser-lib/internal/pdf/extract"
	"github.com/gustavoz65/finparser-lib/internal/pdf/matrix"
	"github.com/gustavoz65/finparser-lib/internal/pdf/spatial"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "finparser-debug:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("finparser-debug", flag.ContinueOnError)
	password := fs.String("password", "", "senha do PDF")
	bank := fs.String("bank", "", "força o perfil de banco")
	out := fs.String("out", ".", "diretório de saída dos SVGs")
	tolerance := fs.Float64("tolerance", 0.5, "tolerância de linha (× fonte)")
	scale := fs.Float64("scale", 5, "grid: pontos do eixo X por caractere")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), "uso: finparser-debug tokens|lines|svg|grid|parse [flags] arquivo")
		fs.PrintDefaults()
	}
	if len(args) < 2 {
		fs.Usage()
		return fmt.Errorf("faltam argumentos")
	}
	cmd, rest := args[0], args[1:]
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("informe um arquivo")
	}
	path := fs.Arg(0)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	switch cmd {
	case "parse":
		opts := []finparser.Option{finparser.WithRowTolerance(*tolerance)}
		if *password != "" {
			opts = append(opts, finparser.WithPassword(*password))
		}
		if *bank != "" {
			opts = append(opts, finparser.WithBank(*bank))
		}
		st, err := finparser.Parse(strings.NewReader(string(data)), opts...)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(st)
	case "tokens", "lines", "svg", "grid":
	default:
		return fmt.Errorf("comando desconhecido %q", cmd)
	}

	toks, err := extract.Default{}.Extract(data, *password)
	if err != nil {
		return err
	}
	switch cmd {
	case "tokens":
		return printTokens(stdout, toks)
	case "lines":
		return printLines(stdout, spatial.Build(toks, *tolerance))
	case "grid":
		return printGrid(stdout, spatial.Build(toks, *tolerance), *scale)
	default:
		files, err := writeSVGs(*out, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), toks, *tolerance)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, strings.Join(files, "\n"))
		return err
	}
}

func printTokens(w io.Writer, toks []extract.Token) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	if _, err := fmt.Fprintln(tw, "Page\tX\tY\tW\tFontSize\tText\t"); err != nil {
		return err
	}
	for _, t := range toks {
		if _, err := fmt.Fprintf(tw, "%d\t%.2f\t%.2f\t%.2f\t%.1f\t%q\t\n", t.Page, t.X, t.Y, t.W, t.FontSize, t.Text); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func printLines(w io.Writer, lines []spatial.Line) error {
	for _, l := range lines {
		if _, err := fmt.Fprintf(w, "p%d y=%7.2f x=[%6.2f,%6.2f] %s\n", l.Page, l.Y, l.X0(), l.X1(), l.Text()); err != nil {
			return err
		}
	}
	return nil
}

// printGrid desenha, por página, a matriz de ocupação que o motor espacial
// usa: uma linha de texto por linha do PDF ('#' = há tinta), e embaixo a
// projeção no eixo X em dígitos (0 = coluna vazia em todas as linhas, 9 =
// ocupada em quase todas). As calhas entre colunas são as faixas de zeros.
func printGrid(w io.Writer, lines []spatial.Line, scale float64) error {
	for from := 0; from < len(lines); {
		to := from
		lo, hi := lines[from].X0(), lines[from].X1()
		for to < len(lines) && lines[to].Page == lines[from].Page {
			lo, hi = min(lo, lines[to].X0()), max(hi, lines[to].X1())
			to++
		}
		page := lines[from:to]
		g := matrix.New(len(page), lo, hi, scale)
		if g == nil {
			return fmt.Errorf("página %d: dimensões inválidas para o grid", lines[from].Page)
		}
		for r, l := range page {
			for _, wd := range l.Words {
				g.Fill(r, wd.X0, wd.X1)
			}
		}
		if _, err := fmt.Fprintf(w, "página %d: %d linhas × %d colunas (%.4g pt por coluna, x de %.1f a %.1f)\n%s",
			lines[from].Page, g.Rows(), g.Cols(), scale, lo, hi, g); err != nil {
			return err
		}
		var sb strings.Builder
		for _, n := range g.Projection(0, g.Rows()) {
			sb.WriteByte(byte('0' + min(9, (int(n)*10+g.Rows()-1)/g.Rows())))
		}
		if _, err := fmt.Fprintf(w, "%s  ← projeção\n\n", sb.String()); err != nil {
			return err
		}
		from = to
	}
	return nil
}

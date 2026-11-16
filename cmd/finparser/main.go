// Comando finparser lê extratos bancários e mostra, confere ou converte.
//
//	finparser extrato.pdf                      tabela, totais e conferência de saldo
//	finparser -to ofx -o extrato.ofx extrato.pdf   converte para OFX
//	finparser -to csv jan.ofx fev.ofx          junta arquivos sem duplicar e grava CSV
//	finparser -check extrato.xlsx              falha se os saldos não fecharem
//
// Formatos de entrada: OFX, QIF, CSV, XLSX, HTML, PDF, camt.053 e MT940,
// detectados pelo conteúdo. Saída (-to): table, json, csv ou ofx.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	finparser "github.com/gustavoz65/finparser-lib"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "finparser:", err)
		}
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("finparser", flag.ContinueOnError)
	to := fs.String("to", "table", "saída: table, json, csv ou ofx")
	out := fs.String("o", "", "arquivo de saída (padrão: saída padrão)")
	bank := fs.String("bank", "", "força o perfil de banco")
	password := fs.String("password", "", "senha do PDF")
	check := fs.Bool("check", false, "falha se a conferência de saldo não fechar")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), "uso: finparser [flags] arquivo [arquivo...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("informe pelo menos um arquivo")
	}
	switch *to {
	case "table", "json", "csv", "ofx":
	default:
		return fmt.Errorf("saída desconhecida %q (use table, json, csv ou ofx)", *to)
	}

	var opts []finparser.Option
	if *bank != "" {
		opts = append(opts, finparser.WithBank(*bank))
	}
	if *password != "" {
		opts = append(opts, finparser.WithPassword(*password))
	}
	var parsed []*finparser.Statement
	for _, path := range fs.Args() {
		st, err := finparser.ParseFile(path, opts...)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		parsed = append(parsed, st)
	}
	st := parsed[0]
	if len(parsed) > 1 {
		st = finparser.Merge(parsed...)
	}
	rec := st.Reconcile()

	w := stdout
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		w = f
	}
	var err error
	switch *to {
	case "json":
		err = st.WriteJSON(w)
	case "csv":
		err = st.WriteCSV(w)
	case "ofx":
		err = st.WriteOFX(w)
	default:
		err = table(w, st, rec)
	}
	if err != nil {
		return err
	}
	if *check && rec.Status != finparser.Verified {
		return fmt.Errorf("conferência de saldo: %s (%d de %d contas falharam)", rec.Status, rec.Failed, rec.Checked)
	}
	return nil
}

func table(w io.Writer, st *finparser.Statement, rec finparser.Reconciliation) error {
	bank := st.Bank
	if bank == "" {
		bank = "banco não identificado"
	}
	if _, err := fmt.Fprintf(w, "%s · %s · %s a %s\n\n", st.Format, bank,
		st.PeriodStart.Format("02/01/2006"), st.PeriodEnd.Format("02/01/2006")); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "DATA\tDESCRIÇÃO\tVALOR\tSALDO\tTIPO\tMEIO")
	for _, t := range st.Transactions {
		balance := ""
		if t.Balance != nil {
			balance = t.Balance.StringFixed(2)
		}
		desc := []rune(t.Description)
		if len(desc) > 48 {
			desc = append(desc[:47], '…')
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", t.Date.Format("02/01/2006"), string(desc),
			t.Amount.StringFixed(2), balance, t.Kind, t.Method)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	sum := st.Summary()
	if _, err := fmt.Fprintf(w, "\n%d lançamentos · entradas %s · saídas %s · líquido %s",
		sum.Count, sum.Income.StringFixed(2), sum.Expense.StringFixed(2), sum.Net.StringFixed(2)); err != nil {
		return err
	}
	if sum.UnknownCount > 0 {
		_, _ = fmt.Fprintf(w, " · %d sem sinal conhecido", sum.UnknownCount)
	}
	switch rec.Status {
	case finparser.Verified:
		_, _ = fmt.Fprintf(w, "\nsaldo confere: %d contas, %d de %d lançamentos cobertos\n", rec.Checked, rec.Covered, rec.Total)
	case finparser.Mismatch:
		_, _ = fmt.Fprintf(w, "\nSALDO NÃO CONFERE: %d de %d contas falharam\n", rec.Failed, rec.Checked)
		for _, m := range rec.Mismatches {
			where := "extrato inteiro"
			if m.Index >= 0 {
				where = "até " + st.Transactions[m.Index].Date.Format("02/01/2006") + " " + st.Transactions[m.Index].Description
			}
			_, _ = fmt.Fprintf(w, "  %s: esperado %s, somado %s\n", where, m.Expected.StringFixed(2), m.Actual.StringFixed(2))
		}
	default:
		_, _ = fmt.Fprintln(w, "\nsem saldos para conferir")
	}
	for _, wn := range st.Warnings {
		_, _ = fmt.Fprintf(w, "aviso (página %d, linha %d): %s\n", wn.Source.Page, wn.Source.Line, wn.Message)
	}
	return nil
}

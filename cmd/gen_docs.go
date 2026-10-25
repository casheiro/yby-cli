/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"os"
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// genDocsCmd represents the gen-docs command
var genDocsCmd = &cobra.Command{
	Use:    "gen-docs [output-dir]",
	Short:  "Gera documentação Markdown para todos os comandos",
	Hidden: true, // Internal tool only
	Args:   cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir := "./docs/wiki"
		if len(args) > 0 {
			outputDir = args[0]
		}

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao criar diretório de saída").
				WithHint("confira o caminho e a permissão de escrita em " + outputDir)
		}

		outputFile := outputDir + "/CLI-Reference.md"

		f, err := os.Create(outputFile)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao criar arquivo de documentação").
				WithHint("confira o caminho e a permissão de escrita em " + outputDir)
		}
		defer f.Close()

		// Write Header (o conteúdo do arquivo gerado não muda; o emoji vai por escape)
		if _, err := f.WriteString("# \U0001F4D6 CLI Reference\n\n"); err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao gravar a documentação").
				WithHint("confira se a pasta de destino pode ser gravada")
		}
		if _, err := f.WriteString("Referência completa de todos os comandos do Yby CLI.\n\n"); err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "falha ao gravar a documentação").
				WithHint("confira se a pasta de destino pode ser gravada")
		}

		// Disable AutoGen tag globally
		rootCmd.DisableAutoGenTag = true

		if err := writeCommandDocs(f, rootCmd); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao gerar a documentação").
				WithHint("confira se a pasta de destino pode ser gravada")
		}

		w := cmd.OutOrStdout()
		tituloComando(w, "gen-docs", "referência da CLI")
		checksAlinhados(w, linhaCheck{ui.CheckOk, "CLI-Reference.md", "gerado", outputFile})
		return nil
	},
}

func writeCommandDocs(f *os.File, cmd *cobra.Command) error {
	// Generate markdown for current command
	// We capture stdout/buffer? No, GenMarkdown writes to a writer.

	// Create a header for the command
	// We want to skip "See Warning" or similar if possible, but standard GenMarkdown is fine.

	// Custom link handler for single-page navigation
	linkHandler := func(s string) string {
		// yby_init.md -> #yby-init
		return "#" + strings.ReplaceAll(strings.TrimSuffix(s, ".md"), "_", "-")
	}

	if err := doc.GenMarkdownCustom(cmd, f, linkHandler); err != nil {
		return err
	}
	if _, err := f.WriteString("\n---\n\n"); err != nil {
		return err
	}

	// Recurse children
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := writeCommandDocs(f, c); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(genDocsCmd)
}

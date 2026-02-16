/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// genDocsCmd represents the gen-docs command
var genDocsCmd = &cobra.Command{
	Use:    "gen-docs [output-dir]",
	Short:  "Gera documentação Markdown para todos os comandos",
	Hidden: true, // Internal tool only
	Args:   cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outputDir := "./docs/wiki"
		if len(args) > 0 {
			outputDir = args[0]
		}

		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Printf("❌ Erro criando diretório de saída: %v\n", err)
			os.Exit(1)
		}

		outputFile := outputDir + "/CLI-Reference.md"
		fmt.Printf("📝 Gerando referência consolidada em '%s'...\n", outputFile)

		f, err := os.Create(outputFile)
		if err != nil {
			fmt.Printf("❌ Erro criando arquivo: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		// Write Header
		if _, err := f.WriteString("# 📖 CLI Reference\n\n"); err != nil {
			fmt.Printf("❌ Erro escrevendo no arquivo: %v\n", err)
			os.Exit(1)
		}
		if _, err := f.WriteString("Referência completa de todos os comandos do Yby CLI.\n\n"); err != nil {
			fmt.Printf("❌ Erro escrevendo no arquivo: %v\n", err)
			os.Exit(1)
		}

		// Disable AutoGen tag globally
		rootCmd.DisableAutoGenTag = true

		if err := writeCommandDocs(f, rootCmd); err != nil {
			fmt.Printf("❌ Erro gerando docs: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("✅ Documentação consolidada gerada com sucesso!")
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

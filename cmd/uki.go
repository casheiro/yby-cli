/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/spf13/cobra"
)

// ukiCmd represents the uki command
var ukiCmd = &cobra.Command{
	Use:   "uki",
	Short: "Gerencia Unidades de Conhecimento Interligada (UKIs)",
	Long: `O comando UKI permite interagir com a camada de governança do Synapstor.
Use 'capture' para transformar intenções e descrições em documentação estruturada assistida por IA.`,
}

// captureCmd represents the capture command
var captureCmd = &cobra.Command{
	Use:   "capture [description]",
	Short: "Captura conhecimento e gera arquivos UKI via IA",
	Long: `Analisa uma descrição em linguagem natural e gera artefatos de governança (Markdown)
dentro do diretório .synapstor/.uki/.

Exemplo:
  yby uki capture "Precisamos de uma política de retenção de logs de 30 dias para compliance PCI-DSS"
  yby uki capture --file minutas/reuniao-seguranca.txt`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// 1. Get Description
		description := ""
		if len(args) > 0 {
			description = strings.Join(args, " ")
		}

		// Optional: Read from file if flag provided (skipped for MVP simplicity unless requested, sticking to args/stdin later)
		// For now, simple arg.

		if description == "" {
			fmt.Println(crossStyle.Render("❌ Erro: Descrição necessária."))
			fmt.Println("Uso: yby uki capture \"Descrição da necessidade ou decisão\"")
			os.Exit(1)
		}

		fmt.Println(titleStyle.Render("🧠 Yby AI - Governance Capture"))
		fmt.Println(stepStyle.Render("🔄 Inicializando provedor de IA..."))

		// 2. Init AI
		providerName, _ := cmd.Flags().GetString("ai-provider")
		provider := ai.GetProvider(ctx, providerName)

		if provider == nil {
			fmt.Println(crossStyle.Render("❌ Nenhum provedor de IA encontrado ou configurado."))
			fmt.Println("Dica: Exporte OPENAI_API_KEY, GEMINI_API_KEY ou rode 'ollama serve'.")
			os.Exit(1)
		}

		fmt.Printf("%s Usando provedor: %s\n", checkStyle.String(), provider.Name())
		fmt.Println(stepStyle.Render("🤔 Analisando e estruturando conhecimento... (Isso pode levar alguns segundos)"))

		// 3. Generate
		blueprint, err := provider.GenerateGovernance(ctx, description)
		if err != nil {
			fmt.Printf("%s Erro na geração: %v\n", crossStyle.String(), err)
			os.Exit(1)
		}

		// 4. Save Files
		baseDir := ".synapstor"
		// Ensure dirs exist
		if err := os.MkdirAll(filepath.Join(baseDir, ".uki"), 0755); err != nil {
			fmt.Printf("%s Falha ao criar diretório .uki: %v\n", crossStyle.String(), err)
			os.Exit(1)
		}
		if err := os.MkdirAll(filepath.Join(baseDir, ".personas"), 0755); err != nil {
			fmt.Printf("%s Falha ao criar diretório .personas: %v\n", crossStyle.String(), err)
			os.Exit(1)
		}

		fmt.Println("")
		fmt.Println(headerStyle.Render("📄 Arquivos Gerados:"))

		for _, f := range blueprint.Files {
			// Security check: clean path to avoid writing outside root
			cleanPath := filepath.Clean(f.Path)
			if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") {
				fmt.Printf("⚠️  Caminho inseguro ignorado: %s\n", f.Path)
				continue
			}

			// Create dir if needed
			dir := filepath.Dir(cleanPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Printf("%s Falha ao criar diretório pai %s: %v\n", crossStyle.String(), dir, err)
				continue
			}

			if err := os.WriteFile(cleanPath, []byte(f.Content), 0644); err != nil {
				fmt.Printf("%s Falha ao escrever %s: %v\n", crossStyle.String(), cleanPath, err)
			} else {
				fmt.Printf("%s %s\n", checkStyle.String(), cleanPath)
			}
		}

		fmt.Println("")
		fmt.Println(checkStyle.Render("✅ Governança capturada com sucesso!"))
	},
}

func init() {
	rootCmd.AddCommand(ukiCmd)
	ukiCmd.AddCommand(captureCmd)

	// Flags
	captureCmd.Flags().String("ai-provider", "", "Forçar provedor de IA (ollama, gemini, openai)")
}

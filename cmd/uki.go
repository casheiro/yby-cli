/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// getAIProvider permite override em testes
var getAIProvider = ai.GetProvider

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
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// 1. Obtém descrição: via --file ou argumento posicional
		var description string
		filePath, _ := cmd.Flags().GetString("file")
		if filePath != "" {
			data, err := os.ReadFile(filePath)
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeIO, "falha ao ler arquivo")
			}
			description = string(data)
		} else if len(args) > 0 {
			description = strings.Join(args, " ")
		} else {
			return errors.New(errors.ErrCodeValidation, "informe uma descrição ou use --file")
		}

		fmt.Println(titleStyle.Render("🧠 Yby AI - Governance Capture"))
		fmt.Println(stepStyle.Render("🔄 Inicializando provedor de IA..."))

		// 2. Init AI
		providerName, _ := cmd.Flags().GetString("ai-provider")
		provider := getAIProvider(ctx, providerName)

		if provider == nil {
			return errors.New(errors.ErrCodeConfig, "Nenhum provedor de IA encontrado. Dica: Exporte OPENAI_API_KEY, GEMINI_API_KEY ou rode 'ollama serve'")
		}

		fmt.Printf("%s Usando provedor: %s\n", checkStyle.String(), provider.Name())
		fmt.Println(stepStyle.Render("🤔 Analisando e estruturando conhecimento... (Isso pode levar alguns segundos)"))

		// 3. Generate via Completion
		blueprint, err := generateGovernanceViaCompletion(ctx, provider, description)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "Erro na geração AI")
		}

		// 4. Save Files
		baseDir := ".synapstor"
		// Ensure dirs exist
		if err := os.MkdirAll(filepath.Join(baseDir, ".uki"), 0755); err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "Falha ao criar diretório .uki")
		}
		if err := os.MkdirAll(filepath.Join(baseDir, ".personas"), 0755); err != nil {
			return errors.Wrap(err, errors.ErrCodeIO, "Falha ao criar diretório .personas")
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
		return nil
	},
}

// generateGovernanceViaCompletion usa Completion com o system prompt de governança
// para gerar um GovernanceBlueprint, substituindo o uso direto de GenerateGovernance.
func generateGovernanceViaCompletion(ctx context.Context, provider ai.Provider, description string) (*ai.GovernanceBlueprint, error) {
	userPrompt := fmt.Sprintf("Descrição do Projeto: %s", description)
	result, err := provider.Completion(ctx, prompts.Get("governance.system"), userPrompt)
	if err != nil {
		return nil, err
	}

	// Limpar possíveis fences de markdown na resposta
	cleanJSON := strings.TrimPrefix(result, "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	var blueprint ai.GovernanceBlueprint
	if err := json.Unmarshal([]byte(cleanJSON), &blueprint); err != nil {
		return nil, fmt.Errorf("falha ao analisar json do blueprint: %w", err)
	}

	return &blueprint, nil
}

func init() {
	rootCmd.AddCommand(ukiCmd)
	ukiCmd.AddCommand(captureCmd)

	// Flags
	captureCmd.Flags().String("ai-provider", "", "Forçar provedor de IA (ollama, gemini, openai)")
	captureCmd.Flags().String("file", "", "Arquivo de texto para capturar como UKI")
}

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	projectContext "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	var req plugin.PluginRequest

	// 1. Check for Environment Variable Protocol (Preferred for Interactive/TUI)
	if envReq := os.Getenv("YBY_PLUGIN_REQUEST"); envReq != "" {
		if err := json.Unmarshal([]byte(envReq), &req); err != nil {
			fmt.Printf("Erro ao analisar YBY_PLUGIN_REQUEST: %v\n", err)
			os.Exit(1)
		}
		handlePluginRequest(req)
		return
	}

	// 2. Check for Stdin Protocol (Legacy/Automation)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data on pipe -> Plugin Request
		if err := json.NewDecoder(os.Stdin).Decode(&req); err == nil {
			handlePluginRequest(req)
			return
		}
	}

	// 3. Fallback / Dev Mode
	// Mock request for development or direct invocation without context
	handlePluginRequest(plugin.PluginRequest{Hook: "command"})
}

func handlePluginRequest(req plugin.PluginRequest) {
	switch req.Hook {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:    "bard",
			Version: "0.1.0",
			Hooks:   []string{"command"},
		})
	case "command":
		startChat(req.Context)
	default:
		// Unknown hook
		// Just exit 0 to not break anything, or error
		os.Exit(0)
	}
}

func startChat(ctxData map[string]interface{}) {
	// Initialize AI
	ctx := context.Background()
	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		fmt.Println("❌ Nenhum provedor de IA disponível. Defina OLLAMA_HOST ou OPENAI_API_KEY.")
		os.Exit(1)
	}

	// 1. Parse Context
	overview, _ := ctxData["overview"].(string)
	backlog, _ := ctxData["backlog"].(string)

	// Parse UKIs safe conversion
	var ukis []projectContext.UKIMetadata
	if v, ok := ctxData["uki_index"]; ok {
		b, _ := json.Marshal(v)
		_ = json.Unmarshal(b, &ukis)
	}

	blueprintSummary := "Nenhum blueprint disponível."
	if bp, ok := ctxData["blueprint"]; ok {
		bytes, _ := json.MarshalIndent(bp, "", "  ")
		blueprintSummary = string(bytes)
	}

	// 2. Build Rich System Prompt
	// We inject the "Mental State" of the project
	contextBlock := fmt.Sprintf(`
## Project Overview
%s

## Backlog & Debt
%s

## Technical Blueprint (Atlas)
%s
`, overview, backlog, blueprintSummary)

	systemPrompt := strings.ReplaceAll(BardSystemPrompt, "{{ blueprint_json_summary }}", contextBlock)

	// UI Setup
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("🤖 Yby Bard"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Digite 'exit' para sair."))
	if len(ukis) > 0 {
		fmt.Printf(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("📚 %d UKIs indexadas para consulta inteligente.\n"), len(ukis))
	}
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render("You > "))
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()
		if input == "exit" || input == "quit" {
			break
		}

		if input == "" {
			continue
		}

		fmt.Print(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("Bard > "))

		// 3. Smart Retrieval (UKI Selector)
		ukiContext := ""
		if len(ukis) > 0 {
			// Serialize index for LLM
			indexBytes, _ := json.Marshal(ukis)
			selectorPrompt := strings.ReplaceAll(BardUKISelectorPrompt, "{{ uki_index_json }}", string(indexBytes))
			selectorPrompt = strings.ReplaceAll(selectorPrompt, "{{ user_question }}", input)

			// Ask LLM to select
			// We use a separate context or simplified interaction?
			// Provider.Completion is synchronous
			selectionJson, err := provider.Completion(ctx, "You are a librarian.", selectorPrompt)
			if err == nil {
				// Parse response (naive: try to find JSON array)
				// Clean markdown code blocks if any
				cleanJson := strings.ReplaceAll(selectionJson, "```json", "")
				cleanJson = strings.ReplaceAll(cleanJson, "```", "")

				var selectedIDs []string
				if err := json.Unmarshal([]byte(cleanJson), &selectedIDs); err == nil && len(selectedIDs) > 0 {
					// Read files
					fmt.Printf(lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("240")).Render("\n(Lendo: %s)... "), strings.Join(selectedIDs, ", "))

					for _, id := range selectedIDs {
						for _, uki := range ukis {
							if uki.ID == id {
								content, err := os.ReadFile(uki.Filename)
								if err == nil {
									ukiContext += fmt.Sprintf("\n--- UKI: %s ---\n%s\n", uki.Title, string(content))
								}
								break
							}
						}
					}
				}
			}
		}

		// 4. Final Answer
		runInput := input
		if ukiContext != "" {
			runInput = fmt.Sprintf("Contexto Adicional da Documentação (UKIs):\n%s\n\nPergunta do Usuário: %s", ukiContext, input)
		}

		err := provider.StreamCompletion(ctx, systemPrompt, runInput, os.Stdout)
		if err != nil {
			fmt.Printf("\nError: %v\n", err)
		}
		fmt.Println() // Newline after stream
	}
}

func respond(data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}

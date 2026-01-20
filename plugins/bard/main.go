package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
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
			Name:        "bard",
			Version:     "0.1.0",
			Description: "Assistente de IA interativo para diagnóstico e operações",
			Hooks:       []string{"command"},
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

	// 1. Initialize Vector Store (Read-Only access effectively)
	cwd, _ := os.Getwd()
	storePath := filepath.Join(cwd, ".synapstor", ".index")
	// Note: We initialize store. If it doesn't exist, search will just return empty or we handle error.
	vectorStore, err := ai.NewVectorStore(ctx, storePath, provider)
	if err != nil {
		// Non-fatal, just means no long-term memory
		// But in this architecture it's critical. Let's warn.
		fmt.Printf(lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("⚠️  Aviso: Memória semântica indisponível (%v)\n"), err)
	}

	// 2. Build Base Context from Payload (still useful for Blueprint/High-level)
	overview, _ := ctxData["overview"].(string)
	backlog, _ := ctxData["backlog"].(string)

	blueprintSummary := "Nenhum blueprint disponível."
	if bp, ok := ctxData["blueprint"]; ok {
		bytes, _ := json.MarshalIndent(bp, "", "  ")
		blueprintSummary = string(bytes)
	}

	// 3. Build Rich System Prompt
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

	if vectorStore != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("🧠 Memória Semântica Ativa."))
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

		// 4. Smart Retrieval (Vector Search)
		ukiContext := ""
		if vectorStore != nil {
			// Search for top 3 relevant chunks
			results, err := vectorStore.Search(ctx, input, 3)
			if err == nil && len(results) > 0 {
				var sources []string
				var sb strings.Builder

				for _, res := range results {
					sources = append(sources, fmt.Sprintf("%s (%.2f)", res.Metadata["filename"], res.Score))
					sb.WriteString(fmt.Sprintf("\n--- Contexto: %s ---\n%s\n", res.Metadata["title"], res.Content))
				}

				ukiContext = sb.String()

				// Show sources in UI (subtle)
				fmt.Printf(lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("240")).Render("\n(Consultando: %s)... "), strings.Join(sources, ", "))
			}
		}

		// 5. Final Answer
		runInput := input
		if ukiContext != "" {
			runInput = fmt.Sprintf("Contexto Adicional Recuperado (Memória Semântica):\n%s\n\nPergunta do Usuário: %s", ukiContext, input)
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

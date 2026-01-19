package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
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

	// Prepare System Prompt
	blueprintSummary := "Nenhum contexto disponível."
	if bp, ok := ctxData["blueprint"]; ok {
		// Convert blueprint to string summary
		// This is naive, relies on fmt/json stringification
		bytes, _ := json.MarshalIndent(bp, "", "  ")
		blueprintSummary = string(bytes)
	}

	systemPrompt := strings.ReplaceAll(BardSystemPrompt, "{{ blueprint_json_summary }}", blueprintSummary)

	// UI Setup
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("🤖 Yby Bard"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Digite 'exit' para sair."))
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

		err := provider.StreamCompletion(ctx, systemPrompt, input, os.Stdout)
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

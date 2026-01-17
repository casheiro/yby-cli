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
	// 1. Check arguments. If run as "yby bard", likely initiated by CLI via Executor
	//    But "plugin" usually implies "command" hook.
	//    The Core CLI invokes binary with JSON on Stdin.
	//    However, for a "command" plugin, it might take over the TUI.

	// Check if stdin has data (Plugin Request) or is interactive
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data on pipe -> Plugin Request
		var req plugin.PluginRequest
		if err := json.NewDecoder(os.Stdin).Decode(&req); err == nil {
			handlePluginRequest(req)
			return
		}
	}

	// Falls back to direct execution (dev mode or if invoked directly)
	// mock request
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
		fmt.Println("❌ No AI provider available. Set OLLAMA_HOST or OPENAI_API_KEY.")
		os.Exit(1)
	}

	// Prepare System Prompt
	blueprintSummary := "No context available."
	if bp, ok := ctxData["blueprint"]; ok {
		// Convert blueprint to string summary
		// This is naive, relies on fmt/json stringification
		bytes, _ := json.MarshalIndent(bp, "", "  ")
		blueprintSummary = string(bytes)
	}

	systemPrompt := strings.ReplaceAll(BardSystemPrompt, "{{ blueprint_json_summary }}", blueprintSummary)

	// UI Setup
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("🤖 Yby Bard"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Type 'exit' to quit."))
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

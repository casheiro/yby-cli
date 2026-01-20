package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/agent"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/indexer"
)

func main() {
	var req plugin.PluginRequest

	// 1. Check for Environment Variable Protocol
	if envReq := os.Getenv("YBY_PLUGIN_REQUEST"); envReq != "" {
		if err := json.Unmarshal([]byte(envReq), &req); err != nil {
			fmt.Printf("Erro ao analisar YBY_PLUGIN_REQUEST: %v\n", err)
			os.Exit(1)
		}
		handlePluginRequest(req)
		return
	}

	// 2. Fallback to Stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		if err := json.NewDecoder(os.Stdin).Decode(&req); err == nil {
			handlePluginRequest(req)
			return
		}
	}

	// 3. CLI Mode (Direct execution for testing or local usage)
	if len(os.Args) > 1 {
		// Mock command execution if arguments are present directly
		cmd := os.Args[1]
		args := os.Args[2:]
		handlePluginRequest(plugin.PluginRequest{
			Hook: "command",
			Args: append([]string{cmd}, args...),
		})
		return
	}

	// Fallback Help
	printHelp()
}

func handlePluginRequest(req plugin.PluginRequest) {
	switch req.Hook {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:        "synapstor",
			Version:     "0.1.0",
			Description: "Governança semântica e gestão de conhecimento (UKIs)",
			Hooks:       []string{"command"},
		})
	case "command":
		if len(req.Args) == 0 {
			printHelp()
			return
		}

		ctx := context.Background()
		provider := ai.GetProvider(ctx, "auto")
		if provider == nil {
			fmt.Println("❌ Nenhum provedor de IA configurado.")
			return
		}

		cwd, _ := os.Getwd()
		agt := agent.NewAgent(provider, cwd)

		cmd := req.Args[0]
		switch cmd {
		case "capture":
			if len(req.Args) < 2 {
				fmt.Println("❌ Uso: yby synapstor capture \"seu texto de input\"")
				return
			}
			input := strings.Join(req.Args[1:], " ")
			if err := agt.Capture(input); err != nil {
				fmt.Printf("❌ Erro: %v\n", err)
			}
		case "study":
			if len(req.Args) < 2 {
				fmt.Println("❌ Uso: yby synapstor study \"tópico ou arquivo\"")
				return
			}
			query := strings.Join(req.Args[1:], " ")
			if err := agt.Study(query); err != nil {
				fmt.Printf("❌ Erro: %v\n", err)
			}
		case "index":
			runIndex()
		default:
			printHelp()
		}
	}
}

func runIndex() {
	ctx := context.Background()
	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		fmt.Println("❌ Nenhum provedor de IA configurado. Defina GEMINI_API_KEY, OPENAI_API_KEY ou OLLAMA_HOST.")
		return
	}

	cwd, _ := os.Getwd()
	idx := indexer.NewIndexer(provider, cwd)

	if err := idx.Run(ctx); err != nil {
		fmt.Printf("❌ Erro na indexação: %v\n", err)
		os.Exit(1)
	}
}

func respond(data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}

func printHelp() {
	fmt.Println("Synapstor Agent Commands:")
	fmt.Println("  capture [text]  - Captura e estrutura conhecimento")
	fmt.Println("  study [topic]   - Lê código e gera documentação")
	fmt.Println("  index           - Atualiza índice de busca")
}

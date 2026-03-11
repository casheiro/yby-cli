package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/plugin/sdk"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/agent"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/indexer"
)

func main() {
	// 1. Tentar inicializar via SDK (lê stdin/env var)
	if err := sdk.Init(); err != nil {
		// 2. Fallback: modo CLI direto (args na linha de comando)
		if len(os.Args) > 1 {
			cmd := os.Args[1]
			args := os.Args[2:]
			handlePluginRequest(plugin.PluginRequest{
				Hook: "command",
				Args: append([]string{cmd}, args...),
			})
			return
		}
		// Sem input via SDK nem CLI
		printHelp()
		return
	}

	// SDK inicializado com sucesso
	hook := sdk.GetHook()
	args := sdk.GetArgs()

	handlePluginRequest(plugin.PluginRequest{
		Hook: hook,
		Args: args,
	})
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
			fullReindex := false
			for _, a := range req.Args[1:] {
				if a == "--full" {
					fullReindex = true
				}
			}
			runIndex(fullReindex)
		default:
			printHelp()
		}
	}
}

func runIndex(fullReindex bool) {
	ctx := context.Background()
	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		fmt.Println("❌ Nenhum provedor de IA configurado. Defina GEMINI_API_KEY, OPENAI_API_KEY ou OLLAMA_HOST.")
		return
	}

	cwd, _ := os.Getwd()
	idx := indexer.NewIndexer(provider, cwd)
	idx.FullReindex = fullReindex

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
	fmt.Println("  index [--full]  - Atualiza índice de busca (--full força reindexação completa)")
}

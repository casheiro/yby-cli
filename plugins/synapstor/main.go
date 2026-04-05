package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
			Hooks:       []string{"command", "context"},
		})
	case "context":
		handleContextHook()
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
		case "search":
			if len(req.Args) < 2 {
				fmt.Println("❌ Uso: yby synapstor search \"sua consulta\" [--top-k N]")
				return
			}
			runSearch(req.Args[1:])
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

	report, err := idx.Run(ctx)
	if err != nil {
		fmt.Printf("❌ Erro na indexação: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Indexação concluída em %.1fs\n", report.Duration.Seconds())
	fmt.Printf("  Arquivos escaneados: %d\n", report.FilesScanned)
	fmt.Printf("  Arquivos ignorados:  %d\n", report.FilesSkipped)
	fmt.Printf("  Chunks gerados:      %d\n", report.ChunksGenerated)
	fmt.Printf("  Embeddings criados:  %d\n", report.EmbeddingsCreated)
}

// handleContextHook retorna dados de contexto do Synapstor para o sistema de plugins.
func handleContextHook() {
	cwd, _ := os.Getwd()
	manifestPath := filepath.Join(cwd, ".synapstor", ".index_manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// Manifest não existe — retornar status "not_indexed"
		respond(plugin.PluginResponse{
			Data: map[string]interface{}{
				"synapstor_indexed_files": 0,
				"synapstor_last_indexed":  "",
				"synapstor_status":        "not_indexed",
			},
		})
		return
	}

	var manifest indexer.IndexManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		respond(plugin.PluginResponse{
			Data: map[string]interface{}{
				"synapstor_indexed_files": 0,
				"synapstor_last_indexed":  "",
				"synapstor_status":        "not_indexed",
			},
		})
		return
	}

	total := len(manifest.Files)
	var lastDate time.Time
	for _, f := range manifest.Files {
		if f.IndexedAt.After(lastDate) {
			lastDate = f.IndexedAt
		}
	}

	lastDateStr := ""
	if !lastDate.IsZero() {
		lastDateStr = lastDate.Format(time.RFC3339)
	}

	respond(plugin.PluginResponse{
		Data: map[string]interface{}{
			"synapstor_indexed_files": total,
			"synapstor_last_indexed":  lastDateStr,
			"synapstor_status":        "active",
		},
	})
}

func respond(data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}

func printHelp() {
	fmt.Println("Synapstor Agent Commands:")
	fmt.Println("  capture [text]        - Captura e estrutura conhecimento")
	fmt.Println("  study [topic]         - Lê código e gera documentação")
	fmt.Println("  search [query]        - Busca semântica no índice de conhecimento [--top-k N]")
	fmt.Println("  index [--full]        - Atualiza índice de busca (--full força reindexação completa)")
}

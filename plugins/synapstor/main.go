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
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/bridge"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/decay"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/exporter"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/graph"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/indexer"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/quality"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/tagger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Tentar inicializar via SDK (lê stdin/env var)
	if err := sdk.Init(); err != nil {
		// 2. Fallback: modo CLI direto (args na linha de comando)
		if len(os.Args) > 1 {
			cmd := os.Args[1]
			args := os.Args[2:]
			return handlePluginRequest(plugin.PluginRequest{
				Hook: "command",
				Args: append([]string{cmd}, args...),
			})
		}
		// Sem input via SDK nem CLI
		printHelp()
		return nil
	}

	// SDK inicializado com sucesso
	hook := sdk.GetHook()
	args := sdk.GetArgs()

	return handlePluginRequest(plugin.PluginRequest{
		Hook: hook,
		Args: args,
	})
}

func handlePluginRequest(req plugin.PluginRequest) error {
	switch req.Hook {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:        "synapstor",
			Version:     "0.1.0",
			Description: "Governança semântica e gestão de conhecimento (UKIs)",
			Hooks:       []string{"command", "context"},
		})
		return nil
	case "context":
		handleContextHook()
		return nil
	case "command":
		if len(req.Args) == 0 {
			printHelp()
			return nil
		}

		ctx := context.Background()
		provider := ai.GetProvider(ctx, "auto")
		if provider == nil {
			return fmt.Errorf("nenhum provedor de IA configurado")
		}

		cwd, _ := os.Getwd()
		agt := agent.NewAgent(provider, cwd)

		cmd := req.Args[0]
		switch cmd {
		case "capture":
			if len(req.Args) < 2 {
				return fmt.Errorf("uso: yby synapstor capture \"seu texto de input\"")
			}
			input := strings.Join(req.Args[1:], " ")
			return agt.Capture(input)
		case "study":
			if len(req.Args) < 2 {
				return fmt.Errorf("uso: yby synapstor study \"tópico ou arquivo\"")
			}
			query := strings.Join(req.Args[1:], " ")
			return agt.Study(query)
		case "search":
			if len(req.Args) < 2 {
				return fmt.Errorf("uso: yby synapstor search \"sua consulta\" [--top-k N]")
			}
			runSearch(req.Args[1:])
			return nil
		case "index":
			fullReindex := false
			for _, a := range req.Args[1:] {
				if a == "--full" {
					fullReindex = true
				}
			}
			return runIndex(fullReindex)
		case "graph":
			return runGraph()
		case "quality":
			return runQuality()
		case "decay":
			return runDecay()
		case "tag":
			return runTag(ctx, provider)
		case "export":
			return runExport(req.Args[1:])
		case "sync-atlas":
			return runSyncAtlas()
		default:
			printHelp()
			return nil
		}
	}
	return nil
}

func runIndex(fullReindex bool) error {
	ctx := context.Background()
	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		return fmt.Errorf("nenhum provedor de IA configurado. Defina GEMINI_API_KEY, OPENAI_API_KEY ou OLLAMA_HOST")
	}

	cwd, _ := os.Getwd()
	idx := indexer.NewIndexer(provider, cwd)
	idx.FullReindex = fullReindex

	report, err := idx.Run(ctx)
	if err != nil {
		return fmt.Errorf("erro na indexação: %w", err)
	}

	fmt.Printf("Indexação concluída em %.1fs\n", report.Duration.Seconds())
	fmt.Printf("  Arquivos escaneados: %d\n", report.FilesScanned)
	fmt.Printf("  Arquivos ignorados:  %d\n", report.FilesSkipped)
	fmt.Printf("  Chunks gerados:      %d\n", report.ChunksGenerated)
	fmt.Printf("  Embeddings criados:  %d\n", report.EmbeddingsCreated)
	return nil
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

func runGraph() error {
	cwd, _ := os.Getwd()
	ukiDir := filepath.Join(cwd, ".synapstor", ".uki")
	graphPath := filepath.Join(cwd, ".synapstor", ".knowledge_graph.json")

	kg, err := graph.BuildGraph(ukiDir)
	if err != nil {
		return fmt.Errorf("erro ao construir knowledge graph: %w", err)
	}

	if err := graph.SaveGraph(kg, graphPath); err != nil {
		return fmt.Errorf("erro ao salvar knowledge graph: %w", err)
	}

	fmt.Printf("Knowledge Graph construído: %d nós, %d arestas\n", len(kg.Nodes), len(kg.Edges))
	fmt.Printf("Salvo em: %s\n", graphPath)
	return nil
}

func runQuality() error {
	cwd, _ := os.Getwd()
	ukiDir := filepath.Join(cwd, ".synapstor", ".uki")

	scores, err := quality.ScoreAll(ukiDir)
	if err != nil {
		return fmt.Errorf("erro ao avaliar qualidade: %w", err)
	}

	if len(scores) == 0 {
		fmt.Println("Nenhum UKI encontrado.")
		return nil
	}

	fmt.Println("Avaliação de Qualidade dos UKIs:")
	fmt.Println()
	for _, s := range scores {
		fmt.Println(quality.FormatScore(s))
	}
	return nil
}

func runDecay() error {
	cwd, _ := os.Getwd()
	ukiDir := filepath.Join(cwd, ".synapstor", ".uki")

	infos, err := decay.AnalyzeDecay(ukiDir, cwd)
	if err != nil {
		return fmt.Errorf("erro ao analisar decay: %w", err)
	}

	if len(infos) == 0 {
		fmt.Println("Nenhum UKI encontrado.")
		return nil
	}

	stale := decay.FindStale(infos)
	fmt.Printf("Análise de Decay: %d UKIs analisados, %d stale (>%d dias)\n", len(infos), len(stale), decay.StaleThresholdDays)
	fmt.Println()

	for _, info := range infos {
		status := "ativo"
		if info.IsStale {
			status = "STALE"
		}
		fmt.Printf("[%s] %s (%d dias sem atividade)\n", status, info.Title, info.DaysSinceActivity)
	}
	return nil
}

func runTag(ctx context.Context, provider ai.Provider) error {
	cwd, _ := os.Getwd()
	ukiDir := filepath.Join(cwd, ".synapstor", ".uki")

	results, err := tagger.TagAll(ctx, provider, ukiDir)
	if err != nil {
		return fmt.Errorf("erro ao tagear UKIs: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("Nenhum UKI encontrado para tagear.")
		return nil
	}

	fmt.Printf("Auto-tagging concluído: %d UKIs processados\n", len(results))
	fmt.Println()
	for _, r := range results {
		fmt.Printf("%s: %s\n", filepath.Base(r.Path), strings.Join(r.Tags, ", "))
	}
	return nil
}

func runExport(args []string) error {
	format := "markdown"
	outputDir := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "--output":
			if i+1 < len(args) {
				outputDir = args[i+1]
				i++
			}
		}
	}

	cwd, _ := os.Getwd()
	ukiDir := filepath.Join(cwd, ".synapstor", ".uki")

	if outputDir == "" {
		outputDir = filepath.Join(cwd, ".synapstor", "export", format)
	}

	ukis, err := exporter.LoadUKIs(ukiDir)
	if err != nil {
		return fmt.Errorf("erro ao carregar UKIs: %w", err)
	}

	exp, err := exporter.NewExporter(format)
	if err != nil {
		return err
	}

	if err := exp.Export(ukis, outputDir); err != nil {
		return fmt.Errorf("erro na exportação: %w", err)
	}

	fmt.Printf("Exportação %s concluída: %d UKIs exportados para %s\n", format, len(ukis), outputDir)
	return nil
}

func runSyncAtlas() error {
	cwd, _ := os.Getwd()
	snapshotPath := filepath.Join(cwd, ".yby", "atlas-snapshot.json")
	ukiDir := filepath.Join(cwd, ".synapstor", ".uki")
	graphPath := filepath.Join(cwd, ".synapstor", ".knowledge_graph.json")

	// Carregar ou criar knowledge graph
	kg, err := graph.LoadGraph(graphPath)
	if err != nil {
		kg = &graph.KnowledgeGraph{}
	}

	report, err := bridge.SyncFromAtlasWithGraph(snapshotPath, ukiDir, kg)
	if err != nil {
		return fmt.Errorf("erro na sincronização Atlas: %w", err)
	}

	// Salvar knowledge graph atualizado
	if err := graph.SaveGraph(kg, graphPath); err != nil {
		return fmt.Errorf("erro ao salvar knowledge graph: %w", err)
	}

	fmt.Printf("Sincronização Atlas → Synapstor concluída:\n")
	fmt.Printf("  Novos UKIs:   %d\n", report.NewUKIs)
	fmt.Printf("  Existentes:   %d\n", report.SkippedExisting)
	fmt.Printf("  Erros:        %d\n", report.Errors)
	return nil
}

func printHelp() {
	fmt.Println("Synapstor Agent Commands:")
	fmt.Println("  capture [text]                              - Captura e estrutura conhecimento")
	fmt.Println("  study [topic]                               - Lê código e gera documentação")
	fmt.Println("  search [query]                              - Busca semântica no índice [--top-k N]")
	fmt.Println("  index [--full]                              - Atualiza índice de busca")
	fmt.Println("  graph                                       - Constrói knowledge graph entre UKIs")
	fmt.Println("  quality                                     - Avalia qualidade da documentação")
	fmt.Println("  decay                                       - Detecta documentação obsoleta")
	fmt.Println("  tag                                         - Auto-tagging via IA")
	fmt.Println("  export --format <docusaurus|obsidian|md>    - Exporta UKIs")
	fmt.Println("  sync-atlas                                  - Sincroniza componentes do Atlas")
}

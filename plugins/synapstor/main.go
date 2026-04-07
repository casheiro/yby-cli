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
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/decay"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/exporter"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/indexer"
	"github.com/casheiro/yby-cli/plugins/synapstor/internal/quality"
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
			Version:     "1.0.0",
			Description: "Gestao de conhecimento com knowledge graph, quality scoring e export multi-formato",
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
		if cmd == "--help" || cmd == "-h" || cmd == "help" {
			printHelp()
			return nil
		}
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
		case "quality":
			return runQuality()
		case "decay":
			return runDecay()
		case "export":
			return runExport(req.Args[1:])
		default:
			printHelp()
			return nil
		}
	}
	return nil
}

func runIndex(fullReindex bool) error {
	ctx := context.Background()
	provider := ai.GetEmbeddingProvider(ctx)
	if provider == nil {
		return fmt.Errorf("nenhum provedor com suporte a embeddings disponivel (ollama, gemini, openai)")
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

func printHelp() {
	fmt.Println("Synapstor - Gestao de conhecimento do projeto")
	fmt.Println()
	fmt.Println("Uso: yby synapstor <subcomando> [opcoes]")
	fmt.Println()
	fmt.Println("Subcomandos:")
	fmt.Println("  capture \"texto\"        Captura e estrutura conhecimento via IA")
	fmt.Println("  study \"topico\"         Analisa codigo e gera documentacao via IA")
	fmt.Println("  search \"query\"         Busca semantica nos UKIs indexados [--top-k N]")
	fmt.Println("  index [--full]         Indexa UKIs com embeddings (incremental)")
	fmt.Println("  quality                Avalia qualidade dos UKIs (score 0-100)")
	fmt.Println("  decay                  Detecta UKIs desatualizados (>90 dias)")
	fmt.Println("  export                 Exporta UKIs (--format docusaurus|obsidian|markdown --output dir)")
}

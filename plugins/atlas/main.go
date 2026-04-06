package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/plugins/atlas/analysis"
	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var req plugin.PluginRequest

	// 1. Verificar protocolo via variável de ambiente
	if envReq := os.Getenv("YBY_PLUGIN_REQUEST"); envReq != "" {
		if err := json.Unmarshal([]byte(envReq), &req); err != nil {
			return fail(fmt.Errorf("falha ao decodificar requisição do env: %w", err))
		}
	} else {
		// 2. Fallback para stdin
		if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
			return fail(fmt.Errorf("falha ao decodificar requisição do stdin: %w", err))
		}
	}

	switch req.Hook {
	case "manifest":
		return respond(plugin.PluginManifest{
			Name:        "atlas",
			Version:     "0.2.0",
			Description: "Mapeamento contínuo de recursos e blueprint do cluster",
			Hooks:       []string{"context", "manifest", "command"},
		})
	case "context":
		blueprint, err := scanProject()
		if err != nil {
			return fail(err)
		}
		return respond(map[string]interface{}{
			"blueprint": blueprint,
		})
	case "command":
		return handleCommand(req.Args)
	default:
		return fail(fmt.Errorf("hook desconhecido: %s", req.Hook))
	}
}

// loadConfig carrega a configuração externa do Atlas a partir de .yby/atlas.yaml.
// Retorna nil se o arquivo não existir ou não puder ser lido.
func loadConfig() *discovery.AtlasConfig {
	configPath := filepath.Join(".yby", "atlas.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil // Arquivo não existe, usar defaults
	}
	var cfg discovery.AtlasConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "aviso: erro ao ler .yby/atlas.yaml: %v\n", err)
		return nil
	}
	return &cfg
}

func respond(data interface{}) error {
	resp := plugin.PluginResponse{Data: data}
	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		return fmt.Errorf("falha ao codificar resposta: %w", err)
	}
	return nil
}

func fail(err error) error {
	resp := plugin.PluginResponse{Error: err.Error()}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
	return err
}

// scanProject executa a descoberta de componentes no diretório atual,
// aplicando configuração externa quando disponível.
func scanProject() (*discovery.Blueprint, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cfg := loadConfig()
	ignores := []string{"node_modules", "vendor", ".git", ".idea", ".vscode"}
	var rules []discovery.Rule
	if cfg != nil {
		if len(cfg.Ignores) > 0 {
			ignores = append(ignores, cfg.Ignores...)
		}
		rules = discovery.MergeRules(cfg.Rules)
	} else {
		rules = discovery.DefaultRules
	}

	return discovery.ScanWithRules(cwd, ignores, rules)
}

// handleCommand roteia subcomandos do hook "command".
func handleCommand(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	subcommand := args[0]
	if subcommand == "--help" || subcommand == "-h" || subcommand == "help" {
		printUsage()
		return nil
	}

	subArgs := args[1:]

	switch subcommand {
	case "diagram":
		return handleDiagram(subArgs)
	default:
		return fail(fmt.Errorf("subcomando desconhecido: %s (disponíveis: diagram)", subcommand))
	}
}

// printUsage exibe a ajuda do Atlas.
func printUsage() {
	fmt.Println("Atlas - Scanner de topologia de infraestrutura")
	fmt.Println()
	fmt.Println("Uso: yby atlas <subcomando> [opcoes]")
	fmt.Println()
	fmt.Println("Subcomandos:")
	fmt.Println("  diagram [flags]       Gera diagrama da topologia de infraestrutura")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --detail full         Mostra todos os recursos (RBAC, CRDs, ConfigMaps...)")
	fmt.Println("  --no-ai               Desabilita refinamento com IA")
	fmt.Println()
	fmt.Println("Exemplos:")
	fmt.Println("  yby atlas diagram                 Topologia principal (com IA se disponivel)")
	fmt.Println("  yby atlas diagram --detail full    Todos os recursos")
	fmt.Println("  yby atlas diagram --no-ai          Sem refinamento IA")
}

// handleDiagram gera diagrama da topologia de infraestrutura e salva em arquivo.
func handleDiagram(args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fail(err)
	}

	// Parsear flags
	detail := analysis.DetailOverview
	useAI := true
	for i, arg := range args {
		switch arg {
		case "--detail", "-d":
			if i+1 < len(args) && args[i+1] == "full" {
				detail = analysis.DetailFull
			}
		case "full":
			// capturado pelo case anterior
		case "--no-ai":
			useAI = false
		}
		if strings.HasPrefix(arg, "--detail=") {
			if strings.TrimPrefix(arg, "--detail=") == "full" {
				detail = analysis.DetailFull
			}
		}
	}

	ignores := []string{"node_modules", "vendor", ".git", ".idea", ".vscode"}
	cfg := loadConfig()
	if cfg != nil && len(cfg.Ignores) > 0 {
		ignores = append(ignores, cfg.Ignores...)
	}

	infraBP, err := discovery.ScanInfra(cwd, ignores)
	if err != nil {
		return fail(err)
	}

	if len(infraBP.Resources) == 0 {
		fmt.Println("Nenhuma topologia de infraestrutura identificada neste projeto.")
		fmt.Println()
		fmt.Println("O Atlas procura por: Helm charts, manifests K8s, docker-compose,")
		fmt.Println("kustomization.yaml e arquivos Terraform (.tf).")
		return nil
	}

	diagram := analysis.GenerateInfraMermaid(infraBP, detail)
	if diagram == "" {
		fmt.Println("Nenhum recurso de topologia encontrado para o nivel de detalhe selecionado.")
		return nil
	}

	// Tentar refinar com IA (se disponível e habilitado)
	refined := false
	aiSource := ""
	if useAI {
		ctx := context.Background()

		fmt.Println("Refinando diagrama com IA...")
		summary := buildResourceSummary(infraBP)
		refinedDiagram, source, refineErr := refineDiagramWithAI(ctx, diagram, summary)
		if refineErr == nil && refinedDiagram != "" {
			diagram = refinedDiagram
			refined = true
			aiSource = source
		} else if refineErr != nil {
			fmt.Fprintf(os.Stderr, "aviso: refinamento IA indisponivel, usando diagrama programatico: %v\n", refineErr)
		}
	}

	filename := "atlas-diagram.mmd"

	// Salvar arquivo
	outputDir := filepath.Join(".yby")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fail(fmt.Errorf("falha ao criar diretorio .yby: %w", err))
	}

	outputPath := filepath.Join(outputDir, filename)
	if err := os.WriteFile(outputPath, []byte(diagram), 0644); err != nil {
		return fail(fmt.Errorf("falha ao salvar diagrama: %w", err))
	}

	stats := analysis.GetInfraStats(infraBP, detail)
	fmt.Printf("Diagrama gerado com sucesso: %s\n", outputPath)
	fmt.Printf("Recursos: %d/%d (visíveis/total) | Relacoes: %d | Analyzers: %s\n",
		stats.VisibleResources, stats.TotalResources, stats.Relations,
		strings.Join(stats.Analyzers, ", "))
	if refined {
		fmt.Printf("Refinado com IA (%s)\n", aiSource)
	}
	if detail == analysis.DetailOverview {
		fmt.Println("Nivel: overview (use --detail full para todos os recursos)")
	}
	fmt.Println()
	fmt.Println("Para visualizar, abra em: https://mermaid.live ou extensao Mermaid do VS Code")
	return nil
}

// refineSystemPrompt é o prompt do sistema para refinamento de diagramas.
const refineSystemPrompt = `Voce e um especialista em infraestrutura Kubernetes e diagramas Mermaid.

Voce vai receber um diagrama Mermaid rascunho e um inventario de recursos. Sua tarefa e produzir um diagrama MACRO — uma visao de alto nivel da topologia que caiba confortavelmente numa tela.

OBJETIVO: alguem olha o diagrama e em 5 segundos entende a arquitetura da infraestrutura.

REGRAS:
1. SIMPLIFIQUE AGRESSIVAMENTE — mostre no maximo 15-25 nos no total
2. Agrupe recursos similares em um unico no (ex: "8 ServiceAccounts" vira um no, nao 8)
3. RBAC, ConfigMaps, Secrets, CRDs, Namespaces NAO devem aparecer como nos individuais — se relevantes, mencione dentro do label do grupo pai
4. Foque em: Charts, Applications, Deployments/Workloads principais, Ingresses de acesso externo, e dependencias externas
5. Use subgraphs por dominio funcional (ex: "Banco de Dados", "Observabilidade", "Aplicacao")
6. Cada no: id["Nome Curto"]
7. Edges: -->|verbo| (implanta, depende de, sincroniza, expoe)
8. NAO invente nos ou relacoes que nao existem no inventario
9. NAO inclua markdown code fences — retorne APENAS codigo Mermaid puro comecando com "flowchart TD"
10. MENOS E MAIS — na duvida, omita`

// buildResourceSummary cria um resumo compacto dos recursos para o prompt da IA.
func buildResourceSummary(bp *discovery.InfraBlueprint) string {
	var b strings.Builder
	b.WriteString("INVENTARIO DE RECURSOS DESCOBERTOS:\n\n")

	// Agrupar por kind
	byKind := make(map[string][]string)
	for _, r := range bp.Resources {
		label := r.Name
		if r.Namespace != "" {
			label = r.Namespace + "/" + r.Name
		}
		byKind[r.Kind] = append(byKind[r.Kind], label)
	}

	// Ordenar kinds
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	for _, kind := range kinds {
		names := byKind[kind]
		b.WriteString(fmt.Sprintf("%s (%d): %s\n", kind, len(names), strings.Join(names, ", ")))
	}

	b.WriteString(fmt.Sprintf("\nRELACOES (%d total):\n", len(bp.Relations)))
	seen := make(map[string]bool)
	for _, rel := range bp.Relations {
		key := fmt.Sprintf("%s --%s--> %s", rel.From, rel.Type, rel.To)
		if seen[key] {
			continue
		}
		seen[key] = true
		b.WriteString(fmt.Sprintf("  %s\n", key))
	}

	return b.String()
}

// refineDiagramWithAI tenta refinar o diagrama usando múltiplos providers em cascata.
// Tenta cada provider disponível até um funcionar. Se todos falharem, retorna erro.
func refineDiagramWithAI(ctx context.Context, mermaidDraft, resourceSummary string) (string, string, error) {
	userPrompt := fmt.Sprintf("DIAGRAMA RASCUNHO:\n\n%s\n\n%s\n\nProduza o diagrama Mermaid refinado:", mermaidDraft, resourceSummary)

	// Montar lista de providers para tentar em ordem
	providers := ai.GetAllAvailableProviders(ctx)
	if len(providers) == 0 {
		return "", "", fmt.Errorf("nenhum provider de IA disponivel")
	}

	var lastErr error
	for _, provider := range providers {
		result, err := provider.Completion(ctx, refineSystemPrompt, userPrompt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "aviso: %s falhou, tentando proximo provider: %v\n", provider.Name(), err)
			lastErr = err
			continue
		}

		refined := extractMermaidFromResponse(result)
		if refined == "" || !strings.Contains(refined, "flowchart") {
			fmt.Fprintf(os.Stderr, "aviso: %s retornou resposta invalida, tentando proximo provider\n", provider.Name())
			lastErr = fmt.Errorf("resposta de %s nao contem Mermaid valido", provider.Name())
			continue
		}

		return refined, provider.Name(), nil
	}

	return "", "", fmt.Errorf("todos os providers falharam, ultimo erro: %v", lastErr)
}

// extractMermaidFromResponse extrai o código Mermaid de uma resposta que pode conter markdown.
func extractMermaidFromResponse(response string) string {
	response = strings.TrimSpace(response)

	// Se começa com flowchart, já é Mermaid puro
	if strings.HasPrefix(response, "flowchart") {
		return response
	}

	// Tentar extrair de bloco ```mermaid ... ``` ou ``` ... ```
	markers := []string{"```mermaid\n", "```mermaid\r\n", "```\n", "```\r\n"}
	for _, marker := range markers {
		if idx := strings.Index(response, marker); idx >= 0 {
			start := idx + len(marker)
			end := strings.Index(response[start:], "```")
			if end >= 0 {
				return strings.TrimSpace(response[start : start+end])
			}
		}
	}

	// Fallback: retornar como está
	return response
}

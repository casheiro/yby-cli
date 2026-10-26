package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/atlas/analysis"
	"github.com/casheiro/yby-cli/plugins/atlas/discovery"
)

// provedoresDeIA lista os providers de IA para o refinamento, em cascata (trocado por
// falsos nos testes; nunca há IA de verdade neles).
var provedoresDeIA = ai.GetAllAvailableProviders

func main() {
	os.Exit(executar(os.Stdout, os.Stderr))
}

// executar roda o plugin e devolve o código de saída. Os hooks manifest e context
// respondem JSON no stdout (com o erro no campo error); o command escreve para pessoas,
// com os erros no formato de ui.WriteError no stderr, e o core só repassa o código.
func executar(saida, erros io.Writer) int {
	var req plugin.PluginRequest

	// 1. Verificar protocolo via ambiente (variável ou arquivo do pedido)
	envReq, err := plugin.PedidoDoAmbiente(os.Getenv)
	if err != nil {
		return falhaJSON(saida, erros, err)
	}
	if envReq != nil {
		if err := json.Unmarshal(envReq, &req); err != nil {
			return falhaJSON(saida, erros, fmt.Errorf("falha ao decodificar requisição do env: %w", err))
		}
	} else {
		// 2. Fallback para stdin
		if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
			return falhaJSON(saida, erros, fmt.Errorf("falha ao decodificar requisição do stdin: %w", err))
		}
	}

	switch req.Hook {
	case "manifest":
		if err := respond(saida, plugin.PluginManifest{
			Name:        "atlas",
			Version:     "1.0.0",
			Description: "Mapa da topologia da infraestrutura em diagramas Mermaid, refinados pela IA",
			Hooks:       []string{"context", "manifest", "command"},
		}); err != nil {
			return falhaJSON(saida, erros, err)
		}
		return 0
	case "context":
		blueprint, err := scanProject()
		if err != nil {
			return falhaJSON(saida, erros, err)
		}
		if err := respond(saida, map[string]interface{}{
			"blueprint": blueprint,
		}); err != nil {
			return falhaJSON(saida, erros, err)
		}
		return 0
	case "command":
		// Log e modo visual (tema, ASCII, sem cor) antes de qualquer saída.
		configurarLog()
		configurarModoVisual(os.Getenv)
		if err := handleCommand(req.Args, saida, erros); err != nil {
			return sairComErro(erros, err)
		}
		return 0
	default:
		return falhaJSON(saida, erros, fmt.Errorf("hook desconhecido: %s", req.Hook))
	}
}

// dicaDoLogDoPlugin é o jeito de ver o log no plugin: o --log-level do core não chega a
// este processo, e o YBY_LOG_LEVEL chega (configurarLog o aplica).
const dicaDoLogDoPlugin = "rode de novo com YBY_LOG_LEVEL=debug"

// sairComErro escreve err em w como ui.WriteError (um erro comum vira ERR_PLUGIN),
// depois de uma linha em branco que o separa do que já saiu, e devolve o código de saída.
// As dicas de depuração que mandam usar o --log-level viram a do YBY_LOG_LEVEL.
func sairComErro(w io.Writer, err error) int {
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) {
		ye = yerrors.Wrap(err, yerrors.ErrCodePlugin, "o Atlas falhou")
	}
	ui.DefinirDicaDoLog(dicaDoLogDoPlugin)
	if strings.Contains(ye.GetHint(), "--log-level") {
		ye = ye.WithHint(dicaDoLogDoPlugin + " para ver os detalhes")
	}
	fmt.Fprintln(w)
	ui.WriteError(w, ye)
	return 1
}

// lerConfig lê a configuração externa do Atlas em .yby/atlas.yaml. Sem o arquivo, devolve
// nil sem erro (valem os padrões); com o arquivo ilegível, devolve nil e o erro.
func lerConfig() (*discovery.AtlasConfig, error) {
	configPath := filepath.Join(".yby", "atlas.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil // Arquivo não existe, usar defaults
	}
	var cfg discovery.AtlasConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// loadConfig carrega a configuração externa do Atlas a partir de .yby/atlas.yaml.
// Retorna nil se o arquivo não existir ou não puder ser lido (o erro vai para o log).
func loadConfig() *discovery.AtlasConfig {
	cfg, err := lerConfig()
	if err != nil {
		slog.Warn("não deu para ler .yby/atlas.yaml; seguindo com o padrão", "erro", err)
	}
	return cfg
}

func respond(w io.Writer, data interface{}) error {
	resp := plugin.PluginResponse{Data: data}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return fmt.Errorf("falha ao codificar resposta: %w", err)
	}
	return nil
}

// falhaJSON responde o erro no campo error do JSON (para o core, nos hooks de máquina),
// repete-o no stderr e devolve o código de saída 1.
func falhaJSON(saida, erros io.Writer, err error) int {
	resp := plugin.PluginResponse{Error: err.Error()}
	_ = json.NewEncoder(saida).Encode(resp)
	fmt.Fprintf(erros, "erro: %v\n", err)
	return 1
}

// ignoradosPadrao são as pastas que o Atlas nunca percorre.
var ignoradosPadrao = []string{"node_modules", "vendor", ".git", ".idea", ".vscode"}

// scanProject executa a descoberta de componentes no diretório atual,
// aplicando configuração externa quando disponível.
func scanProject() (*discovery.Blueprint, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cfg := loadConfig()
	ignores := append([]string(nil), ignoradosPadrao...)
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
func handleCommand(args []string, saida, erros io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		escreverAjuda(saida, "")
		return nil
	}

	subcommand, subArgs := args[0], args[1:]
	switch subcommand {
	case "diagram":
		if pedeAjuda(subArgs) {
			escreverAjuda(saida, subcommand)
			return nil
		}
		return handleDiagram(subArgs, saida, erros)
	default:
		return yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("o Atlas não tem o comando %q", subcommand)).
			WithHint("rode yby atlas --help para ver os comandos")
	}
}

// opcoesDiagrama são as flags de yby atlas diagram.
type opcoesDiagrama struct {
	detail analysis.DetailLevel
	useAI  bool
}

// lerOpcoesDiagrama lê as flags do diagram: --detail (ou -d) overview|full, na forma
// "--detail full" ou "--detail=full", e --no-ai.
func lerOpcoesDiagrama(args []string) (opcoesDiagrama, error) {
	op := opcoesDiagrama{detail: analysis.DetailOverview, useAI: true}
	ajuda := "rode yby atlas diagram --help para ver as flags"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--no-ai":
			op.useAI = false
		case a == "--detail" || a == "-d" || strings.HasPrefix(a, "--detail="):
			valor, ok := strings.CutPrefix(a, "--detail=")
			if !ok {
				if i+1 >= len(args) {
					return op, yerrors.New(yerrors.ErrCodeValidation, "falta o valor de "+a).WithHint(ajuda)
				}
				valor = args[i+1]
				i++
			}
			switch analysis.DetailLevel(valor) {
			case analysis.DetailOverview, analysis.DetailFull:
				op.detail = analysis.DetailLevel(valor)
			default:
				return op, yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("nível de detalhe desconhecido: %s", valor)).
					WithHint("use --detail overview ou --detail full")
			}
		default:
			return op, yerrors.New(yerrors.ErrCodeValidation, fmt.Sprintf("o diagram não reconhece %s", a)).WithHint(ajuda)
		}
	}
	return op, nil
}

// arquivoDoDiagrama é onde o diagrama é gravado, relativo à pasta do projeto.
var arquivoDoDiagrama = filepath.Join(".yby", "atlas-diagram.mmd")

// handleDiagram gera diagrama da topologia de infraestrutura e salva em arquivo.
func handleDiagram(args []string, saida, erros io.Writer) error {
	op, err := lerOpcoesDiagrama(args)
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para ler a pasta atual")
	}

	ui.Title(saida, "atlas", "diagram · "+string(op.detail))
	fmt.Fprintln(saida)
	prog := novoProgresso(saida)
	parar := animar(prog, erros)
	defer parar()

	prog.Iniciar("procurar a infraestrutura")
	ignores := append([]string(nil), ignoradosPadrao...)
	cfg, errCfg := lerConfig()
	if cfg != nil && len(cfg.Ignores) > 0 {
		ignores = append(ignores, cfg.Ignores...)
	}
	infraBP, err := discovery.ScanInfra(cwd, ignores)
	if err != nil {
		prog.Falhar("")
		return yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para ler os arquivos do projeto").
			WithContext("pasta", cwd)
	}
	if len(infraBP.Resources) == 0 {
		prog.Concluir("nenhum recurso")
	} else {
		prog.Concluir(plural(len(infraBP.Resources), "recurso", "recursos"))
	}
	if errCfg != nil {
		prog.Avisar("não deu para ler .yby/atlas.yaml; segui com o padrão")
		slog.Debug("não deu para ler .yby/atlas.yaml", "erro", errCfg)
	}

	if len(infraBP.Resources) == 0 {
		parar()
		fmt.Fprintln(saida)
		ui.Check(saida, ui.CheckNeutro, "nenhuma topologia de infraestrutura neste projeto", "", "")
		ui.Campos(saida, ui.Campo{Rotulo: "o Atlas procura", Valor: "charts do Helm, manifests do Kubernetes, docker-compose, kustomization.yaml e Terraform (.tf)"})
		return nil
	}

	prog.Iniciar("montar o diagrama")
	diagram := analysis.GenerateInfraMermaid(infraBP, op.detail)
	stats := analysis.GetInfraStats(infraBP, op.detail)
	if diagram == "" {
		prog.Concluir("nada no nível " + string(op.detail))
		parar()
		fmt.Fprintln(saida)
		ui.Check(saida, ui.CheckNeutro, "nenhum recurso de topologia no nível "+string(op.detail), "", "")
		ui.Section(saida, "próximo passo")
		ui.Passos(saida, "yby atlas diagram --detail full mostra todos os recursos")
		return nil
	}
	prog.Concluir(fmt.Sprintf("%s · %s", plural(stats.VisibleResources, "nó", "nós"), plural(stats.Relations, "relação", "relações")))

	// Tentar refinar com IA (se disponível e habilitado)
	prog.Iniciar("refinar com a IA")
	if !op.useAI {
		prog.Pular("desligado com --no-ai")
	} else {
		summary := buildResourceSummary(infraBP)
		refinedDiagram, source, refineErr := refineDiagramWithAI(context.Background(), diagram, summary)
		switch {
		case refineErr == nil && refinedDiagram != "":
			diagram = refinedDiagram
			prog.Concluir(source)
		case errors.Is(refineErr, errSemIA):
			prog.Pular("sem IA disponível")
		case errors.Is(refineErr, errSemDiagrama):
			// A IA respondeu, mas sem um Mermaid: o motivo vai na etapa, e a resposta, no
			// log de depuração.
			slog.Debug("a IA respondeu sem diagrama; ficou o programático", "erro", refineErr)
			prog.Pular("a resposta veio sem diagrama")
			prog.Detalhe("ficou o diagrama montado a partir dos arquivos")
		default:
			slog.Debug("a IA falhou; ficou o diagrama programático", "erro", refineErr)
			prog.Pular("a IA não respondeu")
			prog.Detalhe("ficou o diagrama montado a partir dos arquivos")
		}
	}

	// Salvar arquivo
	prog.Iniciar("gravar o diagrama")
	if err := os.MkdirAll(filepath.Dir(arquivoDoDiagrama), 0755); err != nil {
		prog.Falhar("")
		return yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para criar a pasta .yby").
			WithContext("pasta", filepath.Join(cwd, ".yby"))
	}
	if err := os.WriteFile(arquivoDoDiagrama, []byte(diagram), 0644); err != nil {
		prog.Falhar("")
		return yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para gravar o diagrama").
			WithContext("arquivo", filepath.Join(cwd, arquivoDoDiagrama))
	}
	prog.Concluir("")
	parar()

	nivel := "full"
	if op.detail == analysis.DetailOverview {
		nivel = "overview · --detail full mostra todos os recursos"
	}
	ui.Section(saida, "diagrama")
	ui.Campos(saida,
		ui.Campo{Rotulo: "arquivo", Valor: filepath.ToSlash(arquivoDoDiagrama), Caminho: true},
		ui.Campo{Rotulo: "recursos", Valor: fmt.Sprintf("%d de %d no diagrama", stats.VisibleResources, stats.TotalResources)},
		ui.Campo{Rotulo: "relações", Valor: fmt.Sprintf("%d", stats.Relations)},
		ui.Campo{Rotulo: "analisadores", Valor: strings.Join(stats.Analyzers, ", ")},
		ui.Campo{Rotulo: "nível", Valor: nivel},
	)
	ui.Section(saida, "para ver")
	ui.Passos(saida, "abra o arquivo em https://mermaid.live ou na extensão Mermaid do VS Code")
	return nil
}

func plural(n int, singular, plur string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plur)
}

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

// Por que o refinamento não trouxe diagrama: nenhuma IA disponível, ou alguma respondeu,
// mas sem um Mermaid. Uma falha de todos os providers (fora do ar, cota) é outro erro.
var (
	errSemIA       = errors.New("nenhum provider de IA disponível")
	errSemDiagrama = errors.New("a resposta da IA não trouxe um diagrama Mermaid")
)

// refineDiagramWithAI tenta refinar o diagrama usando múltiplos providers em cascata.
// Tenta cada provider disponível até um funcionar. Se nenhum trouxer diagrama, retorna
// errSemIA (não há provider), errSemDiagrama (algum respondeu sem Mermaid) ou o último
// erro dos providers.
func refineDiagramWithAI(ctx context.Context, mermaidDraft, resourceSummary string) (string, string, error) {
	userPrompt := fmt.Sprintf("DIAGRAMA RASCUNHO:\n\n%s\n\n%s\n\nProduza o diagrama Mermaid refinado:", mermaidDraft, resourceSummary)

	// Montar lista de providers para tentar em ordem
	providers := provedoresDeIA(ctx)
	if len(providers) == 0 {
		return "", "", errSemIA
	}

	var lastErr error
	respondeuSemDiagrama := false
	for _, provider := range providers {
		result, err := provider.Completion(ctx, prompts.Get("atlas.refine"), userPrompt)
		if err != nil {
			slog.Debug("o provider falhou; tentando o próximo", "provider", provider.Name(), "erro", err)
			lastErr = err
			continue
		}

		refined := extractMermaidFromResponse(result)
		if refined == "" || !strings.Contains(refined, "flowchart") {
			slog.Debug("o provider não devolveu um Mermaid válido; tentando o próximo", "provider", provider.Name(), "resposta", result)
			respondeuSemDiagrama = true
			continue
		}

		return refined, provider.Name(), nil
	}

	if respondeuSemDiagrama {
		return "", "", errSemDiagrama
	}
	return "", "", fmt.Errorf("todos os providers falharam, último erro: %w", lastErr)
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

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
	"time"

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

// scanProject executa a descoberta de componentes no diretório atual,
// aplicando configuração externa quando disponível.
func scanProject() (*discovery.Blueprint, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	cfg := loadConfig()
	ignores := append([]string(nil), discovery.IgnoradosPadrao...)
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

// arquivoDoDiagrama é onde o diagrama é gravado, relativo à pasta do projeto, e
// arquivoDoRascunho, onde fica sempre o programático (o que a IA recebe para refinar).
var (
	arquivoDoDiagrama = filepath.Join(".yby", "atlas-diagram.mmd")
	arquivoDoRascunho = filepath.Join(".yby", "atlas-diagram.raw.mmd")
)

// prazoDaIA é quanto o refinamento pode levar, somados todos os providers; passado o
// prazo, fica o programático (trocado nos testes).
var prazoDaIA = 60 * time.Second

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
	ignores := append([]string(nil), discovery.IgnoradosPadrao...)
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
	prog.Concluir(fmt.Sprintf("%s · %s", plural(stats.Nodes, "nó", "nós"), plural(stats.Relations, "relação", "relações")))
	rascunho := diagram

	// Tentar refinar com IA (se disponível e habilitado)
	prog.Iniciar("refinar com a IA")
	refinado := false
	switch {
	case !op.useAI:
		prog.Pular("desligado com --no-ai")
	case op.detail == analysis.DetailFull:
		// O full promete todos os recursos; a IA reorganiza e resume, então não entra.
		prog.Pular("o full mostra todos os recursos")
	default:
		summary := buildResourceSummary(infraBP)
		ctx, cancelar := context.WithTimeout(context.Background(), prazoDaIA)
		refinedDiagram, source, refineErr := refineDiagramWithAI(ctx, diagram, summary)
		cancelar()
		switch {
		case refineErr == nil && refinedDiagram != "":
			diagram = refinedDiagram
			refinado = true
			prog.Concluir(source)
		case errors.Is(refineErr, errSemIA):
			prog.Pular("sem IA disponível")
		case errors.Is(refineErr, context.DeadlineExceeded):
			slog.Debug("a IA passou do prazo; ficou o programático", "prazo", prazoDaIA, "erro", refineErr)
			prog.Pular("a IA passou do prazo")
			prog.Detalhe("ficou o diagrama montado a partir dos arquivos")
		case errors.Is(refineErr, errDiagramaInvalido):
			// O motivo (nó fora do rascunho, subgraph sem end) fica no log de depuração.
			slog.Debug("o diagrama da IA não confere; ficou o programático", "erro", refineErr)
			prog.Pular("o diagrama da IA não confere")
			prog.Detalhe("ficou o diagrama montado a partir dos arquivos")
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

	// Salvar arquivo: o diagrama final e, ao lado, o programático, para comparar com o da IA
	prog.Iniciar("gravar o diagrama")
	if err := os.MkdirAll(filepath.Dir(arquivoDoDiagrama), 0755); err != nil {
		prog.Falhar("")
		return yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para criar a pasta .yby").
			WithContext("pasta", filepath.Join(cwd, ".yby"))
	}
	for _, f := range []struct{ arquivo, texto string }{
		{arquivoDoDiagrama, diagram},
		{arquivoDoRascunho, rascunho},
	} {
		if err := os.WriteFile(f.arquivo, []byte(f.texto), 0644); err != nil {
			prog.Falhar("")
			return yerrors.Wrap(err, yerrors.ErrCodeIO, "não deu para gravar o diagrama").
				WithContext("arquivo", filepath.Join(cwd, f.arquivo))
		}
	}
	prog.Concluir("")
	parar()

	// As estatísticas são do que foi gravado: o diagrama da IA tem os nós e as arestas dele.
	recursos := fmt.Sprintf("%d de %d no diagrama", stats.VisibleResources, stats.TotalResources)
	if stats.Agrupados > 0 {
		recursos += fmt.Sprintf(" · %d agrupados por tipo", stats.Agrupados)
	}
	if refinado {
		nos, arestas := analysis.ContarMermaid(diagram)
		recursos = fmt.Sprintf("%s de %s", plural(nos, "nó", "nós"), plural(stats.TotalResources, "recurso", "recursos"))
		stats.Relations = arestas
	}
	nivel := "full"
	if op.detail == analysis.DetailOverview {
		nivel = "overview · --detail full mostra todos os recursos"
	}
	campos := []ui.Campo{{Rotulo: "arquivo", Valor: filepath.ToSlash(arquivoDoDiagrama), Caminho: true}}
	if refinado {
		campos = append(campos, ui.Campo{Rotulo: "programático", Valor: filepath.ToSlash(arquivoDoRascunho), Caminho: true})
	}
	campos = append(campos,
		ui.Campo{Rotulo: "recursos", Valor: recursos},
		ui.Campo{Rotulo: "relações", Valor: fmt.Sprintf("%d", stats.Relations)},
		ui.Campo{Rotulo: "analisadores", Valor: strings.Join(stats.Analyzers, ", ")},
		ui.Campo{Rotulo: "nível", Valor: nivel},
	)
	ui.Section(saida, "diagrama")
	ui.Campos(saida, campos...)
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

// maxNomesPorKind é quantos nomes de cada kind vão no inventário do prompt; o resto vira
// contagem, para um projeto grande não estourar o contexto da IA.
const maxNomesPorKind = 10

// buildResourceSummary cria um resumo compacto dos recursos para o prompt da IA: a
// contagem por kind e os primeiros nomes. As relações não entram: já estão no rascunho,
// com os IDs que a IA tem de usar, e repeti-las aqui ofereceria outros IDs.
func buildResourceSummary(bp *discovery.InfraBlueprint) string {
	var b strings.Builder
	fmt.Fprintf(&b, "INVENTARIO DE RECURSOS DESCOBERTOS (%d recursos; as relacoes estao no rascunho):\n\n", len(bp.Resources))

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
		mais := ""
		if len(names) > maxNomesPorKind {
			mais = fmt.Sprintf(" e mais %d", len(names)-maxNomesPorKind)
			names = names[:maxNomesPorKind]
		}
		fmt.Fprintf(&b, "%s (%d): %s%s\n", kind, len(byKind[kind]), strings.Join(names, ", "), mais)
	}

	return b.String()
}

// Por que o refinamento não trouxe diagrama: nenhuma IA disponível, alguma respondeu sem
// um Mermaid, ou respondeu com um Mermaid que não confere com o rascunho. O prazo vencido
// vem como context.DeadlineExceeded; uma falha de todos os providers (fora do ar, cota) é
// outro erro.
var (
	errSemIA            = errors.New("nenhum provider de IA disponível")
	errSemDiagrama      = errors.New("a resposta da IA não trouxe um diagrama Mermaid")
	errDiagramaInvalido = errors.New("o diagrama da IA não confere com o rascunho")
)

// refineDiagramWithAI tenta refinar o diagrama usando múltiplos providers em cascata,
// até um devolver um Mermaid que passe no analysis.ValidarMermaid contra o rascunho. Se
// nenhum trouxer, retorna o erro do prazo (ctx vencido, sem tentar os seguintes), errSemIA
// (não há provider), errDiagramaInvalido (algum devolveu Mermaid que não confere),
// errSemDiagrama (algum respondeu sem Mermaid) ou o último erro dos providers.
func refineDiagramWithAI(ctx context.Context, mermaidDraft, resourceSummary string) (string, string, error) {
	userPrompt := fmt.Sprintf("DIAGRAMA RASCUNHO:\n\n%s\n\n%s\n\nProduza o diagrama Mermaid refinado, usando so os IDs de nos do rascunho:", mermaidDraft, resourceSummary)

	// Montar lista de providers para tentar em ordem
	providers := provedoresDeIA(ctx)
	if len(providers) == 0 {
		return "", "", errSemIA
	}

	var lastErr, invalido error
	respondeuSemDiagrama := false
	for _, provider := range providers {
		if ctx.Err() != nil {
			break
		}
		result, err := provider.Completion(ctx, prompts.Get("atlas.refine"), userPrompt)
		if err != nil {
			slog.Debug("o provider falhou; tentando o próximo", "provider", provider.Name(), "erro", err)
			lastErr = err
			continue
		}

		refined := extractMermaidFromResponse(result)
		if err := analysis.ValidarMermaid(refined, mermaidDraft); err != nil {
			slog.Debug("o provider não devolveu um Mermaid válido; tentando o próximo", "provider", provider.Name(), "motivo", err, "resposta", result)
			if errors.Is(err, analysis.ErrNaoEhMermaid) {
				respondeuSemDiagrama = true
			} else {
				invalido = err
			}
			continue
		}

		return refined, provider.Name(), nil
	}

	switch {
	case ctx.Err() != nil:
		return "", "", fmt.Errorf("a IA não respondeu no prazo: %w", ctx.Err())
	case invalido != nil:
		return "", "", fmt.Errorf("%w: %w", errDiagramaInvalido, invalido)
	case respondeuSemDiagrama:
		return "", "", errSemDiagrama
	}
	return "", "", fmt.Errorf("todos os providers falharam, último erro: %w", lastErr)
}

// extractMermaidFromResponse extrai o código Mermaid de uma resposta que pode conter markdown.
func extractMermaidFromResponse(response string) string {
	response = strings.TrimSpace(response)

	// Se começa com flowchart ou graph, já é Mermaid puro
	if strings.HasPrefix(response, "flowchart") || strings.HasPrefix(response, "graph") {
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

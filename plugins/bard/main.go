package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
	"github.com/casheiro/yby-cli/plugins/bard/tui"
	"golang.org/x/term"
)

func main() {
	os.Exit(executar())
}

// executar roda o plugin e devolve o código de saída; um erro sai no stderr no formato
// de ui.WriteError.
func executar() int {
	if err := run(); err != nil {
		return sairComErro(os.Stderr, err)
	}
	return 0
}

// saidaSilenciosa encerra o processo com o código dado sem escrever mais nada: os erros
// já saíram (batch) ou a pessoa interrompeu a resposta (ctrl+c).
type saidaSilenciosa struct{ codigo int }

func (s saidaSilenciosa) Error() string { return fmt.Sprintf("saída %d", s.codigo) }

// sairComErro escreve err em w como ui.WriteError (um erro comum vira ERR_PLUGIN),
// depois de uma linha em branco que o separa do que já saiu, e devolve o código de saída.
func sairComErro(w io.Writer, err error) int {
	var silenciosa saidaSilenciosa
	if errors.As(err, &silenciosa) {
		return silenciosa.codigo
	}
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) {
		ye = yerrors.Wrap(err, yerrors.ErrCodePlugin, "o Bard falhou")
	}
	fmt.Fprintln(w)
	ui.WriteError(w, ye)
	return 1
}

// entradaLote é de onde o batch lê as perguntas: o stdin, ou o que sobrou dele depois
// de o run tentar ler ali um pedido JSON.
var entradaLote io.Reader = os.Stdin

func run() error {
	var req plugin.PluginRequest

	// 1. Check for Environment Variable Protocol (Preferred for Interactive/TUI)
	if envReq := os.Getenv("YBY_PLUGIN_REQUEST"); envReq != "" {
		if err := json.Unmarshal([]byte(envReq), &req); err != nil {
			return yerrors.Wrap(err, yerrors.ErrCodePluginRPC, "YBY_PLUGIN_REQUEST inválido")
		}
		return handlePluginRequest(req)
	}

	// 2. Check for Stdin Protocol (Legacy/Automation)
	if stat, err := os.Stdin.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		// Data on pipe -> Plugin Request. O que não for um pedido volta inteiro para o
		// batch, com o resto do stdin.
		var lido bytes.Buffer
		if err := json.NewDecoder(io.TeeReader(os.Stdin, &lido)).Decode(&req); err == nil {
			return handlePluginRequest(req)
		}
		entradaLote = io.MultiReader(&lido, os.Stdin)
	}

	// 3. Fallback / Dev Mode
	// Mock request for development or direct invocation without context
	return handlePluginRequest(plugin.PluginRequest{Hook: "command"})
}

func handlePluginRequest(req plugin.PluginRequest) error {
	switch req.Hook {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:        "bard",
			Version:     "1.0.0",
			Description: "Assistente de IA do cluster: conversa, ferramentas e diagnóstico",
			Hooks:       []string{"command"},
		})
		return nil
	case "command":
		// Modo visual (tema, ASCII, sem cor) antes de qualquer saída, em todos os modos.
		configurarModoVisual(os.Getenv)

		opcoes, err := lerArgs(req.Args)
		switch {
		case err != nil:
			prepararStderr()
			return err
		case opcoes.ajuda:
			ui.EscreverAjuda(os.Stdout, ajudaBard())
			return nil
		case opcoes.temPrompt && opcoes.prompt == "":
			prepararStderr()
			return semPergunta()
		}
		// O kubectl das ferramentas segue o cluster pedido (--context, --namespace).
		tools.DefinirKube(opcoes.contexto, opcoes.namespace)

		if opcoes.temPrompt {
			return runOneShot(req.Context, opcoes.prompt)
		}

		return startChat(req.Context, opcoes)
	default:
		return nil
	}
}

// contextoInterrompivel devolve um contexto cancelado pelo primeiro ctrl+c, para a
// resposta parar e as etapas fecharem; o segundo ctrl+c encerra o processo como sempre.
func contextoInterrompivel() (context.Context, context.CancelFunc) {
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt)
	go func() {
		<-ctx.Done()
		parar()
	}()
	return ctx, parar
}

// runOneShot responde a pergunta do -p e sai.
func runOneShot(ctxData map[string]interface{}, prompt string) error {
	prepararStderr()
	ctx, parar := contextoInterrompivel()
	defer parar()
	d, err := novoDireto(ctx, ctxData)
	if err != nil {
		return err
	}
	return d.perguntar(ctx, prompt)
}

// runBatchMode responde, sem TUI, uma pergunta por linha do stdin.
func runBatchMode(ctxData map[string]interface{}) error {
	prepararStderr()
	ctx, parar := contextoInterrompivel()
	defer parar()
	d, err := novoDireto(ctx, ctxData)
	if err != nil {
		return err
	}
	return d.lote(ctx, entradaLote)
}

// semProvider é o erro de quando nenhum provider de IA responde.
func semProvider() error {
	return yerrors.New(yerrors.ErrCodeConfig, "nenhum provider de IA disponível").
		WithHint("rode ollama serve, exporte OPENAI_API_KEY ou GEMINI_API_KEY, ou defina ai.provider no ~/.yby/config.yaml")
}

// semPergunta é o erro do -p sem a pergunta.
func semPergunta() error {
	return yerrors.New(yerrors.ErrCodeValidation, "falta a pergunta do -p").
		WithHint(`use yby bard -p "pergunta"`)
}

// semTerminal é o erro de quando a entrada é um terminal mas a saída não: a TUI não
// tem onde desenhar e o batch ficaria esperando o que se digita.
func semTerminal() error {
	return yerrors.New(yerrors.ErrCodeValidation, "a conversa do Bard precisa de um terminal").
		WithHint(`use yby bard -p "pergunta" ou mande as perguntas pelo stdin`)
}

func startChat(ctxData map[string]interface{}, opcoes opcoesBard) error {
	// Sem terminal na entrada, as perguntas vêm do stdin (batch). Com terminal na entrada
	// e não na saída, nada abre: a TUI desenharia no pipe.
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		if opcoes.pergunta != "" {
			prepararStderr()
			return rascunhoSemTerminal()
		}
		return runBatchMode(ctxData)
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		prepararStderr()
		return semTerminal()
	}

	// Carregar tools externas do usuário
	tools.LoadExternalTools()

	// Inicializar IA
	ctx := context.Background()
	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		prepararStderr()
		return semProvider()
	}

	// 1. Abrir o índice do Synapstor, se o projeto tiver um (como no -p): sem ele, nada é
	// criado no diretório nem o modelo de embeddings é baixado na primeira pergunta
	cwd, _ := os.Getwd()
	storePath := filepath.Join(cwd, ".synapstor", ".index")
	embProvider := ai.GetEmbeddingProvider(ctx)
	vectorStore := abrirMemoria(ctx)

	// 2. Carregar configuração do Bard
	bardCfg := loadBardConfig()

	// 3. Enriquecer contexto do cluster (non-fatal)
	clusterCtx := EnrichContext(ctx, opcoes.contexto, opcoes.namespace)

	// 4. Gerar SessionID para esta sessão interativa
	sessionID := time.Now().Format("20060102-150405")

	// 5. Carregar histórico de sessões anteriores
	history := loadHistory()

	// 6. Construir system prompt enriquecido
	systemPrompt := buildSystemPrompt(ctxData, bardCfg, history, clusterCtx)

	// 7. TUI (Bubbletea)
	tuiConfig := tui.Config{
		SystemPrompt: systemPrompt,
		SessionID:    sessionID,
		AIModel:      rotuloProvider(provider),
		SaveMessage:  saveMessage,

		TopK:             bardCfg.TopK,
		LimiarRelevancia: bardCfg.RelevanceThreshold,

		Sessoes:       sessoesSalvas,
		RetomarSessao: retomador(ctxData, bardCfg, clusterCtx),
		Modelos:       modelosDetectados,
		Diretorio:     cwd,
		Sanitizar:     sanitizeContent,
		Historico:     contarHistorico(history),
	}
	aplicarCluster(&tuiConfig, clusterCtx, opcoes)
	tuiConfig.Rascunho, tuiConfig.Voltar = opcoes.pergunta, opcoes.voltar
	tuiConfig.Contexto = infoContexto(vectorStore, embProvider, storePath, cwd, tuiConfig.Cluster)
	resolverTerminal()
	defer logDaConversa()()
	return saidaDaConversa(tui.Run(provider, vectorStore, tuiConfig))
}

// codigoCtrlC é o código do Bard interrompido pelo ctrl+c (128 + SIGINT), como no -p, no
// batch e no Viz.
const codigoCtrlC = 130

// saidaDaConversa é o erro do startChat a partir do da TUI: o ctrl+c sai com 130 sem
// escrever nada, e o Viz que abriu o Bard (b) fecha também.
func saidaDaConversa(err error) error {
	if errors.Is(err, tui.ErrInterrompido) {
		return saidaSilenciosa{codigo: codigoCtrlC}
	}
	return err
}

// aplicarCluster põe no cabeçalho o cluster ▸ namespace lido do kubectl; o --context e o
// --namespace valem mesmo quando o kubectl não responde (o Viz sabe onde está).
func aplicarCluster(c *tui.Config, cc *ClusterContext, opcoes opcoesBard) {
	if cc != nil {
		c.Cluster, c.Namespace = cc.Cluster, cc.Namespace
	}
	if opcoes.contexto != "" {
		c.Cluster = opcoes.contexto
	}
	if opcoes.namespace != "" {
		c.Namespace = opcoes.namespace
	}
}

// buildSystemPrompt constrói o system prompt enriquecido com contexto e histórico.
func buildSystemPrompt(ctxData map[string]interface{}, bardCfg BardConfig, history []HistoryEntry, clusterCtx ...*ClusterContext) string {
	overview, _ := ctxData["overview"].(string)
	backlog, _ := ctxData["backlog"].(string)

	blueprintSummary := "Nenhum blueprint disponível."
	if bp, ok := ctxData["blueprint"]; ok {
		bpBytes, _ := json.MarshalIndent(bp, "", "  ")
		blueprintSummary = string(bpBytes)
	}

	contextBlock := fmt.Sprintf(`
## Project Overview
%s

## Backlog & Debt
%s

## Technical Blueprint (Atlas)
%s
`, overview, backlog, blueprintSummary)

	// Injetar contexto do cluster se disponível
	clusterSection := ""
	if len(clusterCtx) > 0 && clusterCtx[0] != nil {
		clusterSection = FormatClusterContext(clusterCtx[0])
	}

	vars := map[string]string{
		"blueprint_json_summary": contextBlock,
		"tools_prompt":           tools.FormatToolsPrompt(),
		"cluster_context":        clusterSection,
	}
	systemPrompt := prompts.GetWithVars("bard.system", vars)

	historyCtx := formatHistoryContext(history)
	if historyCtx != "" {
		systemPrompt += "\n\n" + historyCtx
	}

	if bardCfg.SystemPromptExtra != "" {
		systemPrompt += "\n\n" + bardCfg.SystemPromptExtra
	}

	return systemPrompt
}

func respond(data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}

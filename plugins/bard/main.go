package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/retry"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var req plugin.PluginRequest

	// 1. Check for Environment Variable Protocol (Preferred for Interactive/TUI)
	if envReq := os.Getenv("YBY_PLUGIN_REQUEST"); envReq != "" {
		if err := json.Unmarshal([]byte(envReq), &req); err != nil {
			return fmt.Errorf("erro ao analisar YBY_PLUGIN_REQUEST: %w", err)
		}
		return handlePluginRequest(req)
	}

	// 2. Check for Stdin Protocol (Legacy/Automation)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data on pipe -> Plugin Request
		if err := json.NewDecoder(os.Stdin).Decode(&req); err == nil {
			return handlePluginRequest(req)
		}
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
			Version:     "0.1.0",
			Description: "Assistente de IA interativo para diagnóstico e operações",
			Hooks:       []string{"command"},
		})
		return nil
	case "command":
		return startChat(req.Context)
	default:
		return nil
	}
}

func startChat(ctxData map[string]interface{}) error {
	// Inicializar IA
	ctx := context.Background()
	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		return fmt.Errorf("nenhum provedor de IA disponível. Defina OLLAMA_HOST ou OPENAI_API_KEY")
	}

	// 1. Inicializar Vector Store (acesso somente leitura)
	cwd, _ := os.Getwd()
	storePath := filepath.Join(cwd, ".synapstor", ".index")
	vectorStore, err := ai.NewVectorStore(ctx, storePath, provider)
	if err != nil {
		// Não fatal, apenas sem memória de longo prazo
		fmt.Printf(lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("⚠️  Aviso: Memória semântica indisponível (%v)\n"), err)
	}

	// 2. Carregar configuração do Bard
	bardCfg := loadBardConfig()

	// 3. Detectar modo batch (non-TTY)
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	if !isTTY {
		return runBatchMode(ctx, provider, vectorStore, bardCfg, ctxData)
	}

	// 4. Gerar SessionID para esta sessão interativa
	sessionID := time.Now().Format("20060102-150405")

	// 5. Carregar histórico de sessões anteriores
	history := loadHistory()

	// 6. Construir system prompt enriquecido
	systemPrompt := buildSystemPrompt(ctxData, bardCfg, history)

	historyCtx := formatHistoryContext(history)

	// Configuração da UI
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("🤖 Yby Bard"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Digite 'exit' para sair. '/clear' para limpar histórico. '/sessions' para listar sessões."))

	if vectorStore != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("🧠 Memória Semântica Ativa."))
	}
	fmt.Printf(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("📝 Sessão: %s\n"), sessionID)
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

		// Comando /clear para limpar histórico
		if input == "/clear" {
			clearHistory()
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("✅ Histórico limpo."))
			continue
		}

		// Comando /sessions para listar sessões
		if input == "/sessions" {
			handleSessionsList(sessionID)
			continue
		}

		// Comando /session <id> para carregar sessão específica
		if strings.HasPrefix(input, "/session ") {
			targetID := strings.TrimSpace(strings.TrimPrefix(input, "/session "))
			entries, loadErr := loadAllEntries()
			if loadErr != nil {
				fmt.Printf("Erro ao carregar sessões: %v\n", loadErr)
				continue
			}
			sessionEntries := loadSessionHistory(entries, targetID, maxHistoryEntries)
			if len(sessionEntries) == 0 {
				fmt.Printf(lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("⚠️  Sessão '%s' não encontrada ou vazia.\n"), targetID)
				continue
			}
			// Atualizar contexto de histórico com a sessão carregada
			historyCtx = formatHistoryContext(sessionEntries)
			systemPrompt = buildSystemPrompt(ctxData, bardCfg, sessionEntries)
			fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render(
				fmt.Sprintf("Carregada sessão %s (%d mensagens)", targetID, len(sessionEntries)),
			))
			continue
		}

		// Salvar mensagem do usuário antes da chamada IA
		saveMessage("user", input, sessionID)

		fmt.Print(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("Bard > "))

		// 7. Recuperação inteligente (busca vetorial com threshold)
		ukiContext := ""
		if vectorStore != nil {
			results, searchErr := vectorStore.Search(ctx, input, bardCfg.TopK)
			if searchErr == nil && len(results) > 0 {
				// Filtrar por threshold de relevância
				filtered := filterByThreshold(results, bardCfg.RelevanceThreshold)

				if len(filtered) > 0 {
					var sources []string
					var sb strings.Builder

					for _, res := range filtered {
						sources = append(sources, fmt.Sprintf("%s (%.2f)", res.Metadata["filename"], res.Score))
						sb.WriteString(fmt.Sprintf("\n--- Contexto: %s ---\n%s\n", res.Metadata["title"], res.Content))
					}

					ukiContext = sb.String()

					// Exibir fontes na UI (sutil)
					fmt.Printf(lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("240")).Render("\n(Consultando: %s)... "), strings.Join(sources, ", "))
				}
			}
		}

		// 8. Ajustar contextos ao orçamento de tokens
		truncatedHistory, truncatedRAG := TruncateToFit(
			bardCfg.MaxTokens, systemPrompt, input, historyCtx, ukiContext,
		)

		// Reconstruir system prompt com histórico truncado
		if truncatedHistory != historyCtx {
			currentPrompt := strings.ReplaceAll(systemPrompt, historyCtx, truncatedHistory)
			systemPrompt = currentPrompt
		}

		// 9. Resposta final com retry e captura de output
		runInput := input
		if truncatedRAG != "" {
			runInput = fmt.Sprintf("Contexto Adicional Recuperado (Memória Semântica):\n%s\n\nPergunta do Usuário: %s", truncatedRAG, input)
		}

		var responseBuf bytes.Buffer
		writer := io.MultiWriter(os.Stdout, &responseBuf)

		retryOpts := retry.Options{
			InitialInterval:     2 * time.Second,
			MaxInterval:         10 * time.Second,
			MaxElapsedTime:      30 * time.Second,
			RandomizationFactor: 0.3,
			Multiplier:          2.0,
		}

		attempt := 0
		err := retry.Do(ctx, retryOpts, func() error {
			attempt++
			if attempt > 1 {
				fmt.Print(lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Render("\n🔄 Tentando novamente..."))
				responseBuf.Reset()
			}
			return provider.StreamCompletion(ctx, systemPrompt, runInput, writer)
		})

		if err != nil {
			fmt.Printf("\nErro: %v\n", err)
		} else {
			saveMessage("assistant", responseBuf.String(), sessionID)
		}
		fmt.Println() // Nova linha após o stream
	}
	return nil
}

// buildSystemPrompt constrói o system prompt enriquecido com contexto e histórico.
func buildSystemPrompt(ctxData map[string]interface{}, bardCfg BardConfig, history []HistoryEntry) string {
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

	systemPrompt := strings.ReplaceAll(BardSystemPrompt, "{{ blueprint_json_summary }}", contextBlock)

	historyCtx := formatHistoryContext(history)
	if historyCtx != "" {
		systemPrompt += "\n\n" + historyCtx
	}

	if bardCfg.SystemPromptExtra != "" {
		systemPrompt += "\n\n" + bardCfg.SystemPromptExtra
	}

	return systemPrompt
}

// handleSessionsList exibe a lista de sessões disponíveis.
func handleSessionsList(currentSessionID string) {
	entries, err := loadAllEntries()
	if err != nil {
		fmt.Printf("Erro ao carregar sessões: %v\n", err)
		return
	}

	if len(entries) == 0 {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Nenhuma sessão encontrada."))
		return
	}

	sessions := listSessions(entries)
	fmt.Println(lipgloss.NewStyle().Bold(true).Render("Sessões disponíveis:"))
	for _, s := range sessions {
		marker := "  "
		suffix := ""
		if s.SessionID == currentSessionID {
			marker = "* "
			suffix = " — atual"
		}
		fmt.Printf("  %s%s (%d mensagens)%s\n", marker, s.SessionID, s.MessageCount, suffix)
	}
}

// runBatchMode executa o Bard em modo não-interativo (pipe/batch).
// Processa uma pergunta por linha do stdin, sem styling nem histórico.
func runBatchMode(ctx context.Context, provider ai.Provider, vectorStore *ai.VectorStore, bardCfg BardConfig, ctxData map[string]interface{}) error {
	systemPrompt := buildSystemPrompt(ctxData, bardCfg, nil)

	scanner := bufio.NewScanner(os.Stdin)
	var lastErr error
	first := true

	for scanner.Scan() {
		question := strings.TrimSpace(scanner.Text())
		if question == "" {
			continue
		}

		if !first {
			fmt.Print("\n---\n\n")
		}
		first = false

		// Busca RAG (se vector store disponível)
		runInput := question
		if vectorStore != nil {
			results, searchErr := vectorStore.Search(ctx, question, bardCfg.TopK)
			if searchErr == nil && len(results) > 0 {
				filtered := filterByThreshold(results, bardCfg.RelevanceThreshold)
				if len(filtered) > 0 {
					var sb strings.Builder
					for _, res := range filtered {
						sb.WriteString(fmt.Sprintf("\n--- Contexto: %s ---\n%s\n", res.Metadata["title"], res.Content))
					}
					runInput = fmt.Sprintf("Contexto Adicional Recuperado (Memória Semântica):\n%s\n\nPergunta do Usuário: %s", sb.String(), question)
				}
			}
		}

		err := provider.StreamCompletion(ctx, systemPrompt, runInput, os.Stdout)
		if err != nil {
			slog.Error("falha ao processar pergunta", "pergunta", question, "erro", err)
			lastErr = err
			continue
		}
	}

	return lastErr
}

func respond(data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}

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

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/retry"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	var req plugin.PluginRequest

	// 1. Check for Environment Variable Protocol (Preferred for Interactive/TUI)
	if envReq := os.Getenv("YBY_PLUGIN_REQUEST"); envReq != "" {
		if err := json.Unmarshal([]byte(envReq), &req); err != nil {
			fmt.Printf("Erro ao analisar YBY_PLUGIN_REQUEST: %v\n", err)
			os.Exit(1)
		}
		handlePluginRequest(req)
		return
	}

	// 2. Check for Stdin Protocol (Legacy/Automation)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data on pipe -> Plugin Request
		if err := json.NewDecoder(os.Stdin).Decode(&req); err == nil {
			handlePluginRequest(req)
			return
		}
	}

	// 3. Fallback / Dev Mode
	// Mock request for development or direct invocation without context
	handlePluginRequest(plugin.PluginRequest{Hook: "command"})
}

func handlePluginRequest(req plugin.PluginRequest) {
	switch req.Hook {
	case "manifest":
		respond(plugin.PluginManifest{
			Name:        "bard",
			Version:     "0.1.0",
			Description: "Assistente de IA interativo para diagnóstico e operações",
			Hooks:       []string{"command"},
		})
	case "command":
		startChat(req.Context)
	default:
		// Unknown hook
		// Just exit 0 to not break anything, or error
		os.Exit(0)
	}
}

func startChat(ctxData map[string]interface{}) {
	// Inicializar IA
	ctx := context.Background()
	provider := ai.GetProvider(ctx, "auto")
	if provider == nil {
		fmt.Println("❌ Nenhum provedor de IA disponível. Defina OLLAMA_HOST ou OPENAI_API_KEY.")
		os.Exit(1)
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

	// 3. Carregar histórico de sessões anteriores
	history := loadHistory()

	// 4. Construir contexto base a partir do payload
	overview, _ := ctxData["overview"].(string)
	backlog, _ := ctxData["backlog"].(string)

	blueprintSummary := "Nenhum blueprint disponível."
	if bp, ok := ctxData["blueprint"]; ok {
		bpBytes, _ := json.MarshalIndent(bp, "", "  ")
		blueprintSummary = string(bpBytes)
	}

	// 5. Construir system prompt enriquecido
	contextBlock := fmt.Sprintf(`
## Project Overview
%s

## Backlog & Debt
%s

## Technical Blueprint (Atlas)
%s
`, overview, backlog, blueprintSummary)

	systemPrompt := strings.ReplaceAll(BardSystemPrompt, "{{ blueprint_json_summary }}", contextBlock)

	// Injetar histórico de conversas anteriores
	historyCtx := formatHistoryContext(history)
	if historyCtx != "" {
		systemPrompt += "\n\n" + historyCtx
	}

	// Injetar prompt extra da configuração
	if bardCfg.SystemPromptExtra != "" {
		systemPrompt += "\n\n" + bardCfg.SystemPromptExtra
	}

	// Configuração da UI
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("🤖 Yby Bard"))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Digite 'exit' para sair. '/clear' para limpar histórico."))

	if vectorStore != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("🧠 Memória Semântica Ativa."))
	}
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

		// Salvar mensagem do usuário antes da chamada IA
		saveMessage("user", input)

		fmt.Print(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("Bard > "))

		// 6. Recuperação inteligente (busca vetorial com threshold)
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

		// 7. Resposta final com retry e captura de output
		runInput := input
		if ukiContext != "" {
			runInput = fmt.Sprintf("Contexto Adicional Recuperado (Memória Semântica):\n%s\n\nPergunta do Usuário: %s", ukiContext, input)
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
			saveMessage("assistant", responseBuf.String())
		}
		fmt.Println() // Nova linha após o stream
	}
}

func respond(data interface{}) {
	resp := plugin.PluginResponse{Data: data}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}

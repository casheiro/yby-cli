package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// maxToolIterations limita o número de iterações de tool calling.
const maxToolIterations = 5

// state representa o estado atual da TUI.
type state int

const (
	stateIdle state = iota
	stateStreaming
)

// Config contém a configuração passada para a TUI.
type Config struct {
	SystemPrompt string
	SessionID    string
	Namespace    string
	Cluster      string
	AIModel      string
	SaveMessage  func(role, content, sessionID string)
}

// responseMsg é enviada quando o streaming da IA termina.
type responseMsg struct {
	content string
	err     error
}

// Model é o modelo principal do Bubbletea para o Bard.
type Model struct {
	viewport    viewport.Model
	textarea    textarea.Model
	provider    ai.Provider
	vectorStore *ai.VectorStore
	config      Config
	messages    []chatMessage
	state       state
	width       int
	height      int
	renderer    *glamour.TermRenderer
	err         error
}

type chatMessage struct {
	role    string // "user", "assistant", "tool", "error"
	content string
}

// New cria uma nova instância do modelo da TUI.
func New(provider ai.Provider, vectorStore *ai.VectorStore, config Config) Model {
	ta := textarea.New()
	ta.Placeholder = "Digite sua mensagem... (Enter para enviar, Ctrl+C para sair)"
	ta.Focus()
	ta.CharLimit = 4096
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("")

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(76),
	)

	return Model{
		viewport:    vp,
		textarea:    ta,
		provider:    provider,
		vectorStore: vectorStore,
		config:      config,
		state:       stateIdle,
		renderer:    renderer,
	}
}

// Init inicializa o modelo do Bubbletea.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update processa mensagens do Bubbletea.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			if m.state == stateStreaming {
				return m, nil
			}
			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}

			// Processar comandos
			if input == "exit" || input == "quit" {
				return m, tea.Quit
			}

			// Adicionar mensagem do usuário
			m.messages = append(m.messages, chatMessage{role: "user", content: input})
			m.textarea.Reset()
			m.state = stateStreaming
			m.updateViewport()

			// Salvar mensagem do usuário
			if m.config.SaveMessage != nil {
				m.config.SaveMessage("user", input, m.config.SessionID)
			}

			return m, m.sendMessage(input)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 2
		statusBarHeight := 2
		textareaHeight := 3
		padding := 2
		vpHeight := m.height - headerHeight - statusBarHeight - textareaHeight - padding

		if vpHeight < 3 {
			vpHeight = 3
		}

		m.viewport.Width = m.width - 2
		m.viewport.Height = vpHeight
		m.textarea.SetWidth(m.width - 2)
		m.updateViewport()
		return m, nil

	case responseMsg:
		m.state = stateIdle
		if msg.err != nil {
			m.messages = append(m.messages, chatMessage{role: "error", content: msg.err.Error()})
		} else {
			m.messages = append(m.messages, chatMessage{role: "assistant", content: msg.content})
			if m.config.SaveMessage != nil {
				m.config.SaveMessage("assistant", msg.content, m.config.SessionID)
			}
		}
		m.updateViewport()
		return m, nil
	}

	// Atualizar componentes
	var cmd tea.Cmd

	if m.state == stateIdle {
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renderiza a TUI.
func (m Model) View() string {
	if m.width == 0 {
		return "Carregando..."
	}

	var b strings.Builder

	// Cabeçalho
	b.WriteString(headerStyle.Render("Yby Bard"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Digite 'exit' para sair"))
	b.WriteString("\n")

	// Viewport com mensagens
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	// Status bar
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n")

	// Input
	if m.state == stateStreaming {
		b.WriteString(thinkingStyle.Render("  Processando..."))
	} else {
		b.WriteString(m.textarea.View())
	}

	return b.String()
}

// sendMessage envia a mensagem para a IA e retorna o resultado.
func (m Model) sendMessage(input string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Busca RAG
		runInput := input
		if m.vectorStore != nil {
			results, err := m.vectorStore.Search(ctx, input, 5)
			if err == nil && len(results) > 0 {
				var sb strings.Builder
				for _, res := range results {
					if float64(res.Score) >= 0.6 {
						sb.WriteString(fmt.Sprintf("\n--- Contexto: %s ---\n%s\n", res.Metadata["title"], res.Content))
					}
				}
				if sb.Len() > 0 {
					runInput = fmt.Sprintf("Contexto Adicional Recuperado (Memória Semântica):\n%s\n\nPergunta do Usuário: %s", sb.String(), input)
				}
			}
		}

		// Loop de tool calling
		currentInput := runInput
		for iteration := 0; iteration <= maxToolIterations; iteration++ {
			var responseBuf bytes.Buffer
			err := m.provider.StreamCompletion(ctx, m.config.SystemPrompt, currentInput, io.Writer(&responseBuf))
			if err != nil {
				return responseMsg{err: err}
			}

			response := responseBuf.String()
			toolCalls, remainingText := tools.ParseToolCalls(response)

			if len(toolCalls) == 0 {
				return responseMsg{content: response}
			}

			// Executar tools
			var sb strings.Builder
			if remainingText != "" {
				sb.WriteString(remainingText)
				sb.WriteString("\n\n")
			}

			for _, call := range toolCalls {
				if guardErr := tools.ValidateToolCall(call); guardErr != nil {
					sb.WriteString(fmt.Sprintf("Resultado da ferramenta %s:\nErro: bloqueado por guardrail: %v\n\n", call.Name, guardErr))
					continue
				}

				tool := tools.Get(call.Name)
				if tool == nil {
					sb.WriteString(fmt.Sprintf("Resultado da ferramenta %s:\nErro: ferramenta não encontrada\n\n", call.Name))
					continue
				}

				output, execErr := tool.Execute(ctx, call.Params)
				sb.WriteString(fmt.Sprintf("Resultado da ferramenta %s:\n", call.Name))
				if execErr != nil {
					sb.WriteString(fmt.Sprintf("Erro: %v\n", execErr))
				}
				if output != "" {
					sb.WriteString(output)
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}
			sb.WriteString("Continue respondendo ao usuário com base nos resultados das ferramentas.")
			currentInput = sb.String()
		}

		return responseMsg{content: "Limite de iterações de ferramentas atingido."}
	}
}

// updateViewport atualiza o conteúdo do viewport com as mensagens.
func (m *Model) updateViewport() {
	var content strings.Builder

	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			content.WriteString(userStyle.Render("You > "))
			content.WriteString(msg.content)
			content.WriteString("\n\n")
		case "assistant":
			content.WriteString(bardStyle.Render("Bard > "))
			// Renderizar markdown se disponível
			if m.renderer != nil {
				rendered, err := m.renderer.Render(msg.content)
				if err == nil {
					content.WriteString(strings.TrimSpace(rendered))
				} else {
					content.WriteString(msg.content)
				}
			} else {
				content.WriteString(msg.content)
			}
			content.WriteString("\n\n")
		case "tool":
			content.WriteString(toolStyle.Render("Tool > "))
			content.WriteString(msg.content)
			content.WriteString("\n\n")
		case "error":
			content.WriteString(errorMsgStyle.Render("Erro: "))
			content.WriteString(msg.content)
			content.WriteString("\n\n")
		}
	}

	if m.state == stateStreaming {
		content.WriteString(thinkingStyle.Render("Pensando..."))
		content.WriteString("\n")
	}

	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

// renderStatusBar renderiza a barra de status inferior.
func (m Model) renderStatusBar() string {
	items := []string{}

	if m.config.Cluster != "" {
		items = append(items, statusKeyStyle.Render("Cluster: ")+statusValueStyle.Render(m.config.Cluster))
	}
	if m.config.Namespace != "" {
		items = append(items, statusKeyStyle.Render("NS: ")+statusValueStyle.Render(m.config.Namespace))
	}
	if m.config.SessionID != "" {
		items = append(items, statusKeyStyle.Render("Sessão: ")+statusValueStyle.Render(m.config.SessionID))
	}
	if m.config.AIModel != "" {
		items = append(items, statusKeyStyle.Render("Modelo: ")+statusValueStyle.Render(m.config.AIModel))
	}

	stateStr := "idle"
	if m.state == stateStreaming {
		stateStr = "streaming"
	}
	items = append(items, statusKeyStyle.Render("Status: ")+statusValueStyle.Render(stateStr))

	bar := strings.Join(items, "  |  ")
	return statusBarBg.Width(m.width).Render(lipgloss.NewStyle().PaddingLeft(1).Render(bar))
}

// Run inicia a TUI do Bard.
func Run(provider ai.Provider, vectorStore *ai.VectorStore, config Config) error {
	model := New(provider, vectorStore, config)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

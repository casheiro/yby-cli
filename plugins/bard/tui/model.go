package tui

import (
	"context"
	"fmt"
	"strings"

	"encoding/json"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	"github.com/casheiro/yby-cli/pkg/ui"
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
	// TopK e LimiarRelevancia controlam a busca no Synapstor, como vêm do
	// bard.yaml (padrões 5 e 0,6 no loadBardConfig). TopK 0 vira 5; limiar 0
	// aceita todas as notas.
	TopK             int
	LimiarRelevancia float64
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
	markdown    func(string) (string, error) // glamour; nil mostra o texto cru
	err         error

	// Histórico já renderizado: as mensagens só crescem, então cada uma passa
	// pelo glamour uma vez; a largura nova refaz tudo. Assim um trecho da
	// resposta não paga a renderização da conversa inteira.
	historico        string
	historicoN       int
	historicoLargura int

	// Rodada em andamento: eventos do fluxo, cancelamento (esc) e o que já chegou.
	fluxo         *Fluxo
	rodada        int
	eventos       <-chan tea.Msg
	cancelar      context.CancelFunc
	interrompendo bool
	etapas        []EtapaMsg
	parcial       string
}

type chatMessage struct {
	role    string // "user", "assistant", "tool", "error", "nota"
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

	m := Model{
		viewport:    vp,
		textarea:    ta,
		provider:    provider,
		vectorStore: vectorStore,
		config:      config,
		state:       stateIdle,
		fluxo:       novoFluxo(provider, vectorStore, config),
	}
	if renderer != nil {
		m.markdown = renderer.Render
	}
	return m
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
			if m.cancelar != nil {
				m.cancelar()
			}
			return m, tea.Quit

		case tea.KeyEsc:
			// esc interrompe a resposta em andamento sem fechar o Bard; o
			// FimMsg da rodada é que devolve o estado ocioso.
			if m.state == stateStreaming && m.cancelar != nil && !m.interrompendo {
				m.interrompendo = true
				m.cancelar()
				m.updateViewport()
				return m, nil
			}

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

			return m, m.iniciarRodada(input)
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

	case EtapaMsg:
		if msg.Rodada != m.rodada {
			return m, nil
		}
		if msg.Etapa == EtapaResposta && msg.Tentativa > 1 {
			// A resposta recomeçou: o texto da tentativa anterior não vale.
			m.parcial = ""
		}
		m.registrarEtapa(msg)
		m.updateViewport()
		return m, m.proximoEvento()

	case TrechoMsg:
		if msg.Rodada != m.rodada {
			return m, nil
		}
		m.parcial += msg.Texto
		m.updateViewport()
		return m, m.proximoEvento()

	case FimMsg:
		if msg.Rodada != m.rodada {
			return m, nil
		}
		m.encerrarRodada(msg)
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

// iniciarRodada dispara o fluxo da pergunta e devolve o comando que lê o
// primeiro evento; cada evento tratado em Update pede o seguinte até o FimMsg.
func (m *Model) iniciarRodada(input string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.rodada++
	m.cancelar = cancel
	m.interrompendo = false
	m.etapas = nil
	m.parcial = ""
	m.eventos = m.fluxo.Iniciar(ctx, m.rodada, input)
	return ProximoEvento(m.eventos)
}

// proximoEvento pede o evento seguinte da rodada; sem rodada aberta não há o
// que ler (um canal nil travaria o comando para sempre).
func (m Model) proximoEvento() tea.Cmd {
	if m.eventos == nil {
		return nil
	}
	return ProximoEvento(m.eventos)
}

// registrarEtapa guarda o estado mais recente de cada etapa da rodada.
func (m *Model) registrarEtapa(msg EtapaMsg) {
	for i := range m.etapas {
		if m.etapas[i].Etapa == msg.Etapa && m.etapas[i].Rotulo == msg.Rotulo {
			m.etapas[i] = msg
			return
		}
	}
	m.etapas = append(m.etapas, msg)
}

// encerrarRodada fecha a rodada: registra a resposta (ou o erro) e volta ao
// estado ocioso. Resposta interrompida mostra o que chegou e não vai ao histórico.
func (m *Model) encerrarRodada(msg FimMsg) {
	if m.cancelar != nil {
		m.cancelar()
	}
	m.cancelar = nil
	m.eventos = nil
	m.state = stateIdle
	m.interrompendo = false
	m.parcial = ""

	switch {
	case msg.Interrompida:
		if strings.TrimSpace(msg.Resposta) != "" {
			m.messages = append(m.messages, chatMessage{role: "assistant", content: msg.Resposta})
		}
		m.messages = append(m.messages, chatMessage{role: "nota", content: "resposta interrompida"})
	case msg.Err != nil:
		m.messages = append(m.messages, chatMessage{role: "error", content: msg.Err.Error()})
	default:
		m.messages = append(m.messages, chatMessage{role: "assistant", content: msg.Resposta})
		if m.config.SaveMessage != nil {
			m.config.SaveMessage("assistant", msg.Resposta, m.config.SessionID)
		}
	}
}

// etapaAtual devolve o rótulo da última etapa em andamento, se houver.
func (m Model) etapaAtual() string {
	for i := len(m.etapas) - 1; i >= 0; i-- {
		if m.etapas[i].Estado == ui.EtapaAtual {
			return m.etapas[i].Rotulo
		}
	}
	return ""
}

// renderizarHistorico acrescenta ao cache as mensagens ainda não renderizadas.
func (m *Model) renderizarHistorico() {
	if m.historicoLargura != m.viewport.Width || m.historicoN > len(m.messages) {
		m.historico, m.historicoN = "", 0
		m.historicoLargura = m.viewport.Width
	}
	if m.historicoN == len(m.messages) {
		return
	}
	var content strings.Builder
	content.WriteString(m.historico)

	for _, msg := range m.messages[m.historicoN:] {
		switch msg.role {
		case "user":
			content.WriteString(userStyle.Render("You > "))
			content.WriteString(msg.content)
			content.WriteString("\n\n")
		case "assistant":
			content.WriteString(bardStyle.Render("Bard > "))
			// Renderizar markdown se disponível
			if m.markdown != nil {
				rendered, err := m.markdown(msg.content)
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
		case "nota":
			content.WriteString(ui.S().TintaSuave.Render(msg.content))
			content.WriteString("\n\n")
		}
	}
	m.historico = content.String()
	m.historicoN = len(m.messages)
}

// updateViewport atualiza o conteúdo do viewport com as mensagens. Durante a
// resposta, só o parcial e o indicador mudam a cada trecho.
func (m *Model) updateViewport() {
	m.renderizarHistorico()
	var content strings.Builder
	content.WriteString(m.historico)

	if m.state == stateStreaming {
		// A resposta chega crua enquanto é gerada; o markdown vem no fim.
		if m.parcial != "" {
			content.WriteString(bardStyle.Render("Bard > "))
			content.WriteString(m.parcial)
			content.WriteString("\n\n")
		}
		indicador := "Pensando..."
		switch {
		case m.interrompendo:
			indicador = "Interrompendo..."
		case m.etapaAtual() != "":
			indicador = "Pensando... (" + m.etapaAtual() + ")"
		}
		content.WriteString(thinkingStyle.Render(indicador))
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

// classifyIntent classifica a intenção do usuário via IA.
func classifyIntent(ctx context.Context, provider ai.Provider, input string) *tools.IntentResult {
	allTools := tools.All()
	var intentList strings.Builder
	for _, t := range allTools {
		if len(t.Intents) > 0 {
			fmt.Fprintf(&intentList, "- %s: %s\n", t.Intents[0], t.Description)
		}
	}
	intentList.WriteString("- direct: responder diretamente sem executar ferramenta\n")

	classifyPrompt := prompts.Get("bard.classify")
	if classifyPrompt == "" {
		classifyPrompt = `Classifique a intencao. Responda APENAS JSON:
{"intent":"nome","params":{"chave":"valor"},"direct":false}
Se nao precisa de ferramenta: {"intent":"direct","params":{},"direct":true}`
	}

	userPrompt := fmt.Sprintf("Intencoes:\n%s\nUsuario: %s", intentList.String(), input)

	result, err := provider.Completion(ctx, classifyPrompt, userPrompt)
	if err != nil {
		return &tools.IntentResult{Direct: true}
	}

	var intent tools.IntentResult
	clean := strings.TrimSpace(result)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	if err := json.Unmarshal([]byte(clean), &intent); err != nil {
		return &tools.IntentResult{Direct: true}
	}

	return &intent
}

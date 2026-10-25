package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"encoding/json"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/bard/tools"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// maxToolIterations limita o número de iterações de tool calling.
const maxToolIterations = 5

// alturaCampo é a altura do campo de texto: a linha do prompt e uma para a quebra.
const alturaCampo = 2

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
	// AIModel é o rótulo do provider no cabeçalho (ex.: "claude-cli").
	AIModel     string
	SaveMessage func(role, content, sessionID string)
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
	// markdown renderiza a resposta na largura dada; nil mostra o texto cru.
	markdown func(texto string, largura int) (string, error)
	agora    func() time.Time
	err      error

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

	// ultimoFim é o desfecho da última rodada, para a linha de estado.
	ultimoFim *FimMsg
}

type chatMessage struct {
	role        string // "user", "assistant", "error", "nota"
	content     string
	hora        time.Time
	duracao     time.Duration     // tempo da rodada (respostas e erros)
	ferramentas []ferramentaUsada // executadas antes da resposta
}

// ferramentaUsada resume uma ferramenta da rodada para a linha antes da resposta.
type ferramentaUsada struct {
	rotulo, alvo string
	falhou       bool
	nota         string // na falha, a primeira linha do erro
	linhas       int    // linhas da saída
}

// New cria uma nova instância do modelo da TUI.
func New(provider ai.Provider, vectorStore *ai.VectorStore, config Config) Model {
	s := ui.S()
	ta := textarea.New()
	ta.Placeholder = ""
	ta.CharLimit = 4096
	ta.ShowLineNumbers = false
	// Do padrão do textarea fica só a base; a linha do cursor perde o fundo.
	estilo, _ := textarea.DefaultStyles()
	estilo.CursorLine, estilo.Text, estilo.Placeholder = s.Tinta, s.Tinta, s.TintaSuave
	estilo.Prompt, estilo.EndOfBuffer = s.Ok, s.Fio
	ta.FocusedStyle, ta.BlurredStyle = estilo, estilo
	ta.Cursor.Style = s.Tinta
	ta.SetPromptFunc(2, func(linha int) string {
		if linha == 0 {
			return ui.G().Prompt + " "
		}
		return "  "
	})
	ta.SetHeight(alturaCampo)
	ta.Focus()

	vp := viewport.New(80, 20)
	vp.SetContent("")

	md := &markdownCache{}
	return Model{
		viewport:    vp,
		textarea:    ta,
		provider:    provider,
		vectorStore: vectorStore,
		config:      config,
		state:       stateIdle,
		markdown:    md.renderizar,
		agora:       time.Now,
		fluxo:       novoFluxo(provider, vectorStore, config),
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
			m.messages = append(m.messages, chatMessage{role: "user", content: input, hora: m.agora()})
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
		m.viewport.Width = m.width
		m.viewport.Height = ui.AlturaCorpoTexto(m.height, alturaCampo)
		m.textarea.SetWidth(m.width)
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
	m.ultimoFim = &msg

	resposta := chatMessage{
		role:        "assistant",
		content:     msg.Resposta,
		hora:        m.agora(),
		duracao:     msg.Duracao,
		ferramentas: m.ferramentasDaRodada(),
	}
	switch {
	case msg.Interrompida:
		if strings.TrimSpace(msg.Resposta) != "" {
			m.messages = append(m.messages, resposta)
		}
		m.messages = append(m.messages, chatMessage{role: "nota", content: "resposta interrompida"})
	case msg.Err != nil:
		resposta.role, resposta.content = "error", msg.Err.Error()
		m.messages = append(m.messages, resposta)
	default:
		m.messages = append(m.messages, resposta)
		if m.config.SaveMessage != nil {
			m.config.SaveMessage("assistant", msg.Resposta, m.config.SessionID)
		}
	}
}

// ferramentasDaRodada resume as ferramentas que terminaram na rodada.
func (m Model) ferramentasDaRodada() []ferramentaUsada {
	var usadas []ferramentaUsada
	for _, e := range m.etapas {
		if e.Etapa != EtapaFerramenta || e.Estado == ui.EtapaAtual {
			continue
		}
		f := ferramentaUsada{rotulo: e.Rotulo, alvo: e.Nota, linhas: contarLinhas(e.Saida)}
		if e.Estado == ui.EtapaFalha {
			// Na falha a nota é o erro; o alvo fica só no rótulo.
			f.falhou, f.alvo, f.nota, f.linhas = true, "", e.Nota, 0
		}
		usadas = append(usadas, f)
	}
	return usadas
}

// contarLinhas conta as linhas de uma saída, sem a quebra final.
func contarLinhas(s string) int {
	s = strings.TrimRight(s, "\n")
	if strings.TrimSpace(s) == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
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
		content.WriteString(m.renderizarMensagem(msg))
	}
	m.historico = content.String()
	m.historicoN = len(m.messages)
}

// updateViewport atualiza o conteúdo do viewport com as mensagens. Durante a
// resposta, só o parcial muda a cada trecho.
func (m *Model) updateViewport() {
	m.renderizarHistorico()
	content := m.historico
	if m.state == stateStreaming {
		content += m.renderizarParcial()
	}
	m.viewport.SetContent(strings.TrimRight(content, "\n"))
	m.viewport.GotoBottom()
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

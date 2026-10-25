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
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// maxToolIterations limita o número de iterações de tool calling.
const maxToolIterations = 5

// alturaCampo é a altura mínima do campo de texto: a linha do prompt e uma para a
// quebra. A cada nova linha (ctrl+j) o campo cresce uma linha, até alturaMaxCampo.
const (
	alturaCampo    = 2
	alturaMaxCampo = 6
)

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
	// markdown renderiza a resposta na largura dada, com a tecla da cópia no comando de
	// índice comando (-1 nenhum); nil mostra o texto cru.
	markdown func(texto string, largura, comando int) (string, error)
	agora    func() time.Time
	err      error

	// Histórico já renderizado: as mensagens só crescem, então cada uma passa
	// pelo glamour uma vez; a largura nova refaz tudo. Assim um trecho da
	// resposta não paga a renderização da conversa inteira.
	historico        string
	historicoN       int
	historicoLargura int
	historicoLinhas  int      // linhas do histórico renderizado
	inicios          []int    // linha em que começa cada mensagem do histórico
	blocosHistorico  []string // bloco renderizado de cada mensagem
	// dicaEm é a mensagem renderizada com "↵ ver saída" (-1 nenhuma) e copiaEm a que tem
	// o quadro "COMANDO · ctrl+y copia"; quando uma delas muda de lugar ou some, a
	// mensagem é renderizada de novo.
	dicaEm, copiaEm int

	// lidas conta as mensagens que já passaram pela tela: todas as da última vez que
	// a conversa esteve no fim. As de depois que ainda começam abaixo da janela são
	// as mensagens novas. parcialVisto é o tamanho da resposta em andamento naquela
	// vez: ela só é nova se chegou texto depois.
	lidas        int
	parcialVisto int

	// Rodada em andamento: eventos do fluxo, cancelamento (esc) e o que já chegou.
	fluxo         *Fluxo
	rodada        int
	eventos       <-chan tea.Msg
	cancelar      context.CancelFunc
	interrompendo bool
	etapas        []EtapaMsg
	parcial       string
	parcialCache  cacheParcial

	// Relógio da rodada: o início e o último instante lido (a cada evento ou tique). Os
	// tempos ao vivo saem da diferença, sem ler o relógio ao renderizar.
	inicioRodada  time.Time
	instante      time.Time
	iniciosEtapas []time.Time // início de cada etapa de m.etapas

	// ultimoFim é o desfecho da última rodada, para a linha de estado.
	ultimoFim *FimMsg
	// spinner gira na linha de estado durante a resposta (◈ fixo sem TTY ou sem cor);
	// novoSpinner o cria a cada rodada (trocado nos testes).
	spinner     ui.Spinner
	novoSpinner func(rotulo string) ui.Spinner

	// saida é a camada com a saída de uma ferramenta (↵ no campo vazio); nil fechada.
	saida *vistaSaida
	// copiar manda texto para a área de transferência (OSC 52; trocado nos testes).
	copiar func(texto string) error
	// aviso é o resultado da última cópia na linha de estado, até a próxima tecla ou o
	// fim do prazo; avisos numera os avisos.
	aviso  *avisoEstado
	avisos int
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
	saida        string // saída completa, para o ↵
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

	// O histórico rola só com pgup/pgdn e a roda do mouse: as letras são do campo.
	vp := viewport.New(80, 20)
	vp.KeyMap = viewport.KeyMap{PageUp: ui.DefaultKeys().Texto.RolarCima, PageDown: ui.DefaultKeys().Texto.RolarBaixo}
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
		dicaEm:      -1,
		copiaEm:     -1,
		copiar:      copiarOSC52,
		novoSpinner: ui.NovoSpinner,
	}
}

// Init inicializa o modelo do Bubbletea.
func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update processa mensagens do Bubbletea.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.teclar(msg)

	case tea.MouseMsg:
		// A roda do mouse rola o histórico (ou a saída aberta); o campo não usa o mouse.
		if m.saida != nil {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.rolarSaida(-linhasRoda)
			case tea.MouseButtonWheelDown:
				m.rolarSaida(linhasRoda)
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		m.marcarLidas()
		return m, cmd

	case tea.WindowSizeMsg:
		noFim := m.viewport.AtBottom()
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = m.width
		m.textarea.SetWidth(m.width)
		m.viewport.Height = ui.AlturaCorpoTexto(m.height, m.textarea.Height())
		m.updateViewport()
		if noFim {
			m.viewport.GotoBottom()
			m.marcarLidas()
		}
		m.ajustarSaida()
		return m, nil

	case EtapaMsg:
		if msg.Rodada != m.rodada {
			return m, nil
		}
		m.instante = m.agora()
		if msg.Etapa == EtapaResposta && msg.Tentativa > 1 {
			// A resposta recomeçou: o texto da tentativa anterior não vale.
			m.parcial, m.parcialVisto = "", 0
		}
		m.registrarEtapa(msg)
		m.updateViewport()
		return m, m.proximoEvento()

	case TrechoMsg:
		if msg.Rodada != m.rodada {
			return m, nil
		}
		m.instante = m.agora()
		m.parcial += msg.Texto
		m.updateViewport()
		return m, m.proximoEvento()

	case tiqueMsg:
		// Um intervalo sem eventos: os tempos da tela andam e a leitura continua.
		if msg.rodada != m.rodada || m.eventos == nil {
			return m, nil
		}
		m.instante = m.agora()
		m.updateViewport()
		return m, m.proximoEvento()

	case copiadoMsg:
		return m, m.mostrarAviso(avisoCopia(msg))

	case limparAvisoMsg:
		if m.aviso != nil && m.aviso.id == msg.id {
			m.aviso = nil
		}
		return m, nil

	case ui.SpinnerTickMsg:
		// O spinner só gira durante a resposta: fora dela o tique morre aqui.
		if m.state != stateStreaming {
			return m, nil
		}
		atualizado, cmd := m.spinner.Update(msg)
		m.spinner = atualizado.(ui.Spinner)
		return m, cmd

	case FimMsg:
		if msg.Rodada != m.rodada {
			return m, nil
		}
		m.encerrarRodada(msg)
		m.updateViewport()
		return m, nil
	}

	// O resto (o piscar do cursor) é do campo.
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// teclar trata as teclas como na coluna "Texto" dos atalhos: o teclado é do campo e
// só pgup e pgdn rolam o histórico. Durante a resposta o campo aceita a próxima
// pergunta, mas ↵ só envia quando a rodada termina.
func (m Model) teclar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := ui.DefaultKeys()
	m.aviso = nil
	if msg.Type == tea.KeyCtrlC {
		if m.cancelar != nil {
			m.cancelar()
		}
		return m, tea.Quit
	}
	if m.saida != nil {
		return m.teclarSaida(msg)
	}
	vazio := m.campoVazio()
	switch {
	case msg.Type == tea.KeyEsc:
		// esc interrompe a resposta em andamento sem fechar o Bard; o
		// FimMsg da rodada é que devolve o estado ocioso.
		if m.state == stateStreaming && m.cancelar != nil && !m.interrompendo {
			m.interrompendo = true
			m.cancelar()
			m.updateViewport()
		}
		return m, nil

	case key.Matches(msg, k.Texto.RolarCima, k.Texto.RolarBaixo):
		m.rolarPagina(key.Matches(msg, k.Texto.RolarCima))
		return m, nil

	case novaLinha(msg):
		// A quebra vai pelo próprio textarea (o ↵ dele é "nova linha"), que leva a
		// vista do campo até a linha do cursor.
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m.ajustarCampo()
		m.seCampoMudou(vazio)
		return m, cmd

	case msg.Type == tea.KeyEnter:
		// No campo vazio, ↵ abre a saída da última ferramenta ("↵ ver").
		if m.campoVazio() && m.abrirSaida() {
			return m, nil
		}
		return m.enviar()

	case key.Matches(msg, teclaCopiar):
		// ctrl+y copia o último comando da última resposta; sem ele, não faz nada.
		if !m.podeCopiarComando() {
			return m, nil
		}
		comando, _ := m.ultimoComando()
		return m, m.copiarTexto(comando, "comando")

	case key.Matches(msg, k.Texto.EditarUltima) && m.podeEditarUltima():
		// ↑ no campo vazio traz a última pergunta para editar e enviar de novo.
		m.textarea.SetValue(m.ultimaPergunta())
		m.ajustarCampo()
		m.seCampoMudou(vazio)
		return m, nil
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.ajustarCampo()
	m.seCampoMudou(vazio)
	return m, cmd
}

// seCampoMudou redesenha a conversa quando o campo passa de vazio a escrito ou volta:
// as dicas "↵ ver" só valem com o campo vazio, porque com texto o ↵ envia.
func (m *Model) seCampoMudou(vazioAntes bool) {
	if m.campoVazio() != vazioAntes {
		m.updateViewport()
	}
}

// novaLinha diz se a tecla quebra a linha do campo: ctrl+j, que chega igual em todo
// terminal (no Windows também), ou alt+↵ (ESC + CR). O ⇧↵ da prancha não chega: o
// Bubble Tea v1 não tem shift+enter e recebe a combinação como ↵ (no Windows o
// VK_RETURN ignora o shift; xterm, GNOME Terminal, Terminal.app e o Windows Terminal
// mandam CR puro). Por isso a legenda mostra ctrl+j.
func novaLinha(msg tea.KeyMsg) bool {
	return key.Matches(msg, ui.DefaultKeys().Texto.NovaLinha) || (msg.Type == tea.KeyEnter && msg.Alt)
}

// campoVazio diz se o campo não tem nada para enviar (só espaços também conta).
func (m Model) campoVazio() bool {
	return strings.TrimSpace(m.textarea.Value()) == ""
}

// enviar manda o texto do campo como pergunta e abre a rodada.
func (m Model) enviar() (tea.Model, tea.Cmd) {
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
	m.ajustarCampo()
	m.updateViewport()
	// Quem pergunta quer ver a resposta: a conversa volta para o fim.
	m.viewport.GotoBottom()
	m.marcarLidas()

	// Salvar mensagem do usuário
	if m.config.SaveMessage != nil {
		m.config.SaveMessage("user", input, m.config.SessionID)
	}

	return m, m.iniciarRodada(input)
}

// podeEditarUltima diz se ↑ traz a última pergunta: fora da resposta, com o campo
// vazio e alguma pergunta feita. Com texto no campo, ↑ move o cursor.
func (m Model) podeEditarUltima() bool {
	return m.state == stateIdle && m.textarea.Value() == "" && m.ultimaPergunta() != ""
}

// ultimaPergunta devolve o texto da última mensagem do usuário.
func (m Model) ultimaPergunta() string {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].role == "user" {
			return m.messages[i].content
		}
	}
	return ""
}

// ajustarCampo dá ao campo uma linha por linha do texto, entre alturaCampo e
// alturaMaxCampo, e devolve o resto ao histórico, que segue no fim se já estava.
func (m *Model) ajustarCampo() {
	altura := min(max(m.textarea.LineCount(), alturaCampo), alturaMaxCampo)
	if altura == m.textarea.Height() {
		m.reposicionarCampo()
		return
	}
	defer m.reposicionarCampo()
	noFim := m.viewport.AtBottom()
	m.textarea.SetHeight(altura)
	if m.height > 0 {
		m.viewport.Height = ui.AlturaCorpoTexto(m.height, altura)
	}
	if noFim {
		m.viewport.GotoBottom()
	}
	m.marcarLidas()
}

// reposicionarCampo leva a vista do campo até a linha do cursor. O textarea só faz
// isso no fim do próprio Update e com as linhas do último View: sem este passo, a
// linha nova (ou o fim de uma pergunta trazida pelo ↑) ficaria fora do campo até a
// tecla seguinte.
func (m *Model) reposicionarCampo() {
	_ = m.textarea.View()
	m.textarea, _ = m.textarea.Update(nil)
}

// iniciarRodada dispara o fluxo da pergunta e devolve o comando que lê o
// primeiro evento; cada evento tratado em Update pede o seguinte até o FimMsg.
func (m *Model) iniciarRodada(input string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.rodada++
	m.cancelar = cancel
	m.interrompendo = false
	m.etapas, m.iniciosEtapas = nil, nil
	m.parcial, m.parcialVisto = "", 0
	m.inicioRodada = m.agora()
	m.instante = m.inicioRodada
	m.eventos = m.fluxo.Iniciar(ctx, m.rodada, input)
	m.spinner = m.novoSpinner("")
	return tea.Batch(m.proximoEvento(), m.spinner.Init())
}

// proximoEvento pede o evento seguinte da rodada, ou um tique se ele demorar; sem
// rodada aberta não há o que ler (um canal nil travaria o comando para sempre).
func (m Model) proximoEvento() tea.Cmd {
	if m.eventos == nil {
		return nil
	}
	return esperarEvento(m.eventos, m.rodada)
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
	m.parcialCache = cacheParcial{}
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
		f := ferramentaUsada{rotulo: e.Rotulo, alvo: e.Nota, linhas: contarLinhas(e.Saida), saida: e.Saida}
		if e.Estado == ui.EtapaFalha {
			// Na falha a nota é o erro; o alvo fica só no rótulo.
			f.falhou, f.alvo, f.nota, f.linhas, f.saida = true, "", e.Nota, 0, ""
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

// renderizarHistorico acrescenta ao cache as mensagens ainda não renderizadas. A
// mensagem que ganha ou perde a dica "↵ ver saída" ou o "ctrl+y copia" é renderizada de
// novo, com as seguintes.
func (m *Model) renderizarHistorico() {
	if m.historicoLargura != m.viewport.Width || m.historicoN > len(m.messages) {
		m.cortarHistorico(0)
		m.historicoLargura = m.viewport.Width
	}
	dica, copia := m.mensagemComDica(), m.mensagemComCopia()
	for _, par := range [][2]int{{dica, m.dicaEm}, {copia, m.copiaEm}} {
		if par[0] == par[1] {
			continue
		}
		for _, i := range par {
			if i >= 0 && i < m.historicoN {
				m.cortarHistorico(i)
			}
		}
	}
	m.dicaEm, m.copiaEm = dica, copia
	if m.historicoN == len(m.messages) {
		return
	}
	for i := m.historicoN; i < len(m.messages); i++ {
		bloco := m.renderizarMensagem(m.messages[i], i == dica, i == copia)
		m.inicios = append(m.inicios, m.historicoLinhas)
		m.blocosHistorico = append(m.blocosHistorico, bloco)
		m.historicoLinhas += strings.Count(bloco, "\n")
	}
	m.historico = strings.Join(m.blocosHistorico, "")
	m.historicoN = len(m.messages)
}

// cortarHistorico descarta do cache a mensagem i e as seguintes.
func (m *Model) cortarHistorico(i int) {
	if i >= m.historicoN {
		return
	}
	m.historicoLinhas = 0
	if i < len(m.inicios) {
		m.historicoLinhas = m.inicios[i]
	}
	m.inicios, m.blocosHistorico = m.inicios[:i], m.blocosHistorico[:i]
	m.historico = strings.Join(m.blocosHistorico, "")
	m.historicoN = i
}

// mensagemComDica é a mensagem que mostra "↵ ver saída" na ferramenta: a última
// resposta, fora da rodada e com o campo vazio, se a saída dela pode ser aberta.
func (m Model) mensagemComDica() int {
	if m.state == stateStreaming {
		return -1
	}
	if _, ok := m.saidaDisponivel(); !ok {
		return -1
	}
	return m.ultimaResposta()
}

// updateViewport atualiza o conteúdo do viewport com as mensagens. Durante a
// resposta, só o parcial muda a cada trecho. A conversa segue o fim só se já
// estava nele: quem rolou para cima fica onde está e vê o aviso das mensagens novas.
func (m *Model) updateViewport() {
	noFim := m.viewport.AtBottom()
	m.renderizarHistorico()
	content := m.historico
	if m.state == stateStreaming {
		content += m.renderizarParcial()
	}
	m.viewport.SetContent(strings.TrimRight(content, "\n"))
	if noFim {
		m.viewport.GotoBottom()
	}
	m.marcarLidas()
}

// blocos conta os blocos da conversa: as mensagens e, durante a resposta, o parcial.
func (m Model) blocos() int {
	if m.state == stateStreaming {
		return len(m.messages) + 1
	}
	return len(m.messages)
}

// marcarLidas dá por lidas as mensagens quando a conversa está no fim. Da resposta em
// andamento fica lido o texto que já chegou: o que vier depois que a conversa sair do
// fim ainda é novo.
func (m *Model) marcarLidas() {
	if m.viewport.AtBottom() {
		m.lidas = len(m.messages)
		m.parcialVisto = len(m.parcial)
	}
}

// linhasAviso são as linhas do pé da janela que o aviso de mensagens novas ocupa: uma
// em branco e a do aviso (só a do aviso numa janela de até 2 linhas).
func (m Model) linhasAviso() int {
	if m.viewport.Height > 2 {
		return 2
	}
	return 1
}

// mensagensAnteriores conta as mensagens que começam acima da janela ou nas linhas que o
// aviso do topo cobre. Na conversa do topo (ou numa janela baixa, onde os dois avisos
// não cabem) não há aviso. As notas não contam.
func (m Model) mensagensAnteriores() int {
	if m.viewport.YOffset == 0 || !m.cabemDoisAvisos() {
		return 0
	}
	topo := m.viewport.YOffset + m.linhasAviso()
	n := 0
	for i, inicio := range m.inicios {
		if inicio >= topo || i >= len(m.messages) {
			break
		}
		if m.messages[i].role != "nota" {
			n++
		}
	}
	return n
}

// cabemDoisAvisos diz se a janela tem lugar para o aviso do topo e o do pé com ao menos
// uma linha da conversa entre eles.
func (m Model) cabemDoisAvisos() bool {
	return m.viewport.Height > 2*m.linhasAviso()
}

// rolarPagina anda uma página (pgup ou pgdn) descontando as linhas que os avisos cobrem
// no topo e no pé: o que um aviso escondia aparece na página seguinte, e nada da
// conversa deixa de passar pela tela.
func (m *Model) rolarPagina(acima bool) {
	passo := m.viewport.Height
	if m.mensagensNovas() > 0 {
		passo -= m.linhasAviso()
	}
	if acima {
		if m.mensagensAnteriores() > 0 {
			passo -= m.linhasAviso()
		}
		m.viewport.ScrollUp(max(passo, 1))
	} else {
		// Descendo, o topo da página nova fica sob o aviso das anteriores.
		if m.cabemDoisAvisos() {
			passo -= m.linhasAviso()
		}
		m.viewport.ScrollDown(max(passo, 1))
	}
	m.marcarLidas()
}

// mensagensNovas conta as mensagens que chegaram depois que a conversa saiu do fim e
// ainda começam fora da vista: abaixo da janela ou nas linhas que o aviso cobre. As
// notas (como "resposta interrompida") não contam; a resposta em andamento conta só
// se chegou texto depois da última vez que a conversa esteve no fim.
func (m Model) mensagensNovas() int {
	fim := m.viewport.YOffset + m.viewport.Height - m.linhasAviso()
	n := 0
	for i := m.lidas; i < m.blocos(); i++ {
		inicio := m.historicoLinhas // o parcial começa depois do histórico
		if i < len(m.messages) {
			if m.messages[i].role == "nota" || i >= len(m.inicios) {
				continue
			}
			inicio = m.inicios[i]
		} else if len(m.parcial) <= m.parcialVisto {
			continue
		}
		if inicio >= fim {
			n++
		}
	}
	return n
}

// Run inicia a TUI do Bard.
func Run(provider ai.Provider, vectorStore *ai.VectorStore, config Config) error {
	model := New(provider, vectorStore, config)
	// O mouse vem para o Bard por causa da roda; com isso o arraste simples deixa de
	// selecionar texto, e copiar um trecho pede shift+arraste (Windows Terminal, xterm).
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
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

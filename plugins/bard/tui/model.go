package tui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ai"
	"github.com/casheiro/yby-cli/v5/pkg/ai/prompts"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/casheiro/yby-cli/v5/plugins/bard/tools"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

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
	// AIModel é o rótulo do provider no cabeçalho (ex.: "claude-cli"); ModeloIA é o
	// modelo escolhido no /modelo, que aparece ao lado ("claude-cli · opus"). Vazio, o
	// provider usa o modelo padrão dele.
	AIModel  string
	ModeloIA string
	// Rascunho é a pergunta que abre escrita no campo, sem enviar (yby bard --pergunta,
	// o b do Viz): a pessoa revisa e aperta ↵.
	Rascunho string
	// Voltar é o instrumento que abriu o Bard (yby bard --voltar, o b do Viz): fora da
	// resposta, o esc fecha o Bard e volta a ele, e a legenda mostra "esc voltar ao Viz".
	// Vazio, o esc só interrompe a resposta.
	Voltar      string
	SaveMessage func(role, content, sessionID string)
	// TopK e LimiarRelevancia controlam a busca no Synapstor, como vêm do bard.yaml
	// (padrões 5 e ai.RelevanciaMinima no loadBardConfig). TopK 0 vira 5; limiar 0
	// aceita todas as notas.
	TopK             int
	LimiarRelevancia float64

	// Comandos /. Todos opcionais: sem a função, o comando avisa que não está
	// disponível. Sessoes e RetomarSessao leem o histórico (history.go); Modelos detecta
	// os providers de IA (pode demorar e roda fora do Update); Contexto e Ferramentas
	// alimentam o /contexto e o /ferramentas (Ferramentas nil usa tools.Situacoes).
	Sessoes       func() ([]Sessao, error)
	RetomarSessao func(id string) (Retomada, error)
	Modelos       func(ctx context.Context) []Modelo
	Contexto      func() InfoContexto
	Ferramentas   func() []tools.Situacao
	// Diretorio é onde o /exportar grava (vazio, o diretório atual); Sanitizar tira os
	// segredos do markdown antes de gravar.
	Diretorio string
	Sanitizar func(string) string
	// Historico conta o histórico que o SystemPrompt leva, para o /contexto; o /sessões o
	// troca pelo da sessão retomada.
	Historico HistoricoPrompt
}

// Model é o modelo principal do Bubbletea para o Bard.
type Model struct {
	viewport viewport.Model
	textarea textarea.Model
	// medidor é um campo igual ao textarea, na mesma largura, que mede quantas linhas da
	// tela cada linha do texto ocupa (o LineCount do textarea conta só as do texto).
	medidor     textarea.Model
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
	// conversa são as perguntas desta sessão com as respostas, que vão com a pergunta
	// seguinte (as de uma sessão retomada já vão pelo system prompt); pergunta é a da
	// rodada em andamento.
	conversa []Troca
	pergunta string
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

	// comandoSel é o comando marcado na lista de comandos aberta; seletor é a lista que
	// /sessões e /modelo abrem (nil fechada); pedidos numera as detecções do /modelo.
	comandoSel string
	seletor    *seletor
	pedidos    int
	// trocas numera as trocas do /modelo (uma escolha nova invalida a criação anterior);
	// trocando é o "claude-cli · opus" que está sendo criado, para a linha de estado, e
	// trocaPronta é a criação que chegou durante a resposta e espera o fim da rodada.
	trocas      int
	trocando    string
	trocaPronta *modeloCriadoMsg
	// novoEstado cria o "procurando" do /modelo (trocado nos testes).
	novoEstado func(texto string) ui.Estado
	// ajuda é a camada com as teclas que o ? abre durante a resposta, sem entrar no meio
	// da rodada; fora dela o ? põe o quadro na conversa, como o /ajuda.
	ajuda bool
	// rascunhoIntocado diz que o campo ainda tem o rascunho como abriu, sem tecla
	// nenhuma: a cada largura nova ele volta a mostrar o começo da pergunta.
	rascunhoIntocado bool
	// interrompido diz que o Bard fechou pelo ctrl+c: o Run devolve ErrInterrompido.
	interrompido bool
	// abertura é o system prompt da abertura, para o /limpar voltar a ele; exportados são
	// os arquivos que o /exportar gravou nesta execução e pode regravar.
	abertura struct {
		prompt    string
		historico HistoricoPrompt
	}
	exportados []os.FileInfo
}

type chatMessage struct {
	role        string // "user", "assistant", "error", "nota", "quadro"
	content     string
	comando     bool       // fala do usuário que foi um comando /, que não vai para a IA
	quadro      *quadroMsg // resultado de um comando (role "quadro")
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
	// O histórico rola só com pgup/pgdn e a roda do mouse: as letras são do campo.
	vp := viewport.New(80, 20)
	vp.KeyMap = viewport.KeyMap{PageUp: ui.DefaultKeys().Texto.RolarCima, PageDown: ui.DefaultKeys().Texto.RolarBaixo}
	vp.SetContent("")

	md := &markdownCache{}
	m := Model{
		viewport:    vp,
		textarea:    novoCampo(),
		medidor:     novoCampo(),
		provider:    provider,
		vectorStore: vectorStore,
		config:      config,
		state:       stateIdle,
		markdown:    md.renderizar,
		agora:       time.Now,
		fluxo:       NovoFluxo(provider, vectorStore, config),
		dicaEm:      -1,
		copiaEm:     -1,
		copiar:      copiarOSC52,
		novoSpinner: ui.NovoSpinner,
		novoEstado:  ui.NovoEstadoCarregando,
	}
	m.abertura.prompt, m.abertura.historico = config.SystemPrompt, config.Historico
	if config.Rascunho != "" {
		// Só escreve: o envio é do ↵ da pessoa. O campo cresce com as linhas da pergunta
		// na tela, e a pergunta aparece desde o começo.
		m.textarea.SetValue(config.Rascunho)
		m.rascunhoIntocado = true
		m.ajustarCampo()
		m.mostrarRascunho()
	}
	return m
}

// novoCampo cria o campo de texto da conversa (o do textarea e o do medidor).
func novoCampo() textarea.Model {
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
	return ta
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
		m.medidor.SetWidth(m.width)
		// Na largura nova o texto quebra em outras linhas: o campo muda de altura, e o
		// rascunho intocado volta a mostrar o começo.
		m.ajustarCampo()
		if m.rascunhoIntocado {
			m.mostrarRascunho()
		}
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

	case modelosMsg:
		return m, m.receberModelos(msg)
	case modeloCriadoMsg:
		return m, m.receberModeloCriado(msg)

	case limparAvisoMsg:
		if m.aviso != nil && m.aviso.id == msg.id {
			m.aviso = nil
		}
		return m, nil

	case ui.SpinnerTickMsg:
		// O spinner da rodada só gira durante a resposta e o do /modelo enquanto a
		// detecção não chega: fora disso o tique morre aqui (cada um só aceita o seu).
		girar := m.girarEstado(msg)
		if m.state != stateStreaming {
			return m, girar
		}
		atualizado, cmd := m.spinner.Update(msg)
		m.spinner = atualizado.(ui.Spinner)
		return m, tea.Batch(cmd, girar)

	case FimMsg:
		if msg.Rodada != m.rodada {
			return m, nil
		}
		m.encerrarRodada(msg)
		// A troca do /modelo que chegou durante a resposta vale a partir daqui: a resposta
		// que acabou é do provider antigo e a nota da troca vem depois dela.
		if t := m.trocaPronta; t != nil {
			m.trocaPronta, m.trocando = nil, ""
			m.usarProvider(t.mod, t.modelo, t.provider)
		}
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
	m.rascunhoIntocado = false
	if msg.Type == tea.KeyCtrlC {
		// ctrl+c sai de qualquer lugar (folha Atalhos): o Run devolve ErrInterrompido, o yby
		// sai com 130 e quem abriu o Bard (o Viz) fecha também.
		if m.cancelar != nil {
			m.cancelar()
		}
		m.interrompido = true
		return m, tea.Quit
	}
	if m.saida != nil {
		return m.teclarSaida(msg)
	}
	if m.seletor != nil {
		return m.teclarSeletor(msg)
	}
	if m.ajuda {
		// Com a ajuda aberta só o esc vale: fecha a camada, sem interromper a resposta.
		if msg.Type == tea.KeyEsc {
			m.ajuda = false
		}
		return m, nil
	}
	if lista := m.comandosVisiveis(); len(lista) > 0 {
		if modelo, cmd, tratou := m.teclarComandos(msg, lista); tratou {
			return modelo, cmd
		}
	}
	vazio := m.campoVazio()
	switch {
	case msg.Type == tea.KeyRunes && !msg.Paste && string(msg.Runes) == "?" && m.textarea.Value() == "":
		// ? no campo vazio abre a ajuda; no meio do texto é só um ponto de interrogação.
		// Durante a resposta ela abre numa camada, para não entrar no meio da rodada.
		if m.state == stateStreaming {
			m.ajuda = true
			return m, nil
		}
		return m.executarComando("ajuda", "")

	case msg.Type == tea.KeyEsc:
		// esc interrompe a resposta em andamento sem fechar o Bard; o
		// FimMsg da rodada é que devolve o estado ocioso.
		if m.state == stateStreaming {
			if m.cancelar != nil && !m.interrompendo {
				m.interrompendo = true
				m.cancelar()
				m.updateViewport()
			}
			return m, nil
		}
		// Fora da resposta, com o Bard aberto por outro instrumento (o b do Viz), o esc
		// sobe um nível: fecha o Bard, e o yby sai com 0 de volta a ele.
		if m.config.Voltar != "" {
			return m, tea.Quit
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
	if m.comandosVisiveis() == nil {
		// A lista fechou (o campo esvaziou ou saiu do filtro): a próxima / reabre com o
		// primeiro comando marcado, como na prancha.
		m.comandoSel = ""
	}
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

	// Processar comandos: "/nome argumentos" roda no Bard; uma barra que não é de
	// comando segue como pergunta.
	if nome, args, ok := comandoDigitado(input); ok {
		return m.executarComando(nome, args)
	}
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

// ultimaPergunta devolve o texto da última pergunta feita à IA; os comandos / ficam de
// fora, para o ↑ não trazer "/ajuda" e reabrir a lista.
func (m Model) ultimaPergunta() string {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].role == "user" && !m.messages[i].comando {
			return m.messages[i].content
		}
	}
	return ""
}

// ajustarCampo dá ao campo uma linha por linha do texto na tela, entre alturaCampo e
// alturaMaxCampo, e devolve o resto ao histórico, que segue no fim se já estava.
func (m *Model) ajustarCampo() {
	linhas := m.linhasDoCampo()
	altura := min(max(linhas, alturaCampo), alturaMaxCampo)
	if linhas <= altura {
		// O texto cabe inteiro: a vista do campo começa na primeira linha.
		defer m.vistaDoComeco()
	}
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

// linhasDoCampo conta as linhas que o texto do campo ocupa na tela: cada linha do texto
// ocupa as linhas em que o textarea a quebra na largura atual. O medidor é um campo igual,
// na mesma largura, e quebra do mesmo jeito.
func (m *Model) linhasDoCampo() int {
	n := 0
	for _, linha := range strings.Split(m.textarea.Value(), "\n") {
		m.medidor.SetValue(linha)
		n += max(m.medidor.LineInfo().Height, 1)
	}
	return n
}

// mostrarRascunho mostra o rascunho desde o começo, para quem abriu o Bard pelo Viz ler a
// pergunta antes do ↵: o cursor fica no fim quando ela cabe inteira no campo (já na altura
// nova) e no começo quando não cabe, porque a vista do textarea segue o cursor.
func (m *Model) mostrarRascunho() {
	tecla := tea.KeyCtrlEnd
	if m.linhasDoCampo() > m.textarea.Height() {
		tecla = tea.KeyCtrlHome
	}
	m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tecla})
	m.reposicionarCampo()
	m.vistaDoComeco()
}

// maxPassosCursor limita a volta do cursor em vistaDoComeco (uma por linha da tela).
const maxPassosCursor = 4096

// vistaDoComeco leva a vista do campo à primeira linha quando o texto cabe inteiro nele.
// O textarea só rola até o cursor: quando o campo cresce (o texto quebrou numa linha nova,
// ou a janela alargou), a vista guarda o deslocamento da altura antiga e esconde o começo.
// Sem acesso à vista, o cursor vai ao começo (a vista sobe) e volta aonde estava.
func (m *Model) vistaDoComeco() {
	if m.linhasDoCampo() > m.textarea.Height() || strings.HasPrefix(ansi.Strip(m.textarea.View()), ui.G().Prompt) {
		return // não cabe, ou a primeira linha (a do prompt) já está na vista
	}
	info := m.textarea.LineInfo()
	linha, coluna := m.textarea.Line(), info.StartColumn+info.ColumnOffset
	m.textarea, _ = m.textarea.Update(tea.KeyMsg{Type: tea.KeyCtrlHome})
	m.reposicionarCampo()
	for i := 0; m.textarea.Line() < linha && i < maxPassosCursor; i++ {
		m.textarea.CursorDown()
	}
	m.textarea.SetCursor(coluna)
	m.reposicionarCampo()
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
	m.pergunta = input
	m.eventos = m.fluxo.Iniciar(ctx, m.rodada, input, m.conversa...)
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
// estado ocioso. Resposta interrompida mostra o que chegou e vai ao histórico com a marca.
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
	// Só as últimas MaxTrocas vão à IA; as outras nem ficam guardadas.
	m.conversa = append(m.conversa, TrocaDe(m.pergunta, msg))
	if n := len(m.conversa); n > MaxTrocas {
		m.conversa = append([]Troca(nil), m.conversa[n-MaxTrocas:]...)
	}

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
		// No histórico salvo vai o que chegou com a marca, para o /sessões mostrar a
		// troca inteira e a IA saber, ao retomar, que a resposta foi cortada.
		if m.config.SaveMessage != nil {
			m.config.SaveMessage("assistant", textoInterrompido(msg.Resposta), m.config.SessionID)
		}
	case msg.Err != nil:
		resposta.role, resposta.content = "error", msg.Err.Error()
		m.messages = append(m.messages, resposta)
	default:
		m.messages = append(m.messages, resposta)
		if m.config.SaveMessage != nil {
			m.config.SaveMessage("assistant", msg.Resposta, m.config.SessionID)
		}
	}
	// As etapas somem com a rodada: o que pede ação do usuário fica na conversa.
	for _, e := range m.etapas {
		if e.Aviso != "" {
			m.messages = append(m.messages, chatMessage{role: "nota", content: e.Aviso})
		}
	}
}

// marcaInterrompida fecha, no histórico salvo, uma resposta cortada com esc.
const marcaInterrompida = "*resposta interrompida*"

// textoInterrompido é a resposta parcial seguida da marca; sem nada recebido, só a marca.
func textoInterrompido(parcial string) string {
	if parcial = strings.TrimSpace(parcial); parcial == "" {
		return marcaInterrompida
	}
	return parcial + "\n\n" + marcaInterrompida
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

// linhasAbaixo conta as linhas da conversa abaixo da janela, mais as que o aviso do pé
// cobre: o resto de uma resposta longa que a pessoa rolou para ler do começo. No fim da
// conversa, zero.
func (m Model) linhasAbaixo() int {
	if m.viewport.AtBottom() {
		return 0
	}
	return max(m.viewport.TotalLineCount()-(m.viewport.YOffset+m.viewport.Height-m.linhasAviso()), 0)
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
	if acima {
		// Subindo, o pé da página nova fica sob o aviso do que está abaixo.
		passo -= m.linhasAviso()
		if m.mensagensAnteriores() > 0 {
			passo -= m.linhasAviso()
		}
		m.viewport.ScrollUp(max(passo, 1))
	} else {
		if m.mensagensNovas() > 0 || m.linhasAbaixo() > 0 {
			passo -= m.linhasAviso()
		}
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

// ErrInterrompido é o Run que terminou pelo ctrl+c (ou por um SIGINT): o yby sai com 130,
// como o Viz.
var ErrInterrompido = errors.New("conversa interrompida")

// Run inicia a TUI do Bard. Devolve ErrInterrompido quando a pessoa sai pelo ctrl+c.
func Run(provider ai.Provider, vectorStore *ai.VectorStore, config Config) error {
	model := New(provider, vectorStore, config)
	// O mouse vem para o Bard por causa da roda; com isso o arraste simples deixa de
	// selecionar texto, e o shift+arraste do terminal seleciona (teclaSelecionar).
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	return desfecho(p.Run())
}

// desfecho é o erro do Run a partir do modelo final do programa.
func desfecho(final tea.Model, err error) error {
	if errors.Is(err, tea.ErrInterrupted) {
		return ErrInterrompido
	}
	if err != nil {
		return err
	}
	if m, ok := final.(Model); ok && m.interrompido {
		return ErrInterrompido
	}
	return nil
}

// classifyIntent classifica a intenção do usuário via IA. Falha do provider ou
// resposta ilegível viram resposta direta, com o motivo no log de debug.
func classifyIntent(ctx context.Context, provider ai.Provider, input, conversa, cluster, namespace string) *tools.IntentResult {
	classifyPrompt := prompts.Get("bard.classify")
	if classifyPrompt == "" {
		classifyPrompt = `Classifique a intencao. Responda APENAS JSON:
{"intent":"nome","params":{"chave":"valor"},"direct":false}
Se nao precisa de ferramenta: {"intent":"direct","params":{},"direct":true}`
	}

	// A conversa atual resolve as referências ("e os logs dele?") e completa os parâmetros.
	userPrompt := fmt.Sprintf("Intencoes:\n%sSessao: %s\n%s",
		tools.DescreverIntents(), descreverSessao(cluster, namespace), comConversa(conversa, "Usuario: "+input))

	result, err := provider.Completion(ctx, classifyPrompt, userPrompt)
	if err != nil {
		slog.Debug("classificador falhou; respondendo direto", "erro", err)
		return &tools.IntentResult{Direct: true}
	}

	intent, err := tools.ParseIntent(result)
	if err != nil {
		slog.Debug("resposta do classificador ilegível; respondendo direto", "erro", err, "resposta", result)
		return &tools.IntentResult{Direct: true}
	}
	return intent
}

// descreverSessao diz ao classificador o cluster e o namespace da sessão.
func descreverSessao(cluster, namespace string) string {
	cluster = strings.TrimSpace(cluster)
	if cluster == "" {
		cluster = "contexto atual do kubeconfig"
	}
	if strings.TrimSpace(namespace) == "" {
		return fmt.Sprintf("cluster %s, sem namespace fixo (omita namespace se o usuario nao disser)", cluster)
	}
	return fmt.Sprintf("cluster %s, namespace %s (omita namespace se o usuario nao disser outro)", cluster, namespace)
}

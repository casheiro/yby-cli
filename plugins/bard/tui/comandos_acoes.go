package tui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/v5/pkg/ai"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// O que os comandos / pedem ao plugin: o histórico (history.go), os providers
// detectados e o contexto do projeto chegam pela Config, como funções.

// Sessao resume uma conversa salva, para o /sessões.
type Sessao struct {
	ID          string
	Mensagens   int
	Inicio, Fim time.Time
	Primeira    string // a primeira pergunta
}

// MensagemSalva é uma mensagem do histórico, para retomar uma conversa.
type MensagemSalva struct {
	Papel string // "user" ou "assistant"
	Texto string
	Hora  time.Time
}

// Retomada é o que o /sessões precisa para continuar uma conversa: as mensagens e o
// system prompt refeito com elas.
type Retomada struct {
	Mensagens    []MensagemSalva
	SystemPrompt string          // vazio mantém o atual
	Historico    HistoricoPrompt // o que o SystemPrompt leva do histórico
}

// HistoricoPrompt conta o histórico que foi para o system prompt quando ele foi montado:
// as mensagens e as sessões delas. O /contexto mostra essa conta, e não o arquivo do
// histórico, que cresce com a própria conversa.
type HistoricoPrompt struct {
	Mensagens, Sessoes int
}

// Modelo é um provider de IA detectado, com os modelos dele, para o /modelo.
type Modelo struct {
	Rotulo   string      // nome curto, como no cabeçalho ("claude-cli")
	Provider ai.Provider // com o modelo padrão
	Padrao   string      // o modelo padrão; vazio quando o próprio CLI escolhe
	Modelos  []string    // os modelos conhecidos (ai.ListModels), com o padrão
	Aviso    string      // por que a lista ficou curta ("o Ollama não respondeu")
	// ComModelo cria o provider com o modelo escolhido e devolve o modelo como o provider
	// o conhece ("llama3" vira "llama3:latest"); a regra de cada provider decide se o
	// modelo existe, sem chamar a IA. Nil, o provider não troca de modelo.
	ComModelo func(ctx context.Context, modelo string) (ai.Provider, string, error)
}

// tempoDetectar limita a detecção dos providers do /modelo; tempoCriar, a criação do
// provider com o modelo escolhido.
const (
	tempoDetectar = 30 * time.Second
	tempoCriar    = 10 * time.Second
)

// opcaoModelo é uma linha do /modelo: o provider (índice em seletor.modelos) e o modelo;
// vazio é o padrão do provider.
type opcaoModelo struct {
	mod    int
	modelo string
}

// modeloCriadoMsg traz o provider criado com o modelo escolhido, ou o erro da regra dele;
// troca identifica a escolha, e modelo é o nome que o provider devolveu.
type modeloCriadoMsg struct {
	troca    int
	mod      Modelo
	modelo   string
	provider ai.Provider
	err      error
}

// tipoSeletor diz o que a lista aberta por um comando escolhe.
type tipoSeletor int

const (
	seletorSessoes tipoSeletor = iota
	seletorModelo
)

// seletor é a lista que /sessões e /modelo abrem sobre o campo: ↑↓ escolhem, ↵ usa o
// marcado e esc fecha. As letras não vão para o campo enquanto ela está aberta.
type seletor struct {
	tipo    tipoSeletor
	itens   []itemLista
	marcado int
	estado  *ui.Estado // vazio ou procurando, no lugar dos itens
	sessoes []Sessao
	modelos []Modelo
	opcoes  []opcaoModelo // uma por item do /modelo
	pedido  int           // identifica a detecção dos providers
	pedir   string        // /modelo <nome>: troca assim que a detecção chegar
}

// modelosMsg traz os providers detectados para o /modelo.
type modelosMsg struct {
	pedido int
	lista  []Modelo
}

// rotulo é o rótulo do quadro do seletor.
func (s *seletor) rotulo() string {
	if s.tipo == seletorSessoes {
		return "sessões"
	}
	return "modelo"
}

// quadroSeletor desenha o seletor aberto sobre o campo.
func (m Model) quadroSeletor() string {
	sel := m.seletor
	if sel.estado != nil {
		return ui.Quadro(sel.rotulo(), sel.estado.View(), m.viewport.Width, ui.QuadroFoco)
	}
	return quadroLista(sel.rotulo(), sel.itens, sel.marcado, m.viewport.Width, m.viewport.Height)
}

// estadoSeletor é a frase da linha de estado com o seletor aberto.
func (m Model) estadoSeletor() string {
	if m.seletor.tipo == seletorSessoes {
		return ui.S().TintaSuave.Render("conversas salvas nos últimos 30 dias")
	}
	return ui.S().TintaSuave.Render(textoDicaModelo)
}

// teclasSeletor é a legenda com o seletor aberto.
func (m Model) teclasSeletor() []key.Binding {
	acao := "usar"
	if m.seletor.tipo == seletorSessoes {
		acao = "retomar"
	}
	escolher := teclaEscolher()
	usar := key.NewBinding(key.WithKeys("enter"), key.WithHelp(ui.RotuloTecla("↵"), acao))
	tem := len(m.seletor.itens) > 0
	escolher.SetEnabled(tem)
	usar.SetEnabled(tem)
	return []key.Binding{escolher, usar, teclaFechar()}
}

// teclarSeletor trata as teclas com o seletor aberto; o resto é ignorado até ele fechar.
func (m Model) teclarSeletor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	sel := m.seletor
	n := len(sel.itens)
	switch {
	case msg.Type == tea.KeyEsc:
		m.seletor = nil
	case msg.Type == tea.KeyUp && n > 0:
		sel.marcado = (sel.marcado - 1 + n) % n
	case msg.Type == tea.KeyDown && n > 0:
		sel.marcado = (sel.marcado + 1) % n
	case msg.Type == tea.KeyEnter && n > 0:
		m.seletor = nil
		if sel.tipo == seletorSessoes {
			if id := sel.sessoes[sel.marcado].ID; id != m.config.SessionID {
				return m, m.retomar(id)
			}
			return m, nil
		}
		op := sel.opcoes[sel.marcado]
		return m, m.trocarModelo(sel.modelos[op.mod], op.modelo)
	}
	return m, nil
}

// avisoFalha põe uma falha passageira na linha de estado.
func (m *Model) avisoFalha(texto string) tea.Cmd {
	return m.mostrarAviso(&avisoEstado{texto: texto})
}

// trocarMensagens troca a conversa inteira (/limpar e /sessões) e refaz a tela do zero. A
// memória da conversa recomeça: a sessão retomada vai à IA pelo system prompt, e repeti-la
// nas trocas a mandaria duas vezes.
func (m *Model) trocarMensagens(msgs []chatMessage) {
	m.messages = msgs
	m.conversa = nil
	m.cortarHistorico(0)
	m.dicaEm, m.copiaEm = -1, -1
	m.lidas, m.parcialVisto = 0, 0
	m.ultimoFim = nil
}

// limpar começa uma conversa nova com outra sessão; a atual já está no histórico. O
// system prompt volta ao da abertura: o de uma sessão retomada levaria o histórico dela
// para a conversa nova.
func (m *Model) limpar(agora time.Time) {
	nota := "conversa nova"
	for _, msg := range m.messages {
		if msg.role == "user" && !msg.comando {
			nota = "conversa nova; a anterior fica em /sessões"
			break
		}
	}
	m.trocarMensagens([]chatMessage{{role: "nota", content: nota}})
	m.config.SessionID = agora.Format(formatoSessao)
	m.trocarPrompt(m.abertura.prompt, m.abertura.historico)
	m.irParaOFim()
}

// trocarPrompt passa a sessão para outro system prompt, com a conta do histórico que ele
// leva. O fluxo é trocado, não alterado: uma rodada antiga que ignorou o cancelamento
// continua lendo o dela.
func (m *Model) trocarPrompt(prompt string, historico HistoricoPrompt) {
	m.config.Historico = historico
	if prompt == m.fluxo.SystemPrompt {
		m.config.SystemPrompt = prompt
		return
	}
	m.config.SystemPrompt = prompt
	f := *m.fluxo
	f.SystemPrompt = prompt
	m.fluxo = &f
}

// formatoSessao é o formato do SessionID, que também diz quando a sessão começou.
const formatoSessao = "20060102-150405"

// abrirSessoes abre a lista de conversas salvas; com um id, retoma direto.
func (m *Model) abrirSessoes(id string, agora time.Time) tea.Cmd {
	if m.config.Sessoes == nil || m.config.RetomarSessao == nil {
		return m.avisoFalha("o histórico de conversas não está disponível")
	}
	if id != "" {
		return m.retomar(id)
	}
	lista, err := m.config.Sessoes()
	if err != nil {
		return m.avisoFalha("não deu para ler o histórico de conversas")
	}
	sel := &seletor{tipo: seletorSessoes, sessoes: lista}
	for _, s := range lista {
		descricao := plural(s.Mensagens, "mensagem", "mensagens")
		if p := primeiraLinha(s.Primeira); p != "" {
			descricao += " · " + p
		}
		if s.ID == m.config.SessionID {
			descricao = "esta conversa · " + descricao
		}
		sel.itens = append(sel.itens, itemLista{nome: dataSessao(s.Inicio, agora), descricao: descricao})
	}
	if len(lista) == 0 {
		e := ui.NovoEstadoVazio("nenhuma conversa salva nos últimos 30 dias", "")
		sel.estado = &e
	}
	m.seletor = sel
	return nil
}

// dataSessao é o nome de uma conversa na lista: "hoje 11:45", "ontem 18:20" ou "22/10
// 09:10" (com o ano, se for outro).
func dataSessao(t, agora time.Time) string {
	if t.IsZero() {
		return "sem data"
	}
	t = t.In(agora.Location())
	switch ontem := agora.AddDate(0, 0, -1); {
	case mesmoDia(t, agora):
		return "hoje " + t.Format("15:04")
	case mesmoDia(t, ontem):
		return "ontem " + t.Format("15:04")
	case t.Year() == agora.Year():
		return t.Format("02/01 15:04")
	}
	return t.Format("02/01/2006")
}

func mesmoDia(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// retomar troca a conversa pela sessão id do histórico. Daí em diante as mensagens vão
// para ela, e o system prompt passa a levar o histórico dela.
func (m *Model) retomar(id string) tea.Cmd {
	if m.config.RetomarSessao == nil {
		return m.avisoFalha("o histórico de conversas não está disponível")
	}
	r, err := m.config.RetomarSessao(id)
	if err != nil {
		return m.avisoFalha("não deu para abrir a conversa " + id)
	}
	var msgs []chatMessage
	for _, s := range r.Mensagens {
		if s.Papel == "user" || s.Papel == "assistant" {
			msgs = append(msgs, chatMessage{role: s.Papel, content: s.Texto, hora: s.Hora})
		}
	}
	if len(msgs) == 0 {
		return m.avisoFalha("a conversa " + id + " não está no histórico")
	}
	nota := "conversa retomada"
	if inicio := msgs[0].hora; !inicio.IsZero() {
		nota = "conversa de " + inicio.Format("02/01") + " retomada"
	}
	m.trocarMensagens(append(msgs, chatMessage{role: "nota", content: nota}))
	m.config.SessionID = id
	if r.SystemPrompt != "" {
		m.trocarPrompt(r.SystemPrompt, r.Historico)
	}
	m.irParaOFim()
	return nil
}

// abrirModelos abre a lista de providers e dispara a detecção, que pode levar alguns
// segundos (cada provider confere se está disponível). Com um nome, troca direto quando
// a detecção chegar.
func (m *Model) abrirModelos(nome string) tea.Cmd {
	if m.config.Modelos == nil {
		return m.avisoFalha("a troca de modelo não está disponível")
	}
	m.pedidos++
	// O spinner gira enquanto a detecção não chega (os tiques passam pelo Update).
	e := m.novoEstado("procurando providers de IA")
	m.seletor = &seletor{tipo: seletorModelo, estado: &e, pedido: m.pedidos, pedir: nome}
	detectar, pedido := m.config.Modelos, m.pedidos
	return tea.Batch(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), tempoDetectar)
		defer cancel()
		return modelosMsg{pedido: pedido, lista: detectar(ctx)}
	}, e.Init())
}

// girarEstado repassa o tique ao spinner do seletor que está procurando; nenhum comando
// com o seletor fechado ou já preenchido.
func (m *Model) girarEstado(msg ui.SpinnerTickMsg) tea.Cmd {
	sel := m.seletor
	if sel == nil || sel.estado == nil {
		return nil
	}
	novo, cmd := sel.estado.Update(msg)
	e := novo.(ui.Estado)
	sel.estado = &e
	return cmd
}

// copiarResposta manda a última resposta do Bard, em markdown, para a área de
// transferência.
func (m *Model) copiarResposta() tea.Cmd {
	for i := len(m.messages) - 1; i >= 0; i-- {
		if msg := m.messages[i]; msg.role == "assistant" && strings.TrimSpace(msg.content) != "" {
			return m.copiarTexto(msg.content, "resposta")
		}
	}
	return m.avisoFalha("ainda não há resposta para copiar")
}

// exportar grava a conversa em markdown no diretório do projeto: bard-<sessão>.md, ou o
// nome dado (com .md no fim). Os segredos conhecidos saem antes de gravar. Um arquivo
// que já existe só é regravado se for o bard-<sessão>.md ou um que o próprio /exportar
// gravou nesta execução: "/exportar README" não apaga o README.md do projeto.
func (m *Model) exportar(nome string) tea.Cmd {
	md, n := m.markdownConversa()
	if n == 0 {
		return m.avisoFalha("a conversa ainda não tem mensagens para exportar")
	}
	if m.config.Sanitizar != nil {
		md = m.config.Sanitizar(md)
	}
	padrao := nome == ""
	switch {
	case nome == "" && m.config.SessionID != "":
		nome = "bard-" + m.config.SessionID + ".md"
	case nome == "":
		nome = "bard-conversa.md"
	case !strings.EqualFold(filepath.Ext(nome), ".md"):
		nome += ".md"
	}
	caminho := nome
	if !filepath.IsAbs(caminho) {
		dir := m.config.Diretorio
		if dir == "" {
			dir, _ = os.Getwd()
		}
		caminho = filepath.Join(dir, nome)
	}
	err := os.MkdirAll(filepath.Dir(caminho), 0o755)
	if err == nil {
		err = gravarExportacao(caminho, md, padrao || m.jaExportado(caminho))
	}
	switch {
	case errors.Is(err, fs.ErrExist):
		return m.avisoFalha("já existe " + nome + "; use outro nome")
	case err != nil:
		return m.avisoFalha("não deu para salvar a conversa em " + nome)
	}
	if st, err := os.Stat(caminho); err == nil {
		m.exportados = append(m.exportados, st)
	}
	return m.mostrarAviso(&avisoEstado{ok: true, texto: "conversa salva em " + nome})
}

// gravarExportacao grava o markdown no caminho. Sem regravar, o arquivo é criado com
// O_EXCL e um que já existe dá fs.ErrExist em vez de ser truncado.
func gravarExportacao(caminho, md string, regravar bool) error {
	modo := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if regravar {
		modo = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := os.OpenFile(caminho, modo, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(md)
	if errFechar := f.Close(); err == nil {
		err = errFechar
	}
	return err
}

// jaExportado diz se o caminho é um arquivo que o /exportar gravou nesta execução. A
// comparação é pelo arquivo (os.SameFile), não pelo nome: no Windows "readme.md" e
// "README.md" são o mesmo arquivo.
func (m Model) jaExportado(caminho string) bool {
	st, err := os.Stat(caminho)
	if err != nil {
		return false
	}
	for _, e := range m.exportados {
		if os.SameFile(e, st) {
			return true
		}
	}
	return false
}

// markdownConversa monta a conversa em markdown e conta as mensagens dela. Os comandos,
// os quadros e as notas ficam de fora: são da tela, não da conversa.
func (m Model) markdownConversa() (string, int) {
	var b strings.Builder
	b.WriteString("# Conversa com o Bard")
	if inicio, err := time.ParseInLocation(formatoSessao, m.config.SessionID, time.Local); err == nil {
		b.WriteString(" · " + inicio.Format("02/01/2006 15:04"))
	}
	b.WriteString("\n")
	var onde []string
	for _, v := range []string{m.config.Cluster, m.config.Namespace, m.rotuloIA()} {
		if v != "" {
			onde = append(onde, v)
		}
	}
	if len(onde) > 0 {
		b.WriteString("\n" + strings.Join(onde, " · ") + "\n")
	}
	n := 0
	for _, msg := range m.messages {
		if msg.comando {
			continue
		}
		texto := strings.TrimRight(msg.content, "\n")
		switch msg.role {
		case "user":
			fmt.Fprintf(&b, "\n## Você · %s\n\n%s\n", hora(msg.hora), texto)
		case "assistant", "error":
			cabecalho := "Bard · " + hora(msg.hora)
			if msg.role == "error" {
				cabecalho += " · falhou"
			}
			fmt.Fprintf(&b, "\n## %s\n\n", cabecalho)
			for _, f := range msg.ferramentas {
				fmt.Fprintf(&b, "- ferramenta: `%s`\n", strings.TrimSpace(f.rotulo+" "+f.alvo))
			}
			if len(msg.ferramentas) > 0 {
				b.WriteString("\n")
			}
			b.WriteString(texto + "\n")
		default:
			continue
		}
		n++
	}
	return b.String(), n
}

package ui

import (
	"errors"
	"strconv"
	"strings"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	yui "github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Quadro de logs, YAML e eventos: l, y e e abrem o conteúdo inteiro do
// recurso num ui.Quadro que ocupa o corpo, com rolagem própria (↑↓, pgup/pgdn,
// g/G e a roda do mouse); esc volta para a lista (ou para o painel).

// Conteúdos do quadro.
const (
	quadroLogs    = "logs"
	quadroYAML    = "yaml"
	quadroEventos = "eventos"
)

// linhasDeLog é quantas linhas do fim do log o quadro traz.
const linhasDeLog = 100

// errSemLeitor é o erro do client que não lê logs, YAML nem eventos.
var errSemLeitor = errors.New("este client não lê logs, YAML nem eventos do cluster")

// alvoQuadro é o recurso do quadro, guardado quando ele abre: a lista pode
// mudar por baixo sem trocar o que o quadro mostra.
type alvoQuadro struct {
	nome, namespace, kind string
	nomeYAML              string        // o nome do objeto do YAML (na aba Eventos, o do Event)
	eventos               objetoEventos // de quem são os eventos
	conteiner             string        // o contêiner do log (nos pods)
	variosConteineres     bool          // o pod tem mais de um: a descrição diz de qual é o log
}

// quadroAberto é o quadro na tela.
type quadroAberto struct {
	aberto     bool
	tipo       string
	alvo       alvoQuadro
	id         int
	carregando bool
	recarga    bool            // o r pediu de novo: a rolagem fica onde estava
	texto      string          // logs ou YAML
	anterior   bool            // logs da execução anterior
	eventos    []monitor.Event // eventos
	err        error
	scroll     int
}

// conteudoMsg traz o conteúdo pedido pelo quadro id.
type conteudoMsg struct {
	id       int
	texto    string
	anterior bool // o log é o da execução anterior
	eventos  []monitor.Event
	err      error
}

// alvoDoItem é o alvo do quadro para a linha da lista.
func alvoDoItem(kind string, it itemLista) alvoQuadro {
	a := alvoQuadro{nome: it.nome, namespace: it.namespace, kind: kind, nomeYAML: it.nome, eventos: objetoDosEventos(kind, it)}
	if e, ok := it.obj.(monitor.Event); ok && e.ID != "" {
		a.nomeYAML = e.ID
	}
	if p, ok := it.obj.(monitor.Pod); ok {
		a.conteiner, a.variosConteineres = p.Conteiner, p.Containers > 1
	}
	return a
}

// abrirQuadroDaSelecao abre o quadro do tipo dado para a linha selecionada.
func (m Model) abrirQuadroDaSelecao(tipo string) (tea.Model, tea.Cmd) {
	it, ok := m.itemSelecionado()
	if !ok {
		return m, nil
	}
	return m.abrirQuadro(tipo, alvoDoItem(abas[m.activeTab].kind, it))
}

// abrirQuadro abre o quadro já carregando e pede o conteúdo; logs só existem
// para pods.
func (m Model) abrirQuadro(tipo string, alvo alvoQuadro) (tea.Model, tea.Cmd) {
	if tipo == quadroLogs && alvo.kind != "pod" {
		m.actionFeedback = frase(yui.StatusAlerta, "logs só existem para pods")
		return m, nil
	}
	m.idQuadro++
	m.quadro = quadroAberto{aberto: true, tipo: tipo, alvo: alvo, id: m.idQuadro, carregando: true}
	return m, lerQuadro(m.leitor(), tipo, alvo, m.idQuadro)
}

// recarregarQuadro lê de novo o conteúdo do quadro (o r); a resposta do
// pedido anterior, se ainda não chegou, é descartada.
func (m Model) recarregarQuadro() (tea.Model, tea.Cmd) {
	m.idQuadro++
	m.quadro.id, m.quadro.carregando, m.quadro.recarga = m.idQuadro, true, true
	return m, lerQuadro(m.leitor(), m.quadro.tipo, m.quadro.alvo, m.idQuadro)
}

// lerQuadro lê o conteúdo do quadro.
func lerQuadro(l monitor.LeitorDeDetalhe, tipo string, a alvoQuadro, id int) tea.Cmd {
	return func() tea.Msg {
		msg := conteudoMsg{id: id}
		switch {
		case l == nil:
			msg.err = errSemLeitor
		case tipo == quadroLogs:
			msg.texto, msg.anterior, msg.err = l.LogsDoPod(a.nome, a.namespace, a.conteiner, linhasDeLog)
		case tipo == quadroYAML:
			msg.texto, msg.err = l.YAMLDoRecurso(a.kind, a.nomeYAML, a.namespace)
		default:
			msg.eventos, msg.err = l.EventosDoObjeto(a.eventos.kind, a.eventos.nome, a.eventos.namespace, 0)
		}
		return msg
	}
}

// receberConteudo guarda o conteúdo que chegou, se ainda é o do quadro na
// tela; o de um quadro fechado ou trocado é descartado.
func (m *Model) receberConteudo(msg conteudoMsg) {
	if !m.quadro.aberto || msg.id != m.quadro.id {
		return
	}
	m.quadro.carregando = false
	m.quadro.texto, m.quadro.anterior, m.quadro.eventos, m.quadro.err = msg.texto, msg.anterior, msg.eventos, msg.err
	if !m.quadro.recarga {
		m.quadro.scroll = 0
	}
	m.quadro.recarga = false
	m.rolarQuadro(0) // na recarga, o conteúdo novo pode ser mais curto
}

// fecharQuadro volta para a lista (ou para o painel, se ele estava aberto).
func (m *Model) fecharQuadro() {
	m.quadro = quadroAberto{}
}

// interiorDoQuadro é a largura do texto dentro do quadro (sem borda e respiro).
func (m Model) interiorDoQuadro() int {
	return max(1, m.width-4)
}

// linhasDoQuadro é o conteúdo em linhas, quebradas na largura do quadro: o log
// e o YAML sem cores nem \r, e um evento por linha, dos mais recentes.
func (m Model) linhasDoQuadro() []string {
	q, w := m.quadro, m.interiorDoQuadro()
	var brutas []string
	if q.tipo == quadroEventos {
		for _, e := range q.eventos {
			brutas = append(brutas, linhaDeEvento(e, m.agora()))
		}
	} else if t := strings.TrimRight(strings.ReplaceAll(q.texto, "\r", ""), "\n"); strings.TrimSpace(t) != "" {
		for _, l := range strings.Split(t, "\n") {
			brutas = append(brutas, strings.ReplaceAll(ansi.Strip(l), "\t", "    "))
		}
	}
	var linhas []string
	for _, l := range brutas {
		linhas = append(linhas, strings.Split(ansi.Wrap(l, w, ""), "\n")...)
	}
	return linhas
}

// linhasVisiveisDoQuadro é quanto do conteúdo cabe: o corpo menos a linha em
// branco de cima e as bordas.
func (m Model) linhasVisiveisDoQuadro() int {
	return max(1, m.alturaCorpo()-3)
}

// maxRolagemQuadro é a maior rolagem: quando o conteúdo não cabe, a última
// linha do quadro é o aviso de rolagem.
func (m Model) maxRolagemQuadro() int {
	n, v := len(m.linhasDoQuadro()), m.linhasVisiveisDoQuadro()
	if n <= v {
		return 0
	}
	return n - max(1, v-1)
}

// rolarQuadro move a rolagem do quadro, dentro dos limites.
func (m *Model) rolarQuadro(delta int) {
	m.quadro.scroll = max(0, min(m.quadro.scroll+delta, m.maxRolagemQuadro()))
}

// teclaQuadro trata o quadro: rolagem, l y e trocam o conteúdo do mesmo
// recurso, esc volta.
func (m Model) teclaQuadro(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	g, l := mapa().Global, mapa().Lista
	pagina := max(1, m.linhasVisiveisDoQuadro()-1)
	switch {
	case key.Matches(k, g.Voltar):
		m.fecharQuadro()
	case key.Matches(k, g.Sair):
		m.actionFeedback = avisoQ()
	case key.Matches(k, g.Ajuda):
		m.ajuda, m.ajudaScroll = true, 0
	case key.Matches(k, g.Recarregar):
		return m.recarregarQuadro()
	case key.Matches(k, l.Baixo):
		m.rolarQuadro(1)
	case key.Matches(k, l.Cima):
		m.rolarQuadro(-1)
	case key.Matches(k, l.PaginaBaixo):
		m.rolarQuadro(pagina)
	case key.Matches(k, l.PaginaCima):
		m.rolarQuadro(-pagina)
	case key.Matches(k, l.Inicio):
		m.quadro.scroll = 0
	case key.Matches(k, l.Fim):
		m.quadro.scroll = m.maxRolagemQuadro()
	case key.Matches(k, l.Logs) && m.quadro.tipo != quadroLogs:
		return m.abrirQuadro(quadroLogs, m.quadro.alvo)
	case key.Matches(k, l.Yaml) && m.quadro.tipo != quadroYAML:
		return m.abrirQuadro(quadroYAML, m.quadro.alvo)
	case key.Matches(k, l.Eventos) && m.quadro.tipo != quadroEventos:
		return m.abrirQuadro(quadroEventos, m.quadro.alvo)
	}
	return m, nil
}

// descricaoDoQuadro é a linha acima do quadro: o que ele mostra.
func (m Model) descricaoDoQuadro() string {
	q := m.quadro
	switch q.tipo {
	case quadroLogs:
		de := ""
		if q.alvo.variosConteineres && q.alvo.conteiner != "" {
			de = " de " + q.alvo.conteiner
		}
		if q.anterior && !q.carregando && q.err == nil {
			return "execução anterior: últimas " + strconv.Itoa(linhasDeLog) + " linhas do log" + de + emNamespace(q.alvo.namespace)
		}
		return "últimas " + strconv.Itoa(linhasDeLog) + " linhas do log" + de + emNamespace(q.alvo.namespace)
	case quadroYAML:
		return "manifesto como está no cluster" + emNamespace(q.alvo.namespace)
	}
	txt := "eventos de " + strings.TrimSpace(q.alvo.eventos.kind+" "+q.alvo.eventos.nome)
	if !q.carregando && q.err == nil {
		txt = contarEventos(len(q.eventos)) + " de " + strings.TrimSpace(q.alvo.eventos.kind+" "+q.alvo.eventos.nome)
	}
	return txt + ", mais recentes primeiro"
}

// emNamespace é " · em payments" (vazio para recursos do cluster).
func emNamespace(ns string) string {
	if ns == "" {
		return ""
	}
	return " · em " + ns
}

// contarEventos escreve "1 evento" ou "4 eventos".
func contarEventos(n int) string {
	return abas[TabEvents].contar(n)
}

// vazioDoQuadro é o estado vazio de cada conteúdo.
var vazioDoQuadro = map[string]string{
	quadroLogs: "nenhuma linha de log", quadroYAML: "YAML vazio", quadroEventos: "nenhum evento deste recurso",
}

// corpoDoQuadro são as linhas dentro do quadro, completadas até o fim do
// corpo, como o painel ao lado.
func (m Model) corpoDoQuadro() []string {
	linhas := m.conteudoDoQuadro()
	for len(linhas) < m.linhasVisiveisDoQuadro() {
		linhas = append(linhas, "")
	}
	return linhas
}

// conteudoDoQuadro é o que o quadro mostra: carregando, o erro, o vazio ou a
// janela do conteúdo com o aviso de rolagem.
func (m Model) conteudoDoQuadro() []string {
	q, s := m.quadro, yui.S()
	switch {
	case q.carregando:
		return []string{carregandoNoPainel("lendo " + nomeDoQuadro(q.tipo) + " de " + q.alvo.nome)}
	case q.err != nil:
		linhas := []string{yui.NovoEstadoErro("não deu para ler "+nomeDoQuadro(q.tipo), "esc volta").View(), ""}
		for _, l := range strings.Split(ansi.Wrap(q.err.Error(), m.interiorDoQuadro(), ""), "\n") {
			linhas = append(linhas, s.TintaSuave.Render(l))
		}
		return linhas
	}
	linhas := m.linhasDoQuadro()
	if len(linhas) == 0 {
		return []string{yui.NovoEstadoVazio(vazioDoQuadro[q.tipo], "esc volta").View()}
	}
	v := m.linhasVisiveisDoQuadro()
	if len(linhas) <= v {
		return linhas
	}
	ini := min(q.scroll, m.maxRolagemQuadro())
	fim := min(len(linhas), ini+max(1, v-1))
	return append(append([]string(nil), linhas[ini:fim]...), avisoDeLinhas(ini, len(linhas)-fim))
}

// nomeDoQuadro é "os logs", "o YAML" ou "os eventos" sem o artigo.
func nomeDoQuadro(tipo string) string {
	if tipo == quadroYAML {
		return "o YAML"
	}
	return "os " + tipo
}

// avisoDeLinhas é "↑ 12 linhas acima   ↓ 80 linhas abaixo".
func avisoDeLinhas(acima, abaixo int) string {
	contar := func(n int) string {
		if n == 1 {
			return "1 linha"
		}
		return strconv.Itoa(n) + " linhas"
	}
	var partes []string
	if acima > 0 {
		partes = append(partes, yui.AvisoRolagem(yui.SetaAcima, contar(acima)+" acima"))
	}
	if abaixo > 0 {
		partes = append(partes, yui.AvisoRolagem(yui.SetaAbaixo, contar(abaixo)+" abaixo"))
	}
	return strings.Join(partes, "   ")
}

// teclasDoQuadro: rolar, página, início e fim, os outros conteúdos do recurso
// e esc.
func (m Model) teclasDoQuadro() []key.Binding {
	l := mapa().Lista
	t := []key.Binding{atalho(setas(), "rolar", "up", "down"), atalho("pgup/pgdn", "página", "pgup", "pgdown"),
		atalho("g G", "início e fim", "g", "G")}
	outros := map[string]key.Binding{quadroLogs: l.Logs, quadroYAML: l.Yaml, quadroEventos: l.Eventos}
	for _, tipo := range []string{quadroLogs, quadroYAML, quadroEventos} {
		if tipo != m.quadro.tipo && (tipo != quadroLogs || m.quadro.alvo.kind == "pod") {
			t = append(t, outros[tipo])
		}
	}
	voltar := atalho("esc", "voltar", "esc")
	if m.width < yui.LarguraColuna {
		return append([]key.Binding{voltar}, t...) // a legenda compacta guarda três teclas e põe "? mais"
	}
	return append(t, voltar, mapa().Global.Ajuda)
}

// viewQuadro mostra o quadro no lugar da lista, com a moldura em volta.
func (m Model) viewQuadro() string {
	p := m.partesBase()
	p.Filtros = yui.S().TintaSuave.Render(m.descricaoDoQuadro())
	rotulo := map[string]string{quadroLogs: "logs", quadroYAML: "yaml", quadroEventos: "eventos"}[m.quadro.tipo]
	p.Lista = "\n" + yui.QuadroNomeado(rotulo, m.quadro.alvo.nome, strings.Join(m.corpoDoQuadro(), "\n"), m.width, yui.QuadroFoco)
	p.Mensagem = m.actionFeedback
	p.Teclas = m.teclasDoQuadro()
	p.Sair = mapa().Global.SairForcado
	return yui.Moldura(m.width, m.height, p)
}

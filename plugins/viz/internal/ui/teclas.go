package ui

import (
	"strconv"
	"strings"
	"unicode/utf8"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"k8s.io/apimachinery/pkg/labels"
)

// Mapa de teclas do Viz (folha Atalhos): as teclas são as do yui.Teclas, em
// três níveis. esc fecha a camada de cima (campo, menu, confirmação, ajuda,
// detalhe) e, sem camada aberta, limpa os filtros; q sai só da lista (num
// campo, é letra); ctrl+c sai de qualquer lugar.

// mapa é o keymap padrão do pkg/ui; os rótulos seguem o modo visual atual.
func mapa() yui.Teclas { return yui.DefaultKeys() }

// passoRoda é quantas linhas a roda do mouse rola por vez.
const passoRoda = 3

// Campos de texto da linha de estado.
const (
	campoBusca     = "search"
	campoNamespace = "namespace"
	campoFiltro    = "filtro"
)

// acao é uma entrada do menu x: a tecla, o rótulo e o modo que ela abre.
type acao struct {
	tecla, rotulo string
	modo          ActionMode
}

// acoesDoTipo são as ações do menu x para o tipo de recurso, na ordem do
// menu; nós e eventos não têm ações.
func acoesDoTipo(kind string) []acao {
	excluir := acao{"d", "excluir…", ActionConfirmDelete}
	switch kind {
	case "node", "event":
		return nil
	case "deployment", "statefulset":
		return []acao{{"r", "reiniciar", ActionConfirmRestart}, {"s", "escalar…", ActionInputScale}, excluir}
	}
	return []acao{excluir}
}

// temAcao diz se o tipo tem a ação do modo dado.
func temAcao(kind string, modo ActionMode) bool {
	for _, a := range acoesDoTipo(kind) {
		if a.modo == modo {
			return true
		}
	}
	return false
}

// emX escreve onde a ação foi parar: "x ▸ d".
func emX(tecla string) string {
	return "x " + yui.G().Caminho + " " + tecla
}

// tecla despacha a tecla para a camada de cima.
func (m Model) tecla(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case m.filter.InputMode:
		return m.teclaCampo(k)
	case m.ajuda: // a ajuda abre por cima do menu, do painel e do quadro
		return m.teclaAjuda(k)
	case m.actionMode == ActionMenu:
		return m.teclaMenu(k)
	case m.actionMode != ActionNone:
		return m.teclaConfirmacao(k)
	case m.quadro.aberto:
		return m.teclaQuadro(k)
	}
	return m.teclaLista(k)
}

// avisoQ explica, numa camada, que o q só sai da lista.
func avisoQ() string {
	return frase(yui.StatusParado, "q sai só da lista; esc volta")
}

// teclaLista trata as teclas da lista, o nível de cima.
func (m Model) teclaLista(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	g, l := mapa().Global, mapa().Lista
	count := m.itemCount()
	h := m.linhasDeDados()
	maxScroll := max(0, count-h)
	if !m.listaNaTela() && !m.valeSemLista(k) {
		return m, nil
	}

	switch {
	case key.Matches(k, g.Sair) && m.painelPorCima():
		m.actionFeedback = avisoQ() // o painel é um nível abaixo da lista
		return m, nil
	case key.Matches(k, g.Sair):
		return m, tea.Quit
	case key.Matches(k, g.Ajuda):
		m.ajuda, m.ajudaScroll = true, 0
		return m, nil
	case key.Matches(k, g.Voltar) && m.painelPorCima():
		m.detalheAberto = false // fecha o painel por cima; ao lado, ele fica
		m.seguirSelecao()
		return m, nil
	case key.Matches(k, g.Voltar):
		// Limpa label, status e busca; o namespace é o contexto e troca por n.
		// O painel aberto que não está na tela (a lista ficou vazia) fecha junto.
		m.detalheAberto = false
		if m.algumFiltro() {
			m.filter.LabelSelector, m.filter.StatusFilter, m.filter.SearchQuery = "", "", ""
			m.scrollY, m.selectedIndex = 0, 0
		}
		return m, nil
	case key.Matches(k, g.Recarregar):
		if m.client == nil || m.recarregando {
			return m, nil // uma recarga por vez: segurar o r não vira uma rajada de leituras
		}
		m.recarregando = true
		cmd := tea.Batch(m.recarregar(), m.esquecerExtras())
		return m, cmd
	case key.Matches(k, l.AbaSeguinte):
		return m.irParaAba((m.activeTab + 1) % tabCount)
	case key.Matches(k, l.AbaAnterior):
		return m.irParaAba((m.activeTab - 1 + tabCount) % tabCount)
	case key.Matches(k, l.AbaNumero):
		return m.irParaAba(ResourceTab(k.String()[0] - '1'))
	case key.Matches(k, l.Baixo):
		if count > 0 && m.selectedIndex < count-1 {
			m.selectedIndex++
			m.seguirSelecao()
		}
		return m, nil
	case key.Matches(k, l.Cima):
		if m.selectedIndex > 0 {
			m.selectedIndex--
			m.seguirSelecao()
		}
		return m, nil
	case key.Matches(k, l.PaginaBaixo):
		// A seleção desce junto: a ação atinge sempre uma linha da tela.
		if count > 0 {
			m.selectedIndex = min(count-1, m.selectedIndex+h)
			m.scrollY = min(maxScroll, m.scrollY+h)
			m.seguirSelecao()
		}
		return m, nil
	case key.Matches(k, l.PaginaCima):
		m.selectedIndex = max(0, m.selectedIndex-h)
		m.scrollY = max(0, m.scrollY-h)
		m.seguirSelecao()
		return m, nil
	case key.Matches(k, l.Inicio):
		m.scrollY, m.selectedIndex = 0, 0
		return m, nil
	case key.Matches(k, l.Fim):
		m.scrollY = maxScroll
		m.selectedIndex = max(0, count-1)
		return m, nil
	case key.Matches(k, l.Buscar):
		// A busca abre com o termo atual, para editar; o esc devolve o de antes.
		m.abrirCampo(campoBusca, m.filter.SearchQuery)
		m.filter.buscaAnterior = m.filter.SearchQuery
		return m, nil
	case key.Matches(k, l.Namespace):
		// O namespace atual está no masthead; o campo abre vazio para o novo.
		m.abrirCampo(campoNamespace, "")
		return m, nil
	case key.Matches(k, l.Filtrar):
		m.abrirCampo(campoFiltro, "")
		return m, nil
	case key.Matches(k, l.Acoes):
		return m.abrirAcoes()
	case key.Matches(k, l.Abrir):
		// ↵ desce um nível: abaixo de 120 colunas abre o painel; com o painel
		// na tela, o YAML do recurso (na aba Eventos, os eventos do objeto).
		if count == 0 {
			return m, nil
		}
		if !m.painelAberto() {
			m.detalheAberto = true
			m.seguirSelecao() // a lista encolhe embaixo do painel: a seleção fica na janela
			return m, nil
		}
		if abas[m.activeTab].kind == "event" {
			return m.abrirQuadroDaSelecao(quadroEventos)
		}
		return m.abrirQuadroDaSelecao(quadroYAML)
	case key.Matches(k, l.Logs):
		return m.abrirQuadroDaSelecao(quadroLogs)
	case key.Matches(k, l.Yaml):
		return m.abrirQuadroDaSelecao(quadroYAML)
	case key.Matches(k, l.Eventos):
		return m.abrirQuadroDaSelecao(quadroEventos)
	}
	m.actionFeedback = m.avisoTeclaAntiga(k.String())
	return m, nil
}

// janelaPequena diz se a tela é só o aviso "Aumente a janela" (abaixo de
// 60x16). Antes do primeiro tamanho da janela não há tela nenhuma ainda.
func (m Model) janelaPequena() bool {
	return m.width > 0 && m.height > 0 && yui.ModoPara(m.width, m.height) == yui.ModoJanelaPequena
}

// listaNaTela diz se a lista está desenhada: sem conexão o corpo mostra o
// erro, e abaixo de 60x16 a tela só pede para aumentar a janela.
func (m Model) listaNaTela() bool {
	return m.err == nil && !m.janelaPequena()
}

// valeSemLista diz se a tecla vale com a lista fora da tela. As que agem
// sobre a linha selecionada (mover, ↵, l y e, x) e as que filtram (/ f) não
// valem: a ação atingiria uma linha que a pessoa não vê. Sem conexão valem
// q, ?, esc, r, n e as abas; com a janela pequena, só q e r.
func (m Model) valeSemLista(k tea.KeyMsg) bool {
	g, l := mapa().Global, mapa().Lista
	if m.janelaPequena() {
		return key.Matches(k, g.Sair, g.Recarregar)
	}
	return key.Matches(k, g.Sair, g.Ajuda, g.Voltar, g.Recarregar, l.AbaSeguinte, l.AbaAnterior, l.AbaNumero, l.Namespace)
}

// avisoTeclaAntiga diz onde foi parar a ação de uma tecla que mudou de lugar
// (d, s, L, S); para as outras teclas, nada.
func (m Model) avisoTeclaAntiga(k string) string {
	d := abas[m.activeTab]
	switch k {
	case "d":
		if !temAcao(d.kind, ActionConfirmDelete) {
			return frase(yui.StatusAlerta, "não dá para excluir "+d.plural)
		}
		return frase(yui.StatusParado, "excluir agora fica em "+emX("d"))
	case "s":
		if !temAcao(d.kind, ActionInputScale) {
			return frase(yui.StatusAlerta, "escalar só vale para Deployments e StatefulSets")
		}
		return frase(yui.StatusParado, "escalar agora fica em "+emX("s"))
	case "L":
		return frase(yui.StatusParado, "label agora fica em f (app=web)")
	case "S":
		return frase(yui.StatusParado, "status agora fica em f (Running)")
	}
	return ""
}

// recarregar lê a aba ativa na hora; a resposta avisa na linha de estado.
func (m Model) recarregar() tea.Cmd {
	ler := m.ler(false, m.activeTab)
	return func() tea.Msg {
		msg, _ := ler().(leituraMsg)
		msg.recarga = true
		return msg
	}
}

// fimDaRecarga escreve na linha de estado o resultado do r, nomeando a aba
// que foi lida (a pessoa pode ter trocado de aba antes da resposta).
func (m *Model) fimDaRecarga(msg leituraMsg) {
	m.recarregando = false
	lida := m.activeTab
	if len(msg.abas) > 0 {
		lida = msg.abas[0]
	}
	d := abas[lida]
	if len(msg.respostas) > 0 {
		if err, ok := msg.respostas[0].(error); ok {
			m.actionFeedback = frase(yui.StatusFalha, "não deu para recarregar: "+err.Error())
			return
		}
	}
	texto := d.plural + " recarregados"
	if temAcao(d.kind, ActionConfirmRestart) {
		texto += " · reiniciar fica em " + emX("r")
	}
	m.actionFeedback = frase(yui.StatusRodando, texto)
}

// abrirCampo abre um campo de texto na linha de estado com o valor atual.
func (m *Model) abrirCampo(campo, valor string) {
	m.filter.InputMode = true
	m.filter.InputField = campo
	m.filter.InputBuffer = valor
}

// fecharCampo fecha o campo de texto.
func (m *Model) fecharCampo() {
	m.filter.InputMode = false
	m.filter.InputBuffer = ""
	m.filter.InputField = ""
}

// buscarAgora aplica a busca enquanto ela é digitada, mantendo a seleção no
// mesmo recurso quando ele continua na lista.
func (m *Model) buscarAgora(termo string) {
	chave, antes := m.chaveSelecionada(), m.selectedIndex
	m.filter.SearchQuery = termo
	m.reselecionar(chave, antes)
}

// teclaCampo trata as teclas de um campo de texto: tudo que não é ↵, esc ou
// apagar é texto (inclusive q).
func (m Model) teclaCampo(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	campo := m.filter.InputField
	switch k.Type {
	case tea.KeyEnter:
		valor := strings.TrimSpace(m.filter.InputBuffer)
		m.fecharCampo()
		return m.aplicarCampo(campo, valor)
	case tea.KeyEsc:
		m.fecharCampo()
		if campo == campoBusca {
			m.buscarAgora(m.filter.buscaAnterior) // volta ao valor de antes
		}
		return m, nil
	case tea.KeyBackspace:
		if _, n := utf8.DecodeLastRuneInString(m.filter.InputBuffer); n > 0 {
			m.filter.InputBuffer = m.filter.InputBuffer[:len(m.filter.InputBuffer)-n]
		}
	case tea.KeyRunes, tea.KeySpace:
		m.filter.InputBuffer += string(k.Runes)
	default:
		return m, nil
	}
	if campo == campoBusca {
		m.buscarAgora(strings.TrimSpace(m.filter.InputBuffer))
	}
	return m, nil
}

// pareceLabel diz se o texto do f é um label selector (app=web, tier!=db,
// env in (a,b), !canary, a=1,b=2); senão é um status.
func pareceLabel(valor string) bool {
	return strings.ContainsAny(valor, "=!,()")
}

// aplicarCampo aplica o valor confirmado com ↵.
func (m Model) aplicarCampo(campo, valor string) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch campo {
	case campoNamespace:
		if valor == m.filter.Namespace {
			return m, nil
		}
		// As listas lidas são do namespace anterior.
		m.filter.Namespace = valor
		m.esquecerListas()
		cmd = m.ler(false, m.activeTab)
	case campoBusca:
		m.buscarAgora(valor)
		return m, nil
	case campoFiltro:
		// Um filtro por vez: o label troca o status, e o contrário; vazio limpa os dois.
		switch {
		case valor == "":
			m.filter.LabelSelector, m.filter.StatusFilter = "", ""
		case pareceLabel(valor):
			if _, err := labels.Parse(valor); err != nil {
				m.actionFeedback = frase(yui.StatusAlerta, "label inválido: "+valor)
				return m, nil
			}
			m.filter.LabelSelector, m.filter.StatusFilter = valor, ""
		default:
			m.filter.LabelSelector, m.filter.StatusFilter = "", valor
			if !abas[m.activeTab].filtroStatus {
				m.actionFeedback = frase(yui.StatusAlerta, "o status filtra só a aba Pods")
			}
		}
	}
	m.scrollY, m.selectedIndex = 0, 0
	return m, cmd
}

// abrirAcoes abre o menu x da linha selecionada, guardando o alvo.
func (m Model) abrirAcoes() (tea.Model, tea.Cmd) {
	name, ns, kind := m.selectedResourceInfo()
	if name == "" {
		return m, nil
	}
	if len(acoesDoTipo(kind)) == 0 {
		m.actionFeedback = frase(yui.StatusAlerta, abas[m.activeTab].plural+" não têm ações")
		return m, nil
	}
	m.actionMode = ActionMenu
	m.actionTarget, m.actionNS, m.actionKind = name, ns, kind
	m.actionBuffer = ""
	return m, nil
}

// fecharAcoes fecha o menu e esquece o alvo.
func (m *Model) fecharAcoes() {
	m.actionMode = ActionNone
	m.actionBuffer = ""
	m.actionTarget, m.actionNS, m.actionKind = "", "", ""
}

// teclaMenu trata o menu x: a letra da ação abre a confirmação dela.
func (m Model) teclaMenu(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(k, mapa().Global.Voltar):
		m.fecharAcoes()
		return m, nil
	case key.Matches(k, mapa().Global.Sair):
		m.actionFeedback = avisoQ()
		return m, nil
	case key.Matches(k, mapa().Global.Ajuda):
		m.ajuda, m.ajudaScroll = true, 0
		return m, nil
	}
	for _, a := range acoesDoTipo(m.actionKind) {
		if k.String() == a.tecla {
			m.actionMode = a.modo
			m.actionBuffer = ""
		}
	}
	return m, nil
}

// teclaConfirmacao trata a confirmação de uma ação: ↵ executa, esc volta ao
// menu e o resto é o texto digitado.
func (m Model) teclaConfirmacao(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyEnter:
		mode := m.actionMode
		alvo := alvoDaAcao{nome: m.actionTarget, namespace: m.actionNS, kind: m.actionKind}
		buf := m.actionBuffer
		m.fecharAcoes()
		return m, m.executeAction(mode, alvo, buf)
	case tea.KeyEsc:
		m.actionMode = ActionMenu
		m.actionBuffer = ""
	case tea.KeyBackspace:
		if _, n := utf8.DecodeLastRuneInString(m.actionBuffer); n > 0 {
			m.actionBuffer = m.actionBuffer[:len(m.actionBuffer)-n]
		}
	case tea.KeyRunes:
		m.actionBuffer += string(k.Runes)
	}
	return m, nil
}

// teclaAjuda trata a tela ?: esc ou ? fecham; as setas rolam quando ela não cabe.
func (m Model) teclaAjuda(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	g, l := mapa().Global, mapa().Lista
	switch {
	case key.Matches(k, g.Voltar), key.Matches(k, g.Ajuda):
		m.ajuda, m.ajudaScroll = false, 0
	case key.Matches(k, g.Sair):
		m.actionFeedback = avisoQ()
	case key.Matches(k, l.Baixo):
		m.ajudaScroll++
	case key.Matches(k, l.Cima):
		m.ajudaScroll--
	case key.Matches(k, l.PaginaBaixo):
		m.ajudaScroll += m.alturaCorpo()
	case key.Matches(k, l.PaginaCima):
		m.ajudaScroll -= m.alturaCorpo()
	case key.Matches(k, l.Inicio):
		m.ajudaScroll = 0
	case key.Matches(k, l.Fim):
		m.ajudaScroll = len(m.linhasDaAjuda())
	}
	m.ajudaScroll = max(0, min(m.ajudaScroll, m.maxRolagemAjuda()))
	return m, nil
}

// mouse: a roda rola a lista (ou o quadro e a ajuda); o clique seleciona a
// linha e troca de aba. Com campo, menu ou confirmação aberta, o mouse não age.
func (m Model) mouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.filter.InputMode || m.actionMode != ActionNone || m.janelaPequena() {
		return m, nil
	}
	delta := 0
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		delta = -passoRoda
	case tea.MouseButtonWheelDown:
		delta = passoRoda
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionPress && !m.ajuda && !m.quadro.aberto {
			return m.clique(msg.X, msg.Y)
		}
		return m, nil
	default:
		return m, nil
	}
	switch {
	case m.ajuda:
		m.ajudaScroll = max(0, min(m.ajudaScroll+delta, m.maxRolagemAjuda()))
	case m.quadro.aberto:
		m.rolarQuadro(delta)
	case m.err == nil: // sem conexão, a lista não está na tela
		m.rolarLista(delta)
	}
	return m, nil
}

// rolarLista rola a janela da lista e traz a seleção para dentro dela: a ação
// atinge sempre uma linha que está na tela.
func (m *Model) rolarLista(delta int) {
	count := m.itemCount()
	if count == 0 {
		return
	}
	h := m.linhasDeDados()
	m.scrollY = max(0, min(m.scrollY+delta, max(0, count-h)))
	m.selectedIndex = max(m.scrollY, min(m.selectedIndex, min(count-1, m.scrollY+h-1)))
}

// Geometria da lista na moldura: masthead e régua em cima, depois as abas e
// os filtros; o corpo começa com uma linha em branco e o cabeçalho da tabela.
const (
	linhaDasAbas     = 2
	linhasAntesDaAba = 2 // masthead e régua
)

// abasNaTela é a linha de abas que a moldura desenha na largura atual.
func (m Model) abasNaTela() string {
	normais, compactas := m.linhaAbas()
	if yui.ModoPara(m.width, m.height) == yui.ModoCompacto {
		return compactas
	}
	return normais
}

// primeiraLinhaDeDados é a linha da tela (0 no topo) da primeira linha da
// tabela; com o painel por cima, a lista começa abaixo dele e do fio.
func (m Model) primeiraLinhaDeDados() int {
	filtros := 0
	if m.err == nil && m.linhaFiltros(m.itensDaAba(m.activeTab)) != "" {
		filtros = 1
	}
	painel := 0
	if m.painelPorCima() {
		painel = m.alturaDoPainelPorCima() + 1
	}
	return linhasAntesDaAba + 1 + filtros + painel + m.linhasAntesDaTabela()
}

// regiaoAba é o trecho de colunas [ini, fim) de uma aba na linha de abas.
type regiaoAba struct {
	ini, fim int
	aba      ResourceTab
}

// regioesDasAbas acha cada aba na linha desenhada: o número e o começo do
// título ("2 Dep"), em ordem; a aba vai do separador anterior ao seguinte.
// Abas fora da janela ("⇥ mais") não têm região.
func (m Model) regioesDasAbas() []regiaoAba {
	linha := ansi.Strip(m.abasNaTela())
	sep := "   "
	if yui.ModoPara(m.width, m.height) == yui.ModoCompacto {
		sep = " · "
	}
	var regioes []regiaoAba
	de := 0
	for t := ResourceTab(0); t < tabCount; t++ {
		titulo := []rune(abas[t].titulo)
		alvo := strconv.Itoa(int(t)+1) + " " + string(titulo[:min(3, len(titulo))])
		i := strings.Index(linha[de:], alvo)
		if i < 0 {
			continue
		}
		i += de
		ini := 0
		if j := strings.LastIndex(linha[:i], sep); j >= 0 {
			ini = j + len(sep)
		}
		fim := len(linha)
		if j := strings.Index(linha[i:], sep); j >= 0 {
			fim = i + j
		}
		regioes = append(regioes, regiaoAba{ini: ansi.StringWidth(linha[:ini]), fim: ansi.StringWidth(linha[:fim]), aba: t})
		de = fim
	}
	return regioes
}

// clique seleciona a linha da tabela ou troca para a aba clicada.
func (m Model) clique(x, y int) (tea.Model, tea.Cmd) {
	if m.janelaPequena() {
		return m, nil
	}
	if y == linhaDasAbas {
		for _, r := range m.regioesDasAbas() {
			if x >= r.ini && x < r.fim {
				if r.aba == m.activeTab {
					return m, nil
				}
				return m.irParaAba(r.aba)
			}
		}
		return m, nil
	}
	if m.err != nil {
		return m, nil
	}
	i := y - m.primeiraLinhaDeDados()
	visiveis := min(m.linhasDeDados(), m.itemCount()-m.scrollY)
	if i >= 0 && i < visiveis && x < m.larguraDaLista() { // o clique no painel ao lado não seleciona
		m.selectedIndex = m.scrollY + i
	}
	return m, nil
}

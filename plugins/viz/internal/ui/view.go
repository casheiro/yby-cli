package ui

import (
	"fmt"
	"strconv"
	"strings"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Textos fixos da moldura do Viz.
const (
	instrumento    = "VIZ"
	rotuloViz      = "INSTR. 05 · OBSERVAÇÃO"
	textoAoVivo    = "ao vivo · 2 s"
	todosOsNS      = "todos os namespaces"
	dicaConexao    = "Confira se o cluster está no ar e se o kubeconfig (~/.kube/config ou KUBECONFIG) aponta para ele."
	espacoContexto = "   "
)

// View renderiza a interface do Viz
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "" // o Bubble Tea manda o tamanho da janela logo no início
	}
	if m.viewMode == ModeDetail {
		return m.viewDetalhe()
	}

	p := m.partesBase()
	itens := m.itensDaAba(m.activeTab)
	if m.err == nil {
		p.Filtros = m.linhaFiltros(itens)
	}
	p.Lista = m.corpoLista(itens)
	p.Mensagem = m.mensagem(itens)
	p.Teclas = m.teclas()
	return yui.Moldura(m.width, m.height, p)
}

// partesBase monta o que toda tela do Viz tem: masthead, abas e hora.
func (m Model) partesBase() yui.Partes {
	normais, compactas := m.linhaAbas()
	return yui.Partes{
		Instrumento:   instrumento,
		Rotulo:        rotuloViz,
		Contexto:      m.contexto(),
		Abas:          normais,
		AbasCompactas: compactas,
		Hora:          m.atualizado,
	}
}

// namespaceNaTela é o namespace escolhido, ou "todos os namespaces".
func (m Model) namespaceNaTela() string {
	if m.filter.Namespace == "" {
		return todosOsNS
	}
	return m.filter.Namespace
}

// contexto é "cluster ▸ namespace" (só o namespace sem contexto no
// kubeconfig) e o indicador ao vivo enquanto a conexão está de pé.
func (m Model) contexto() string {
	segs := []string{m.namespaceNaTela()}
	if c := m.contextoDoCluster(); c != "" {
		segs = []string{c, m.namespaceNaTela()}
	}
	ctx := yui.CaminhoContexto(segs...)
	if m.err == nil && !m.reconnecting {
		ctx += espacoContexto + yui.IndicadorVivo(textoAoVivo)
	}
	return ctx
}

// linhaAbas devolve as abas completas ("◆ 1 Pods 8") e as compactas (abaixo
// de 80 colunas). Aba que ainda não chegou do cluster fica sem número.
func (m Model) linhaAbas() (normais, compactas string) {
	lista := make([]yui.Aba, tabCount)
	curtas := make([]yui.Aba, tabCount)
	for t := ResourceTab(0); t < tabCount; t++ {
		n := -1
		if m.carregadas&(1<<uint(t)) != 0 {
			n = len(m.itensDaAba(t))
		}
		lista[t] = yui.Aba{Titulo: strconv.Itoa(int(t)+1) + " " + abas[t].titulo, Contagem: n}
		curtas[t] = yui.Aba{Titulo: abas[t].titulo, Contagem: n}
	}
	return yui.Abas(lista, int(m.activeTab), m.width), yui.Abas(curtas, int(m.activeTab), m.width)
}

// carregada diz se a lista da aba já chegou do cluster ao menos uma vez.
func (m Model) carregada(tab ResourceTab) bool {
	return m.carregadas&(1<<uint(tab)) != 0
}

// chips são os filtros ativos da lista da aba. O namespace não entra: ele é
// o contexto, no masthead; o status só entra nas abas em que ele vale.
func (m Model) chips() []yui.Chip {
	var chips []yui.Chip
	if m.filter.LabelSelector != "" {
		chips = append(chips, yui.Chip{Chave: m.filter.LabelSelector})
	}
	if m.filter.StatusFilter != "" && abas[m.activeTab].filtroStatus {
		chips = append(chips, yui.Chip{Chave: "status", Valor: m.filter.StatusFilter})
	}
	if m.filter.SearchQuery != "" {
		chips = append(chips, yui.Chip{Chave: "busca", Valor: m.filter.SearchQuery})
	}
	return chips
}

// linhaFiltros: a ordem da lista, os chips com "N de M" à direita ou, sem
// filtro, o resumo ("8 pods · 2 com problema").
func (m Model) linhaFiltros(itens []itemLista) string {
	s := yui.S()
	prefixo := s.TintaSuave.Render("ordem") + " " + s.Tinta.Render(abas[m.activeTab].ordem) + "   "
	resto := m.width - lipgloss.Width(prefixo)
	if chips := m.chips(); len(chips) > 0 {
		total := len(abas[m.activeTab].itens(m)) // a lista inteira do namespace: os filtros são da tela
		return prefixo + yui.Filtros(chips, len(itens), total, resto)
	}
	// Sem espaço para tudo, a dica de busca sai antes do resumo.
	esq := prefixo + yui.Filtros(nil, 0, 0, resto)
	busca := teclaBuscar()
	comDica := esq + "   " + s.TintaSuave.Render(busca.Help().Key+" "+busca.Help().Desc)
	resumo := m.resumo(itens)
	if lipgloss.Width(comDica)+2+lipgloss.Width(resumo) <= m.width {
		esq = comDica
	}
	return esqDir(esq, resumo, m.width)
}

// resumo conta a lista e os problemas dela, em tinta suave.
func (m Model) resumo(itens []itemLista) string {
	if !m.carregada(m.activeTab) {
		return ""
	}
	d := abas[m.activeTab]
	txt := d.contar(len(itens))
	problemas := 0
	for _, it := range itens {
		if it.grupo.problema() {
			problemas++
		}
	}
	switch {
	case problemas == 1 && m.activeTab == TabEvents:
		txt += " · 1 aviso"
	case problemas > 0 && m.activeTab == TabEvents:
		txt += " · " + strconv.Itoa(problemas) + " avisos"
	case problemas > 0:
		txt += " · " + strconv.Itoa(problemas) + " com problema"
	}
	return yui.S().TintaSuave.Render(txt)
}

// esqDir põe dir no canto direito de uma linha de largura células; sem
// espaço para os dois, fica só a esquerda (a moldura trunca o excesso).
func esqDir(esq, dir string, largura int) string {
	le, ld := lipgloss.Width(esq), lipgloss.Width(dir)
	if dir == "" || le+2+ld > largura {
		return esq
	}
	return esq + strings.Repeat(" ", largura-le-ld) + dir
}

// temFiltro diz se algum filtro da lista da aba está ativo (um chip na tela).
func (m Model) temFiltro() bool {
	return len(m.chips()) > 0
}

// algumFiltro diz se label, status ou busca estão escolhidos (o esc os limpa),
// mesmo que o status não valha na aba ativa.
func (m Model) algumFiltro() bool {
	return m.filter.LabelSelector != "" || m.filter.StatusFilter != "" || m.filter.SearchQuery != ""
}

// corpoLista é a tabela da aba (ou o estado vazio, carregando ou erro), com
// uma linha em branco acima, como na prancha. Quando a lista não cabe, a
// última linha diz quantas ficaram acima e abaixo da janela.
func (m Model) corpoLista(itens []itemLista) string {
	if m.err != nil {
		return "\n" + m.estadoErro()
	}
	if len(itens) == 0 {
		return "\n" + m.listaVazia()
	}
	colunas, linhas := m.tabelaDaAba(m.activeTab, itens)
	// A tabela inteira mede as colunas (a largura não muda ao rolar); a janela
	// mostra o cabeçalho e as linhas a partir de scrollY.
	tabela := strings.Split(yui.Tabela(colunas, linhas, m.selectedIndex, m.width), "\n")
	m.clampScroll(len(itens))
	ini := m.scrollY
	fim := min(len(itens), ini+m.linhasDeDados())
	corpo := append(tabela[:1], tabela[1+ini:1+fim]...)
	if ini > 0 || fim < len(itens) {
		corpo = append(corpo, m.avisoRolagem(ini, len(itens)-fim))
	}
	return "\n" + strings.Join(corpo, "\n")
}

// avisoRolagem é a nota "↑ 3 pods acima   ↓ 29 pods abaixo" da lista que
// não cabe na janela.
func (m Model) avisoRolagem(acima, abaixo int) string {
	d := abas[m.activeTab]
	var partes []string
	if acima > 0 {
		partes = append(partes, yui.AvisoRolagem(yui.SetaAcima, d.contar(acima)+" acima"))
	}
	if abaixo > 0 {
		partes = append(partes, yui.AvisoRolagem(yui.SetaAbaixo, d.contar(abaixo)+" abaixo"))
	}
	return strings.Join(partes, "   ")
}

// listaVazia explica a lista vazia da aba ativa: ainda carregando, nada que
// passe nos filtros ou nada no cluster.
func (m Model) listaVazia() string {
	d := abas[m.activeTab]
	onde := "em " + m.namespaceNaTela()
	if d.semNamespace || m.filter.Namespace == "" {
		onde = "no cluster"
	}
	switch {
	case !m.carregada(m.activeTab):
		de := "de " + m.namespaceNaTela()
		if d.semNamespace {
			de = "do cluster"
		}
		return yui.NovoEstadoCarregandoCom("lendo "+d.plural+" "+de, func() bool { return false }).View()
	case m.temFiltro():
		return yui.NovoEstadoVazio("nenhum "+d.singular+" com esse filtro", "esc limpa").View()
	}
	return yui.NovoEstadoVazio("nenhum "+d.singular+" "+onde, "").View()
}

// estadoErro mostra a falha de conexão, o erro e o que conferir.
func (m Model) estadoErro() string {
	s := yui.S()
	linhas := []string{yui.NovoEstadoErro("sem acesso ao cluster", "q sai").View(), ""}
	for _, l := range strings.Split(ansi.Wrap(m.err.Error(), max(20, m.width), ""), "\n") {
		linhas = append(linhas, s.TintaSuave.Render(l))
	}
	for _, l := range strings.Split(ansi.Wrap(dicaConexao, max(20, m.width), ""), "\n") {
		linhas = append(linhas, s.TintaSuave.Render(l))
	}
	return strings.Join(linhas, "\n")
}

// rotulosDosCampos nomeia o campo de filtro sendo digitado.
var rotulosDosCampos = map[string]string{
	"namespace": "namespace",
	"label":     "label",
	"status":    "status",
	"search":    "buscar por nome",
}

// cursor é o bloco em tinta invertida no fim do texto digitado.
func cursor() string {
	return yui.S().Selecionado.Render(" ")
}

// mensagem é a frase mais importante agora: o campo sendo digitado, a ação
// pedindo confirmação, o resultado da última ação, a reconexão, o erro ou o
// primeiro problema da lista (a ordem já põe o mais grave no topo).
func (m Model) mensagem(itens []itemLista) string {
	s, g := yui.S(), yui.G()
	switch {
	case m.filter.InputMode:
		return s.Negrito.Render(rotulosDosCampos[m.filter.InputField]) + " " + s.TintaSuave.Render(g.Prompt) + " " +
			s.Tinta.Render(m.filter.InputBuffer) + cursor()
	case m.actionMode != ActionNone:
		return renderActionPrompt(m.actionMode, m.actionKind, m.actionTarget, m.actionBuffer)
	case m.actionFeedback != "":
		return m.actionFeedback
	case m.reconnecting:
		return s.Aviso.Render(g.Andamento) + " " + s.Tinta.Render("reconectando ao cluster…")
	case m.err != nil:
		return frase(yui.StatusFalha, "sem conexão com o cluster")
	}
	for _, it := range itens {
		if it.grupo.problema() && it.frase != "" {
			return it.frase
		}
	}
	d := abas[m.activeTab]
	if m.carregada(m.activeTab) && len(itens) > 0 && d.ordem == ordemProblemas {
		return frase(yui.StatusRodando, "nenhum "+d.singular+" com problema")
	}
	return ""
}

// atalho cria um binding só para a legenda, com o rótulo no modo visual atual.
func atalho(rotulo, descricao string, teclas ...string) key.Binding {
	return key.NewBinding(key.WithKeys(teclas...), key.WithHelp(yui.RotuloTecla(rotulo), descricao))
}

// setas é o rótulo de "mover/rolar": ↑↓ no Unicode, up/down no ASCII (como
// na especificação e no Bard).
func setas() string {
	if yui.Atual().ASCII {
		return "up/down"
	}
	return "↑↓"
}

func teclaBuscar() key.Binding { return atalho("f", "buscar", "f") }

// teclas é a legenda da tela, em ordem de importância (a moldura descarta do
// fim quando não cabe e põe "? mais"). As teclas são as de hoje; a troca é a T020.
func (m Model) teclas() []key.Binding {
	cancelar := atalho("esc", "cancelar", "esc")
	switch {
	case m.err != nil:
		return nil
	case m.filter.InputMode:
		return []key.Binding{atalho("↵", "aplicar", "enter"), cancelar}
	case m.actionMode != ActionNone:
		return []key.Binding{atalho("↵", "confirmar", "enter"), cancelar}
	}
	return append(m.teclasDaLista(false), atalho("?", "ajuda", "?"))
}

// teclasDaLista são as teclas da lista da aba ativa; completa traz também as
// que a legenda deixa de fora (página, início e fim, números, esc), para o ?.
func (m Model) teclasDaLista(completa bool) []key.Binding {
	d := abas[m.activeTab]
	t := []key.Binding{atalho(setas(), "mover", "up", "down", "j", "k"), atalho("⇥", "aba", "tab")}
	if completa {
		t = append(t, atalho("⇧⇥", "aba anterior", "shift+tab"), atalho("1-9", "ir para a aba", "1"),
			atalho("pgup/pgdn", "página", "pgup", "pgdown"), atalho("home/end", "início e fim", "home", "end"))
	}
	t = append(t, teclaBuscar(), atalho("/", "namespace", "/"))
	if d.kind == "pod" {
		t = append(t, atalho("l", "logs", "l"))
	}
	t = append(t, atalho("y", "yaml", "y"), atalho("e", "eventos", "e"))
	if d.kind != "node" && d.kind != "event" {
		t = append(t, atalho("d", "excluir", "d"))
	}
	if d.kind == "deployment" || d.kind == "statefulset" {
		t = append(t, atalho("s", "escalar", "s"), atalho("r", "reiniciar", "r"))
	}
	if completa || m.temFiltro() {
		t = append(t, atalho("esc", "limpar filtros", "esc"))
	}
	t = append(t, atalho("L", "label", "L"))
	if d.filtroStatus {
		t = append(t, atalho("S", "status", "S"))
	}
	if completa {
		t = append(t, atalho("q", "sair", "q"), atalho("ctrl+c", "sair de qualquer lugar", "ctrl+c"))
	}
	return t
}

// textoDasTeclas é a lista inteira das teclas da aba, uma por linha, que o ?
// abre no detalhe até a tela de ajuda da T020.
func (m Model) textoDasTeclas() string {
	teclas := m.teclasDaLista(true)
	largura := 0
	for _, b := range teclas {
		largura = max(largura, lipgloss.Width(b.Help().Key))
	}
	linhas := make([]string, 0, len(teclas))
	for _, b := range teclas {
		h := b.Help()
		linhas = append(linhas, h.Key+strings.Repeat(" ", largura-lipgloss.Width(h.Key)+3)+h.Desc)
	}
	return strings.Join(linhas, "\n")
}

// viewDetalhe mostra logs, YAML ou eventos na moldura, com rolagem própria.
func (m Model) viewDetalhe() string {
	s := yui.S()
	p := m.partesBase()
	linhas := strings.Split(strings.ReplaceAll(strings.TrimRight(m.detailContent, "\n"), "\t", "    "), "\n")
	h := m.alturaCorpo()
	ini := min(max(0, m.detailScrollY), max(0, len(linhas)-h))
	fim := min(len(linhas), ini+h)
	p.Filtros = esqDir(s.Rotulo.Render(strings.ToUpper(m.detailTitle)),
		s.TintaSuave.Render(fmt.Sprintf("linha %d de %d", ini+1, len(linhas))), m.width)
	p.Lista = strings.Join(linhas[ini:fim], "\n")
	p.Teclas = []key.Binding{atalho(setas(), "rolar", "up", "down"), atalho("pgup/pgdn", "página", "pgup", "pgdown"),
		atalho("esc", "voltar", "esc")}
	return yui.Moldura(m.width, m.height, p)
}

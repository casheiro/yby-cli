package ui

import (
	"strconv"
	"strings"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
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
	dicaCredencial = "O plugin de credencial do kubeconfig não entregou o token: renove-o com 'yby cloud refresh' e tecle r."
	espacoContexto = "   "
)

// View renderiza a interface do Viz
func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "" // o Bubble Tea manda o tamanho da janela logo no início
	}
	if m.ajuda {
		return m.viewAjuda()
	}
	if m.quadro.aberto {
		return m.viewQuadro()
	}

	p := m.partesBase()
	itens := m.itensDaAba(m.activeTab)
	if m.err == nil {
		p.Filtros = m.linhaFiltros(itens)
	}
	if m.acao.aberta() && m.listaNaTela() {
		p.Lista, p.Detalhe = m.corpoComAcao(itens)
	} else {
		p.Lista = m.corpoLista(itens)
		p.Detalhe = m.detalheNaTela()
	}
	p.Mensagem = m.mensagem(itens)
	p.Teclas, p.Sair = m.teclas()
	// No campo e na confirmação o ? é letra (ou não abre a ajuda): a legenda não oferece "? mais".
	p.SemAjuda = m.filter.InputMode || (m.acao.aberta() && m.acao.etapa != noMenu)
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

// nomeDoCluster encurta para o masthead os contextos que os CLIs das clouds gravam: o ARN
// do EKS (arn:aws:eks:<região>:<conta>:cluster/<nome>) e o gke_<projeto>_<local>_<nome>
// do GKE viram o nome do cluster. Os outros contextos ficam como estão.
func nomeDoCluster(contexto string) string {
	if strings.HasPrefix(contexto, "arn:aws:eks:") {
		if _, nome, ok := strings.Cut(contexto, ":cluster/"); ok && nome != "" {
			return nome
		}
	}
	if partes := strings.SplitN(contexto, "_", 4); len(partes) == 4 && partes[0] == "gke" && partes[3] != "" {
		return partes[3]
	}
	return contexto
}

// contexto é "cluster ▸ namespace" (só o namespace sem contexto no
// kubeconfig) e o indicador ao vivo enquanto a conexão está de pé.
func (m Model) contexto() string {
	segs := []string{m.namespaceNaTela()}
	if c := m.contextoDoCluster(); c != "" {
		segs = []string{nomeDoCluster(c), m.namespaceNaTela()}
	}
	ctx := yui.CaminhoContexto(segs...)
	if m.err == nil && !m.reconnecting {
		ctx += espacoContexto + yui.IndicadorVivo(textoAoVivo)
	}
	return ctx
}

// linhaAbas devolve as abas completas ("◆ 1 Pods 8") e as compactas (abaixo
// de 80 colunas). O número de cada aba é o da lista que ela mostraria aberta:
// a lista do namespace (pelo watch, ao vivo) com os filtros da tela (label e
// busca em todas, status só nos pods). Aba que ainda não chegou do cluster,
// ou que o RBAC não deixa listar, fica sem número.
func (m Model) linhaAbas() (normais, compactas string) {
	lista := make([]yui.Aba, tabCount)
	curtas := make([]yui.Aba, tabCount)
	for t := ResourceTab(0); t < tabCount; t++ {
		n := -1
		if m.carregadas&(1<<uint(t)) != 0 {
			n = m.contagemDaAba(t)
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
		total := m.tamanhoDaFonte(m.activeTab) // a lista inteira do namespace: os filtros são da tela
		return prefixo + yui.Filtros(chips, len(itens), total, resto)
	}
	// Sem espaço para tudo, a dica de busca sai antes do resumo.
	esq := prefixo + yui.Filtros(nil, 0, 0, resto)
	busca := mapa().Lista.Buscar
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
	if err := m.negadas[m.activeTab]; err != nil {
		return "\n" + m.estadoNegado(err)
	}
	if len(itens) == 0 {
		return "\n" + m.listaVazia()
	}
	colunas, linhas := m.tabelaDaAba(m.activeTab, itens)
	// A tabela inteira mede as colunas (a largura não muda ao rolar); só o
	// cabeçalho e as linhas da janela, a partir de scrollY, são desenhados.
	m.clampScroll(len(itens))
	ini := m.scrollY
	fim := min(len(itens), ini+m.linhasDeDados())
	corpo := strings.Split(yui.TabelaJanela(colunas, linhas, m.selectedIndex, m.larguraDaLista(), ini, fim), "\n")
	if ini > 0 || fim < len(itens) {
		corpo = append(corpo, m.avisoRolagem(ini, len(itens)-fim))
	}
	if m.painelPorCima() {
		return strings.Join(corpo, "\n") // a moldura já põe o fio entre o painel e a lista
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

// estadoErro mostra a falha de conexão, o erro e o que conferir: a
// credencial do plugin exec do kubeconfig ou o cluster e o kubeconfig.
func (m Model) estadoErro() string {
	dica := "q sai"
	if m.client != nil {
		dica = "r tenta de novo"
	}
	conferir := dicaConexao
	if monitor.ErroDeCredencial(m.err) {
		conferir = dicaCredencial
	}
	return m.estadoComErro(yui.NovoEstadoErro("sem acesso ao cluster", dica), m.err, conferir)
}

// estadoNegado mostra a aba que o RBAC não deixa listar: o que falta, onde e
// o que tentar (outro namespace, quando o recurso é de namespace).
func (m Model) estadoNegado(err error) string {
	d := abas[m.activeTab]
	onde, dica := "em "+m.filter.Namespace, "n troca o namespace"
	switch {
	case d.semNamespace:
		onde, dica = "no cluster", "r tenta de novo"
	case m.filter.Namespace == "":
		onde, dica = "em "+todosOsNS, "n escolhe um namespace"
	}
	return m.estadoComErro(yui.NovoEstadoErro("sem permissão para listar "+d.plural+" "+onde, dica), err, "")
}

// estadoComErro é o estado de erro com o erro e o texto de apoio embaixo,
// quebrados na largura da janela.
func (m Model) estadoComErro(estado yui.Estado, err error, apoio string) string {
	s := yui.S()
	linhas := []string{estado.View(), ""}
	for _, texto := range []string{err.Error(), apoio} {
		if texto == "" {
			continue
		}
		for _, l := range strings.Split(ansi.Wrap(texto, max(20, m.width), ""), "\n") {
			linhas = append(linhas, s.TintaSuave.Render(l))
		}
	}
	return strings.Join(linhas, "\n")
}

// rotulosDosCampos nomeia o campo de texto sendo digitado.
var rotulosDosCampos = map[string]string{
	campoBusca:     "buscar",
	campoNamespace: "namespace",
	campoFiltro:    "filtrar",
}

// dicaDoCampo explica, ao lado do campo, o que ele aceita.
func (m Model) dicaDoCampo() string {
	switch m.filter.InputField {
	case campoBusca:
		return "filtra por nome enquanto digita"
	case campoNamespace:
		return "vazio mostra todos os namespaces"
	}
	if !abas[m.activeTab].filtroStatus {
		return "label (app=web, tier!=db) · vazio limpa"
	}
	return "status (Running) ou label (app=web, tier!=db) · vazio limpa"
}

// cursor é o bloco em tinta invertida no fim do texto digitado.
func cursor() string {
	return yui.S().Selecionado.Render(" ")
}

// mensagem é a frase mais importante agora: o campo sendo digitado, o aviso
// da última tecla, a ação em andamento, o resultado da última (o Toast), a recarga, a
// reconexão, o erro ou o primeiro problema da lista (a ordem já põe o mais
// grave no topo). O menu e a confirmação ficam no corpo.
func (m Model) mensagem(itens []itemLista) string {
	s, g := yui.S(), yui.G()
	switch {
	case m.filter.InputMode:
		return s.Negrito.Render(rotulosDosCampos[m.filter.InputField]) + " " + s.TintaSuave.Render(g.Prompt) + " " +
			s.Tinta.Render(m.filter.InputBuffer) + cursor() + "   " + s.TintaSuave.Render(m.dicaDoCampo())
	case m.actionFeedback != "":
		return m.actionFeedback
	case m.executando != "":
		return s.Aviso.Render(g.Andamento) + " " + s.Tinta.Render(m.executando+g.Reticencias)
	case m.toast.Visivel():
		return m.toast.View()
	case m.recarregando:
		return s.Aviso.Render(g.Andamento) + " " + s.Tinta.Render("recarregando "+abas[m.activeTab].plural+g.Reticencias)
	case m.reconnecting:
		return s.Aviso.Render(g.Andamento) + " " + s.Tinta.Render("reconectando ao cluster"+g.Reticencias)
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

// teclas é a legenda da tela, só com as teclas que funcionam nela, em ordem
// de importância (a moldura descarta do fim quando não cabe e põe "? mais"),
// e a tecla de saída do canto direito: vazia é "q sair"; nas camadas, em que
// q é letra ou não sai, ctrl+c.
func (m Model) teclas() ([]key.Binding, key.Binding) {
	k := mapa()
	g, l := k.Global, k.Lista
	voltar := atalho("esc", "voltar", "esc")
	switch {
	case m.filter.InputMode:
		aplicar := "aplicar"
		if m.filter.InputField == campoBusca {
			aplicar = "confirmar"
		}
		return []key.Binding{atalho("↵", aplicar, "enter"), atalho("esc", "cancelar", "esc")}, g.SairForcado
	case m.acao.aberta():
		return m.teclasDaAcao(), g.SairForcado
	case m.err != nil:
		if m.client != nil {
			return []key.Binding{atalho("r", "tentar de novo", "r")}, key.Binding{}
		}
		return nil, key.Binding{}
	case m.painelPorCima():
		// O painel por cima não tem rodapé: as teclas dele vêm para a legenda.
		t := append([]key.Binding{atalho(setas(), "mover", "up", "down", "j", "k")}, teclasDoPainel(abas[m.activeTab].kind, 0)...)
		if len(acoesDoTipo(abas[m.activeTab].kind)) > 0 {
			t = append(t, l.Acoes)
		}
		if m.width < yui.LarguraColuna {
			// A legenda compacta guarda só três teclas: a saída do painel vem primeiro.
			return append([]key.Binding{voltar}, t...), g.SairForcado
		}
		return append(t, voltar, g.Ajuda), g.SairForcado
	}
	t := []key.Binding{atalho(setas(), "mover", "up", "down", "j", "k"), l.Abrir, l.Buscar, l.Filtrar}
	if len(acoesDoTipo(abas[m.activeTab].kind)) > 0 {
		t = append(t, l.Acoes)
	}
	t = append(t, l.Namespace)
	if m.temFiltro() {
		t = append(t, atalho("esc", "limpar filtros", "esc"))
	}
	if !m.painelAoLado() && m.itemCount() > 0 {
		// Sem o painel ao lado (abaixo de 120 colunas), o b do painel vem para a
		// legenda depois das teclas da prancha, só quando cabe inteira: sem
		// espaço, ele não tira o "? ajuda" do lugar.
		comBard := append(append([]key.Binding(nil), t...), l.Bard, g.Ajuda)
		if larguraDaLegenda(comBard)+2+larguraDaLegenda([]key.Binding{g.Sair}) <= m.width {
			t = append(t, l.Bard)
		}
	}
	return append(t, g.Ajuda), key.Binding{}
}

// linhaAjuda é uma tecla da tela ? e o que ela faz.
type linhaAjuda struct{ tecla, desc string }

// grupoAjuda é um bloco da tela ?, como na folha Atalhos.
type grupoAjuda struct {
	titulo string
	linhas []linhaAjuda
}

// gruposDaAjuda são as teclas da tela, agrupadas como na folha Atalhos: as de
// toda TUI e as da lista da aba ativa (só as que valem nela).
func (m Model) gruposDaAjuda() []grupoAjuda {
	d := abas[m.activeTab]
	todas := grupoAjuda{"em toda TUI", []linhaAjuda{
		{"?", "ajuda com todas as teclas da tela"},
		{"esc", "fecha o que está aberto, limpa filtros, cancela"},
		{"q", "sai (só no nível de cima)"},
		{"ctrl+c", "sai de qualquer lugar"},
		{"r", "recarrega os dados"},
		{"mouse", "a roda rola; o clique seleciona linha e aba"},
	}}
	filtrar := "filtrar por status e label"
	if !d.filtroStatus {
		filtrar = "filtrar por label"
	}
	lista := grupoAjuda{"lista · " + d.titulo, []linhaAjuda{
		{setas() + " j k", "mover"},
		{"pgup pgdn", "página"},
		{"g G", "início e fim (home end)"},
		{yui.RotuloTecla("⇥ ⇧⇥") + " 1-9", "trocar de aba"},
		{"/", "buscar por nome"},
		{"f", filtrar},
		{"n", "trocar namespace"},
		{yui.RotuloTecla("↵"), "abrir"},
	}}
	if acoes := acoesDoTipo(d.kind); len(acoes) > 0 {
		nomes := make([]string, len(acoes))
		for i, a := range acoes {
			nomes[i] = a.tecla + " " + a.rotulo
		}
		lista.linhas = append(lista.linhas, linhaAjuda{"x", "ações: " + strings.Join(nomes, " · ")})
	}
	if d.kind == "pod" {
		lista.linhas = append(lista.linhas, linhaAjuda{"l y e", "logs · yaml · eventos"})
	} else {
		lista.linhas = append(lista.linhas, linhaAjuda{"y e", "yaml · eventos"})
	}
	lista.linhas = append(lista.linhas, linhaAjuda{"b", "perguntar ao Bard"})
	return []grupoAjuda{todas, lista}
}

// linhasDoGrupo desenha um grupo: o título e as teclas com a coluna da tecla
// na largura dada.
func linhasDoGrupo(gr grupoAjuda, larguraTecla int) []string {
	s := yui.S()
	linhas := []string{s.Rotulo.Render(strings.ToUpper(gr.titulo))}
	for _, l := range gr.linhas {
		linhas = append(linhas, s.Negrito.Render(l.tecla)+strings.Repeat(" ", larguraTecla-lipgloss.Width(l.tecla)+2)+
			s.TintaSuave.Render(l.desc))
	}
	return linhas
}

// larguraDasLinhas é a maior largura entre as linhas.
func larguraDasLinhas(linhas []string) int {
	w := 0
	for _, l := range linhas {
		w = max(w, lipgloss.Width(l))
	}
	return w
}

// linhasDaAjuda é o conteúdo da tela ?: os dois grupos lado a lado quando
// cabem, um embaixo do outro quando não.
func (m Model) linhasDaAjuda() []string {
	grupos := m.gruposDaAjuda()
	kw := 0
	for _, gr := range grupos {
		for _, l := range gr.linhas {
			kw = max(kw, lipgloss.Width(l.tecla))
		}
	}
	a, b := linhasDoGrupo(grupos[0], kw), linhasDoGrupo(grupos[1], kw)
	wa := larguraDasLinhas(a)
	const entre = 4
	if wa+entre+larguraDasLinhas(b)+4 > m.width {
		return append(append(a, ""), b...)
	}
	linhas := make([]string, max(len(a), len(b)))
	for i := range linhas {
		esq := ""
		if i < len(a) {
			esq = a[i]
		}
		if i < len(b) {
			esq += strings.Repeat(" ", wa+entre-lipgloss.Width(esq)) + b[i]
		}
		linhas[i] = esq
	}
	return linhas
}

// linhasVisiveisDaAjuda é quanto da ajuda cabe no corpo: menos a linha em
// branco de cima e as bordas do quadro.
func (m Model) linhasVisiveisDaAjuda() int {
	return max(1, m.alturaCorpo()-3)
}

// maxRolagemAjuda é a maior rolagem da ajuda (zero quando ela cabe inteira).
func (m Model) maxRolagemAjuda() int {
	return max(0, len(m.linhasDaAjuda())-m.linhasVisiveisDaAjuda())
}

// viewAjuda mostra a tela ? no lugar da lista, num quadro, com a moldura em volta.
func (m Model) viewAjuda() string {
	p := m.partesBase()
	if m.err == nil {
		p.Filtros = m.linhaFiltros(m.itensDaAba(m.activeTab))
	}
	linhas := m.linhasDaAjuda()
	ini := min(m.ajudaScroll, m.maxRolagemAjuda())
	fim := min(len(linhas), ini+m.linhasVisiveisDaAjuda())
	largura := min(m.width, larguraDasLinhas(linhas)+4)
	p.Lista = "\n" + yui.Quadro("teclas · "+abas[m.activeTab].titulo, strings.Join(linhas[ini:fim], "\n"), largura, yui.QuadroFoco)
	p.Mensagem = m.actionFeedback
	p.Teclas = []key.Binding{atalho("esc", "fechar", "esc")}
	if m.maxRolagemAjuda() > 0 {
		p.Teclas = append(p.Teclas, atalho(setas(), "rolar", "up", "down"))
	}
	p.Sair = mapa().Global.SairForcado
	p.SemAjuda = true // aqui o ? fecha a ajuda
	return yui.Moldura(m.width, m.height, p)
}

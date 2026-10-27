package cmd

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Linhas fixas da moldura do painel: masthead, régua e resumo em cima; régua, estado e
// legenda embaixo.
const (
	linhasTopo   = 3
	linhasRodape = 3
)

// larguraChaveDetalhe é a coluna das chaves (conta, papel…) no detalhe.
const larguraChaveDetalhe = 10

// internoDuasColunas é a largura interna mínima do detalhe para IDENTIDADE e AUDITORIA
// lado a lado (a partir de 80 colunas).
const internoDuasColunas = 76

// larguraAcao é a coluna "✓ renova" da auditoria, com o respiro até a descrição.
const larguraAcao = 10

// auditoriaNoDetalhe é quantos registros da auditoria o detalhe mostra.
const auditoriaNoDetalhe = 3

// layoutPainel é onde o detalhe do cluster selecionado aparece.
//
// A partir de 120 colunas o detalhe fica sempre na tela, acompanhando a seleção: abaixo
// da tabela, como na prancha, quando a altura comporta ao menos 3 linhas da tabela (com o
// aviso de rolagem, se houver) e o detalhe inteiro do cluster de detalhe mais alto entre
// os visíveis, para o layout não mudar ao mover a seleção; senão, ao lado da tabela (60/40
// da moldura), numa coluna só, encolhido até a altura do corpo. Abaixo de 120 colunas, o ↵
// abre o detalhe por cima da lista (ele fica com o que precisa, e a tabela com o resto,
// ao menos a linha selecionada) e o esc o fecha.
type layoutPainel int

const (
	layoutSemDetalhe layoutPainel = iota
	layoutAbaixo
	layoutAoLado
	layoutPorCima
)

func (m painelCloud) alturaCorpo() int {
	return max(1, m.altura-linhasTopo-linhasRodape)
}

// temTabela diz se o corpo mostra a tabela (e não um estado vazio, carregando ou erro).
func (m painelCloud) temTabela() bool {
	return m.carregado && len(m.visiveis()) > 0
}

func (m painelCloud) layout() layoutPainel {
	if !m.temTabela() {
		return layoutSemDetalhe
	}
	switch ui.ModoPara(m.largura, m.altura) {
	case ui.ModoLadoALado:
		if m.linhasAbaixo() >= min(len(m.visiveis()), 3) {
			return layoutAbaixo
		}
		return layoutAoLado
	case ui.ModoColuna, ui.ModoCompacto:
		if m.detalheAberto {
			return layoutPorCima
		}
	}
	return layoutSemDetalhe
}

// linhasAbaixo é quantas linhas de cluster cabem com o detalhe abaixo da tabela: o corpo
// menos a linha em branco, o cabeçalho, a linha entre a tabela e o detalhe, o detalhe mais
// alto entre os visíveis e, quando a lista não cabe, o aviso de rolagem.
func (m painelCloud) linhasAbaixo() int {
	linhas := m.alturaCorpo() - 3 - m.alturaDetalheMaior(m.largura)
	if len(m.visiveis()) > linhas {
		linhas--
	}
	return linhas
}

// larguraDaTabela é a largura da tabela no layout atual.
func (m painelCloud) larguraDaTabela() int {
	if m.layout() == layoutAoLado {
		w, _ := ui.LargurasLadoALado(m.largura)
		return w
	}
	return m.largura
}

// linhasDeDados é quantas linhas de cluster cabem na janela da tabela (descontada a
// linha do aviso de rolagem quando a lista não cabe).
func (m painelCloud) linhasDeDados() int {
	h := m.alturaCorpo()
	var linhas int
	switch m.layout() {
	case layoutAbaixo:
		return max(1, m.linhasAbaixo())
	case layoutPorCima:
		if h < 6 {
			return 1 // o corpo mostra só o detalhe
		}
		linhas = h - m.brancoPorCima() - len(strings.Split(m.detalhePorCima(), "\n")) - 1 - 1 // branco, detalhe, fio e cabeçalho
	default:
		linhas = h - 2
	}
	if len(m.visiveis()) > linhas {
		linhas-- // a última linha diz quantos ficaram fora
	}
	return max(1, linhas)
}

// View desenha o painel.
func (m painelCloud) View() string {
	if m.largura <= 0 || m.altura <= 0 {
		return "" // o Bubble Tea manda o tamanho da janela logo no início
	}
	p := ui.Partes{
		Instrumento: "CLOUD",
		Rotulo:      "PAINEL DE CLUSTERS",
		Contexto:    m.contexto(),
		Filtros:     m.linhaResumo(),
		Hora:        m.atualizado,
	}
	if m.ajuda {
		return m.viewAjuda(p)
	}
	lay := m.layout()
	p.Lista = m.corpo(lay)
	switch lay {
	case layoutAoLado:
		p.Detalhe = m.detalheAoLado()
	case layoutPorCima:
		// O detalhe por cima é montado aqui (e não na moldura, que o limita a meia altura):
		// ele fica com o que precisa e a tabela com o resto.
		p.Lista = strings.Repeat("\n", m.brancoPorCima()) + m.detalhePorCima()
		if m.alturaCorpo() >= 6 {
			p.Lista += "\n" + ui.Rule(m.largura) + "\n" + m.corpo(lay)
		}
	}
	p.Mensagem = m.mensagem()
	if !m.campo && (m.aviso != "" || m.falhaCurta(p.Mensagem)) && lipgloss.Width(p.Mensagem)+1+len("atualizado 15:04:05") > m.largura {
		// a frase da última tecla (ex.: o yby cloud connect a copiar) e a do provider sem
		// resposta (com a dica do que fazer) valem mais que a hora
		p.Hora = time.Time{}
	}
	p.Teclas, p.Sair = m.teclas(lay)
	p.SemAjuda = m.campo // no campo, o ? é letra
	return ui.Moldura(m.largura, m.altura, p)
}

// contexto é o lado direito do masthead: "4 clusters · 3 contas   ◈ próxima leitura em 18 s".
func (m painelCloud) contexto() string {
	s, g := ui.S(), ui.G()
	var partes []string
	// Abaixo de 80 colunas só cabe o indicador da leitura.
	if m.carregado && len(m.clusters) > 0 && m.largura >= ui.LarguraColuna {
		txt := plural(len(m.clusters), "cluster", "clusters")
		if n := m.contas(); n > 0 {
			txt += " " + g.Ponto + " " + plural(n, "conta", "contas")
		}
		partes = append(partes, s.TintaSuave.Render(txt))
	}
	vivo := "próxima leitura em " + strconv.Itoa(m.segundosParaLeitura()) + " s"
	if m.lendo {
		vivo = "lendo as clouds" + g.Reticencias
	}
	partes = append(partes, s.Tinta.Render(g.Andamento)+" "+s.TintaSuave.Render(vivo))
	return strings.Join(partes, "   ")
}

// segundosParaLeitura conta, arredondando para cima, quanto falta para a próxima leitura.
func (m painelCloud) segundosParaLeitura() int {
	falta := m.proxima.Sub(m.agora())
	if falta <= 0 {
		return 0
	}
	return int((falta + time.Second - 1) / time.Second)
}

// contas conta as contas distintas (conta AWS, assinatura Azure, projeto GCP) conhecidas.
func (m painelCloud) contas() int {
	vistas := map[string]bool{}
	for _, c := range m.clusters {
		if c.conta != "" {
			vistas[c.info.Provider+"/"+c.conta] = true
		}
	}
	return len(vistas)
}

func plural(n int, um, varios string) string {
	if n == 1 {
		return "1 " + um
	}
	return strconv.Itoa(n) + " " + varios
}

// grupoResumo é um grupo da linha de resumo: o glifo, o texto inteiro e o curto.
type grupoResumo struct {
	glifo          string
	estilo         lipgloss.Style
	texto, curto   string
	descartarAntes int // na falta de espaço, sai primeiro o de menor valor
}

// linhaResumo é a linha antes da tabela: "✓ 2 prontos   ▲ 1 token expira em breve   ✕ 1
// token expirado", só com os grupos que têm contagem; com busca, "busca: x · N de M" à
// direita. Sem nada para contar, a linha fica em branco (a tabela não muda de lugar).
// Quando não cabe, os grupos passam à forma curta ("1 expira logo") e, se ainda faltar,
// saem inteiros, dos menos urgentes aos mais: nunca um grupo cortado no meio, que pode
// mudar o sentido ("1 token expira" no lugar de "expirado").
func (m painelCloud) linhaResumo() string {
	s, g := ui.S(), ui.G()
	agora := m.agora()
	conta := map[estadoCluster]int{}
	for _, c := range m.clusters {
		conta[c.estadoEm(agora)]++
	}
	var grupos []grupoResumo
	if len(m.falhas) > 0 {
		// Os clusters desses providers (da leitura anterior) não entram nos outros grupos.
		t := strings.Join(m.providersSemResposta(), ", ") + " sem resposta"
		grupos = append(grupos, grupoResumo{g.Falha, s.Erro, t, t, 5})
	}
	tipos := []struct {
		estado                    estadoCluster
		glifo                     string
		estilo                    lipgloss.Style
		um, varios, curto, curtos string
		descartarAntes            int
	}{
		{estadoPronto, g.Feito, s.Ok, "pronto", "prontos", "pronto", "prontos", 0},
		{estadoExpiraLogo, g.Alerta, s.Aviso, "token expira em breve", "tokens expiram em breve", "expira logo", "expiram logo", 2},
		{estadoExpirado, g.Falha, s.Erro, "token expirado", "tokens expirados", "expirado", "expirados", 3},
		{estadoSemLogin, g.Falha, s.Erro, "sem login", "sem login", "sem login", "sem login", 4},
		{estadoNaoAtivo, g.Alerta, s.Aviso, "cluster não ativo", "clusters não ativos", "não ativo", "não ativos", 1},
	}
	for _, tp := range tipos {
		if n := conta[tp.estado]; n > 0 {
			grupos = append(grupos, grupoResumo{tp.glifo, tp.estilo, plural(n, tp.um, tp.varios), plural(n, tp.curto, tp.curtos), tp.descartarAntes})
		}
	}
	desenhar := func(gs []grupoResumo, curto bool) string {
		partes := make([]string, len(gs))
		for i, gr := range gs {
			t := gr.texto
			if curto {
				t = gr.curto
			}
			partes[i] = gr.estilo.Render(gr.glifo) + " " + s.Tinta.Render(t)
		}
		return strings.Join(partes, "   ")
	}
	esq := desenhar(grupos, false)
	if lipgloss.Width(esq) > m.largura {
		esq = desenhar(grupos, true)
	}
	for lipgloss.Width(esq) > m.largura && len(grupos) > 1 {
		menor := 0
		for i, gr := range grupos {
			if gr.descartarAntes < grupos[menor].descartarAntes {
				menor = i
			}
		}
		grupos = append(grupos[:menor:menor], grupos[menor+1:]...)
		esq = desenhar(grupos, true)
	}
	if m.busca != "" {
		dir := s.TintaSuave.Render("busca: " + m.busca + " " + g.Ponto + " " + strconv.Itoa(len(m.visiveis())) + " de " + strconv.Itoa(len(m.clusters)))
		if lipgloss.Width(esq)+3+lipgloss.Width(dir) <= m.largura {
			esq += strings.Repeat(" ", m.largura-lipgloss.Width(esq)-lipgloss.Width(dir)) + dir
		}
	}
	if esq == "" {
		return " "
	}
	return esq
}

// corpo é a tabela (ou o estado vazio, carregando ou erro) com o detalhe abaixo quando o
// layout pede.
func (m painelCloud) corpo(lay layoutPainel) string {
	if !m.temTabela() {
		return "\n" + m.semTabela()
	}
	v := m.indicesVisiveis()
	largura := m.larguraDaTabela()
	// A tabela de todos os clusters mede as colunas: a largura não muda ao rolar nem com a
	// busca. A janela mostra o cabeçalho e as linhas visíveis a partir da rolagem.
	colunas, linhas := m.tabela(m.clusters, largura)
	selecionada := -1
	if m.cursor >= 0 && m.cursor < len(v) {
		selecionada = v[m.cursor]
	}
	tabela := strings.Split(ui.Tabela(colunas, linhas, selecionada, largura), "\n")
	ini := m.rolagem
	fim := min(len(v), ini+m.linhasDeDados())
	saida := []string{tabela[0]}
	for _, i := range v[ini:fim] {
		saida = append(saida, tabela[1+i])
	}
	if ini > 0 || fim < len(v) {
		var avisos []string
		if ini > 0 {
			avisos = append(avisos, ui.AvisoRolagem(ui.SetaAcima, plural(ini, "cluster acima", "clusters acima")))
		}
		if fim < len(v) {
			avisos = append(avisos, ui.AvisoRolagem(ui.SetaAbaixo, plural(len(v)-fim, "cluster abaixo", "clusters abaixo")))
		}
		saida = append(saida, strings.Join(avisos, "   "))
	}
	corpo := strings.Join(saida, "\n")
	if lay == layoutAbaixo {
		corpo += "\n\n" + m.detalhe(m.largura, 0)
	}
	if lay == layoutPorCima {
		return corpo // a moldura já põe o fio entre o detalhe e a lista
	}
	return "\n" + corpo
}

// Índices das colunas da tabela.
const (
	colunaRegiao = 2
	colunaK8S    = 3
	colunaToken  = 5
)

// versaoQueCabe é a versão do Kubernetes em até w células: inteira quando cabe; senão sem
// o sufixo do provider (1.30.5-gke.1014001 vira 1.30.5) e, se ainda não couber, só
// major.minor (1.30). Nunca "1.30.…", que não diz nada.
func versaoQueCabe(versao string, w int) string {
	if lipgloss.Width(versao) <= w {
		return versao
	}
	curta := versao
	if i := strings.IndexAny(versao, "-+"); i > 0 {
		curta = versao[:i]
	}
	if lipgloss.Width(curta) <= w {
		return curta
	}
	if partes := strings.SplitN(curta, ".", 3); len(partes) >= 2 {
		return partes[0] + "." + partes[1]
	}
	return curta
}

// tabela monta as colunas e as linhas da tabela na largura dada. REGIÃO e K8S crescem até
// o conteúdo quando há espaço; sem espaço, a REGIÃO corta e a versão encurta
// (versaoQueCabe). A coluna TOKEN nunca corta o tempo, que é o dado principal: a barra
// ideal encolhe para o que sobra na coluna e, com menos de 4 células, sai (fica só o
// tempo). As linhas da leitura anterior (provider sem resposta) saem apagadas.
func (m painelCloud) tabela(v []clusterPainel, largura int) ([]ui.Coluna, []ui.Linha) {
	s := ui.S()
	agora := m.agora()
	textoMaior, textoComBarra, nomeMaior, statusMaior, regiaoMaior := 0, 0, 0, 0, lipgloss.Width("REGIÃO")
	versoes := make([]string, len(v))
	for i, c := range v {
		versoes[i] = c.info.Version
		nomeMaior = max(nomeMaior, lipgloss.Width(c.info.Name))
		statusMaior = max(statusMaior, lipgloss.Width(c.celulaStatus(agora)))
		regiaoMaior = max(regiaoMaior, lipgloss.Width(c.info.Region))
		w := lipgloss.Width(c.celulaToken(agora, 0))
		textoMaior = max(textoMaior, w)
		if val := validadeEm(c.expiraEm, agora); c.autenticado && !c.desatualizado && val.conhecida && !val.expirado {
			textoComBarra = max(textoComBarra, w)
		}
	}
	colunas := []ui.Coluna{
		{Titulo: "CLOUD", Min: 5, Max: 6, Prioridade: 3, Nome: true},
		// O nome do cluster não é cortado enquanto couber (até 24): somem antes REGIÃO e K8S.
		{Titulo: "CLUSTER", Min: max(8, min(24, nomeMaior)), Max: 24, Prioridade: 0, Nome: true},
		{Titulo: "REGIÃO", Min: 6, Prioridade: 4},
		{Titulo: "K8S", Min: 4, Prioridade: 5},
		// O STATUS não corta ("✕ sem re…" muda o sentido): sai antes a REGIÃO.
		{Titulo: "STATUS", Min: max(9, min(16, statusMaior)), Max: 16, Prioridade: 1},
		{Titulo: "TOKEN", Min: max(8, textoMaior), Prioridade: 2},
	}
	montar := func(barra int) []ui.Linha {
		linhas := make([]ui.Linha, len(v))
		for i, c := range v {
			cel := []string{c.info.Provider, c.info.Name, c.info.Region, versoes[i]}
			if c.desatualizado {
				for j := range cel {
					cel[j] = s.TintaSuave.Render(cel[j])
				}
			}
			linhas[i] = ui.Linha{Celulas: append(cel, c.celulaStatus(agora), c.celulaToken(agora, barra))}
		}
		return linhas
	}
	barra := min(16, max(6, largura/8))
	linhas := montar(barra)
	// encurtar põe as versões na forma que f dá; a encurtada não pode ser cortada de novo
	// pela tabela, por isso o mínimo da K8S passa a ser ela.
	encurtar := func(f func(string) string, wk int) {
		maior, encurtou := 0, false
		for i, c := range v {
			if versoes[i] = f(c.info.Version); versoes[i] != c.info.Version {
				encurtou = true
			}
			maior = max(maior, lipgloss.Width(versoes[i]))
		}
		if encurtou {
			colunas[colunaK8S].Min = max(4, min(maior, wk))
			linhas = montar(barra)
		}
	}
	if wk := ui.LargurasDasColunas(colunas, linhas, largura)[colunaK8S]; wk > 0 {
		encurtar(func(ver string) string { return versaoQueCabe(ver, wk) }, wk)
		// Com a REGIÃO ainda cortada, a versão vai a major.minor (1.30, como na prancha):
		// o patch só fica quando a região cabe inteira.
		ws := ui.LargurasDasColunas(colunas, linhas, largura)
		if ws[colunaRegiao] > 0 && ws[colunaRegiao] < regiaoMaior {
			encurtar(func(ver string) string { return versaoQueCabe(ver, 0) }, ws[colunaK8S])
		}
	}
	w := ui.LargurasDasColunas(colunas, linhas, largura)[colunaToken]
	if textoComBarra == 0 || w == 0 || barra+2+textoComBarra <= w {
		return colunas, linhas
	}
	linhas = montar(w - 2 - textoComBarra)
	// O mínimo da coluna passa a ser o conteúdo novo, para a tabela não a encolher de novo.
	maior := 0
	for _, l := range linhas {
		maior = max(maior, lipgloss.Width(l.Celulas[colunaToken]))
	}
	colunas[colunaToken].Min = min(maior, w)
	return colunas, linhas
}

// semTabela explica o corpo sem linhas: lendo, sem cloud, sem cluster, busca vazia ou erro.
func (m painelCloud) semTabela() string {
	s := ui.S()
	switch {
	case !m.carregado:
		return ui.NovoEstadoCarregandoCom("lendo os clusters das clouds", func() bool { return false }).View()
	case m.err != nil && len(m.clusters) == 0:
		linhas := []string{ui.NovoEstadoErro("não deu para ler as clouds", "r tenta de novo").View(), ""}
		for _, l := range strings.Split(ansi.Wrap(resumoDoErro(m.err), max(20, m.largura), ""), "\n") {
			linhas = append(linhas, s.TintaSuave.Render(l))
		}
		return strings.Join(linhas, "\n")
	case m.provedores == 0:
		return ui.NovoEstadoVazio("nenhuma cloud no kubeconfig", "yby cloud connect liga um cluster EKS, AKS ou GKE").View()
	case len(m.clusters) == 0:
		return ui.NovoEstadoVazio("nenhum cluster nas contas conectadas", "r recarrega").View()
	}
	return ui.NovoEstadoVazio("nenhum cluster com esse nome", "esc limpa a busca").View()
}

// tipoDoCluster é o nome do serviço gerenciado do provider.
func tipoDoCluster(provider string) string {
	switch provider {
	case "aws":
		return "EKS"
	case "azure":
		return "AKS"
	case "gcp":
		return "GKE"
	}
	return strings.ToUpper(provider)
}

// opcaoDetalhe é uma versão do detalhe, da mais completa à mais curta.
type opcaoDetalhe struct {
	eventos       int  // registros da auditoria (0: sem a seção)
	rodape        bool // "a auditoria completa ▸ yby cloud audit"
	ambiente      bool // a linha do .yby/environments
	tokenPrimeiro bool // o token logo abaixo de IDENTIDADE, antes de conta e papel
	semStatus     bool // sem a linha status quando ela só repete a coluna STATUS (cluster ativo)
	semBranco     bool // AUDITORIA RECENTE logo abaixo da IDENTIDADE, sem a linha em branco (coluna única)
}

// opcoesDetalhe é a ordem em que o detalhe encolhe quando a altura não comporta: sai o
// rodapé, depois os registros da auditoria (3, 2, 1), a linha status que só repete a
// tabela, a linha do ambiente, a linha em branco entre as seções e, por fim, a auditoria
// inteira, com o token subindo para o alto (e o ambiente, enquanto couber). Se nem assim couber, o quadro corta as linhas de
// baixo e fecha a borda (com … no lugar da última).
var opcoesDetalhe = []opcaoDetalhe{
	{eventos: auditoriaNoDetalhe, rodape: true, ambiente: true},
	{eventos: auditoriaNoDetalhe, ambiente: true},
	{eventos: 2, ambiente: true},
	{eventos: 1, ambiente: true},
	{eventos: 1, ambiente: true, semStatus: true},
	{eventos: 1, semStatus: true},
	{eventos: 1, semStatus: true, semBranco: true},
	{tokenPrimeiro: true, ambiente: true, semStatus: true},
	{tokenPrimeiro: true, semStatus: true},
}

// alturaDetalheMaior é a altura do detalhe completo mais alto entre os clusters visíveis.
func (m painelCloud) alturaDetalheMaior(largura int) int {
	h := 0
	for _, c := range m.visiveis() {
		h = max(h, len(m.corpoDoDetalhe(c, largura, opcoesDetalhe[0]))+2)
	}
	return h
}

// detalhe é o quadro do cluster selecionado, com no máximo limite linhas (0: sem limite).
func (m painelCloud) detalhe(largura, limite int) string {
	c, ok := m.selecionado()
	if !ok {
		return ""
	}
	return m.detalheDe(c, largura, limite)
}

// detalhePorCima é o detalhe aberto pelo ↵ abaixo de 120 colunas: fica com o que precisa,
// deixando para a tabela, embaixo do fio, ao menos o cabeçalho e a linha selecionada (e o
// aviso de rolagem). Num corpo de menos de 6 linhas, só o detalhe.
func (m painelCloud) detalhePorCima() string {
	h := m.alturaCorpo() - m.brancoPorCima()
	if m.alturaCorpo() < 6 {
		return m.detalhe(m.largura, h)
	}
	tabela := 2
	if len(m.visiveis()) > 1 {
		tabela = 3
	}
	return m.detalhe(m.largura, h-1-tabela)
}

// detalheAoLado é o detalhe à direita da tabela (a partir de 120 colunas, com pouca altura):
// uma linha abaixo do resumo, como a tabela e o detalhe por cima, quando a linha em branco
// não custa conteúdo ao quadro (a linha status que só repete a tabela pode sair); se
// custar, o quadro sobe e fica com ela.
func (m painelCloud) detalheAoLado() string {
	c, ok := m.selecionado()
	if !ok {
		return ""
	}
	_, w := ui.LargurasLadoALado(m.largura)
	h := m.alturaCorpo()
	conteudo := func(i int) opcaoDetalhe {
		op := opcoesDetalhe[i]
		op.semStatus = false
		return op
	}
	if h >= 4 && conteudo(m.opcaoQueCabe(c, w, h-1)) == conteudo(m.opcaoQueCabe(c, w, h)) {
		return "\n" + m.detalheDe(c, w, h-1)
	}
	return m.detalheDe(c, w, h)
}

// brancoPorCima é a linha em branco entre o resumo e o detalhe por cima (como entre o resumo
// e a tabela); num corpo de menos de 4 linhas, o detalhe fica com ela.
func (m painelCloud) brancoPorCima() int {
	if m.alturaCorpo() < 4 {
		return 0
	}
	return 1
}

// detalheDe é o quadro do cluster: IDENTIDADE e AUDITORIA RECENTE, lado a lado quando
// cabem, uma embaixo da outra quando não; encolhido (opcoesDetalhe) até caber no limite.
func (m painelCloud) detalheDe(c clusterPainel, largura, limite int) string {
	corpo := m.corpoDoDetalhe(c, largura, opcoesDetalhe[m.opcaoQueCabe(c, largura, limite)])
	if n := limite - 2; limite > 0 && len(corpo) > n {
		if n >= 2 {
			corpo = append(corpo[:n-1], ui.S().TintaSuave.Render(ui.G().Reticencias))
		} else {
			corpo = corpo[:max(1, n)]
		}
	}
	return ui.QuadroComNome(tipoDoCluster(c.info.Provider), c.info.Name, strings.Join(corpo, "\n"), largura, ui.QuadroFoco)
}

// opcaoQueCabe é o índice, em opcoesDetalhe, da primeira versão do detalhe que cabe em
// limite linhas com as bordas (0: sem limite); nenhuma cabendo, a última.
func (m painelCloud) opcaoQueCabe(c clusterPainel, largura, limite int) int {
	for i, op := range opcoesDetalhe {
		if limite <= 0 || len(m.corpoDoDetalhe(c, largura, op))+2 <= limite {
			return i
		}
	}
	return len(opcoesDetalhe) - 1
}

// corpoDoDetalhe são as linhas de dentro do quadro na versão op.
func (m painelCloud) corpoDoDetalhe(c clusterPainel, largura int, op opcaoDetalhe) []string {
	interno := largura - 4
	duas := interno >= internoDuasColunas
	wCol := (interno - 3) / 2
	wIdent := interno
	if duas && op.eventos > 0 {
		wIdent = wCol
	}
	ident := m.linhasIdentidade(c, op, wIdent)
	if op.eventos == 0 {
		return ident
	}
	audit := m.linhasAuditoria(c, duas, op)
	if !duas {
		if !op.semBranco {
			ident = append(ident, "")
		}
		return append(ident, audit...)
	}
	var corpo []string
	for i := 0; i < max(len(ident), len(audit)); i++ {
		esq, dir := "", ""
		if i < len(ident) {
			esq = ansi.Truncate(ident[i], wCol, ui.G().Reticencias)
		}
		if i < len(audit) {
			dir = audit[i]
		}
		corpo = append(corpo, esq+strings.Repeat(" ", wCol-lipgloss.Width(esq)+3)+dir)
	}
	return corpo
}

// linhaChave é uma linha "chave  valor" do detalhe.
func linhaChave(chave, valor string) string {
	return ui.S().TintaSuave.Render(chave+strings.Repeat(" ", max(1, larguraChaveDetalhe-lipgloss.Width(chave)))) + valor
}

// ouTraco devolve v, ou o glifo de vazio ("—") em tinta suave.
func ouTraco(v string) string {
	if v == "" {
		return ui.S().TintaSuave.Render(ui.G().Vazio)
	}
	return v
}

// mascararConta esconde o meio de um número de conta ou de uma assinatura
// (4821·…·9310); nomes legíveis, como um projeto GCP ou um e-mail, ficam como estão.
func mascararConta(conta string) string {
	if len(conta) < 10 || strings.ContainsAny(conta, "@") {
		return conta
	}
	for _, r := range conta {
		if !strings.ContainsRune("0123456789abcdefABCDEF-", r) {
			return conta
		}
	}
	if ui.Atual().ASCII {
		return conta[:4] + "..." + conta[len(conta)-4:]
	}
	return conta[:4] + "·" + ui.G().Reticencias + "·" + conta[len(conta)-4:]
}

// linhasIdentidade são as linhas de IDENTIDADE em largura células (a linha do token passa
// à forma curta quando a inteira cortaria).
func (m painelCloud) linhasIdentidade(c clusterPainel, op opcaoDetalhe, largura int) []string {
	s := ui.S()
	agora := m.agora()
	token := linhaChave("token", c.tokenQueCabe(agora, largura))
	estado := []string{linhaChave("status", c.statusNoDetalhe(agora)), token}
	if op.semStatus && !c.desatualizado && clusterAtivo(c.info.Status) {
		estado = []string{token}
	}
	linhas := []string{s.Rotulo.Render("IDENTIDADE")}
	if op.tokenPrimeiro {
		// no aperto o token vem antes de tudo: é o dado que a pessoa age sobre (t renova)
		linhas = append(linhas, token)
		linhas = append(linhas, estado[:len(estado)-1]...)
	}
	// No Azure a credencial é de um usuário, sem papel: a linha diz "usuário".
	quem := linhaChave("papel", ouTraco(c.papel))
	if c.papel == "" && c.usuario != "" {
		quem = linhaChave("usuário", c.usuario)
	}
	linhas = append(linhas,
		linhaChave("conta", ouTraco(mascararConta(c.conta))),
		quem,
		linhaChave("método", ouTraco(c.metodo)),
	)
	if !op.tokenPrimeiro {
		linhas = append(linhas, estado...)
	}
	if c.ambiente != "" && op.ambiente {
		linhas = append(linhas, linhaChave("ambiente", c.ambiente+" "+s.TintaSuave.Render("(.yby/environments)")))
	}
	return linhas
}

// acoesAuditoria dá nome curto às ações do audit log.
var acoesAuditoria = map[string]string{
	"authenticate": "login",
	"refresh":      "renova",
	"assume-role":  "assume",
}

// eventosDoCluster são os n registros mais recentes do provider que dizem respeito ao
// cluster (ou à credencial do provider, sem cluster), do mais novo ao mais antigo.
func (m painelCloud) eventosDoCluster(c clusterPainel, n int) []cloud.CloudAuditEvent {
	var saida []cloud.CloudAuditEvent
	for _, e := range m.auditoria {
		if e.Provider == c.info.Provider && (e.Cluster == "" || e.Cluster == c.info.Name) {
			saida = append(saida, e)
		}
	}
	sort.SliceStable(saida, func(a, b int) bool { return saida[a].Timestamp.After(saida[b].Timestamp) })
	return saida[:min(len(saida), n)]
}

func (m painelCloud) linhasAuditoria(c clusterPainel, recuar bool, op opcaoDetalhe) []string {
	s, g := ui.S(), ui.G()
	agora := m.agora()
	linhas := []string{s.Rotulo.Render("AUDITORIA RECENTE")}
	eventos := m.eventosDoCluster(c, op.eventos)
	if len(eventos) == 0 {
		linhas = append(linhas, s.TintaSuave.Render("nenhum registro nos últimos 30 dias"))
	}
	for _, e := range eventos {
		quando := e.Timestamp.In(agora.Location())
		hora := quando.Format("15:04")
		if quando.Format("2006-01-02") != agora.Format("2006-01-02") {
			hora = quando.Format("02/01")
		}
		acao := acoesAuditoria[e.Action]
		if acao == "" {
			acao = e.Action
		}
		glifo, estilo := g.Feito, s.Ok
		if !e.Success {
			glifo, estilo = g.Falha, s.Erro
		}
		marca := estilo.Render(glifo + " " + acao)
		linhas = append(linhas, s.TintaSuave.Render(hora)+"  "+marca+strings.Repeat(" ", max(1, larguraAcao-lipgloss.Width(marca)))+descricaoEvento(e))
	}
	if !op.rodape {
		return linhas
	}
	recuo := ""
	if recuar {
		recuo = strings.Repeat(" ", 5+2+larguraAcao) // embaixo da descrição, como na prancha
	}
	return append(linhas,
		recuo+s.TintaSuave.Render("a auditoria completa"),
		recuo+s.TintaSuave.Render(g.Caminho+" yby cloud audit"))
}

// descricaoEvento resume o registro: o método, o papel assumido ou, na falha, a causa em
// poucas palavras (o erro do CLI inteiro fica no yby cloud audit).
func descricaoEvento(e cloud.CloudAuditEvent) string {
	if !e.Success {
		return causaDaFalha(e)
	}
	switch e.Action {
	case "authenticate":
		if e.Method != "" {
			return e.Method
		}
		return e.Identity
	case "refresh":
		if e.Method != "" {
			return "token " + ui.G().Ponto + " " + e.Method
		}
		return "token"
	case "assume-role":
		return e.Role
	}
	return e.Cluster
}

// causaConhecida resume em poucas palavras o erro de um CLI de cloud ("SSO vencido",
// "acesso negado"); vazio quando a causa não é uma das conhecidas.
func causaConhecida(erro string) string {
	erro = strings.ToLower(erro)
	switch {
	case strings.Contains(erro, "sso") && strings.Contains(erro, "expired"):
		return "SSO vencido"
	case strings.Contains(erro, "expired"):
		return "credencial vencida"
	case strings.Contains(erro, "denied") || strings.Contains(erro, "unauthorized") || strings.Contains(erro, "forbidden"):
		return "acesso negado"
	}
	return ""
}

// causaDaFalha é a descrição curta de um registro que falhou.
func causaDaFalha(e cloud.CloudAuditEvent) string {
	if c := causaConhecida(e.Error); c != "" {
		return c
	}
	switch e.Action {
	case "refresh":
		return "token não renovado"
	case "authenticate":
		return "login recusado"
	case "assume-role":
		return "papel não assumido"
	}
	return "falhou"
}

// mensagem é a frase mais urgente agora: o campo, o aviso da última tecla, o resultado da
// última ação (o Toast), a renovação em andamento, a leitura, o erro ou o cluster com o
// problema mais grave.
func (m painelCloud) mensagem() string {
	s, g := ui.S(), ui.G()
	switch {
	case m.campo:
		return s.Negrito.Render("buscar") + " " + s.TintaSuave.Render(g.Prompt) + " " + s.Tinta.Render(m.textoCampo) +
			s.Selecionado.Render(" ") + "   " + s.TintaSuave.Render("filtra por nome do cluster enquanto digita")
	case m.aviso != "":
		return m.aviso
	case m.toast.Visivel():
		return m.toast.View()
	case len(m.renovando) > 0:
		nomes := make([]string, 0, len(m.renovando))
		for _, n := range m.renovando {
			nomes = append(nomes, n)
		}
		sort.Strings(nomes)
		return s.Aviso.Render(g.Andamento) + " " + s.Tinta.Render("renovando o token de "+strings.Join(nomes, ", ")+g.Reticencias)
	case !m.carregado:
		return ""
	case m.err != nil && len(m.clusters) == 0:
		return fraseErro("sem resposta das clouds")
	case m.err != nil:
		return s.Aviso.Render(g.Alerta) + " " + s.Tinta.Render("a última leitura falhou; os dados são de "+m.atualizado.Format("15:04:05"))
	case len(m.falhas) > 0:
		return m.fraseDasFalhas()
	}
	return m.fraseDoMaisUrgente()
}

// providersSemResposta são os nomes, em ordem, dos providers que falharam na leitura.
func (m painelCloud) providersSemResposta() []string {
	nomes := make([]string, 0, len(m.falhas))
	for n := range m.falhas {
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)
	return nomes
}

// fraseDasFalhas: "✕ aws sem resposta: SSO vencido (rode aws sso login) · r tenta de novo".
// Uma causa conhecida sai em poucas palavras, como na auditoria; uma desconhecida vem
// depois da tecla, com a linha do CLI, para o corte da linha de estado levar só o fim
// dela. Um provider que não responde esconde clusters inteiros, por isso vem antes dos
// problemas de cluster.
func (m painelCloud) fraseDasFalhas() string {
	s, g := ui.S(), ui.G()
	nomes := m.providersSemResposta()
	err := m.falhas[nomes[0]]
	sep := s.TintaSuave.Render(" " + g.Ponto + " ")
	r := s.Negrito.Render("r") + s.Tinta.Render(" tenta de novo")
	frase := s.Erro.Render(g.Falha) + " " + s.Tinta.Render(strings.Join(nomes, ", ")+" sem resposta")
	causa := causaConhecida(err.Error())
	if causa == "" {
		return frase + sep + r + sep + s.TintaSuave.Render(resumoDoErro(err))
	}
	frase += s.Tinta.Render(": " + causa)
	if nomes[0] == "aws" && strings.Contains(strings.ToLower(err.Error()), "sso") {
		frase += s.TintaSuave.Render(" (rode aws sso login)")
	}
	if lipgloss.Width(frase+sep+r) > m.largura {
		// sem espaço, a dica fica e o r sai: sem o login, tentar de novo não adianta
		return frase
	}
	return frase + sep + r
}

// falhaCurta diz se a frase da linha de estado é a de um provider sem resposta com causa
// conhecida: curta, ela vale mais que a hora quando as duas não cabem.
func (m painelCloud) falhaCurta(frase string) bool {
	if len(m.falhas) == 0 || frase != m.fraseDasFalhas() {
		return false
	}
	return causaConhecida(m.falhas[m.providersSemResposta()[0]].Error()) != ""
}

// fraseDoMaisUrgente fala do cluster no estado mais grave (o primeiro da lista, no empate).
func (m painelCloud) fraseDoMaisUrgente() string {
	if len(m.clusters) == 0 {
		return ""
	}
	s, g := ui.S(), ui.G()
	agora := m.agora()
	pior, estado := m.clusters[0], m.clusters[0].estadoEm(agora)
	for _, c := range m.clusters[1:] {
		if e := c.estadoEm(agora); e < estado {
			pior, estado = c, e
		}
	}
	t := s.Negrito.Render("t")
	nome := pior.info.Name
	switch estado {
	case estadoSemLogin:
		return s.Erro.Render(g.Falha) + " " + s.Tinta.Render(nome+" sem login: faça o login no CLI do "+pior.info.Provider+" e ") +
			s.Negrito.Render("r") + s.Tinta.Render(" recarrega")
	case estadoExpirado:
		frase := s.Erro.Render(g.Falha) + " " + s.Tinta.Render(nome+" sem token: ") + t + s.Tinta.Render(" renova")
		if pior.sso && pior.info.Provider == "aws" {
			frase += s.TintaSuave.Render(" (com o SSO vencido, rode aws sso login antes)")
		}
		return frase
	case estadoExpiraLogo:
		v := validadeEm(pior.expiraEm, agora)
		return s.Aviso.Render(g.Alerta) + " " + s.Tinta.Render("o token de "+nome+" expira em "+v.texto()+": ") + t + s.Tinta.Render(" renova")
	case estadoNaoAtivo:
		return s.Aviso.Render(g.Alerta) + " " + s.Tinta.Render(nome+" não está ativo ("+statusLegivel(pior.info.Status)+")")
	case estadoSemResposta:
		return s.Erro.Render(g.Falha) + " " + s.Tinta.Render(nome+": o "+pior.info.Provider+" não respondeu; os dados são da leitura anterior")
	}
	return s.Ok.Render(g.Feito) + " " + s.Tinta.Render("todos os clusters prontos")
}

func fraseErro(texto string) string {
	return ui.S().Erro.Render(ui.G().Falha) + " " + ui.S().Tinta.Render(texto)
}

func fraseNeutra(texto string) string {
	return ui.S().TintaSuave.Render(ui.G().Parado) + " " + ui.S().Tinta.Render(texto)
}

// fraseComTeclas é a fraseNeutra com as teclas em negrito, como nas outras frases da linha
// de estado; no texto, a tecla vem entre crases ("`q` sai só da lista; `esc` volta").
func fraseComTeclas(texto string) string {
	s := ui.S()
	var b strings.Builder
	b.WriteString(s.TintaSuave.Render(ui.G().Parado) + " ")
	for i, parte := range strings.Split(texto, "`") {
		switch {
		case parte == "":
		case i%2 == 1:
			b.WriteString(s.Negrito.Render(parte))
		default:
			b.WriteString(s.Tinta.Render(parte))
		}
	}
	return b.String()
}

// teclas é a legenda da tela, só com as teclas que funcionam nela, e a tecla de saída
// do canto direito (vazia é "q sair"; nas camadas, ctrl+c).
func (m painelCloud) teclas(lay layoutPainel) ([]key.Binding, key.Binding) {
	g, l := ui.DefaultKeys().Global, ui.DefaultKeys().Lista
	mover := atalhoPainel(setasPainel(), "mover", "up", "down", "j", "k")
	switch {
	case m.campo:
		return []key.Binding{atalhoPainel("↵", "confirmar", "enter"), atalhoPainel("esc", "cancelar", "esc")}, g.SairForcado
	case !m.temTabela() && m.carregado && m.err != nil && len(m.clusters) == 0:
		return []key.Binding{atalhoPainel("r", "tentar de novo", "r"), g.Ajuda}, key.Binding{}
	case !m.temTabela():
		t := []key.Binding{g.Recarregar}
		if m.busca != "" {
			t = append(t, atalhoPainel("esc", "limpar busca", "esc"))
		}
		return append(t, g.Ajuda), key.Binding{}
	case lay == layoutPorCima:
		voltar := atalhoPainel("esc", "voltar", "esc")
		if m.largura < ui.LarguraColuna {
			// no compacto cabem três: as ações do cluster valem mais que mover
			return []key.Binding{voltar, teclaViz(), teclaRenovar()}, g.SairForcado
		}
		// o esc vem logo depois do mover: é a única saída da camada e tem de sobreviver ao
		// corte da legenda em 80 colunas, também no ASCII ("up/down", "enter")
		return []key.Binding{mover, voltar, teclaViz(), teclaRenovar(), g.Recarregar, g.Ajuda}, g.SairForcado
	}
	t := []key.Binding{mover}
	if lay == layoutSemDetalhe {
		t = append(t, atalhoPainel("↵", "detalhe", "enter"))
	} else {
		t = append(t, teclaViz())
	}
	t = append(t, teclaRenovar(), g.Recarregar, l.Buscar)
	if m.busca != "" {
		t = append(t, atalhoPainel("esc", "limpar busca", "esc"))
	}
	return append(t, g.Ajuda), key.Binding{}
}

// grupoAjuda é um bloco da tela ?: o título e as linhas (tecla, o que faz).
type grupoAjuda struct {
	titulo string
	linhas [][2]string
}

// gruposDaAjuda são as teclas da tela, agrupadas como na folha Atalhos: as de toda TUI e as
// da lista de clusters.
func (m painelCloud) gruposDaAjuda() []grupoAjuda {
	todas := grupoAjuda{"em toda TUI", [][2]string{
		{"?", "ajuda com todas as teclas da tela"},
		{"esc", "fecha o detalhe ou limpa a busca"},
		{"q", "sai (só no nível de cima)"},
		{"ctrl+c", "sai de qualquer lugar"},
		{"r", "recarrega os clusters"},
		{"mouse", "a roda move, o clique seleciona"},
	}}
	lista := grupoAjuda{"lista " + ui.G().Ponto + " clusters", [][2]string{
		{setasPainel() + " j k", "mover"},
		{"pgup pgdn", "página"},
		{"g G", "início e fim"},
		{"/", "buscar por nome do cluster"},
		{"t", "renovar o token do cluster"},
	}}
	if ui.ModoPara(m.largura, m.altura) == ui.ModoLadoALado {
		lista.linhas = append(lista.linhas, [2]string{ui.RotuloTecla("↵"), "abrir o cluster no Viz"})
	} else {
		lista.linhas = append(lista.linhas, [2]string{ui.RotuloTecla("↵"), "abrir o detalhe; nele, abre no Viz"})
	}
	return []grupoAjuda{todas, lista}
}

// linhasDaAjuda é o conteúdo da tela ?: os dois grupos lado a lado quando cabem, um
// embaixo do outro quando não (como no Viz).
func (m painelCloud) linhasDaAjuda() []string {
	s := ui.S()
	grupos := m.gruposDaAjuda()
	kw := 0
	for _, gr := range grupos {
		for _, l := range gr.linhas {
			kw = max(kw, lipgloss.Width(l[0]))
		}
	}
	desenhar := func(gr grupoAjuda) []string {
		linhas := []string{s.Rotulo.Render(strings.ToUpper(gr.titulo))}
		for _, l := range gr.linhas {
			linhas = append(linhas, s.Negrito.Render(l[0])+strings.Repeat(" ", kw-lipgloss.Width(l[0])+2)+s.TintaSuave.Render(l[1]))
		}
		return linhas
	}
	larguraDe := func(linhas []string) int {
		w := 0
		for _, l := range linhas {
			w = max(w, lipgloss.Width(l))
		}
		return w
	}
	a, b := desenhar(grupos[0]), desenhar(grupos[1])
	wa := larguraDe(a)
	const entre = 4
	if wa+entre+larguraDe(b)+4 > m.largura {
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

// linhasVisiveisDaAjuda é quanto da ajuda cabe no corpo: menos a linha em branco de cima e
// as bordas do quadro.
func (m painelCloud) linhasVisiveisDaAjuda() int {
	return max(1, m.alturaCorpo()-3)
}

// maxRolagemAjuda é a maior rolagem da ajuda (zero quando ela cabe inteira).
func (m painelCloud) maxRolagemAjuda() int {
	return max(0, len(m.linhasDaAjuda())-m.linhasVisiveisDaAjuda())
}

// viewAjuda mostra a tela ? no lugar do corpo, num quadro que rola quando não cabe.
func (m painelCloud) viewAjuda(p ui.Partes) string {
	linhas := m.linhasDaAjuda()
	ini := min(m.rolagemAjuda, m.maxRolagemAjuda())
	fim := min(len(linhas), ini+m.linhasVisiveisDaAjuda())
	w := 0
	for _, l := range linhas {
		w = max(w, lipgloss.Width(l))
	}
	p.Lista = "\n" + ui.Quadro("teclas "+ui.G().Ponto+" cloud", strings.Join(linhas[ini:fim], "\n"), min(m.largura, w+4), ui.QuadroFoco)
	p.Mensagem = m.aviso
	p.Teclas = []key.Binding{atalhoPainel("esc", "fechar", "esc")}
	if m.maxRolagemAjuda() > 0 {
		p.Teclas = append(p.Teclas, atalhoPainel(setasPainel(), "rolar", "up", "down"))
	}
	p.Sair = ui.DefaultKeys().Global.SairForcado
	p.SemAjuda = true // aqui o ? fecha a ajuda
	return ui.Moldura(m.largura, m.altura, p)
}

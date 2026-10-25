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
		linhas = h - len(strings.Split(m.detalhePorCima(), "\n")) - 1 - 1 // detalhe, fio e cabeçalho
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
		_, w := ui.LargurasLadoALado(m.largura)
		p.Detalhe = m.detalhe(w, m.alturaCorpo())
	case layoutPorCima:
		// O detalhe por cima é montado aqui (e não na moldura, que o limita a meia altura):
		// ele fica com o que precisa e a tabela com o resto.
		p.Lista = m.detalhePorCima()
		if m.alturaCorpo() >= 6 {
			p.Lista += "\n" + ui.Rule(m.largura) + "\n" + m.corpo(lay)
		}
	}
	p.Mensagem = m.mensagem()
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
			txt += " · " + plural(n, "conta", "contas")
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

// linhaResumo é a linha antes da tabela: "✓ 2 prontos   ▲ 1 token expira em breve   ✕ 1
// token expirado", só com os grupos que têm contagem; com busca, "busca: x · N de M" à
// direita. Sem nada para contar, a linha fica em branco (a tabela não muda de lugar).
func (m painelCloud) linhaResumo() string {
	s, g := ui.S(), ui.G()
	agora := m.agora()
	conta := map[estadoCluster]int{}
	for _, c := range m.clusters {
		conta[c.estadoEm(agora)]++
	}
	grupos := []struct {
		estado     estadoCluster
		glifo      string
		estilo     lipgloss.Style
		um, varios string
	}{
		{estadoPronto, g.Feito, s.Ok, "pronto", "prontos"},
		{estadoExpiraLogo, g.Alerta, s.Aviso, "token expira em breve", "tokens expiram em breve"},
		{estadoExpirado, g.Falha, s.Erro, "token expirado", "tokens expirados"},
		{estadoSemLogin, g.Falha, s.Erro, "sem login", "sem login"},
		{estadoNaoAtivo, g.Alerta, s.Aviso, "cluster não ativo", "clusters não ativos"},
	}
	var partes []string
	if len(m.falhas) > 0 {
		// Os clusters desses providers (da leitura anterior) não entram nos outros grupos.
		partes = append(partes, s.Erro.Render(g.Falha)+" "+s.Tinta.Render(strings.Join(m.providersSemResposta(), ", ")+" sem resposta"))
	}
	for _, gr := range grupos {
		if n := conta[gr.estado]; n > 0 {
			partes = append(partes, gr.estilo.Render(gr.glifo)+" "+s.Tinta.Render(plural(n, gr.um, gr.varios)))
		}
	}
	esq := strings.Join(partes, "   ")
	if m.busca != "" {
		dir := s.TintaSuave.Render("busca: " + m.busca + " · " + strconv.Itoa(len(m.visiveis())) + " de " + strconv.Itoa(len(m.clusters)))
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
	v := m.visiveis()
	largura := m.larguraDaTabela()
	colunas, linhas := m.tabela(v, largura)
	// A tabela inteira mede as colunas (a largura não muda ao rolar); a janela mostra o
	// cabeçalho e as linhas a partir da rolagem.
	tabela := strings.Split(ui.Tabela(colunas, linhas, m.cursor, largura), "\n")
	ini := m.rolagem
	fim := min(len(v), ini+m.linhasDeDados())
	saida := append([]string{tabela[0]}, tabela[1+ini:1+fim]...)
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

// colunaToken é o índice da coluna TOKEN.
const colunaToken = 5

// tabela monta as colunas e as linhas da tabela na largura dada. A coluna TOKEN nunca corta
// o tempo, que é o dado principal: a barra ideal encolhe para o que sobra na coluna e, com
// menos de 4 células, sai (fica só o tempo).
func (m painelCloud) tabela(v []clusterPainel, largura int) ([]ui.Coluna, []ui.Linha) {
	agora := m.agora()
	textoMaior, textoComBarra, nomeMaior := 0, 0, 0
	for _, c := range v {
		nomeMaior = max(nomeMaior, lipgloss.Width(c.info.Name))
		w := lipgloss.Width(c.celulaToken(agora, 0))
		textoMaior = max(textoMaior, w)
		if val := validadeEm(c.expiraEm, agora); c.autenticado && val.conhecida && !val.expirado {
			textoComBarra = max(textoComBarra, w)
		}
	}
	colunas := []ui.Coluna{
		{Titulo: "CLOUD", Min: 5, Max: 6, Prioridade: 3, Nome: true},
		// O nome do cluster não é cortado enquanto couber (até 24): somem antes REGIÃO e K8S.
		{Titulo: "CLUSTER", Min: max(8, min(24, nomeMaior)), Max: 24, Prioridade: 0, Nome: true},
		{Titulo: "REGIÃO", Min: 6, Max: 16, Prioridade: 4},
		{Titulo: "K8S", Min: 4, Max: 6, Prioridade: 5},
		{Titulo: "STATUS", Min: 9, Max: 16, Prioridade: 1},
		{Titulo: "TOKEN", Min: max(8, textoMaior), Prioridade: 2},
	}
	montar := func(barra int) []ui.Linha {
		linhas := make([]ui.Linha, len(v))
		for i, c := range v {
			linhas[i] = ui.Linha{Celulas: []string{
				c.info.Provider, c.info.Name, c.info.Region, c.info.Version,
				c.celulaStatus(agora), c.celulaToken(agora, barra),
			}}
		}
		return linhas
	}
	barra := min(16, max(6, largura/8))
	linhas := montar(barra)
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
}

// opcoesDetalhe é a ordem em que o detalhe encolhe quando a altura não comporta: sai o
// rodapé, depois os registros da auditoria (3, 2, 1), a linha do ambiente e, por fim, a
// auditoria inteira, com o token subindo para o alto. Se nem assim couber, o quadro corta
// as linhas de baixo e fecha a borda (com … no lugar da última).
var opcoesDetalhe = []opcaoDetalhe{
	{eventos: auditoriaNoDetalhe, rodape: true, ambiente: true},
	{eventos: auditoriaNoDetalhe, ambiente: true},
	{eventos: 2, ambiente: true},
	{eventos: 1, ambiente: true},
	{eventos: 1},
	{tokenPrimeiro: true},
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
	h := m.alturaCorpo()
	if h < 6 {
		return m.detalhe(m.largura, h)
	}
	tabela := 2
	if len(m.visiveis()) > 1 {
		tabela = 3
	}
	return m.detalhe(m.largura, h-1-tabela)
}

// detalheDe é o quadro do cluster: IDENTIDADE e AUDITORIA RECENTE, lado a lado quando
// cabem, uma embaixo da outra quando não; encolhido (opcoesDetalhe) até caber no limite.
func (m painelCloud) detalheDe(c clusterPainel, largura, limite int) string {
	var corpo []string
	for _, op := range opcoesDetalhe {
		corpo = m.corpoDoDetalhe(c, largura, op)
		if limite <= 0 || len(corpo)+2 <= limite {
			break
		}
	}
	if n := limite - 2; limite > 0 && len(corpo) > n {
		if n >= 2 {
			corpo = append(corpo[:n-1], ui.S().TintaSuave.Render(ui.G().Reticencias))
		} else {
			corpo = corpo[:max(1, n)]
		}
	}
	return ui.QuadroNomeado(tipoDoCluster(c.info.Provider), c.info.Name, strings.Join(corpo, "\n"), largura, ui.QuadroFoco)
}

// corpoDoDetalhe são as linhas de dentro do quadro na versão op.
func (m painelCloud) corpoDoDetalhe(c clusterPainel, largura int, op opcaoDetalhe) []string {
	interno := largura - 4
	duas := interno >= internoDuasColunas
	ident := m.linhasIdentidade(c, op)
	if op.eventos == 0 {
		return ident
	}
	audit := m.linhasAuditoria(c, duas, op)
	if !duas {
		return append(append(ident, ""), audit...)
	}
	wCol := (interno - 3) / 2
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
	return conta[:4] + "·" + ui.G().Reticencias + "·" + conta[len(conta)-4:]
}

func (m painelCloud) linhasIdentidade(c clusterPainel, op opcaoDetalhe) []string {
	s := ui.S()
	token := linhaChave("token", c.tokenNoDetalhe(m.agora()))
	linhas := []string{s.Rotulo.Render("IDENTIDADE")}
	if op.tokenPrimeiro {
		linhas = append(linhas, token)
	}
	linhas = append(linhas,
		linhaChave("conta", ouTraco(mascararConta(c.conta))),
		linhaChave("papel", ouTraco(c.papel)),
		linhaChave("método", ouTraco(c.metodo)),
	)
	if !op.tokenPrimeiro {
		linhas = append(linhas, token)
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
			return "token · " + e.Method
		}
		return "token"
	case "assume-role":
		return e.Role
	}
	return e.Cluster
}

// causaDaFalha é a descrição curta de um registro que falhou.
func causaDaFalha(e cloud.CloudAuditEvent) string {
	erro := strings.ToLower(e.Error)
	switch {
	case strings.Contains(erro, "sso") && strings.Contains(erro, "expired"):
		return "SSO vencido"
	case strings.Contains(erro, "expired"):
		return "credencial vencida"
	case strings.Contains(erro, "denied") || strings.Contains(erro, "unauthorized") || strings.Contains(erro, "forbidden"):
		return "acesso negado"
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

// fraseDasFalhas: "✕ aws sem resposta: Token has expired… · r tenta de novo". Um provider
// que não responde esconde clusters inteiros, por isso vem antes dos problemas de cluster.
func (m painelCloud) fraseDasFalhas() string {
	s, g := ui.S(), ui.G()
	nomes := m.providersSemResposta()
	err := m.falhas[nomes[0]]
	frase := s.Erro.Render(g.Falha) + " " + s.Tinta.Render(strings.Join(nomes, ", ")+" sem resposta: ") +
		s.TintaSuave.Render(resumoDoErro(err))
	if nomes[0] == "aws" && strings.Contains(strings.ToLower(err.Error()), "sso") {
		frase += s.TintaSuave.Render(" (rode aws sso login)")
	}
	return frase + s.TintaSuave.Render(" · ") + s.Negrito.Render("r") + s.Tinta.Render(" tenta de novo")
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
			return []key.Binding{voltar, mover, teclaRenovar()}, g.SairForcado
		}
		return []key.Binding{mover, teclaRenovar(), g.Recarregar, voltar, g.Ajuda}, g.SairForcado
	}
	t := []key.Binding{mover}
	if lay == layoutSemDetalhe {
		t = append(t, atalhoPainel("↵", "detalhe", "enter"))
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
	lista := grupoAjuda{"lista · clusters", [][2]string{
		{setasPainel() + " j k", "mover"},
		{"pgup pgdn", "página"},
		{"g G", "início e fim"},
		{"/", "buscar por nome do cluster"},
		{"t", "renovar o token do cluster"},
	}}
	if ui.ModoPara(m.largura, m.altura) != ui.ModoLadoALado {
		lista.linhas = append(lista.linhas, [2]string{ui.RotuloTecla("↵"), "abrir o detalhe"})
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
	p.Lista = "\n" + ui.Quadro("teclas · cloud", strings.Join(linhas[ini:fim], "\n"), min(m.largura, w+4), ui.QuadroFoco)
	p.Mensagem = m.aviso
	p.Teclas = []key.Binding{atalhoPainel("esc", "fechar", "esc")}
	if m.maxRolagemAjuda() > 0 {
		p.Teclas = append(p.Teclas, atalhoPainel(setasPainel(), "rolar", "up", "down"))
	}
	p.Sair = ui.DefaultKeys().Global.SairForcado
	p.SemAjuda = true // aqui o ? fecha a ajuda
	return ui.Moldura(m.largura, m.altura, p)
}

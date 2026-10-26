package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Painel de detalhe do recurso selecionado (folha VizPods). A partir de 120
// colunas ele fica ao lado da lista e acompanha a seleção sempre; abaixo
// disso, ↵ abre o painel por cima da lista e esc fecha. Os campos vêm da
// própria lista; os eventos e a última linha de log são lidos só quando a
// seleção para num recurso (atrasoDetalhe), com cache por recurso, e a
// resposta que chega para um recurso que não é mais o selecionado é descartada.

const (
	// atrasoDetalhe é quanto a seleção precisa ficar parada para o painel ler
	// os eventos e o log: segurar ↓ não vira uma leitura por linha.
	atrasoDetalhe = 300 * time.Millisecond
	// validadeExtras é por quanto tempo os extras lidos valem; depois disso o
	// relógio lê de novo os do recurso na tela.
	validadeExtras = 30 * time.Second
	// eventosNoPainel é quantos eventos o painel mostra (os mais recentes).
	eventosNoPainel = 3
	// linhasParaUltima é quantas linhas do fim do log o painel lê para achar a última com texto.
	linhasParaUltima = 5
	// maxExtras limita o cache: passou disso, ele recomeça.
	maxExtras = 64
	// larguraRotulo é a coluna dos rótulos dos campos ("recursos" e o respiro).
	larguraRotulo = 10
)

// extrasDetalhe são os eventos e a última linha de log de um recurso.
type extrasDetalhe struct {
	eventos    []monitor.Event
	errEventos error
	log        string // última linha do log (só pods)
	errLog     error
	lidoEm     time.Time
}

// paradaMsg chega atrasoDetalhe depois de a seleção mudar; só a do pedido
// mais novo lê os extras.
type paradaMsg struct{ id int }

// extrasMsg traz os extras lidos para o recurso chave pela leitura id.
type extrasMsg struct {
	id     int
	chave  string
	extras extrasDetalhe
}

// leitor é o client que lê eventos, logs e YAML; nil se o client não lê.
func (m Model) leitor() monitor.LeitorDeDetalhe {
	l, _ := m.client.(monitor.LeitorDeDetalhe)
	return l
}

// painelAberto diz se o painel de detalhe está na tela: com a lista na tela e
// uma linha selecionada, ao lado a partir de 120 colunas e, abaixo, depois do ↵.
func (m Model) painelAberto() bool {
	if m.width <= 0 || m.height <= 0 || !m.listaNaTela() || m.itemCount() == 0 {
		return false
	}
	return yui.ModoPara(m.width, m.height) == yui.ModoLadoALado || m.detalheAberto
}

// painelAoLado diz se o painel está ao lado da lista (120 colunas ou mais).
func (m Model) painelAoLado() bool {
	return m.painelAberto() && yui.ModoPara(m.width, m.height) == yui.ModoLadoALado
}

// painelPorCima diz se o painel está por cima da lista (abaixo de 120 colunas).
func (m Model) painelPorCima() bool {
	return m.painelAberto() && yui.ModoPara(m.width, m.height) != yui.ModoLadoALado
}

// larguraDaLista é a largura da tabela: ao lado do painel, a parte da lista.
func (m Model) larguraDaLista() int {
	if m.painelAoLado() {
		l, _ := yui.LargurasLadoALado(m.width)
		return l
	}
	return m.width
}

// alturaDoPainelPorCima é a altura do painel por cima da lista: o limite da
// moldura, sempre o mesmo, para a lista não pular quando os extras chegam.
func (m Model) alturaDoPainelPorCima() int {
	return yui.LimiteDetalheEmpilhado(m.alturaCorpo())
}

// linhasAntesDaTabela são as linhas da lista acima dos dados: a linha em
// branco e o cabeçalho; com o painel por cima, só o cabeçalho (a moldura já
// põe o fio entre os dois).
func (m Model) linhasAntesDaTabela() int {
	if m.painelPorCima() {
		return 1
	}
	return 2
}

// alturaDaLista é a altura da lista no corpo: com o painel por cima, o que
// sobra abaixo dele e do fio.
func (m Model) alturaDaLista() int {
	h := m.alturaCorpo()
	if !m.painelPorCima() {
		return h
	}
	return max(0, h-m.alturaDoPainelPorCima()-1)
}

// itemSelecionado é a linha selecionada da lista da tela.
func (m Model) itemSelecionado() (itemLista, bool) {
	itens := m.itensDaAba(m.activeTab)
	if m.selectedIndex < 0 || m.selectedIndex >= len(itens) {
		return itemLista{}, false
	}
	return itens[m.selectedIndex], true
}

// chaveDetalhe identifica o recurso selecionado entre as abas.
func (m Model) chaveDetalhe() string {
	if c := m.chaveSelecionada(); c != "" {
		return abas[m.activeTab].kind + "|" + c
	}
	return ""
}

// acompanharDetalhe roda depois de cada mensagem: se o recurso do painel
// mudou, agenda a leitura dos extras para quando a seleção parar (um atraso
// por tea.Tick com id; o atraso de um recurso que já saiu não lê nada). Toda
// troca de recurso invalida os atrasos pendentes, mesmo a que vai para um
// recurso com os extras no cache.
func (m *Model) acompanharDetalhe() tea.Cmd {
	alvo := ""
	if m.painelAberto() {
		alvo = m.chaveDetalhe()
	}
	if alvo == m.alvoDetalhe {
		return nil
	}
	m.alvoDetalhe = alvo
	m.pedidoDetalhe++
	if alvo == "" || m.leitor() == nil || m.tique == nil || m.extrasValidos(alvo) {
		return nil
	}
	id := m.pedidoDetalhe
	return m.tique(atrasoDetalhe, func(time.Time) tea.Msg { return paradaMsg{id: id} })
}

// extrasValidos diz se os extras do recurso já foram lidos há pouco.
func (m Model) extrasValidos(chave string) bool {
	x, ok := m.extras[chave]
	return ok && m.agora().Sub(x.lidoEm) < validadeExtras
}

// selecaoParou lê os extras do painel se o atraso é o mais novo e o recurso
// ainda é o da tela.
func (m *Model) selecaoParou(msg paradaMsg) tea.Cmd {
	if msg.id != m.pedidoDetalhe || m.alvoDetalhe == "" || m.alvoDetalhe != m.chaveDetalhe() || m.extrasValidos(m.alvoDetalhe) {
		return nil
	}
	return m.lerExtras(false)
}

// renovarExtras, a cada leitura do relógio, lê de novo os extras do recurso
// na tela quando eles passaram da validade.
func (m *Model) renovarExtras() tea.Cmd {
	if m.alvoDetalhe == "" || m.alvoDetalhe != m.chaveDetalhe() {
		return nil
	}
	if _, lidos := m.extras[m.alvoDetalhe]; !lidos || m.extrasValidos(m.alvoDetalhe) {
		return nil // os que nunca chegaram vêm pelo atraso
	}
	return m.lerExtras(false)
}

// esquecerExtras tira do cache os extras do recurso na tela e lê de novo (o
// r), mesmo com uma leitura em andamento: a resposta dela, de antes do r, é
// descartada.
func (m *Model) esquecerExtras() tea.Cmd {
	if m.alvoDetalhe == "" {
		return nil
	}
	if _, ok := m.extras[m.alvoDetalhe]; ok {
		novo := make(map[string]extrasDetalhe, len(m.extras))
		for k, v := range m.extras {
			if k != m.alvoDetalhe {
				novo[k] = v
			}
		}
		m.extras = novo
	}
	return m.lerExtras(true)
}

// lerExtras lê os eventos (e, nos pods, a última linha do log) do recurso na
// tela; uma leitura por recurso de cada vez, salvo forcar (o r). As leituras
// do monitor têm prazo: a resposta sempre chega, nem que seja com erro.
func (m *Model) lerExtras(forcar bool) tea.Cmd {
	l := m.leitor()
	it, ok := m.itemSelecionado()
	if l == nil || !ok || (!forcar && m.lendoExtras == m.alvoDetalhe) {
		return nil
	}
	chave, kind := m.alvoDetalhe, abas[m.activeTab].kind
	m.leituraExtras++
	m.lendoExtras = chave
	id, obj, conteiner := m.leituraExtras, objetoDosEventos(kind, it), conteinerDoItem(it)
	agora := m.agora
	return func() tea.Msg {
		var x extrasDetalhe
		x.eventos, x.errEventos = l.EventosDoObjeto(obj.kind, obj.nome, obj.namespace, eventosNoPainel)
		if kind == "pod" {
			var logs string
			logs, x.errLog = l.LogsDoPod(it.nome, it.namespace, conteiner, linhasParaUltima)
			x.log = ultimaLinha(logs)
		}
		x.lidoEm = agora()
		return extrasMsg{id: id, chave: chave, extras: x}
	}
}

// conteinerDoItem é o contêiner cujo log o detalhe lê (vazio fora dos pods).
func conteinerDoItem(it itemLista) string {
	if p, ok := it.obj.(monitor.Pod); ok {
		return p.Conteiner
	}
	return ""
}

// guardarExtras guarda os extras que chegaram, se são da leitura mais nova e
// ainda do recurso na tela; os de uma leitura que o r refez ou de um recurso
// que a seleção já deixou são descartados.
func (m *Model) guardarExtras(msg extrasMsg) {
	if msg.id != m.leituraExtras {
		return
	}
	m.lendoExtras = ""
	if msg.chave != m.alvoDetalhe {
		return
	}
	// Mapa novo a cada escrita: o Model é valor e cópias antigas não mudam.
	novo := make(map[string]extrasDetalhe, len(m.extras)+1)
	if len(m.extras) < maxExtras {
		for k, v := range m.extras {
			novo[k] = v
		}
	}
	novo[msg.chave] = msg.extras
	m.extras = novo
}

// objetoEventos é o objeto cujos eventos o detalhe lê.
type objetoEventos struct{ kind, nome, namespace string }

// kindsDaAPI é o kind da API de cada tipo das abas.
var kindsDaAPI = map[string]string{
	"pod": "Pod", "deployment": "Deployment", "statefulset": "StatefulSet", "service": "Service",
	"node": "Node", "job": "Job", "ingress": "Ingress", "configmap": "ConfigMap",
}

// objetoDosEventos: os eventos do próprio recurso; na aba Eventos, os do
// objeto do evento; os de um nó, em qualquer namespace.
func objetoDosEventos(kind string, it itemLista) objetoEventos {
	if e, ok := it.obj.(monitor.Event); ok {
		return objetoEventos{e.Kind, e.Name, e.Namespace}
	}
	ns := it.namespace
	if kind == "node" {
		ns = ""
	}
	return objetoEventos{kindsDaAPI[kind], it.nome, ns}
}

// ultimaLinha é a última linha com texto do log, sem cores nem \r.
func ultimaLinha(logs string) string {
	linhas := strings.Split(strings.ReplaceAll(logs, "\r", ""), "\n")
	for i := len(linhas) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(ansi.Strip(linhas[i])); l != "" {
			return strings.ReplaceAll(l, "\t", "    ")
		}
	}
	return ""
}

// campo é uma linha "rótulo  valor" do painel; o valor já vem com estilo.
type campo struct{ rotulo, valor string }

// tinta é o texto de dado do painel.
func tinta(s string) string { return yui.S().Tinta.Render(umaLinha(s)) }

// comGlifo é o glifo do status na cor dele e o texto em tinta, como na prancha.
func comGlifo(tipo yui.TipoStatus, texto string) string {
	return yui.Status(tipo, "") + tinta(texto)
}

// juntar une os pedaços não vazios com " · ".
func juntar(partes ...string) string {
	var r []string
	for _, p := range partes {
		if p != "" {
			r = append(r, p)
		}
	}
	return strings.Join(r, " · ")
}

// pedidoELimite escreve "120m/500m", só o pedido ou só o limite ("/500m").
func pedidoELimite(nome, pedido, limite string) string {
	switch {
	case pedido == "" && limite == "":
		return ""
	case limite == "":
		return nome + " " + pedido
	}
	return nome + " " + pedido + "/" + limite
}

// camposDoItem são os campos do painel para o tipo do recurso, na ordem da
// prancha; campos sem valor ficam de fora.
func (m Model) camposDoItem(it itemLista) []campo {
	agora := m.agora()
	var cs []campo
	add := func(rotulo, valor string) {
		if valor != "" {
			cs = append(cs, campo{rotulo, tinta(valor)})
		}
	}
	addEstilo := func(rotulo, valor string) { cs = append(cs, campo{rotulo, valor}) }
	switch o := it.obj.(type) {
	case monitor.Pod:
		addEstilo("status", comGlifo(it.tipo, o.Status))
		add("motivo", o.Motivo)
		add("nó", o.No)
		add("imagem", strings.Join(o.Imagens, ", "))
		add("dono", o.Dono)
		add("recursos", juntar(pedidoELimite("cpu", o.Recursos.CPUPedido, o.Recursos.CPULimite),
			pedidoELimite("mem", o.Recursos.MemPedido, o.Recursos.MemLimite)))
	case monitor.Deployment:
		add("réplicas", juntar(strconv.Itoa(int(o.Replicas)), fmt.Sprintf("%d atualizadas", o.Atualizados),
			fmt.Sprintf("%d disponíveis", o.Available)))
		addEstilo("prontos", comGlifo(it.tipo, fmt.Sprintf("%d/%d", o.Ready, o.Replicas)))
		add("estratégia", o.Estrategia)
		add("imagem", strings.Join(o.Imagens, ", "))
		add("seletor", o.Seletor)
	case monitor.StatefulSet:
		add("réplicas", juntar(strconv.Itoa(int(o.Replicas)), fmt.Sprintf("%d atualizadas", o.Atualizados)))
		addEstilo("prontos", comGlifo(it.tipo, fmt.Sprintf("%d/%d", o.Ready, o.Replicas)))
		add("serviço", o.Servico)
		add("imagem", strings.Join(o.Imagens, ", "))
		add("seletor", o.Seletor)
	case monitor.Service:
		add("tipo", o.Type)
		add("IP", o.ClusterIP)
		add("externos", o.Externos)
		add("portas", o.Ports)
		add("seletor", o.Seletor)
	case monitor.Node:
		addEstilo("status", comGlifo(it.tipo, o.Status))
		add("papéis", o.Papeis)
		add("versão", o.Version)
		add("capacidade", juntar(prefixar("cpu ", o.CPUCapacity), prefixar("mem ", o.MemoryCapacity)))
		add("IP", o.IPInterno)
	case monitor.Job:
		addEstilo("status", comGlifo(it.tipo, it.status))
		add("conclusões", conclusoes(o))
		add("ativos", strconv.Itoa(int(o.Active)))
		add("falhas", strconv.Itoa(int(o.Failed)))
	case monitor.Ingress:
		add("classe", o.Class)
		add("hosts", o.Hosts)
		add("caminhos", o.Paths)
		add("endereço", o.Endereco)
	case monitor.ConfigMap:
		add("chaves", juntar(strconv.Itoa(o.Keys), strings.Join(o.Chaves, ", ")))
		add("tamanho", o.DataSize)
	case monitor.Event:
		add("objeto", strings.TrimSpace(o.Kind+" "+o.Name))
		addEstilo("tipo", comGlifo(it.tipo, o.Type))
		add("razão", o.Reason)
		add("mensagem", o.Message)
		if o.Quantas > 1 {
			add("vezes", strconv.Itoa(int(o.Quantas)))
		}
		add("idade", idadeDoEvento(o, agora))
	}
	for i := range cs {
		cs[i].rotulo = yui.S().TintaSuave.Render(cs[i].rotulo + strings.Repeat(" ", max(1, larguraRotulo-lipgloss.Width(cs[i].rotulo))))
	}
	return cs
}

// prefixar põe o prefixo no valor, se houver valor.
func prefixar(prefixo, valor string) string {
	if valor == "" {
		return ""
	}
	return prefixo + valor
}

// idadeDoEvento é a idade da última ocorrência, como o kubectl.
func idadeDoEvento(e monitor.Event, agora time.Time) string {
	if !e.Quando.IsZero() {
		return monitor.Idade(e.Quando, agora)
	}
	return e.Age
}

// linhaDeEvento é "2m   ▲ BackOff reiniciando container": a idade numa
// coluna, o glifo do tipo (▲ Warning, ✓ Normal), a razão e a mensagem.
func linhaDeEvento(e monitor.Event, agora time.Time) string {
	s, g := yui.S(), yui.G()
	glifo := s.Ok.Render(g.Feito)
	if e.Type == "Warning" {
		glifo = s.Aviso.Render(g.Alerta)
	}
	idade := idadeDoEvento(e, agora)
	texto := juntar(textoDoEvento(e), vezesDoEvento(e))
	return s.TintaSuave.Render(idade+strings.Repeat(" ", max(1, 5-lipgloss.Width(idade)))) + glifo + " " + s.Tinta.Render(texto)
}

// textoDoEvento junta a razão e a mensagem numa linha, sem repetir a razão
// quando a mensagem já começa por ela ("Created container: worker").
func textoDoEvento(e monitor.Event) string {
	msg := umaLinha(e.Message)
	if e.Reason != "" && strings.HasPrefix(strings.ToLower(msg), strings.ToLower(e.Reason)) {
		return msg
	}
	return strings.TrimSpace(e.Reason + " " + msg)
}

// vezesDoEvento é "14 vezes" para o evento repetido.
func vezesDoEvento(e monitor.Event) string {
	if e.Quantas > 1 {
		return vezes(e.Quantas)
	}
	return ""
}

// palavrasDeErro marcam a linha de log que conta uma falha (a primeira palavra).
var palavrasDeErro = map[string]bool{"panic": true, "fatal": true, "error": true, "err": true, "erro": true, "exception": true}

// linhaDeLog destaca em vermelho a primeira palavra da linha quando ela diz
// que houve falha ("panic:", "ERROR"), como na prancha.
func linhaDeLog(l string) string {
	s := yui.S()
	palavra, resto, _ := strings.Cut(l, " ")
	chave := strings.ToLower(strings.Trim(palavra, ":[]"))
	if !palavrasDeErro[chave] {
		return s.Tinta.Render(l)
	}
	if resto != "" {
		resto = " " + resto
	}
	return s.Erro.Render(palavra) + s.Tinta.Render(resto)
}

// secao é um bloco do painel: o título e as linhas (ao menos uma).
type secao struct {
	titulo string
	linhas []string
}

// carregandoNoPainel é o estado carregando, num quadro fixo (sem animação).
func carregandoNoPainel(texto string) string {
	return yui.NovoEstadoCarregandoCom(texto, func() bool { return false }).View()
}

// secoesExtras são as seções de eventos e de log do painel: carregando
// enquanto os extras não chegam; sem leitor, nenhuma.
func (m Model) secoesExtras(it itemLista, interno int) []secao {
	if m.leitor() == nil {
		return nil
	}
	pod := abas[m.activeTab].kind == "pod"
	x, lidos := m.extras[m.chaveDetalhe()]
	eventos := secao{titulo: "ÚLTIMOS EVENTOS"}
	logs := secao{titulo: "ÚLTIMA LINHA DE LOG"}
	switch {
	case !lidos:
		eventos.linhas = []string{carregandoNoPainel("lendo eventos")}
		logs.linhas = []string{carregandoNoPainel("lendo o log")}
	default:
		switch {
		case x.errEventos != nil:
			eventos.linhas = []string{yui.NovoEstadoErro("sem acesso aos eventos", "e mostra o erro").View()}
		case len(x.eventos) == 0:
			eventos.linhas = []string{yui.NovoEstadoVazio("nenhum evento recente", "").View()}
		}
		for _, e := range x.eventos {
			eventos.linhas = append(eventos.linhas, linhaDeEvento(e, m.agora()))
		}
		switch {
		case x.errLog != nil:
			logs.linhas = []string{yui.NovoEstadoErro("sem acesso ao log", "l mostra o erro").View()}
		case x.log == "":
			logs.linhas = []string{yui.NovoEstadoVazio("nenhuma linha de log", "").View()}
		default:
			logs.linhas = duasLinhas(x.log, interno)
		}
	}
	if !pod {
		return []secao{eventos}
	}
	return []secao{eventos, logs}
}

// duasLinhas quebra a linha de log na largura e fica com duas linhas; o que
// sobra sai com reticências.
func duasLinhas(l string, largura int) []string {
	partes := strings.Split(ansi.Wrap(l, max(1, largura), ""), "\n")
	if len(partes) > 2 {
		partes = []string{partes[0], ansi.Truncate(strings.Join(partes[1:], " "), largura, yui.G().Reticencias)}
	}
	partes[0] = linhaDeLog(partes[0])
	if len(partes) == 2 {
		partes[1] = yui.S().Tinta.Render(partes[1])
	}
	return partes
}

// teclasDoPainel são as teclas do painel, na ordem da prancha VizPods: logs
// (pods), yaml, eventos e perguntar ao Bard. Com largura > 0 (o rodapé do
// painel ao lado), quando as quatro não cabem, o b fica "b Bard": a ligação
// com o Bard não some do painel por causa do texto comprido dela.
func teclasDoPainel(kind string, largura int) []key.Binding {
	l := mapa().Lista
	teclas := []key.Binding{l.Yaml, l.Eventos, l.Bard}
	if kind == "pod" {
		teclas = []key.Binding{l.Logs, l.Yaml, l.Eventos, l.Bard}
	}
	if largura > 0 && larguraDaLegenda(teclas) > largura {
		h := l.Bard.Help()
		curta := key.NewBinding(key.WithKeys(l.Bard.Keys()...), key.WithHelp(h.Key, "Bard"))
		teclas[len(teclas)-1] = curta
	}
	return teclas
}

// larguraDaLegenda é a largura, em células, das teclas lado a lado como a
// legenda as mostra ("l logs  y yaml"): tecla, espaço, ação e 2 espaços entre
// elas.
func larguraDaLegenda(teclas []key.Binding) int {
	total := 0
	for i, t := range teclas {
		h := t.Help()
		if i > 0 {
			total += 2
		}
		total += lipgloss.Width(h.Key) + 1 + lipgloss.Width(h.Desc)
	}
	return total
}

// camposMinimos é quantos campos o painel guarda quando falta espaço (no pod,
// status e motivo).
const camposMinimos = 2

// formaDoPainel é um jeito de encaixar o painel: quantos campos (os do
// começo), com ou sem respiro antes das seções, as seções inteiras ou com uma
// linha cada, e quantas seções (as do começo).
type formaDoPainel struct {
	campos  int
	respiro bool
	curtas  bool
	secoes  int
}

// montar escreve as linhas do painel na forma f.
func (f formaDoPainel) montar(campos []string, secoes []secao) []string {
	linhas := append([]string(nil), campos[:f.campos]...)
	for _, sc := range secoes[:f.secoes] {
		if f.respiro {
			linhas = append(linhas, "")
		}
		corpo := sc.linhas
		if f.curtas {
			corpo = corpo[:1]
		}
		linhas = append(append(linhas, yui.S().Rotulo.Render(sc.titulo)), corpo...)
	}
	return linhas
}

// encaixar escolhe a primeira forma que cabe em espaco linhas. Sem espaço,
// sai primeiro o respiro, depois os campos do fim (no pod, recursos, dono,
// imagem e nó), depois as linhas a mais de cada seção e, por fim, as seções
// do fim: os últimos eventos e a última linha de log valem mais que os
// campos secundários.
func encaixar(campos []string, secoes []secao, espaco int) []string {
	minimo := min(len(campos), camposMinimos)
	formas := []formaDoPainel{{len(campos), true, false, len(secoes)}}
	for n := len(campos); n >= minimo; n-- {
		formas = append(formas, formaDoPainel{n, false, false, len(secoes)})
	}
	for n := len(secoes); n >= 0; n-- {
		formas = append(formas, formaDoPainel{minimo, false, true, n})
	}
	for _, f := range formas {
		if linhas := f.montar(campos, secoes); len(linhas) <= espaco {
			return linhas
		}
	}
	return campos[:min(len(campos), espaco)]
}

// painel desenha o detalhe do recurso selecionado num Quadro de largura x
// altura: os campos, os últimos eventos e a última linha de log, e, ao lado
// da lista, as teclas no rodapé; sem espaço, o encaixar escolhe o que sai.
func (m Model) painel(largura, altura int, comTeclas bool) string {
	it, ok := m.itemSelecionado()
	if !ok || altura < 3 || largura < 8 {
		return ""
	}
	d := abas[m.activeTab]
	interno, espaco := largura-4, altura-2
	var rodape string
	if comTeclas && espaco >= 2 {
		rodape = yui.TeclasDoPainel(interno, teclasDoPainel(d.kind, interno)...)
		espaco--
	}
	var campos []string
	for _, c := range m.camposDoItem(it) {
		campos = append(campos, c.rotulo+c.valor)
	}
	linhas := encaixar(campos, m.secoesExtras(it, interno), espaco)
	for len(linhas) < espaco {
		linhas = append(linhas, "")
	}
	if rodape != "" {
		linhas = append(linhas, rodape)
	}
	return yui.QuadroNomeado(d.singular, it.nome, strings.Join(linhas, "\n"), largura, yui.QuadroFoco)
}

// detalheNaTela é o painel no tamanho que a moldura dá a ele: ao lado, a
// coluna dos 40% na altura do corpo; por cima, a janela toda no limite.
func (m Model) detalheNaTela() string {
	switch {
	case m.painelAoLado():
		_, w := yui.LargurasLadoALado(m.width)
		return m.painel(w, m.alturaCorpo(), true)
	case m.painelPorCima():
		return m.painel(m.width, m.alturaDoPainelPorCima(), false)
	}
	return ""
}

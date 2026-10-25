package ui

import (
	"fmt"
	"time"

	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type tickMsg time.Time

// Model é o modelo principal do Bubbletea para o Viz
type Model struct {
	client         monitor.Client
	activeTab      ResourceTab
	pods           []monitor.Pod
	deployments    []monitor.Deployment
	services       []monitor.Service
	nodes          []monitor.Node
	statefulsets   []monitor.StatefulSet
	jobs           []monitor.Job
	ingresses      []monitor.Ingress
	configmaps     []monitor.ConfigMap
	events         []monitor.Event
	err            error
	width          int
	height         int
	scrollY        int // primeira linha da lista na janela
	selectedIndex  int // linha selecionada na lista visível
	reconnecting   bool
	retryCount     int
	filter         FilterState
	actionMode     ActionMode
	actionTarget   string           // nome do recurso alvo da ação
	actionKind     string           // tipo do recurso alvo, guardado quando a ação abre
	actionNS       string           // namespace do recurso alvo, guardado quando a ação abre
	actionBuffer   string           // buffer de input para ações (scale, confirmação)
	actionFeedback string           // resultado da última ação, já com glifo (linha de estado)
	carregadas     uint16           // bit por aba: a lista já chegou do cluster ao menos uma vez
	atualizado     time.Time        // hora da última lista recebida (linha de estado)
	agora          func() time.Time // relógio, injetável nos testes
	rodizio        ResourceTab      // última aba de fundo relida pelo relógio
	ajuda          bool             // a tela ? está aberta
	ajudaScroll    int              // primeira linha da ajuda na janela
	recarregando   bool             // o r pediu uma leitura que ainda não voltou
	detalheAberto  bool             // o ↵ abriu o painel (abaixo de 120 colunas ele só aparece assim)
	alvoDetalhe    string           // recurso do painel na tela (tipo|chave); vazio sem painel
	pedidoDetalhe  int              // atraso mais novo: só ele lê os extras do painel
	lendoExtras    string           // recurso cujos extras estão sendo lidos
	leituraExtras  int              // leitura de extras mais nova: a resposta de outra é descartada
	extras         map[string]extrasDetalhe
	quadro         quadroAberto // logs, YAML ou eventos ocupando o corpo
	idQuadro       int          // último pedido de conteúdo do quadro: resposta de outro é descartada
	// tique agenda a próxima leitura do relógio (tea.Tick; injetável nos testes).
	tique func(time.Duration, func(time.Time) tea.Msg) tea.Cmd
}

// FilterState mantém o estado dos filtros de busca
type FilterState struct {
	Namespace     string
	LabelSelector string
	StatusFilter  string
	SearchQuery   string
	InputMode     bool
	InputBuffer   string
	InputField    string // "search" (/), "namespace" (n) ou "filtro" (f: status ou label)
	buscaAnterior string // a busca de antes do /, que o esc devolve
}

// NewModel cria o modelo com injeção do client
func NewModel(client monitor.Client) Model {
	var err error
	if client == nil {
		err = fmt.Errorf("client K8s não disponível")
	}
	return Model{
		client: client,
		err:    err,
		agora:  time.Now,
		tique:  tea.Tick,
	}
}

// NewModelComErro cria o modelo sem client, para mostrar na TUI por que a
// conexão com o cluster falhou (kubeconfig inválido, cluster fora do ar).
func NewModelComErro(err error) Model {
	m := NewModel(nil)
	if err != nil {
		m.err = err
	}
	return m
}

// Init inicializa o modelo do Bubbletea: a primeira leitura abre o relógio,
// e cada resposta do relógio agenda a seguinte (uma leitura por vez).
func (m Model) Init() tea.Cmd {
	if m.err != nil {
		return nil
	}
	return m.ler(true, m.activeTab)
}

// leituraMsg é a resposta de uma leitura do cluster: a lista (ou o erro) de
// cada aba lida, a primeira sendo a aba ativa no pedido, e o namespace da leitura.
type leituraMsg struct {
	namespace string
	relogio   bool          // leitura do relógio: a resposta agenda a próxima
	recarga   bool          // leitura pedida pelo r: a resposta avisa na linha de estado
	abas      []ResourceTab // a aba de cada resposta
	respostas []tea.Msg     // []monitor.Pod, []monitor.Deployment… ou error
}

// ler devolve o comando que lê as abas dadas no namespace atual. O label, o
// status e a busca são filtros da tela: a leitura traz a lista inteira, que
// também dá o total do "N de M".
func (m Model) ler(relogio bool, tabs ...ResourceTab) tea.Cmd {
	client, ns := m.client, m.filter.Namespace
	return func() tea.Msg {
		msg := leituraMsg{namespace: ns, relogio: relogio, abas: tabs}
		for _, t := range tabs {
			msg.respostas = append(msg.respostas, lerAba(client, ns, t))
		}
		return msg
	}
}

// lerAba busca a lista de uma aba; devolve a lista ou o erro.
func lerAba(client monitor.Client, ns string, tab ResourceTab) tea.Msg {
	if client == nil {
		return fmt.Errorf("client K8s não disponível")
	}
	filter := monitor.ListFilter{Namespace: ns}
	var data any
	var err error
	switch tab {
	case TabPods:
		data, err = client.GetPods(filter)
	case TabDeployments:
		data, err = client.GetDeployments(filter)
	case TabServices:
		data, err = client.GetServices(filter)
	case TabNodes:
		data, err = client.GetNodes(filter)
	case TabStatefulSets:
		data, err = client.GetStatefulSets(filter)
	case TabJobs:
		data, err = client.GetJobs(filter)
	case TabIngresses:
		data, err = client.GetIngresses(filter)
	case TabConfigMaps:
		data, err = client.GetConfigMaps(filter)
	case TabEvents:
		data, err = client.GetEvents(filter)
	default:
		return fmt.Errorf("aba desconhecida: %d", tab)
	}
	if err != nil {
		return err
	}
	return data
}

// alturaCorpo é a altura do corpo da moldura: a janela menos masthead, régua,
// abas e filtros em cima e régua, estado e legenda embaixo.
func (m Model) alturaCorpo() int {
	return max(1, m.height-7)
}

// linhasDeDados é quantas linhas da tabela cabem na lista (sem a linha em
// branco e o cabeçalho acima delas e, quando a lista não cabe, sem a última
// linha, que avisa quantas ficaram fora); sem tamanho de janela, não há limite.
func (m Model) linhasDeDados() int {
	if m.height <= 0 {
		return 1000
	}
	h := max(1, m.alturaDaLista()-m.linhasAntesDaTabela())
	if m.itemCount() > h {
		h = max(1, h-1)
	}
	return h
}

// itemCount retorna o número de linhas da lista visível da aba ativa.
func (m Model) itemCount() int {
	return len(m.itensDaAba(m.activeTab))
}

// seguirSelecao rola a lista para a linha selecionada ficar na janela, sem
// passar do fim: o scrollY do modelo é sempre o que a tela desenha (o clique
// e a ação contam as linhas a partir dele).
func (m *Model) seguirSelecao() {
	h := m.linhasDeDados()
	if m.selectedIndex >= m.scrollY+h {
		m.scrollY = m.selectedIndex - h + 1
	}
	if m.selectedIndex < m.scrollY {
		m.scrollY = m.selectedIndex
	}
	m.clampScroll(m.itemCount())
}

// clampScroll garante que scrollY está dentro dos limites válidos
func (m *Model) clampScroll(total int) {
	maxScroll := total - m.linhasDeDados()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollY > maxScroll {
		m.scrollY = maxScroll
	}
	if m.scrollY < 0 {
		m.scrollY = 0
	}
}

// reselecionar mantém a seleção no mesmo recurso quando a lista chega de novo
// (na ordem "problemas primeiro" um recurso muda de lugar quando muda de
// estado); se ele saiu da lista, a seleção fica dentro dela. A janela só
// acompanha a seleção quando ela muda de linha: a rolagem do usuário fica.
func (m *Model) reselecionar(chave string, antes int) {
	itens := m.itensDaAba(m.activeTab)
	novo := -1
	if chave != "" {
		if antes >= 0 && antes < len(itens) && itens[antes].chave == chave {
			novo = antes // chave repetida (eventos sem ID) fica na mesma linha
		} else {
			for i, it := range itens {
				if it.chave == chave {
					novo = i
					break
				}
			}
		}
	}
	if novo < 0 {
		novo = max(0, min(antes, len(itens)-1))
	}
	mudou := novo != m.selectedIndex
	m.selectedIndex = novo
	if mudou {
		m.seguirSelecao()
		return
	}
	m.clampScroll(len(itens))
}

// intervaloAtualizacao é o tempo entre leituras do cluster ("ao vivo · 2 s").
const intervaloAtualizacao = 2 * time.Second

// intervaloSemConexao é o tempo entre tentativas depois que a conexão caiu.
const intervaloSemConexao = 5 * time.Second

// proximoTique agenda a próxima leitura do relógio: a cada 2 s, com espera
// crescente enquanto reconecta e devagar sem conexão.
func (m Model) proximoTique() tea.Cmd {
	if m.client == nil || m.tique == nil {
		return nil
	}
	d := intervaloAtualizacao
	switch {
	case m.err != nil:
		d = intervaloSemConexao
	case m.reconnecting:
		d = time.Duration(1<<uint(max(0, m.retryCount-1))) * time.Second
	}
	return m.tique(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// abaDeFundo escolhe a próxima aba já lida, fora a ativa, para o relógio
// reler junto (uma por vez, em rodízio): as contagens das abas seguem vivas.
func (m Model) abaDeFundo() (ResourceTab, bool) {
	for i := ResourceTab(1); i <= tabCount; i++ {
		t := (m.rodizio + i) % tabCount
		if t != m.activeTab && m.carregada(t) {
			return t, true
		}
	}
	return 0, false
}

// receber guarda a lista que chegou e devolve se a mensagem era uma lista.
func (m *Model) receber(msg tea.Msg) bool {
	chave, antes := m.chaveSelecionada(), m.selectedIndex
	var tab ResourceTab
	switch d := msg.(type) {
	case []monitor.Pod:
		tab, m.pods = TabPods, d
	case []monitor.Deployment:
		tab, m.deployments = TabDeployments, d
	case []monitor.Service:
		tab, m.services = TabServices, d
	case []monitor.Node:
		tab, m.nodes = TabNodes, d
	case []monitor.StatefulSet:
		tab, m.statefulsets = TabStatefulSets, d
	case []monitor.Job:
		tab, m.jobs = TabJobs, d
	case []monitor.Ingress:
		tab, m.ingresses = TabIngresses, d
	case []monitor.ConfigMap:
		tab, m.configmaps = TabConfigMaps, d
	case []monitor.Event:
		tab, m.events = TabEvents, d
	default:
		return false
	}
	m.carregadas |= 1 << uint(tab)
	m.err = nil
	m.reconnecting = false
	m.retryCount = 0
	m.atualizado = m.agora()
	if tab == m.activeTab {
		m.reselecionar(chave, antes)
	}
	return true
}

// temDados diz se alguma aba tem lista para mostrar enquanto reconecta.
func (m Model) temDados() bool {
	return len(m.pods) > 0 || len(m.deployments) > 0 || len(m.services) > 0 || len(m.nodes) > 0 ||
		len(m.statefulsets) > 0 || len(m.jobs) > 0 || len(m.ingresses) > 0 || len(m.configmaps) > 0 || len(m.events) > 0
}

// falhar registra uma leitura que falhou: sem dados, mostra o erro; com
// dados, tenta reconectar preservando o último estado até maxRetries.
func (m *Model) falhar(err error) {
	if !m.temDados() {
		m.err = err
		return
	}
	m.retryCount++
	if m.retryCount >= maxRetries {
		m.err = err
		m.reconnecting = false
		return
	}
	m.reconnecting = true
}

// esquecerListas apaga as listas lidas (o namespace mudou): as abas voltam
// sem número e a ativa mostra "lendo …" até a próxima leitura.
func (m *Model) esquecerListas() {
	m.pods, m.deployments, m.services, m.nodes, m.statefulsets = nil, nil, nil, nil, nil
	m.jobs, m.ingresses, m.configmaps, m.events = nil, nil, nil, nil
	m.carregadas = 0
	m.scrollY, m.selectedIndex = 0, 0
}

// irParaAba troca a aba ativa, volta ao topo e lê a lista nova.
func (m Model) irParaAba(tab ResourceTab) (tea.Model, tea.Cmd) {
	m.activeTab = tab
	m.scrollY = 0
	m.selectedIndex = 0
	return m, m.ler(false, tab)
}

// actionResultMsg carrega o resultado de uma ação executada
type actionResultMsg struct {
	feedback  string
	err       error
	cancelada bool // a confirmação não bateu: nada foi feito
}

// maxRetries é o número máximo de tentativas de reconexão antes de exibir erro fatal
const maxRetries = 3

// selectedResourceInfo retorna nome, namespace e kind do recurso selecionado
// na aba ativa, contado na lista como aparece na tela (filtros, busca e ordem)
func (m Model) selectedResourceInfo() (name, namespace, kind string) {
	itens := m.itensDaAba(m.activeTab)
	if m.selectedIndex < 0 || m.selectedIndex >= len(itens) {
		return "", "", ""
	}
	it := itens[m.selectedIndex]
	return it.nome, it.namespace, abas[m.activeTab].kind
}

// chaveSelecionada é a identidade do recurso selecionado (vazia sem seleção).
func (m Model) chaveSelecionada() string {
	itens := m.itensDaAba(m.activeTab)
	if m.selectedIndex < 0 || m.selectedIndex >= len(itens) {
		return ""
	}
	return itens[m.selectedIndex].chave
}

// Update processa mensagens e atualiza o estado do modelo; depois de cada
// mensagem, o painel de detalhe confere se a seleção mudou de recurso.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	nm, cmd := m.processar(msg)
	novo, ok := nm.(Model)
	if !ok {
		return nm, cmd
	}
	if c := novo.acompanharDetalhe(); c != nil {
		cmd = tea.Batch(cmd, c)
	}
	return novo, cmd
}

// processar trata uma mensagem.
func (m Model) processar(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// ctrl+c sai de qualquer lugar: campo, menu, confirmação, ajuda, quadro, detalhe ou lista.
		if key.Matches(msg, mapa().Global.SairForcado) {
			return m, tea.Quit
		}
		m.actionFeedback = "" // a frase da última ação some na tecla seguinte
		return m.tecla(msg)

	case tea.MouseMsg:
		return m.mouse(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if yui.ModoPara(m.width, m.height) == yui.ModoLadoALado {
			m.detalheAberto = false // ao lado, o painel não precisa do ↵; voltando a estreitar, ele começa fechado
		}
		m.seguirSelecao()
		m.rolarQuadro(0) // a rolagem do quadro fica dentro do máximo da janela nova
		return m, nil

	case tickMsg:
		tabs := []ResourceTab{m.activeTab}
		if t, ok := m.abaDeFundo(); ok {
			m.rodizio = t
			tabs = append(tabs, t)
		}
		cmd := tea.Batch(m.ler(true, tabs...), m.renovarExtras()) // renovarExtras muda m: antes do return
		return m, cmd

	case leituraMsg:
		// Uma leitura de outro namespace chegou atrasada: os dados não valem.
		if msg.namespace == m.filter.Namespace {
			for i, r := range msg.respostas {
				if err, ok := r.(error); ok {
					// Só a falha da aba ativa derruba a tela: a de uma aba de fundo
					// (RBAC) ou de uma aba que a pessoa já deixou, não.
					if i < len(msg.abas) && msg.abas[i] == m.activeTab {
						m.falhar(err)
					}
					continue
				}
				m.receber(r)
			}
			if msg.recarga {
				m.fimDaRecarga(msg)
			}
		}
		if msg.recarga {
			m.recarregando = false
		}
		if msg.relogio {
			return m, m.proximoTique()
		}
		return m, nil

	case paradaMsg:
		cmd := m.selecaoParou(msg)
		return m, cmd

	case extrasMsg:
		m.guardarExtras(msg)
		return m, nil

	case conteudoMsg:
		m.receberConteudo(msg)
		return m, nil

	case actionResultMsg:
		switch {
		case msg.err != nil:
			m.actionFeedback = frase(yui.StatusFalha, fmt.Sprintf("erro: %v", msg.err))
		case msg.cancelada:
			m.actionFeedback = frase(yui.StatusParado, msg.feedback)
		default:
			m.actionFeedback = frase(yui.StatusRodando, msg.feedback)
		}
		return m, m.ler(false, m.activeTab)

	case error:
		m.falhar(msg)

	default:
		m.receber(msg) // lista avulsa (testes)
	}

	return m, nil
}

// acessoDireto é o client que alcança a API do cluster além das listagens
// (as ações): o K8sClient, direto ou dentro do RetryClient
type acessoDireto interface {
	Clientset() kubernetes.Interface
	RESTConfig() *rest.Config
}

// resolveK8sClient extrai o acesso direto do client (direto ou dentro de RetryClient)
func resolveK8sClient(c monitor.Client) acessoDireto {
	if k, ok := c.(acessoDireto); ok {
		return k
	}
	if rc, ok := c.(*monitor.RetryClient); ok {
		if k, ok2 := rc.Inner().(acessoDireto); ok2 {
			return k
		}
	}
	return nil
}

// comContexto é o client que conhece o contexto atual do kubeconfig (o
// K8sClient e o RetryClient); o nome vai para o masthead.
type comContexto interface {
	Contexto() string
}

// contextoDoCluster devolve o contexto do kubeconfig, ou vazio se o client não o conhece.
func (m Model) contextoDoCluster() string {
	if c, ok := m.client.(comContexto); ok {
		return c.Contexto()
	}
	return ""
}

// alvoDaAcao é o recurso escolhido quando a ação abriu: a lista pode mudar
// (atualização, busca) até a confirmação, e a ação não pode mudar de alvo
type alvoDaAcao struct {
	nome, namespace, kind string
}

// executeAction executa a ação confirmada pelo usuário
func (m Model) executeAction(mode ActionMode, alvo alvoDaAcao, input string) tea.Cmd {
	return func() tea.Msg {
		k8sClient := resolveK8sClient(m.client)
		if k8sClient == nil {
			return actionResultMsg{err: fmt.Errorf("client não suporta ações diretas")}
		}

		target, ns, kind := alvo.nome, alvo.namespace, alvo.kind
		clientset := k8sClient.Clientset()

		switch mode {
		case ActionConfirmDelete:
			if input != "yes" {
				return actionResultMsg{feedback: "exclusão cancelada", cancelada: true}
			}
			err := monitor.DeleteResource(clientset, kind, target, ns)
			if err != nil {
				return actionResultMsg{err: err}
			}
			return actionResultMsg{feedback: fmt.Sprintf("%s %s excluído", nomeDoTipo(kind), target)}

		case ActionInputScale:
			var replicas int32
			if _, err := fmt.Sscanf(input, "%d", &replicas); err != nil {
				return actionResultMsg{err: fmt.Errorf("valor inválido para réplicas: %s", input)}
			}
			escalar, rotulo := monitor.ScaleDeployment, "Deployment"
			if kind == "statefulset" {
				escalar, rotulo = monitor.ScaleStatefulSet, "StatefulSet"
			}
			if err := escalar(clientset, target, ns, replicas); err != nil {
				return actionResultMsg{err: err}
			}
			return actionResultMsg{feedback: fmt.Sprintf("%s '%s' escalado para %d réplicas.", rotulo, target, replicas)}

		case ActionConfirmRestart:
			if input != "yes" {
				return actionResultMsg{feedback: "reinício cancelado", cancelada: true}
			}
			reiniciar, rotulo := monitor.RestartDeployment, "Deployment"
			if kind == "statefulset" {
				reiniciar, rotulo = monitor.RestartStatefulSet, "StatefulSet"
			}
			if err := reiniciar(clientset, target, ns); err != nil {
				return actionResultMsg{err: err}
			}
			return actionResultMsg{feedback: fmt.Sprintf("%s '%s' reiniciado.", rotulo, target)}
		}

		return actionResultMsg{err: fmt.Errorf("ação desconhecida")}
	}
}

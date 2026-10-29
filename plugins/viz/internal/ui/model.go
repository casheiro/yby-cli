package ui

import (
	"fmt"
	"time"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	yui "github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type tickMsg time.Time

// Model é o modelo principal do Bubbletea para o Viz
type Model struct {
	// interrompido: a TUI fechou com ctrl+c, que sai de qualquer lugar (o q sai só da lista).
	interrompido   bool
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
	acao           acaoAberta         // menu x ou confirmação na tela
	toast          yui.Toast          // resultado da última ação (linha de estado; some em 6 s)
	executando     string             // ação mandada ao cluster sem resposta ainda ("reiniciando Deployment api")
	copiar         func(string) error // manda o nome para a área de transferência (OSC 52; trocado nos testes)
	actionFeedback string             // aviso da última tecla, já com glifo (linha de estado)
	carregadas     uint16             // bit por aba: a lista já chegou do cluster ao menos uma vez
	atualizado     time.Time          // hora da última lista recebida (linha de estado)
	agora          func() time.Time   // relógio, injetável nos testes
	vigia          observacao         // o watch aberto no namespace atual
	vigiadas       uint16             // bit por aba: a lista vem do watch (o relógio não a troca)
	negadas        [tabCount]error    // o Forbidden de cada aba que o RBAC não deixa listar (o relógio não a lista)
	cancelouAcao   bool               // a janela pequena fechou uma camada: o aviso sai quando ela crescer
	ajuda          bool               // a tela ? está aberta
	ajudaScroll    int                // primeira linha da ajuda na janela
	recarregando   bool               // o r pediu uma leitura que ainda não voltou
	detalheAberto  bool               // o ↵ abriu o painel (abaixo de 120 colunas ele só aparece assim)
	alvoDetalhe    string             // recurso do painel na tela (tipo|chave); vazio sem painel
	pedidoDetalhe  int                // atraso mais novo: só ele lê os extras do painel
	lendoExtras    string             // recurso cujos extras estão sendo lidos
	leituraExtras  int                // leitura de extras mais nova: a resposta de outra é descartada
	extras         map[string]extrasDetalhe
	quadro         quadroAberto  // logs, YAML ou eventos ocupando o corpo
	idQuadro       int           // último pedido de conteúdo do quadro: resposta de outro é descartada
	bard           ligacaoBard   // o b abre o Bard (bard.go)
	bardAbrindo    bool          // o yby do b rodando: outro b espera ele voltar
	cache          *cacheDeItens // a lista de cada aba já filtrada e ordenada (itensDaAba)
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
	m := Model{
		client: client,
		err:    err,
		agora:  time.Now,
		tique:  tea.Tick,
		toast:  yui.NovoToast(),
		copiar: yui.CopiarOSC52,
		bard:   novaLigacaoBard(),
		cache:  &cacheDeItens{},
	}
	// Começa no namespace pedido (--namespace) ou no do contexto do kubeconfig,
	// como o kubectl; n troca.
	if c, ok := client.(interface{ NamespaceInicial() string }); ok {
		m.filter.Namespace = c.NamespaceInicial()
	}
	return m
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
// e cada resposta do relógio agenda a seguinte (uma leitura por vez); com um
// client que observa, o watch abre junto.
func (m Model) Init() tea.Cmd {
	if m.err != nil {
		return nil
	}
	ler := m.ler(true, m.activeTab)
	if _, ok := m.client.(monitor.Observador); ok {
		return tea.Batch(ler, func() tea.Msg { return vigiarMsg{} })
	}
	return ler
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
	client, ns, vigiadas, negadas := m.client, m.filter.Namespace, m.vigiadas, m.negadas
	return func() tea.Msg {
		msg := leituraMsg{namespace: ns, relogio: relogio, abas: tabs}
		for _, t := range tabs {
			negada := negadas[t] != nil
			if sondador, ok := client.(monitor.Sondador); ok && relogio && (vigiadas&(1<<uint(t)) != 0 || negada) {
				// A lista é do watch, ou o RBAC não deixa listar (repetir o LIST a
				// cada 2 s só encheria o log da API): o relógio só confirma que o
				// cluster responde.
				if err := sondador.Sondar(); err != nil {
					msg.respostas = append(msg.respostas, err)
				} else {
					msg.respostas = append(msg.respostas, sondaOK{})
				}
				continue
			}
			if relogio && negada {
				msg.respostas = append(msg.respostas, sondaOK{}) // sem sonda, nada a confirmar
				continue
			}
			msg.respostas = append(msg.respostas, lerAba(client, ns, t))
		}
		return msg
	}
}

// sondaOK é a resposta da sonda de conexão de uma aba vigiada: não traz lista.
type sondaOK struct{}

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
	return m.contagemDaAba(m.activeTab)
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

// receber guarda a lista que chegou, marca a conexão de pé e devolve se a
// mensagem era uma lista.
func (m *Model) receber(msg tea.Msg) bool {
	chave, antes := m.chaveSelecionada(), m.selectedIndex
	tab, ok := m.guardarLista(msg)
	if !ok {
		return false
	}
	m.conexaoDePe()
	if tab == m.activeTab {
		m.reselecionar(chave, antes)
	}
	return true
}

// conexaoDePe registra uma leitura que deu certo: sai o erro e a reconexão.
func (m *Model) conexaoDePe() {
	m.err = nil
	m.reconnecting = false
	m.retryCount = 0
	m.atualizado = m.agora()
}

// guardarLista guarda a lista na aba dela, sem mexer no estado da conexão
// (a lista de uma aba de fundo, pelo watch); devolve a aba.
func (m *Model) guardarLista(msg tea.Msg) (ResourceTab, bool) {
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
		return 0, false
	}
	m.carregadas |= 1 << uint(tab)
	m.negadas[tab] = nil // a lista chegou: o RBAC deixa listar
	m.atualizado = m.agora()
	return tab, true
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
	m.carregadas, m.vigiadas = 0, 0
	m.negadas = [tabCount]error{} // a permissão do namespace novo é outra
	m.scrollY, m.selectedIndex = 0, 0
}

// esquecerAba apaga a lista de uma aba (o RBAC não deixa listar o tipo): ela
// fica sem número.
func (m *Model) esquecerAba(tab ResourceTab) {
	chave, antes := m.chaveSelecionada(), m.selectedIndex
	switch tab {
	case TabPods:
		m.pods = nil
	case TabDeployments:
		m.deployments = nil
	case TabServices:
		m.services = nil
	case TabNodes:
		m.nodes = nil
	case TabStatefulSets:
		m.statefulsets = nil
	case TabJobs:
		m.jobs = nil
	case TabIngresses:
		m.ingresses = nil
	case TabConfigMaps:
		m.configmaps = nil
	case TabEvents:
		m.events = nil
	}
	m.carregadas &^= 1 << uint(tab)
	if tab == m.activeTab {
		m.reselecionar(chave, antes)
	}
}

// negar registra que o RBAC não deixa listar a aba: ela perde a lista e o
// número, e o relógio deixa de listá-la até a troca de namespace, o r ou uma
// lista que chegue (a troca de aba lê uma vez).
func (m *Model) negar(tab ResourceTab, err error) {
	m.negadas[tab] = err
	m.esquecerAba(tab)
}

// negada diz se o RBAC não deixa listar a aba.
func (m Model) negada(tab ResourceTab) bool {
	return m.negadas[tab] != nil
}

// irParaAba troca a aba ativa, volta ao topo e lê a lista nova.
func (m Model) irParaAba(tab ResourceTab) (tea.Model, tea.Cmd) {
	m.activeTab = tab
	m.scrollY = 0
	m.selectedIndex = 0
	return m, m.ler(false, tab)
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
	if novo.acao.aberta() && !novo.listaNaTela() {
		novo.cancelarAcaoSemLista()
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
		msg, fica := yui.TeclaAltGr(msg)
		if !fica {
			return m, nil
		}
		// ctrl+c sai de qualquer lugar: campo, menu, confirmação, ajuda, quadro, detalhe ou lista.
		if key.Matches(msg, mapa().Global.SairForcado) {
			m.interrompido = true
			return m.sair()
		}
		if m.janelaPequena() {
			return m.teclaJanelaPequena(msg)
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
		var cmd tea.Cmd
		if m.janelaPequena() {
			m.cancelarPelaJanela()
		} else if m.cancelouAcao {
			m.cancelouAcao = false
			m.actionFeedback = "" // o aviso é a frase mais nova
			m.toast, cmd = m.toast.Mostrar(yui.ToastAviso, "ação cancelada: a janela ficou pequena demais")
		}
		m.seguirSelecao()
		m.rolarQuadro(0) // a rolagem do quadro fica dentro do máximo da janela nova
		return m, cmd

	case tickMsg:
		cmd := tea.Batch(m.ler(true, m.activeTab), m.renovarExtras()) // renovarExtras muda m: antes do return
		return m, cmd

	case vigiarMsg:
		if m.vigia.canal != nil {
			return m, nil // já aberto
		}
		cmd := m.vigiar()
		return m, cmd

	case loteMsg:
		cmd := m.receberLote(msg)
		return m, cmd

	case leituraMsg:
		// Uma leitura de outro namespace chegou atrasada: os dados não valem.
		if msg.namespace == m.filter.Namespace {
			for i, r := range msg.respostas {
				tab, temAba := m.activeTab, i < len(msg.abas)
				if temAba {
					tab = msg.abas[i]
				}
				if err, ok := r.(error); ok {
					switch {
					case temAba && apierrors.IsForbidden(err):
						// Falta de permissão não é queda: o cluster respondeu. A aba
						// diz o que falta, sem reconectar.
						m.negar(tab, err)
						if tab == m.activeTab {
							m.conexaoDePe()
						}
					case temAba && tab == m.activeTab:
						// Só a falha da aba ativa derruba a tela: a de uma aba de
						// fundo ou de uma aba que a pessoa já deixou, não.
						m.falhar(err)
					}
					continue
				}
				if _, sonda := r.(sondaOK); sonda {
					// A aba pode ter deixado o watch (ou ganhado a permissão) entre o
					// pedido e a resposta: o próximo tique volta a ler a lista.
					if temAba && (m.vigiada(tab) || m.negada(tab)) {
						m.conexaoDePe()
					}
					continue
				}
				if temAba && m.vigiada(tab) {
					// A lista é do watch: trocá-la por esta leitura poderia desfazer
					// uma mudança que o watch já trouxe. A leitura só mostra que a
					// conexão está de pé.
					m.conexaoDePe()
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

	case bardFechadoMsg:
		return m.bardFechado(msg)

	case resultadoAcaoMsg:
		return m.mostrarResultado(msg)

	case yui.ToastSomeMsg:
		t, _ := m.toast.Update(msg)
		m.toast = t.(yui.Toast)
		return m, nil

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

// Interrompido informa se a TUI fechou com ctrl+c.
func (m Model) Interrompido() bool { return m.interrompido }

// contextoDoCluster devolve o contexto do kubeconfig, ou vazio se o client não o conhece.
func (m Model) contextoDoCluster() string {
	if c, ok := m.client.(comContexto); ok {
		return c.Contexto()
	}
	return ""
}

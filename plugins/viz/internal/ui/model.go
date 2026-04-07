package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
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
	scrollY        int
	selectedIndex  int
	reconnecting   bool
	retryCount     int
	filter         FilterState
	viewMode       ViewMode
	detailContent  string
	detailTitle    string
	detailScrollY  int
	actionMode     ActionMode
	actionTarget   string // nome do recurso alvo da ação
	actionBuffer   string // buffer de input para ações (scale, confirmação)
	actionFeedback string // mensagem de feedback após ação
}

// FilterState mantém o estado dos filtros de busca
type FilterState struct {
	Namespace     string
	LabelSelector string
	StatusFilter  string
	SearchQuery   string
	InputMode     bool
	InputBuffer   string
	InputField    string // "namespace", "label", "status", "search"
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
	}
}

// Init inicializa o modelo do Bubbletea
func (m Model) Init() tea.Cmd {
	if m.err != nil {
		return nil
	}
	return tea.Batch(
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
		m.fetchResources,
	)
}

// fetchResources busca os recursos da aba ativa
func (m Model) fetchResources() tea.Msg {
	if m.client == nil {
		return nil
	}
	filter := monitor.ListFilter{
		Namespace:     m.filter.Namespace,
		LabelSelector: m.filter.LabelSelector,
	}
	switch m.activeTab {
	case TabPods:
		data, err := m.client.GetPods(filter)
		if err != nil {
			return err
		}
		return data
	case TabDeployments:
		data, err := m.client.GetDeployments(filter)
		if err != nil {
			return err
		}
		return data
	case TabServices:
		data, err := m.client.GetServices(filter)
		if err != nil {
			return err
		}
		return data
	case TabNodes:
		data, err := m.client.GetNodes(filter)
		if err != nil {
			return err
		}
		return data
	case TabStatefulSets:
		data, err := m.client.GetStatefulSets(filter)
		if err != nil {
			return err
		}
		return data
	case TabJobs:
		data, err := m.client.GetJobs(filter)
		if err != nil {
			return err
		}
		return data
	case TabIngresses:
		data, err := m.client.GetIngresses(filter)
		if err != nil {
			return err
		}
		return data
	case TabConfigMaps:
		data, err := m.client.GetConfigMaps(filter)
		if err != nil {
			return err
		}
		return data
	case TabEvents:
		data, err := m.client.GetEvents(filter)
		if err != nil {
			return err
		}
		return data
	}
	return nil
}

// visibleHeight retorna o número de linhas visíveis (reservando espaço para UI)
func (m Model) visibleHeight() int {
	if m.height <= 0 {
		return 1000 // sem limite quando tamanho da janela não foi definido
	}
	h := m.height - 5
	if h < 1 {
		h = 1
	}
	return h
}

// itemCount retorna o número de itens de dados na aba ativa (sem o header), com filtros aplicados
func (m Model) itemCount() int {
	switch m.activeTab {
	case TabPods:
		pods := m.filteredPods()
		return len(filterByName(pods, m.filter.SearchQuery, func(p monitor.Pod) string { return p.Name }))
	case TabDeployments:
		return len(m.filteredDeployments())
	case TabServices:
		return len(m.filteredServices())
	case TabNodes:
		return len(m.filteredNodes())
	case TabStatefulSets:
		return len(m.filteredStatefulSets())
	case TabJobs:
		return len(m.filteredJobs())
	case TabIngresses:
		return len(m.filteredIngresses())
	case TabConfigMaps:
		return len(m.filteredConfigMaps())
	case TabEvents:
		return len(m.filteredEvents())
	}
	return 0
}

// filteredItems retorna os itens da aba ativa com filtros de status e busca aplicados
func (m Model) filteredDeployments() []monitor.Deployment {
	return filterByName(m.deployments, m.filter.SearchQuery, func(d monitor.Deployment) string { return d.Name })
}

func (m Model) filteredServices() []monitor.Service {
	return filterByName(m.services, m.filter.SearchQuery, func(s monitor.Service) string { return s.Name })
}

func (m Model) filteredNodes() []monitor.Node {
	return filterByName(m.nodes, m.filter.SearchQuery, func(n monitor.Node) string { return n.Name })
}

func (m Model) filteredStatefulSets() []monitor.StatefulSet {
	return filterByName(m.statefulsets, m.filter.SearchQuery, func(s monitor.StatefulSet) string { return s.Name })
}

func (m Model) filteredJobs() []monitor.Job {
	return filterByName(m.jobs, m.filter.SearchQuery, func(j monitor.Job) string { return j.Name })
}

func (m Model) filteredIngresses() []monitor.Ingress {
	return filterByName(m.ingresses, m.filter.SearchQuery, func(i monitor.Ingress) string { return i.Name })
}

func (m Model) filteredConfigMaps() []monitor.ConfigMap {
	return filterByName(m.configmaps, m.filter.SearchQuery, func(c monitor.ConfigMap) string { return c.Name })
}

func (m Model) filteredEvents() []monitor.Event {
	return filterByName(m.events, m.filter.SearchQuery, func(e monitor.Event) string { return e.Name })
}

// contentLines retorna o conteúdo renderizado da aba ativa
func (m Model) contentLines() []string {
	var content string
	switch m.activeTab {
	case TabPods:
		pods := m.filteredPods()
		pods = filterByName(pods, m.filter.SearchQuery, func(p monitor.Pod) string { return p.Name })
		if len(pods) == 0 {
			content = "  Carregando pods...\n"
		} else {
			content = renderPodTable(pods)
		}
	case TabDeployments:
		deps := m.filteredDeployments()
		if len(deps) == 0 {
			content = "  Carregando deployments...\n"
		} else {
			content = renderDeploymentTable(deps)
		}
	case TabServices:
		svcs := m.filteredServices()
		if len(svcs) == 0 {
			content = "  Carregando services...\n"
		} else {
			content = renderServiceTable(svcs)
		}
	case TabNodes:
		nodes := m.filteredNodes()
		if len(nodes) == 0 {
			content = "  Carregando nodes...\n"
		} else {
			content = renderNodeTable(nodes)
		}
	case TabStatefulSets:
		sets := m.filteredStatefulSets()
		if len(sets) == 0 {
			content = "  Carregando statefulsets...\n"
		} else {
			content = renderStatefulSetTable(sets)
		}
	case TabJobs:
		jobs := m.filteredJobs()
		if len(jobs) == 0 {
			content = "  Carregando jobs...\n"
		} else {
			content = renderJobTable(jobs)
		}
	case TabIngresses:
		ings := m.filteredIngresses()
		if len(ings) == 0 {
			content = "  Carregando ingresses...\n"
		} else {
			content = renderIngressTable(ings)
		}
	case TabConfigMaps:
		cms := m.filteredConfigMaps()
		if len(cms) == 0 {
			content = "  Carregando configmaps...\n"
		} else {
			content = renderConfigMapTable(cms)
		}
	case TabEvents:
		evts := m.filteredEvents()
		if len(evts) == 0 {
			content = "  Carregando eventos...\n"
		} else {
			content = renderEventTable(evts)
		}
	}

	lines := strings.Split(content, "\n")

	// Aplicar highlight na linha selecionada
	// Header com BorderBottom gera 2 linhas (header + borda), dados a partir da linha 2
	count := m.itemCount()
	if count > 0 && m.selectedIndex >= 0 && m.selectedIndex < count {
		lineIdx := m.selectedIndex + 2
		if lineIdx < len(lines) {
			lines[lineIdx] = selectedStyle.Render(lines[lineIdx])
		}
	}

	return lines
}

// filteredPods aplica filtro de status client-side nos pods
func (m Model) filteredPods() []monitor.Pod {
	if m.filter.StatusFilter == "" {
		return m.pods
	}
	var filtered []monitor.Pod
	for _, p := range m.pods {
		if strings.EqualFold(p.Status, m.filter.StatusFilter) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// clampScroll garante que scrollY está dentro dos limites válidos
func (m *Model) clampScroll(totalLines int) {
	maxScroll := totalLines - m.visibleHeight()
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

// reconnectMsg sinaliza uma tentativa de reconexão
type reconnectMsg struct{}

// detailMsg carrega conteúdo para exibir no modo detalhe
type detailMsg struct {
	title   string
	content string
}

// actionResultMsg carrega o resultado de uma ação executada
type actionResultMsg struct {
	feedback string
	err      error
}

// maxRetries é o número máximo de tentativas de reconexão antes de exibir erro fatal
const maxRetries = 3

// selectedResourceInfo retorna nome, namespace e kind do recurso selecionado na aba ativa
func (m Model) selectedResourceInfo() (name, namespace, kind string) {
	idx := m.selectedIndex
	switch m.activeTab {
	case TabPods:
		pods := m.filteredPods()
		if idx >= 0 && idx < len(pods) {
			return pods[idx].Name, pods[idx].Namespace, "pod"
		}
	case TabDeployments:
		if idx >= 0 && idx < len(m.deployments) {
			return m.deployments[idx].Name, m.deployments[idx].Namespace, "deployment"
		}
	case TabServices:
		if idx >= 0 && idx < len(m.services) {
			return m.services[idx].Name, m.services[idx].Namespace, "service"
		}
	case TabNodes:
		if idx >= 0 && idx < len(m.nodes) {
			return m.nodes[idx].Name, "", "node"
		}
	case TabStatefulSets:
		if idx >= 0 && idx < len(m.statefulsets) {
			return m.statefulsets[idx].Name, m.statefulsets[idx].Namespace, "statefulset"
		}
	case TabJobs:
		if idx >= 0 && idx < len(m.jobs) {
			return m.jobs[idx].Name, m.jobs[idx].Namespace, "job"
		}
	case TabIngresses:
		if idx >= 0 && idx < len(m.ingresses) {
			return m.ingresses[idx].Name, m.ingresses[idx].Namespace, "ingress"
		}
	case TabConfigMaps:
		if idx >= 0 && idx < len(m.configmaps) {
			return m.configmaps[idx].Name, m.configmaps[idx].Namespace, "configmap"
		}
	case TabEvents:
		if idx >= 0 && idx < len(m.events) {
			return m.events[idx].Name, m.events[idx].Namespace, "event"
		}
	}
	return "", "", ""
}

// Update processa mensagens e atualiza o estado do modelo
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Limpar feedback após qualquer tecla
	if _, ok := msg.(tea.KeyMsg); ok && m.actionFeedback != "" {
		m.actionFeedback = ""
	}

	// No modo de ação, capturar teclas para confirmação/input
	if m.actionMode != ActionNone {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				mode := m.actionMode
				target := m.actionTarget
				buf := m.actionBuffer
				m.actionMode = ActionNone
				m.actionBuffer = ""
				return m, m.executeAction(mode, target, buf)
			case "esc":
				m.actionMode = ActionNone
				m.actionBuffer = ""
				m.actionTarget = ""
				return m, nil
			case "backspace":
				if len(m.actionBuffer) > 0 {
					m.actionBuffer = m.actionBuffer[:len(m.actionBuffer)-1]
				}
				return m, nil
			default:
				if len(keyMsg.String()) == 1 {
					m.actionBuffer += keyMsg.String()
				}
				return m, nil
			}
		}
		return m, nil
	}

	// No modo de detalhe, scroll independente e Esc para voltar
	if m.viewMode == ModeDetail {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "esc", "q":
				m.viewMode = ModeList
				m.detailContent = ""
				m.detailTitle = ""
				m.detailScrollY = 0
				return m, nil
			case "j", "down":
				m.detailScrollY++
				return m, nil
			case "k", "up":
				if m.detailScrollY > 0 {
					m.detailScrollY--
				}
				return m, nil
			case "pgdown":
				m.detailScrollY += m.visibleHeight()
				return m, nil
			case "pgup":
				m.detailScrollY -= m.visibleHeight()
				if m.detailScrollY < 0 {
					m.detailScrollY = 0
				}
				return m, nil
			case "home":
				m.detailScrollY = 0
				return m, nil
			}
		}
		// Tratar mensagens de dados mesmo no detail mode (para não bloquear o ticker)
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.width = msg.Width
			m.height = msg.Height
		case detailMsg:
			m.detailTitle = msg.title
			m.detailContent = msg.content
			m.detailScrollY = 0
			m.viewMode = ModeDetail
		}
		return m, nil
	}

	// No modo de input de filtro, capturar teclas para o buffer
	if m.filter.InputMode {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				m.filter.InputMode = false
				switch m.filter.InputField {
				case "namespace":
					m.filter.Namespace = m.filter.InputBuffer
				case "label":
					m.filter.LabelSelector = m.filter.InputBuffer
				case "status":
					m.filter.StatusFilter = m.filter.InputBuffer
				case "search":
					m.filter.SearchQuery = m.filter.InputBuffer
				}
				m.filter.InputBuffer = ""
				m.filter.InputField = ""
				m.scrollY = 0
				return m, m.fetchResources
			case "esc":
				m.filter.InputMode = false
				m.filter.InputBuffer = ""
				m.filter.InputField = ""
				return m, nil
			case "backspace":
				if len(m.filter.InputBuffer) > 0 {
					m.filter.InputBuffer = m.filter.InputBuffer[:len(m.filter.InputBuffer)-1]
				}
				return m, nil
			default:
				if len(keyMsg.String()) == 1 {
					m.filter.InputBuffer += keyMsg.String()
				}
				return m, nil
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		totalLines := len(m.contentLines())
		visibleH := m.visibleHeight()
		maxScroll := totalLines - visibleH
		if maxScroll < 0 {
			maxScroll = 0
		}
		count := m.itemCount()

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % tabCount
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + tabCount) % tabCount
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "1":
			m.activeTab = TabPods
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "2":
			m.activeTab = TabDeployments
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "3":
			m.activeTab = TabServices
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "4":
			m.activeTab = TabNodes
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "5":
			m.activeTab = TabStatefulSets
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "6":
			m.activeTab = TabJobs
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "7":
			m.activeTab = TabIngresses
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "8":
			m.activeTab = TabConfigMaps
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "9":
			m.activeTab = TabEvents
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		case "j", "down":
			if count > 0 && m.selectedIndex < count-1 {
				m.selectedIndex++
				// Scroll automático: header com borda = 2 linhas, dados a partir da linha 2
				itemLine := m.selectedIndex + 2
				if itemLine >= m.scrollY+visibleH {
					m.scrollY = itemLine - visibleH + 1
				}
			}
			return m, nil
		case "k", "up":
			if m.selectedIndex > 0 {
				m.selectedIndex--
				// Scroll automático: garantir que o item selecionado está visível
				itemLine := m.selectedIndex + 2
				if itemLine < m.scrollY {
					m.scrollY = itemLine
				}
			}
			return m, nil
		case "pgdown":
			m.scrollY += visibleH
			if m.scrollY > maxScroll {
				m.scrollY = maxScroll
			}
			return m, nil
		case "pgup":
			m.scrollY -= visibleH
			if m.scrollY < 0 {
				m.scrollY = 0
			}
			return m, nil
		case "home":
			m.scrollY = 0
			m.selectedIndex = 0
			return m, nil
		case "end":
			m.scrollY = maxScroll
			if count > 0 {
				m.selectedIndex = count - 1
			}
			return m, nil
		case "/":
			m.filter.InputMode = true
			m.filter.InputField = "namespace"
			m.filter.InputBuffer = ""
			return m, nil
		case "L":
			m.filter.InputMode = true
			m.filter.InputField = "label"
			m.filter.InputBuffer = ""
			return m, nil
		case "S":
			m.filter.InputMode = true
			m.filter.InputField = "status"
			m.filter.InputBuffer = ""
			return m, nil
		case "f":
			// Busca por nome (search)
			m.filter.InputMode = true
			m.filter.InputField = "search"
			m.filter.InputBuffer = ""
			return m, nil
		case "l":
			// Logs do pod selecionado
			name, ns, kind := m.selectedResourceInfo()
			if name == "" {
				return m, nil
			}
			if kind != "pod" {
				m.actionFeedback = "Logs disponível apenas para Pods"
				return m, nil
			}
			return m, m.fetchPodLogs(name, ns)
		case "y":
			// YAML do recurso selecionado
			name, ns, kind := m.selectedResourceInfo()
			if name != "" {
				return m, m.fetchResourceYAML(kind, name, ns)
			}
			return m, nil
		case "e":
			// Eventos do recurso selecionado
			name, ns, _ := m.selectedResourceInfo()
			if name != "" {
				return m, m.fetchResourceEvents(name, ns)
			}
			return m, nil
		case "d":
			// Deletar recurso selecionado
			name, _, kind := m.selectedResourceInfo()
			if name == "" {
				return m, nil
			}
			if kind == "node" || kind == "event" {
				m.actionFeedback = "Não é possível deletar " + kind
				return m, nil
			}
			m.actionMode = ActionConfirmDelete
			m.actionTarget = name
			m.actionBuffer = ""
			return m, nil
		case "s":
			// Escalar deployment/statefulset selecionado
			name, _, kind := m.selectedResourceInfo()
			if name == "" {
				return m, nil
			}
			if kind != "deployment" && kind != "statefulset" {
				m.actionFeedback = "Scale disponível apenas para Deployments e StatefulSets"
				return m, nil
			}
			m.actionMode = ActionInputScale
			m.actionTarget = name
			m.actionBuffer = ""
			return m, nil
		case "r":
			// Reiniciar deployment/statefulset selecionado
			name, _, kind := m.selectedResourceInfo()
			if name == "" {
				return m, nil
			}
			if kind != "deployment" && kind != "statefulset" {
				m.actionFeedback = "Restart disponível apenas para Deployments e StatefulSets"
				return m, nil
			}
			m.actionMode = ActionConfirmRestart
			m.actionTarget = name
			m.actionBuffer = ""
			return m, nil
		case "esc":
			// Limpar todos os filtros
			m.filter = FilterState{}
			m.scrollY = 0
			m.selectedIndex = 0
			return m, m.fetchResources
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, m.fetchResources

	case reconnectMsg:
		return m, m.fetchResources

	case detailMsg:
		m.detailTitle = msg.title
		m.detailContent = msg.content
		m.detailScrollY = 0
		m.viewMode = ModeDetail
		return m, nil

	case actionResultMsg:
		if msg.err != nil {
			m.actionFeedback = fmt.Sprintf("Erro: %v", msg.err)
		} else {
			m.actionFeedback = msg.feedback
		}
		return m, m.fetchResources

	// Mensagens de dados por tipo de recurso
	case []monitor.Pod:
		m.pods = msg
		m.reconnecting = false
		m.retryCount = 0
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.Deployment:
		m.deployments = msg
		m.reconnecting = false
		m.retryCount = 0
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.Service:
		m.services = msg
		m.reconnecting = false
		m.retryCount = 0
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.Node:
		m.nodes = msg
		m.reconnecting = false
		m.retryCount = 0
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.StatefulSet:
		m.statefulsets = msg
		m.reconnecting = false
		m.retryCount = 0
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.Job:
		m.jobs = msg
		m.reconnecting = false
		m.retryCount = 0
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.Ingress:
		m.ingresses = msg
		m.reconnecting = false
		m.retryCount = 0
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.ConfigMap:
		m.configmaps = msg
		m.reconnecting = false
		m.retryCount = 0
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.Event:
		m.events = msg
		m.reconnecting = false
		m.retryCount = 0
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})

	case error:
		hasData := len(m.pods) > 0 || len(m.deployments) > 0 || len(m.services) > 0 || len(m.nodes) > 0 ||
			len(m.statefulsets) > 0 || len(m.jobs) > 0 || len(m.ingresses) > 0 || len(m.configmaps) > 0 || len(m.events) > 0
		if !hasData {
			// Sem dados prévios: exibir erro imediatamente
			m.err = msg
		} else {
			// Com dados prévios: tentar reconectar preservando o último estado
			m.retryCount++
			if m.retryCount >= maxRetries {
				m.err = msg
				m.reconnecting = false
			} else {
				m.reconnecting = true
				delay := time.Duration(1<<uint(m.retryCount-1)) * time.Second
				return m, tea.Tick(delay, func(t time.Time) tea.Msg {
					return reconnectMsg{}
				})
			}
		}
	}

	return m, nil
}

// resolveK8sClient extrai o *K8sClient do client (direto ou dentro de RetryClient)
func resolveK8sClient(c monitor.Client) *monitor.K8sClient {
	if k, ok := c.(*monitor.K8sClient); ok {
		return k
	}
	if rc, ok := c.(*monitor.RetryClient); ok {
		if k, ok2 := rc.Inner().(*monitor.K8sClient); ok2 {
			return k
		}
	}
	return nil
}

// fetchPodLogs busca logs do pod selecionado de forma assíncrona
func (m Model) fetchPodLogs(podName, namespace string) tea.Cmd {
	return func() tea.Msg {
		k8sClient := resolveK8sClient(m.client)
		if k8sClient == nil {
			return detailMsg{title: "Logs: " + podName, content: "(logs não disponíveis: client não suporta acesso direto)"}
		}
		logs, err := monitor.GetPodLogs(k8sClient.Clientset(), namespace, podName, 100)
		if err != nil {
			return detailMsg{title: "Logs: " + podName, content: fmt.Sprintf("Erro ao buscar logs: %v", err)}
		}
		return detailMsg{title: "Logs: " + podName, content: logs}
	}
}

// fetchResourceYAML busca o YAML do recurso selecionado de forma assíncrona
func (m Model) fetchResourceYAML(kind, name, namespace string) tea.Cmd {
	return func() tea.Msg {
		k8sClient := resolveK8sClient(m.client)
		if k8sClient == nil {
			return detailMsg{title: "YAML: " + name, content: "(YAML não disponível: client não suporta acesso direto)"}
		}
		yaml, err := monitor.GetResourceYAML(k8sClient.RESTConfig(), kind, name, namespace)
		if err != nil {
			return detailMsg{title: "YAML: " + name, content: fmt.Sprintf("Erro ao buscar YAML: %v", err)}
		}
		return detailMsg{title: "YAML: " + name, content: yaml}
	}
}

// fetchResourceEvents busca eventos filtrados pelo recurso selecionado
func (m Model) fetchResourceEvents(resourceName, namespace string) tea.Cmd {
	return func() tea.Msg {
		events, err := m.client.GetEvents(monitor.ListFilter{Namespace: namespace})
		if err != nil {
			return detailMsg{title: "Eventos: " + resourceName, content: fmt.Sprintf("Erro ao buscar eventos: %v", err)}
		}
		var sb strings.Builder
		found := false
		for _, e := range events {
			if e.Name == resourceName {
				found = true
				sb.WriteString(fmt.Sprintf("[%s] %s - %s: %s\n", e.Age, e.Type, e.Reason, e.Message))
			}
		}
		if !found {
			sb.WriteString("Nenhum evento encontrado para este recurso.")
		}
		return detailMsg{title: "Eventos: " + resourceName, content: sb.String()}
	}
}

// executeAction executa a ação confirmada pelo usuário
func (m Model) executeAction(mode ActionMode, target, input string) tea.Cmd {
	return func() tea.Msg {
		k8sClient := resolveK8sClient(m.client)
		if k8sClient == nil {
			return actionResultMsg{err: fmt.Errorf("client não suporta ações diretas")}
		}

		_, ns, kind := m.selectedResourceInfo()
		clientset := k8sClient.Clientset()

		switch mode {
		case ActionConfirmDelete:
			if input != "yes" {
				return actionResultMsg{feedback: "Ação cancelada."}
			}
			err := monitor.DeleteResource(clientset, kind, target, ns)
			if err != nil {
				return actionResultMsg{err: err}
			}
			return actionResultMsg{feedback: fmt.Sprintf("%s '%s' deletado.", kind, target)}

		case ActionInputScale:
			var replicas int32
			if _, err := fmt.Sscanf(input, "%d", &replicas); err != nil {
				return actionResultMsg{err: fmt.Errorf("valor inválido para réplicas: %s", input)}
			}
			err := monitor.ScaleDeployment(clientset, target, ns, replicas)
			if err != nil {
				return actionResultMsg{err: err}
			}
			return actionResultMsg{feedback: fmt.Sprintf("Deployment '%s' escalado para %d réplicas.", target, replicas)}

		case ActionConfirmRestart:
			if input != "yes" {
				return actionResultMsg{feedback: "Ação cancelada."}
			}
			err := monitor.RestartDeployment(clientset, target, ns)
			if err != nil {
				return actionResultMsg{err: err}
			}
			return actionResultMsg{feedback: fmt.Sprintf("Deployment '%s' reiniciado.", target)}
		}

		return actionResultMsg{feedback: "Ação desconhecida."}
	}
}

// View renderiza a interface do Viz
func (m Model) View() string {
	// Modo detalhe: renderizar view dedicada
	if m.viewMode == ModeDetail {
		return renderDetailView(m.detailTitle, m.detailContent, m.detailScrollY, m.visibleHeight())
	}

	var sb strings.Builder

	sb.WriteString(titleStyle.Render("🌿 Yby Viz") + "  " + subtitleStyle.Render("Cluster Monitor") + "\n")
	sb.WriteString(separatorStyle.Render(strings.Repeat("─", 80)) + "\n")
	sb.WriteString(renderTabBar(m.activeTab))

	if m.err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf(
			"\n❌ Erro de conexão K8s: %v\nVerifique se o cluster está rodando e ~/.kube/config existe.",
			m.err)))
		sb.WriteString("\n\nPressione 'q' para sair.")
		return sb.String()
	}

	// Modo de ação: exibir prompt sobre o conteúdo
	if m.actionMode != ActionNone {
		// Renderizar tabela normalmente
		lines := m.contentLines()
		totalLines := len(lines)
		m.clampScroll(totalLines)
		visibleLines := lines[m.scrollY:]
		maxVisible := m.visibleHeight()
		if maxVisible > 0 && len(visibleLines) > maxVisible {
			visibleLines = visibleLines[:maxVisible]
		}
		sb.WriteString(strings.Join(visibleLines, "\n"))
		sb.WriteString("\n\n")
		sb.WriteString(renderActionPrompt(m.actionMode, m.actionTarget, m.actionBuffer))
		return sb.String()
	}

	// Renderizar tabela do recurso ativo
	lines := m.contentLines()
	totalLines := len(lines)

	// Clampar scroll
	m.clampScroll(totalLines)
	scrollY := m.scrollY

	visibleLines := lines[scrollY:]
	maxVisible := m.visibleHeight()
	if maxVisible > 0 && len(visibleLines) > maxVisible {
		visibleLines = visibleLines[:maxVisible]
	}
	sb.WriteString(strings.Join(visibleLines, "\n"))

	// Montar status bar
	var statusParts []string

	// Indicador de posição
	if totalLines > 0 {
		statusParts = append(statusParts, fmt.Sprintf("Linha %d/%d", scrollY+1, totalLines))
	}

	// Filtros ativos
	if m.filter.Namespace != "" {
		statusParts = append(statusParts, filterStyle.Render(fmt.Sprintf("[ns:%s]", m.filter.Namespace)))
	}
	if m.filter.LabelSelector != "" {
		statusParts = append(statusParts, filterStyle.Render(fmt.Sprintf("[label:%s]", m.filter.LabelSelector)))
	}
	if m.filter.StatusFilter != "" {
		statusParts = append(statusParts, filterStyle.Render(fmt.Sprintf("[status:%s]", m.filter.StatusFilter)))
	}
	if m.filter.SearchQuery != "" {
		statusParts = append(statusParts, filterStyle.Render(fmt.Sprintf("[busca:%s]", m.filter.SearchQuery)))
	}

	// Feedback de ação
	if m.actionFeedback != "" {
		statusParts = append(statusParts, renderFeedback(m.actionFeedback))
	}

	// Indicador de reconexão
	if m.reconnecting {
		statusParts = append(statusParts, reconnectingStyle.Render("Reconectando..."))
	}

	// Modo de input de filtro
	if m.filter.InputMode {
		statusParts = append(statusParts, filterStyle.Render(fmt.Sprintf("Filtro %s: %s_", m.filter.InputField, m.filter.InputBuffer)))
	}

	// Atalhos base (disponíveis em todas as abas)
	statusLine := " " +
		keyStyle.Render("Tab/1-9") + descStyle.Render(" navegar") + "  " +
		keyStyle.Render("j/k") + descStyle.Render(" selecionar") + "  " +
		keyStyle.Render("f") + descStyle.Render(" buscar") + "  " +
		keyStyle.Render("y") + descStyle.Render(" yaml") + "  " +
		keyStyle.Render("e") + descStyle.Render(" eventos")

	// Atalhos contextuais por aba
	switch m.activeTab {
	case TabPods:
		statusLine += "  " + keyStyle.Render("l") + descStyle.Render(" logs") +
			"  " + keyStyle.Render("d") + descStyle.Render(" deletar")
	case TabDeployments, TabStatefulSets:
		statusLine += "  " + keyStyle.Render("d") + descStyle.Render(" deletar") +
			"  " + keyStyle.Render("s") + descStyle.Render(" escalar") +
			"  " + keyStyle.Render("r") + descStyle.Render(" restart")
	case TabJobs:
		statusLine += "  " + keyStyle.Render("d") + descStyle.Render(" deletar")
	}

	statusLine += "  " + keyStyle.Render("q") + descStyle.Render(" sair")
	if len(statusParts) > 0 {
		statusLine = strings.Join(statusParts, " ") + " | " + statusLine
	}
	sb.WriteString(statusBarStyle.Render("\n" + statusLine))
	return sb.String()
}

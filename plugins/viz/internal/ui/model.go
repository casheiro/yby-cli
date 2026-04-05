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
	client       monitor.Client
	activeTab    ResourceTab
	pods         []monitor.Pod
	deployments  []monitor.Deployment
	services     []monitor.Service
	nodes        []monitor.Node
	err          error
	width        int
	height       int
	scrollY      int
	reconnecting bool
	retryCount   int
	filter       FilterState
}

// FilterState mantém o estado dos filtros de busca
type FilterState struct {
	Namespace     string
	LabelSelector string
	StatusFilter  string
	InputMode     bool
	InputBuffer   string
	InputField    string // "namespace", "label", "status"
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

// contentLines retorna o conteúdo renderizado da aba ativa
func (m Model) contentLines() []string {
	var content string
	switch m.activeTab {
	case TabPods:
		pods := m.filteredPods()
		if len(pods) == 0 {
			content = "  Carregando pods...\n"
		} else {
			content = renderPodTable(pods)
		}
	case TabDeployments:
		if len(m.deployments) == 0 {
			content = "  Carregando deployments...\n"
		} else {
			content = renderDeploymentTable(m.deployments)
		}
	case TabServices:
		if len(m.services) == 0 {
			content = "  Carregando services...\n"
		} else {
			content = renderServiceTable(m.services)
		}
	case TabNodes:
		if len(m.nodes) == 0 {
			content = "  Carregando nodes...\n"
		} else {
			content = renderNodeTable(m.nodes)
		}
	}
	return strings.Split(content, "\n")
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

// maxRetries é o número máximo de tentativas de reconexão antes de exibir erro fatal
const maxRetries = 3

// Update processa mensagens e atualiza o estado do modelo
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % tabCount
			m.scrollY = 0
			return m, m.fetchResources
		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + tabCount) % tabCount
			m.scrollY = 0
			return m, m.fetchResources
		case "1":
			m.activeTab = TabPods
			m.scrollY = 0
			return m, m.fetchResources
		case "2":
			m.activeTab = TabDeployments
			m.scrollY = 0
			return m, m.fetchResources
		case "3":
			m.activeTab = TabServices
			m.scrollY = 0
			return m, m.fetchResources
		case "4":
			m.activeTab = TabNodes
			m.scrollY = 0
			return m, m.fetchResources
		case "j", "down":
			m.scrollY++
			if m.scrollY > maxScroll {
				m.scrollY = maxScroll
			}
			return m, nil
		case "k", "up":
			if m.scrollY > 0 {
				m.scrollY--
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
			return m, nil
		case "end":
			m.scrollY = maxScroll
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
		case "esc":
			// Limpar todos os filtros
			m.filter = FilterState{}
			m.scrollY = 0
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

	case error:
		hasData := len(m.pods) > 0 || len(m.deployments) > 0 || len(m.services) > 0 || len(m.nodes) > 0
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

// View renderiza a interface do Viz
func (m Model) View() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Yby Viz - Cluster Monitor") + "\n")
	sb.WriteString(renderTabBar(m.activeTab))

	if m.err != nil {
		sb.WriteString(errorStyle.Render(fmt.Sprintf(
			"\n❌ Erro de conexão K8s: %v\nVerifique se o cluster está rodando e ~/.kube/config existe.",
			m.err)))
		sb.WriteString("\n\nPressione 'q' para sair.")
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

	// Indicador de reconexão
	if m.reconnecting {
		statusParts = append(statusParts, reconnectingStyle.Render("Reconectando..."))
	}

	// Modo de input de filtro
	if m.filter.InputMode {
		statusParts = append(statusParts, filterStyle.Render(fmt.Sprintf("Filtro %s: %s_", m.filter.InputField, m.filter.InputBuffer)))
	}

	statusLine := " Tab/1-4: navegar | j/k: scroll | /: filtro ns | L: label | S: status | q: sair"
	if len(statusParts) > 0 {
		statusLine = strings.Join(statusParts, " ") + " | " + statusLine
	}
	sb.WriteString(statusBarStyle.Render("\n" + statusLine))
	return sb.String()
}

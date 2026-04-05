package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
)

type tickMsg time.Time

// Model é o modelo principal do Bubbletea para o Viz
type Model struct {
	client      monitor.Client
	activeTab   ResourceTab
	pods        []monitor.Pod
	deployments []monitor.Deployment
	services    []monitor.Service
	nodes       []monitor.Node
	err         error
	width       int
	height      int
	scrollY     int
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
	switch m.activeTab {
	case TabPods:
		data, err := m.client.GetPods()
		if err != nil {
			return err
		}
		return data
	case TabDeployments:
		data, err := m.client.GetDeployments()
		if err != nil {
			return err
		}
		return data
	case TabServices:
		data, err := m.client.GetServices()
		if err != nil {
			return err
		}
		return data
	case TabNodes:
		data, err := m.client.GetNodes()
		if err != nil {
			return err
		}
		return data
	}
	return nil
}

// Update processa mensagens e atualiza o estado do modelo
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
			return m, nil
		case "k", "up":
			if m.scrollY > 0 {
				m.scrollY--
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, m.fetchResources

	// Mensagens de dados por tipo de recurso
	case []monitor.Pod:
		m.pods = msg
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.Deployment:
		m.deployments = msg
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.Service:
		m.services = msg
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.Node:
		m.nodes = msg
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})

	case error:
		m.err = msg
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
	var content string
	switch m.activeTab {
	case TabPods:
		if len(m.pods) == 0 {
			content = "  Carregando pods...\n"
		} else {
			content = renderPodTable(m.pods)
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

	// Aplicar scroll
	lines := strings.Split(content, "\n")
	scrollY := m.scrollY
	if scrollY >= len(lines) {
		scrollY = len(lines) - 1
	}
	if scrollY < 0 {
		scrollY = 0
	}
	visibleLines := lines[scrollY:]
	// Limitar ao espaço disponível (reservar 5 linhas para título, tabs, status bar)
	maxVisible := m.height - 5
	if maxVisible > 0 && len(visibleLines) > maxVisible {
		visibleLines = visibleLines[:maxVisible]
	}
	sb.WriteString(strings.Join(visibleLines, "\n"))

	sb.WriteString(statusBarStyle.Render("\n Tab/1-4: navegar | j/k: scroll | q: sair"))
	return sb.String()
}

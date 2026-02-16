package ui

import (
	"fmt"
	"time"

	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

type Model struct {
	client monitor.Client
	pods   []monitor.Pod
	err    error
}

func NewModel() Model {
	client, err := monitor.NewK8sClient()

	// Convert concrete to interface safely
	var mClient monitor.Client
	if err == nil {
		mClient = client
	}

	return Model{
		client: mClient,
		pods:   []monitor.Pod{},
		err:    err,
	}
}

func (m Model) Init() tea.Cmd {
	if m.err != nil {
		return nil
	}
	return tea.Batch(
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
		m.fetchPods,
	)
}

func (m Model) fetchPods() tea.Msg {
	if m.client == nil {
		return nil
	}
	pods, err := m.client.GetPods()
	if err != nil {
		return err
	}
	return pods
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tickMsg:
		return m, func() tea.Msg {
			// Loop refresh
			time.Sleep(2 * time.Second)
			return m.fetchPods()
		}
	case []monitor.Pod:
		m.pods = msg
		// Schedule next tick
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case error:
		m.err = msg
	}
	return m, nil
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).MarginBottom(1)
	runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#43BF6D"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F05D5E"))

	s := titleStyle.Render("Yby Viz - Cluster Monitor (Real K8s)") + "\n"

	if m.err != nil {
		s += errorStyle.Render(fmt.Sprintf("\n❌ Erro de conexão K8s: %v\nVerifique se o cluster está rodando e ~/.kube/config existe.", m.err))
		s += "\n\nPressione 'q' para sair."
		return s
	}

	if len(m.pods) == 0 {
		s += "Carregando pods...\n"
	}

	for _, pod := range m.pods {
		var statusStyle lipgloss.Style
		icon := "●"
		if pod.Status == "Running" || pod.Status == "Executando" {
			statusStyle = runningStyle
		} else {
			statusStyle = errorStyle
			icon = "✖"
		}

		s += fmt.Sprintf("%s %-30s %-15s %s\n",
			statusStyle.Render(icon),
			pod.Name,
			statusStyle.Render(pod.Status),
			pod.Namespace,
		)
	}

	s += "\nPressione 'q' para sair."
	return s
}

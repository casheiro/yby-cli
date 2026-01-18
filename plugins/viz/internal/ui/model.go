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
	return Model{
		client: monitor.NewMockClient(),
		pods:   []monitor.Pod{},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		}),
		m.fetchPods,
	)
}

func (m Model) fetchPods() tea.Msg {
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
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return tickMsg(t)
		})
	case []monitor.Pod:
		m.pods = msg
	case error:
		m.err = msg
	}
	return m, nil
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).MarginBottom(1)
	runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#43BF6D"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F05D5E"))

	s := titleStyle.Render("Yby Viz - Cluster Monitor (Mock)") + "\n"

	if m.err != nil {
		s += errorStyle.Render(fmt.Sprintf("\nError: %v", m.err))
		return s
	}

	for _, pod := range m.pods {
		var statusStyle lipgloss.Style
		icon := "●"
		if pod.Status == "Executando" {
			statusStyle = runningStyle
		} else {
			statusStyle = errorStyle
			icon = "✖"
		}

		s += fmt.Sprintf("%s %-20s %s (%s)\n",
			statusStyle.Render(icon),
			pod.Name,
			statusStyle.Render(pod.Status),
			pod.CPU,
		)
	}

	s += "\nPressione 'q' para sair."
	return s
}

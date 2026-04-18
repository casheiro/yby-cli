/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	gocontext "context"
	"fmt"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/cloud"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var cloudDashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Short:   "Dashboard interativo de clusters cloud",
	Example: `  yby cloud dashboard`,
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &shared.RealRunner{}
		m := newDashboardModel(runner)
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	cloudCmd.AddCommand(cloudDashboardCmd)
}

// clusterEntry representa um cluster na tabela do dashboard.
type clusterEntry struct {
	Provider    string
	Name        string
	Region      string
	Version     string
	Status      string
	TokenStatus string // "valid", "expired", "unknown"
	TokenExpiry string
}

// dashboardModel é o modelo Bubbletea do dashboard cloud.
type dashboardModel struct {
	runner     shared.Runner
	clusters   []clusterEntry
	cursor     int
	loading    bool
	err        error
	width      int
	height     int
	lastUpdate time.Time
	detail     bool // mostra detalhes do cluster selecionado
}

// mensagens assíncronas
type clustersLoadedMsg struct {
	clusters []clusterEntry
	err      error
}

type refreshDoneMsg struct {
	index   int
	success bool
	err     error
}

type tickMsg time.Time

func newDashboardModel(runner shared.Runner) dashboardModel {
	return dashboardModel{
		runner:  runner,
		loading: true,
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(
		loadClusters(m.runner),
		tickEvery(30*time.Second),
	)
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Voltar da tela de detalhes
		if m.detail {
			switch msg.String() {
			case "q", "esc", "enter", "backspace":
				m.detail = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.clusters)-1 {
				m.cursor++
			}
		case "r":
			if len(m.clusters) > 0 && m.cursor < len(m.clusters) {
				m.loading = true
				return m, refreshToken(m.runner, m.clusters[m.cursor], m.cursor)
			}
		case "R":
			m.loading = true
			return m, loadClusters(m.runner)
		case "enter":
			if len(m.clusters) > 0 {
				m.detail = true
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case clustersLoadedMsg:
		m.loading = false
		m.lastUpdate = time.Now()
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.clusters = msg.clusters
			m.err = nil
			if m.cursor >= len(m.clusters) && len(m.clusters) > 0 {
				m.cursor = len(m.clusters) - 1
			}
		}

	case refreshDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else if msg.success && msg.index < len(m.clusters) {
			m.clusters[msg.index].TokenStatus = "valid"
			m.clusters[msg.index].TokenExpiry = "renovado"
			m.err = nil
		}

	case tickMsg:
		return m, tea.Batch(
			loadClusters(m.runner),
			tickEvery(30*time.Second),
		)
	}

	return m, nil
}

func (m dashboardModel) View() string {
	if m.width == 0 {
		return "Carregando..."
	}

	if m.detail && len(m.clusters) > 0 {
		return m.viewDetail()
	}

	var b strings.Builder

	// Cabeçalho
	header := dashboardTitleStyle.Render("  Yby Cloud Dashboard")
	b.WriteString(header)
	b.WriteString("\n")

	// Status de carregamento e última atualização
	var statusLine string
	if m.loading {
		statusLine = grayStyle.Render("  Carregando...")
	} else if !m.lastUpdate.IsZero() {
		statusLine = grayStyle.Render(fmt.Sprintf("  Atualizado: %s", m.lastUpdate.Format("15:04:05")))
	}
	b.WriteString(statusLine)
	b.WriteString("\n\n")

	// Erro
	if m.err != nil {
		b.WriteString(crossStyle.Render(""))
		b.WriteString(lipgloss.NewStyle().Foreground(errorColor).Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	// Tabela
	if len(m.clusters) == 0 && !m.loading {
		b.WriteString(grayStyle.Render("  Nenhum cluster encontrado."))
		b.WriteString("\n")
		b.WriteString(grayStyle.Render("  Verifique se os CLIs cloud estao instalados e autenticados."))
		b.WriteString("\n")
	} else {
		// Cabeçalho da tabela
		headerRow := fmt.Sprintf("  %-3s %-10s %-20s %-15s %-10s %-8s %-5s",
			"", "Provider", "Cluster", "Regiao", "Versao", "Status", "Token")
		b.WriteString(dashboardHeaderStyle.Render(headerRow))
		b.WriteString("\n")

		// Linhas
		for i, c := range m.clusters {
			cursor := "  "
			rowStyle := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = "> "
				rowStyle = rowStyle.Bold(true).Foreground(primaryColor)
			}

			tokenIcon := tokenIndicator(c.TokenStatus)

			row := fmt.Sprintf("%s %-10s %-20s %-15s %-10s %-8s %s",
				cursor, c.Provider, truncate(c.Name, 20), truncate(c.Region, 15),
				truncate(c.Version, 10), truncate(c.Status, 8), tokenIcon)
			b.WriteString(rowStyle.Render(row))
			b.WriteString("\n")
		}
	}

	// Preencher espaço restante
	lines := strings.Count(b.String(), "\n")
	remaining := m.height - lines - 3
	for i := 0; i < remaining; i++ {
		b.WriteString("\n")
	}

	// Barra de ajuda
	help := grayStyle.Render("  r: refresh token | R: recarregar | enter: detalhes | j/k: navegar | q: sair")
	b.WriteString(help)
	b.WriteString("\n")

	return b.String()
}

// viewDetail renderiza a tela de detalhes do cluster selecionado.
func (m dashboardModel) viewDetail() string {
	c := m.clusters[m.cursor]
	var b strings.Builder

	header := dashboardTitleStyle.Render(fmt.Sprintf("  Detalhes: %s", c.Name))
	b.WriteString(header)
	b.WriteString("\n\n")

	detailLabel := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	detailValue := lipgloss.NewStyle()

	details := []struct{ k, v string }{
		{"Provider", c.Provider},
		{"Nome", c.Name},
		{"Regiao", c.Region},
		{"Versao", c.Version},
		{"Status", c.Status},
		{"Token", tokenIndicator(c.TokenStatus) + " " + c.TokenExpiry},
	}

	for _, d := range details {
		b.WriteString(fmt.Sprintf("  %s %s\n",
			detailLabel.Render(d.k+":"),
			detailValue.Render(d.v),
		))
	}

	b.WriteString("\n")
	b.WriteString(grayStyle.Render("  Pressione esc/enter/q para voltar"))
	b.WriteString("\n")

	return b.String()
}

// --- Estilos do dashboard ---

var (
	dashboardTitleStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				Padding(1, 0, 0, 0)

	dashboardHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true)
)

// --- Funções auxiliares ---

func tokenIndicator(status string) string {
	switch status {
	case "valid":
		return lipgloss.NewStyle().Foreground(secondaryColor).Render("[OK]")
	case "expired":
		return lipgloss.NewStyle().Foreground(errorColor).Render("[EXP]")
	default:
		return lipgloss.NewStyle().Foreground(warningColor).Render("[?]")
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// --- Comandos assíncronos ---

func loadClusters(runner shared.Runner) tea.Cmd {
	return func() tea.Msg {
		ctx := gocontext.Background()
		providers := cloud.Detect(ctx, runner)
		var entries []clusterEntry

		for _, p := range providers {
			clusters, err := p.ListClusters(ctx, cloud.ListOptions{})
			if err != nil {
				continue
			}

			cred, _ := p.ValidateCredentials(ctx)

			for _, c := range clusters {
				entry := clusterEntry{
					Provider: p.Name(),
					Name:     c.Name,
					Region:   c.Region,
					Version:  c.Version,
					Status:   c.Status,
				}

				if cred != nil && cred.ExpiresAt != nil {
					if cred.ExpiresAt.Before(time.Now()) {
						entry.TokenStatus = "expired"
						entry.TokenExpiry = "expirado"
					} else {
						entry.TokenStatus = "valid"
						entry.TokenExpiry = formatDuration(time.Until(*cred.ExpiresAt)) + " restante"
					}
				} else {
					entry.TokenStatus = "unknown"
				}

				entries = append(entries, entry)
			}
		}

		return clustersLoadedMsg{clusters: entries}
	}
}

func refreshToken(runner shared.Runner, entry clusterEntry, index int) tea.Cmd {
	return func() tea.Msg {
		ctx := gocontext.Background()
		p := cloud.GetProvider(runner, entry.Provider)
		if p == nil {
			return refreshDoneMsg{index: index, err: fmt.Errorf("provider %s nao encontrado", entry.Provider)}
		}

		err := p.RefreshToken(ctx, cloud.ClusterInfo{
			Name:   entry.Name,
			Region: entry.Region,
		})
		if err != nil {
			return refreshDoneMsg{index: index, err: err}
		}

		return refreshDoneMsg{index: index, success: true}
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

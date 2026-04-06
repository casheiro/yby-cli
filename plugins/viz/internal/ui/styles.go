package ui

import "github.com/charmbracelet/lipgloss"

var (
	// titleStyle estiliza o título principal com gradiente visual
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			PaddingLeft(1)

	// subtitleStyle estiliza o subtítulo/contexto do cluster
	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			PaddingLeft(1)

	// activeTabStyle estiliza a aba atualmente selecionada
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	// inactiveTabStyle estiliza as abas não selecionadas
	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Padding(0, 1)

	// runningStyle estiliza recursos em estado saudável
	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#43BF6D"))

	// warningStyle estiliza recursos em estado de aviso
	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700"))

	// errorStyle estiliza recursos em estado de erro
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F05D5E"))

	// completedStyle estiliza recursos completados
	completedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	// headerStyle estiliza os cabeçalhos das tabelas
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#444444"))

	// statusBarStyle estiliza a barra de status inferior
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			MarginTop(1).
			PaddingLeft(1)

	// filterStyle estiliza indicadores de filtro ativo na status bar
	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true)

	// reconnectingStyle estiliza o indicador de reconexão
	reconnectingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD700")).
				Bold(true)

	// selectedStyle estiliza a linha selecionada com destaque forte
	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#4A3A8A")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

	// keyStyle estiliza teclas de atalho na status bar
	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	// descStyle estiliza descrições de atalho na status bar
	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	// countStyle estiliza contadores (totais de recursos)
	countStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	// separatorStyle estiliza separadores visuais
	separatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444444"))

	// namespaceStyle estiliza nomes de namespace
	namespaceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C8FD6"))
)

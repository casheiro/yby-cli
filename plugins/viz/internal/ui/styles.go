package ui

import "github.com/charmbracelet/lipgloss"

var (
	// titleStyle estiliza o título principal da aplicação
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	// activeTabStyle estiliza a aba atualmente selecionada
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Background(lipgloss.Color("#2D2D2D")).
			Padding(0, 2)

	// inactiveTabStyle estiliza as abas não selecionadas
	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#666666")).
				Padding(0, 2)

	// runningStyle estiliza recursos em estado saudável
	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#43BF6D"))

	// errorStyle estiliza recursos em estado de erro
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F05D5E"))

	// headerStyle estiliza os cabeçalhos das tabelas
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA"))

	// statusBarStyle estiliza a barra de status inferior
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			MarginTop(1)

	// filterStyle estiliza indicadores de filtro ativo na status bar
	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	// reconnectingStyle estiliza o indicador de reconexão
	reconnectingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFD700")).
				Bold(true)
)

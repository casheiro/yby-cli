package tui

import "github.com/charmbracelet/lipgloss"

var (
	// userStyle estiliza mensagens do usuário
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00CED1")).
			Bold(true)

	// bardStyle estiliza mensagens do Bard
	bardStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DA70D6"))

	// errorMsgStyle estiliza mensagens de erro
	errorMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F05D5E"))

	// toolStyle estiliza indicadores de ferramentas
	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700"))

	// statusBarBg define o estilo da barra de status
	statusBarBg = lipgloss.NewStyle().
			Background(lipgloss.Color("#333333")).
			Foreground(lipgloss.Color("#AAAAAA"))

	// statusKeyStyle estiliza teclas na status bar
	statusKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	// statusValueStyle estiliza valores na status bar
	statusValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CCCCCC"))

	// promptStyle estiliza o indicador de prompt
	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00CED1")).
			Bold(true)

	// thinkingStyle estiliza o indicador de processamento
	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Italic(true)

	// headerStyle estiliza o cabeçalho da TUI
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DA70D6")).
			Bold(true).
			PaddingLeft(1)

	// dimStyle estiliza texto discreto
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))
)

package cmd

import "github.com/charmbracelet/lipgloss"

// Common Styles
var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4") // Purple
	secondaryColor = lipgloss.Color("#04B575") // Green
	warningColor   = lipgloss.Color("#FFB000") // Amber
	errorColor     = lipgloss.Color("#FF3B30") // Red
	grayColor      = lipgloss.Color("#6E6E6E")

	// Text Styles
	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(grayColor).
			MarginTop(1).
			MarginBottom(1)

	stepStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	checkStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			SetString("✅ ")

	crossStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			SetString("❌ ")

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			SetString("⚠️ ")

	itemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	grayStyle = lipgloss.NewStyle().
			Foreground(grayColor)
)

func icon(ok bool) string {
	if ok {
		return checkStyle.String()
	}
	return crossStyle.String()
}

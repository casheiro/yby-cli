package ui

import (
	"fmt"
	"strings"
)

// ViewMode controla se a tela mostra lista ou detalhe
type ViewMode int

const (
	// ModeList é o modo padrão de listagem de recursos
	ModeList ViewMode = iota
	// ModeDetail exibe conteúdo detalhado (logs, YAML, eventos)
	ModeDetail
)

// renderDetailView renderiza a view de detalhe com scroll independente
func renderDetailView(title, content string, scrollY, height int) string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render(title) + "\n")
	sb.WriteString(strings.Repeat("─", 60) + "\n")

	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Ajustar scroll dentro dos limites
	maxScroll := totalLines - height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollY > maxScroll {
		scrollY = maxScroll
	}
	if scrollY < 0 {
		scrollY = 0
	}

	// Aplicar scroll
	visible := lines[scrollY:]
	if height > 0 && len(visible) > height {
		visible = visible[:height]
	}

	sb.WriteString(strings.Join(visible, "\n"))
	sb.WriteString("\n")

	// Status bar do detalhe
	statusLine := fmt.Sprintf("Linha %d/%d | Esc: voltar | j/k: scroll | PgUp/PgDn: página", scrollY+1, totalLines)
	sb.WriteString(statusBarStyle.Render(statusLine))

	return sb.String()
}

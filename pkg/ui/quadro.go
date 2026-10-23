package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// VarianteQuadro escolhe a cor do fio do Quadro.
type VarianteQuadro int

const (
	// QuadroNormal usa o fio em Fio.
	QuadroNormal VarianteQuadro = iota
	// QuadroFoco usa o fio em Tinta (painel com foco).
	QuadroFoco
	// QuadroDestrutivo usa o fio em Erro (ação destrutiva).
	QuadroDestrutivo
)

// larguraMinQuadro é a menor largura total que ainda comporta borda, respiro e conteúdo.
const larguraMinQuadro = 8

// Quadro desenha um painel com borda e o rótulo em caixa-alta embutido no fio de cima:
//
//	┌ DETALHE · POD ──────┐
//	│ conteúdo            │
//	└─────────────────────┘
//
// largura é a largura total em células, incluindo a borda; linhas do corpo mais largas
// que o espaço interno são truncadas com reticências. Função pura: não guarda estado.
func Quadro(rotulo, corpo string, largura int, v VarianteQuadro) string {
	largura = max(largura, larguraMinQuadro)
	s := S()
	estilo := s.Fio
	switch v {
	case QuadroFoco:
		estilo = s.Tinta
	case QuadroDestrutivo:
		estilo = s.Erro
	}

	cantoSE, cantoSD, cantoIE, cantoID, horiz, vert := "┌", "┐", "└", "┘", "─", "│"
	if Atual().ASCII {
		cantoSE, cantoSD, cantoIE, cantoID, horiz, vert = "+", "+", "+", "+", "-", "|"
	}

	interno := largura - 4 // borda + 1 célula de respiro de cada lado

	// Fio de cima: canto, espaço, rótulo, espaço, preenchimento, canto.
	var topo string
	if rotulo != "" {
		texto := truncarCelulas(strings.ToUpper(rotulo), largura-6)
		falta := largura - 2 - lipgloss.Width(texto) - 2
		topo = estilo.Render(cantoSE+" ") + estilo.Bold(true).Render(texto) +
			estilo.Render(" "+strings.Repeat(horiz, max(falta, 0))+cantoSD)
	} else {
		topo = estilo.Render(cantoSE + strings.Repeat(horiz, largura-2) + cantoSD)
	}

	linhas := []string{topo}
	for _, l := range strings.Split(corpo, "\n") {
		l = truncarCelulas(l, interno)
		pad := strings.Repeat(" ", interno-lipgloss.Width(l))
		linhas = append(linhas, estilo.Render(vert)+" "+l+pad+" "+estilo.Render(vert))
	}
	linhas = append(linhas, estilo.Render(cantoIE+strings.Repeat(horiz, largura-2)+cantoID))
	return strings.Join(linhas, "\n")
}

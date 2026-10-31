package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// QuadroAberto é o Quadro sem as bordas dos lados, para o texto que a pessoa seleciona com
// o mouse: a linha copiada sai sem o "│" e sem o preenchimento até a borda da direita.
//
//	─ COMANDO · ctrl+y copia ───────
//	  kubectl get pods -n web
//	────────────────────────────────
//
// O rótulo vai em caixa-alta; a tecla e a ação ficam como vieram e só entram se couberem
// no fio. O corpo leva o mesmo recuo do Quadro e não é cortado: quem chama quebra as
// linhas na largura de dentro (largura - 4), como no Quadro. Função pura.
func QuadroAberto(rotulo, tecla, acao, corpo string, largura int) string {
	largura = max(largura, larguraMinQuadro)
	s := S()
	horiz := "─"
	if Atual().ASCII {
		horiz = "-"
	}

	texto := strings.ToUpper(rotulo)
	if tecla != "" && lipgloss.Width(texto+" · "+tecla+" "+acao) <= largura-4 {
		texto += " · " + tecla + " " + acao
	}
	texto = truncarCelulas(texto, largura-4)
	falta := largura - lipgloss.Width(texto) - 3
	topo := s.Fio.Render(horiz+" ") + s.TintaSuave.Render(texto) + s.Fio.Render(" "+strings.Repeat(horiz, max(falta, 0)))

	linhas := []string{topo}
	for _, l := range strings.Split(corpo, "\n") {
		if strings.TrimSpace(l) == "" {
			linhas = append(linhas, "")
			continue
		}
		linhas = append(linhas, "  "+s.Tinta.Render(l))
	}
	linhas = append(linhas, s.Fio.Render(strings.Repeat(horiz, largura)))
	return strings.Join(linhas, "\n")
}

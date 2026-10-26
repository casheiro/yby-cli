package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestQuadroGolden(t *testing.T) {
	corpo := "nome      api-7f9c-x2k\nnamespace payments\numa linha bem comprida que precisa ser truncada"
	etapasPorModo(t, "quadro_normal", func() string { return Quadro("detalhe · pod", corpo, 40, QuadroNormal) })
	etapasPorModo(t, "quadro_foco", func() string { return Quadro("detalhe · pod", corpo, 40, QuadroFoco) })
	etapasPorModo(t, "quadro_destrutivo", func() string { return Quadro("excluir pod", "digite api", 40, QuadroDestrutivo) })
}

func TestQuadroGeometria(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	for _, w := range []int{4, 8, 20, 50} {
		for _, rotulo := range []string{"", "curto", "um rótulo extremamente longo para o fio"} {
			saida := Quadro(rotulo, "ação\n\ncoração de açúcar muito longo para a caixa", w, QuadroNormal)
			etapasLarguraMax(t, saida, max(w, larguraMinQuadro))
			linhas := strings.Split(saida, "\n")
			// Todas as linhas têm a mesma largura (borda reta).
			for _, l := range linhas {
				if got, want := lipgloss.Width(l), lipgloss.Width(linhas[0]); got != want {
					t.Errorf("w=%d rótulo=%q: linha torta (%d != %d): %q", w, rotulo, got, want, l)
				}
			}
		}
	}
}

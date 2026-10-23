package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Rota renderiza o progresso por marcos em duas linhas:
//
//	◆━━━◆━━━◆┄┄┄◇┄┄┄◇  03/06
//	PROJETO · CLUSTER · ARGO CD · ...
//
// atual é o índice (base 0) do marco atual; marcos anteriores aparecem como feitos.
// largura é a total em células (<= 0 usa trechos de 3 células).
func Rota(rotulos []string, atual, largura int) string {
	n := len(rotulos)
	if n == 0 {
		return ""
	}
	atual = min(max(atual, 0), n-1)
	s, g := S(), G()
	destaque := s.Negrito.Inherit(s.Tinta)

	contador := fmt.Sprintf("%02d/%02d", atual+1, n)
	trecho := 3
	if largura > 0 && n > 1 {
		// n marcos + (n-1) trechos + 2 espaços + contador.
		trecho = (largura - n - 2 - lipgloss.Width(contador)) / (n - 1)
		trecho = min(max(trecho, 1), 3)
	}

	var b strings.Builder
	for i := 0; i < n; i++ {
		switch {
		case i < atual:
			b.WriteString(s.Ok.Render(g.Ativo))
		case i == atual:
			b.WriteString(destaque.Render(g.Ativo))
		default:
			b.WriteString(s.TintaSuave.Render(g.Pendente))
		}
		if i == n-1 {
			break
		}
		if i < atual {
			b.WriteString(s.Tinta.Render(strings.Repeat(g.Cheio, trecho)))
		} else {
			b.WriteString(s.Fio.Render(strings.Repeat(g.Tracejado, trecho)))
		}
	}
	b.WriteString("  " + s.TintaSuave.Render(contador))

	nomes := make([]string, n)
	for i, r := range rotulos {
		r = strings.ToUpper(r)
		if i == atual {
			nomes[i] = destaque.Render(r)
		} else {
			nomes[i] = s.TintaSuave.Render(r)
		}
	}
	linhaRotulos := strings.Join(nomes, s.TintaSuave.Render(" · "))
	if largura > 0 {
		linhaRotulos = truncarCelulas(linhaRotulos, largura)
	}
	return b.String() + "\n" + linhaRotulos
}

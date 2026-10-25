package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// QuadroComTecla desenha um Quadro com a tecla de uma ação ao lado do rótulo, no fio de
// cima, como no bloco "COMANDO · c copia" da prancha do Bard:
//
//	┌ COMANDO · c copia ──────────┐
//	│ kubectl delete pv pvc-3f1a  │
//	└─────────────────────────────┘
//
// O rótulo vai em caixa-alta; a tecla e a ação ficam como vieram. O texto do rótulo sai em
// TintaSuave (Tinta com foco, Erro no destrutivo), legível sobre o fundo; os cantos e o fio
// seguem a variante, como no Quadro. Se o fio não comporta rótulo e tecla, a tecla sai e
// fica o Quadro comum. Função pura.
func QuadroComTecla(rotulo, tecla, acao, corpo string, largura int, v VarianteQuadro) string {
	largura = max(largura, larguraMinQuadro)
	q := Quadro(rotulo, corpo, largura, v)
	texto := strings.ToUpper(rotulo) + " · " + tecla + " " + acao
	falta := largura - 2 - lipgloss.Width(texto) - 2
	if rotulo == "" || tecla == "" || falta < 1 {
		return q
	}

	s := S()
	fio, rot := s.Fio, s.TintaSuave
	switch v {
	case QuadroFoco:
		fio, rot = s.Tinta, s.Tinta
	case QuadroDestrutivo:
		fio, rot = s.Erro, s.Erro
	}
	cantoSE, cantoSD, horiz := "┌", "┐", "─"
	if Atual().ASCII {
		cantoSE, cantoSD, horiz = "+", "+", "-"
	}
	topo := fio.Render(cantoSE+" ") + rot.Render(texto) +
		fio.Render(" "+strings.Repeat(horiz, falta)+cantoSD)

	linhas := strings.Split(q, "\n")
	linhas[0] = topo
	return strings.Join(linhas, "\n")
}

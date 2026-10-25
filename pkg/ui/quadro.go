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
	// QuadroSuave usa o fio em Fio, como o QuadroNormal, e o rótulo em TintaSuave sem
	// negrito, como o do QuadroComTecla: o quadro de leitura das saídas de comando (o
	// diagnóstico do Sentinel), com o rótulo legível sobre o fundo como nas pranchas.
	QuadroSuave
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
	return quadro(strings.ToUpper(rotulo), corpo, largura, v)
}

// QuadroNomeado é o Quadro com o nome de um recurso depois do rótulo, como no painel do
// Viz (┌ POD · worker-5d8b6-q7tvn ───┐): o rótulo vai em caixa-alta e o nome fica como
// veio, porque os nomes do Kubernetes são em minúsculas. Sem nome, é o Quadro comum.
func QuadroNomeado(rotulo, nome, corpo string, largura int, v VarianteQuadro) string {
	texto := strings.ToUpper(rotulo)
	if nome != "" {
		texto += " · " + nome
	}
	return quadro(texto, corpo, largura, v)
}

// QuadroComNome é o QuadroNomeado com o separador do modo visual (o glifo Ponto: "·", ou
// "/" no ASCII), para as telas novas: ┌ AKS · aks-dados ───┐. O QuadroNomeado guarda o
// "·" também no ASCII, como as telas do Viz já o desenham.
func QuadroComNome(rotulo, nome, corpo string, largura int, v VarianteQuadro) string {
	texto := strings.ToUpper(rotulo)
	if nome != "" {
		texto += " " + G().Ponto + " " + nome
	}
	return quadro(texto, corpo, largura, v)
}

// quadro desenha o Quadro com o texto do fio de cima já pronto.
func quadro(rotulo, corpo string, largura int, v VarianteQuadro) string {
	largura = max(largura, larguraMinQuadro)
	s := S()
	estilo := s.Fio
	rot := estilo.Bold(true)
	switch v {
	case QuadroFoco:
		estilo = s.Tinta
		rot = estilo.Bold(true)
	case QuadroDestrutivo:
		estilo = s.Erro
		rot = estilo.Bold(true)
	case QuadroSuave:
		rot = s.TintaSuave
	}

	cantoSE, cantoSD, cantoIE, cantoID, horiz, vert := "┌", "┐", "└", "┘", "─", "│"
	if Atual().ASCII {
		cantoSE, cantoSD, cantoIE, cantoID, horiz, vert = "+", "+", "+", "+", "-", "|"
	}

	interno := largura - 4 // borda + 1 célula de respiro de cada lado

	// Fio de cima: canto, espaço, rótulo, espaço, preenchimento, canto.
	var topo string
	if rotulo != "" {
		texto := truncarCelulas(rotulo, largura-6)
		falta := largura - 2 - lipgloss.Width(texto) - 2
		topo = estilo.Render(cantoSE+" ") + rot.Render(texto) +
			estilo.Render(" "+strings.Repeat(horiz, max(falta, 0))+cantoSD)
	} else {
		topo = estilo.Render(cantoSE + strings.Repeat(horiz, largura-2) + cantoSD)
	}

	linhas := []string{topo}
	for _, l := range strings.Split(corpo, "\n") {
		l = truncarCelulas(l, interno)
		pad := strings.Repeat(" ", interno-lipgloss.Width(l))
		linhas = append(linhas, estilo.Render(vert)+" "+s.Tinta.Render(l)+pad+" "+estilo.Render(vert))
	}
	linhas = append(linhas, estilo.Render(cantoIE+strings.Repeat(horiz, largura-2)+cantoID))
	return strings.Join(linhas, "\n")
}

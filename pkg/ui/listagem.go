package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Listagem escreve em w uma lista em colunas para saídas de comando (yby env list,
// plugin list, cloud list): o cabeçalho com os títulos em TintaSuave e caixa-alta e uma
// linha por item, tudo recuado em 2 células como as linhas de Check:
//
//	   AMBIENTE  TIPO    DESCRIÇÃO
//	◆  local     local   cluster k3d
//	   prod      remote  produção
//
// Cada coluna tem a largura da célula mais larga (limitada por Max, com reticências) e as
// colunas se separam por 2 espaços; a última não é completada, então as linhas não têm
// espaços à direita. As células podem conter ANSI (por exemplo, de Status). Quando w é um
// terminal mais estreito que a lista, ela encolhe como em Tabela: some primeiro a coluna
// de maior Prioridade e o texto é truncado.
func Listagem(w io.Writer, colunas []Coluna, linhas []Linha) {
	if len(colunas) == 0 {
		return
	}
	larguras := make([]int, len(colunas))
	total := 2 + espacoColunas*(len(colunas)-1)
	for i, c := range colunas {
		larg := lipgloss.Width(c.Titulo)
		for _, l := range linhas {
			if i < len(l.Celulas) {
				larg = max(larg, lipgloss.Width(l.Celulas[i]))
			}
		}
		if c.Max > 0 {
			larg = min(larg, c.Max)
		}
		larguras[i] = max(larg, c.Min)
		total += larguras[i]
	}

	if l := larguraDe(w); l > 0 && total > l {
		for _, linha := range strings.Split(Tabela(colunas, linhas, -1, max(l-2, 1)), "\n") {
			fmt.Fprintln(w, "  "+linha)
		}
		return
	}

	s := S()
	titulos := make([]string, len(colunas))
	for i, c := range colunas {
		titulos[i] = strings.ToUpper(c.Titulo)
	}
	fmt.Fprintln(w, "  "+s.Rotulo.Render(juntarCelulas(titulos, colunas, larguras)))
	for _, l := range linhas {
		estilo := s.Tinta
		if l.Concluida {
			estilo = s.TintaSuave
		}
		celulas := make([]string, len(colunas))
		for i := range colunas {
			if i >= len(l.Celulas) || l.Celulas[i] == "" {
				continue
			}
			celulas[i] = l.Celulas[i]
			// a célula que já traz cor (de Status, por exemplo) fica como veio: o reset
			// dela encerraria a tinta de fora no meio da célula
			if ansi.Strip(celulas[i]) == celulas[i] {
				celulas[i] = estilo.Render(celulas[i])
			}
		}
		fmt.Fprintln(w, strings.TrimRight("  "+juntarCelulas(celulas, colunas, larguras), " "))
	}
}

// juntarCelulas alinha as células nas larguras dadas, separadas por espacoColunas, sem
// completar a última célula não vazia (nem deixar espaços à direita).
func juntarCelulas(celulas []string, colunas []Coluna, larguras []int) string {
	ultima := len(celulas) - 1
	for ultima > 0 && celulas[ultima] == "" {
		ultima--
	}
	partes := make([]string, ultima+1)
	for i := 0; i <= ultima; i++ {
		txt := celulas[i]
		if i == ultima && !colunas[i].Direita {
			if lipgloss.Width(txt) > larguras[i] {
				txt = celula(txt, larguras[i], false)
			}
			partes[i] = txt
			continue
		}
		partes[i] = celula(txt, larguras[i], colunas[i].Direita)
	}
	return strings.TrimRight(strings.Join(partes, strings.Repeat(" ", espacoColunas)), " ")
}

// Campo é um par rótulo e valor de Campos. Com Caminho, o valor é um caminho de arquivo:
// a home do usuário vira ~ (AbreviarHome) e, quando ele não cabe no terminal, quebra só
// nos separadores (QuebrarCaminho), nunca no meio de um nome.
type Campo struct {
	Rotulo, Valor string
	Caminho       bool
}

// Campos escreve pares rótulo e valor, um por linha e recuados em 2 células, como nas
// linhas "causa" e "faça" de RenderError: o rótulo em TintaSuave, numa coluna com a
// largura do mais largo mais 2 espaços, e o valor em Tinta. Um par com valor vazio não
// aparece. Quando w é um terminal e o valor não cabe, ele quebra entre palavras (um
// caminho, nos separadores), alinhado sob o começo do valor.
func Campos(w io.Writer, campos ...Campo) {
	s := S()
	col := 0
	for _, c := range campos {
		if c.Valor != "" {
			col = max(col, lipgloss.Width(c.Rotulo)+2)
		}
	}
	for _, c := range campos {
		if c.Valor == "" {
			continue
		}
		valor := c.Valor
		if c.Caminho {
			valor = AbreviarHome(valor)
		}
		partes := []string{valor}
		if l := larguraDe(w); l > 0 {
			if c.Caminho {
				partes = QuebrarCaminho(valor, l-2-col)
			} else {
				partes = Quebrar(valor, l-2-col)
			}
		}
		fmt.Fprintln(w, "  "+s.Rotulo.Render(padDireita(c.Rotulo, col))+s.Tinta.Render(partes[0]))
		for _, p := range partes[1:] {
			fmt.Fprintln(w, strings.Repeat(" ", 2+col)+s.Tinta.Render(p))
		}
	}
}

// Passos escreve uma lista numerada, como os próximos passos de um comando:
//
//  1. edite o arquivo gerado
//  2. aplique com kubectl apply -f bootstrap/secret-store-vault.yaml
//
// recuada em 2 células, com o número em TintaSuave e o texto em Tinta. Quando w é um
// terminal e o texto não cabe, ele quebra entre palavras, alinhado sob o começo do texto.
func Passos(w io.Writer, passos ...string) {
	s := S()
	col := len(fmt.Sprintf("%d. ", len(passos)))
	for i, p := range passos {
		partes := []string{p}
		if l := larguraDe(w); l > 0 {
			partes = Quebrar(p, l-2-col)
		}
		numero := padDireita(fmt.Sprintf("%d.", i+1), col)
		fmt.Fprintln(w, "  "+s.TintaSuave.Render(numero)+s.Tinta.Render(partes[0]))
		for _, resto := range partes[1:] {
			fmt.Fprintln(w, strings.Repeat(" ", 2+col)+s.Tinta.Render(resto))
		}
	}
}

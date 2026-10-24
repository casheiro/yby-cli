package ui

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// minQuebra é a largura mínima, em células, para quebrar um texto: numa coluna mais
// estreita a quebra pioraria a leitura, e a linha fica inteira.
const minQuebra = 16

// larguraDe devolve a largura, em células, do terminal ligado a w, ou 0 quando w não é
// um terminal (arquivo, pipe, buffer de teste). Variável para os testes fixarem a largura.
var larguraDe = func(w io.Writer) int {
	f, ok := w.(interface{ Fd() uintptr })
	if !ok {
		return 0
	}
	larg, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}
	return larg
}

// Largura devolve a largura, em células, do terminal ligado a w, ou 0 quando w não é um
// terminal; nesse caso as saídas não quebram linha.
func Largura(w io.Writer) int { return larguraDe(w) }

// Quebrar divide texto em linhas de até largura células, quebrando entre palavras. Uma
// palavra mais larga que a linha fica inteira na sua. Com largura menor que minQuebra,
// ou quando o texto já cabe, devolve o texto como está.
func Quebrar(texto string, largura int) []string {
	if largura < minQuebra || lipgloss.Width(texto) <= largura {
		return []string{texto}
	}
	palavras := strings.Fields(texto)
	if len(palavras) == 0 {
		return []string{""}
	}
	var linhas []string
	atual := palavras[0]
	for _, p := range palavras[1:] {
		if lipgloss.Width(atual)+1+lipgloss.Width(p) > largura {
			linhas = append(linhas, atual)
			atual = p
			continue
		}
		atual += " " + p
	}
	return append(linhas, atual)
}

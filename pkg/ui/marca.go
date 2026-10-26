package ui

import (
	_ "embed"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Desenhos da logo oficial (docs/assets/brand/yby-logo.svg), feitos à mão pixel a pixel.
// Os de blocos usam só ' ▀▄█', que todo terminal desenha; o traço é ASCII puro.
var (
	//go:embed marca/grande.txt
	marcaGrande string
	//go:embed marca/compacta.txt
	marcaCompacta string
	//go:embed marca/traco.txt
	marcaTraco string
)

// desenho devolve as linhas de um desenho embutido, sem \r (checkout com CRLF) e sem a
// quebra final.
func desenho(s string) []string {
	return strings.Split(strings.TrimRight(strings.ReplaceAll(s, "\r", ""), "\n"), "\n")
}

// cabe informa se as linhas cabem em largura×altura células; altura <= 0 não limita.
func cabe(linhas []string, largura, altura int) bool {
	if altura > 0 && len(linhas) > altura {
		return false
	}
	for _, l := range linhas {
		if lipgloss.Width(l) > largura {
			return false
		}
	}
	return true
}

// Marca devolve a logo que cabe em largura×altura células (altura <= 0 não limita), na cor
// Tinta: a grande (64×12) ou a compacta (42×8), em blocos. Em modo ASCII usa o desenho a
// traço (60×12). Sem espaço para nenhum desenho, devolve a marca em texto, "◆ YBY".
func Marca(largura, altura int) string {
	opcoes := [][]string{desenho(marcaGrande), desenho(marcaCompacta)}
	if Atual().ASCII {
		opcoes = [][]string{desenho(marcaTraco)}
	}
	s := S()
	for _, linhas := range opcoes {
		if !cabe(linhas, largura, altura) {
			continue
		}
		// cor linha a linha: num bloco, o lipgloss completaria as linhas com espaços
		for i, l := range linhas {
			linhas[i] = s.Tinta.Render(l)
		}
		return strings.Join(linhas, "\n")
	}
	return s.Ok.Render(G().Ativo) + " " + s.Negrito.Render("YBY")
}

// Logo escreve em w a logo que cabe na largura do terminal, seguida de uma linha em branco.
// Fora de um terminal (pipe, arquivo) não escreve nada, para não sujar a saída.
func Logo(w io.Writer) {
	largura := larguraDe(w)
	if largura <= 0 {
		return
	}
	for _, l := range strings.Split(Marca(largura-2, 0), "\n") {
		fmt.Fprintln(w, "  "+l)
	}
	fmt.Fprintln(w)
}

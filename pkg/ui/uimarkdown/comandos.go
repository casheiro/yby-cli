package uimarkdown

import (
	"strings"

	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/x/ansi"
)

// Comandos devolve, na ordem em que aparecem, os blocos de shell do Markdown, os que
// Renderizar desenha no quadro "COMANDO". Cada um vem pronto para colar no shell: o
// texto entre as cercas, sem as linhas vazias do fim e sem o prompt. Num bloco console,
// que traz o prompt e a saída, ficam só as linhas de comando ("$ " ou "# ") sem o
// prompt; nas outras linguagens, o "$ " sai quando todas as linhas o têm.
func Comandos(md string) []string {
	var comandos []string
	for _, b := range separarBlocos(md) {
		if c, ok := comandoDoBloco(b); ok {
			comandos = append(comandos, c)
		}
	}
	return comandos
}

// comandoDoBloco devolve o texto a copiar de um bloco de shell; falso se o bloco não é
// de shell ou está vazio.
func comandoDoBloco(b bloco) (string, bool) {
	if b.tipo != blocoCodigo || !linguagensShell[b.linguagem] {
		return "", false
	}
	linhas := semPrompt(b.linguagem, apararFim(b.linhas))
	if len(linhas) == 0 {
		return "", false
	}
	return strings.Join(linhas, "\n"), true
}

// apararFim tira as linhas vazias do fim.
func apararFim(linhas []string) []string {
	for len(linhas) > 0 && strings.TrimSpace(linhas[len(linhas)-1]) == "" {
		linhas = linhas[:len(linhas)-1]
	}
	return linhas
}

// semPrompt tira o prompt das linhas de comando. No console as linhas sem prompt são a
// saída e ficam de fora (sem nenhum prompt no bloco, tudo fica); nas outras linguagens
// o "# " é comentário, e só o "$ " sai, e só se estiver em todas as linhas.
func semPrompt(linguagem string, linhas []string) []string {
	prompts := []string{"$ "}
	if linguagem == "console" {
		prompts = append(prompts, "# ")
	}
	tirar := func(l string) (string, bool) {
		for _, p := range prompts {
			if r, ok := strings.CutPrefix(strings.TrimLeft(l, " "), p); ok {
				return r, true
			}
		}
		return l, false
	}
	var comandos []string
	todas := true
	for _, l := range linhas {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if c, ok := tirar(l); ok {
			comandos = append(comandos, c)
		} else {
			todas = false
		}
	}
	if len(comandos) == 0 || (linguagem != "console" && !todas) {
		return linhas
	}
	return comandos
}

// RenderizarComTecla é como Renderizar, mas só o comando de índice indice (na ordem de
// Comandos) leva a tecla no rótulo, como "COMANDO · ctrl+y copia"; os outros quadros de
// shell ficam só "COMANDO". Assim o rótulo mostra a tecla apenas no bloco que ela
// copia. indice -1 deixa todos sem tecla.
func (r *Renderizador) RenderizarComTecla(md string, indice int, tecla, acao string) (string, error) {
	var saida []string
	n := 0
	for _, b := range separarBlocos(md) {
		var texto string
		switch {
		case b.tipo == blocoCodigo && linguagensShell[b.linguagem]:
			corpo := corpoCodigo(b.linhas, r.largura)
			if _, ok := comandoDoBloco(b); ok {
				if n == indice {
					texto = ui.QuadroComTecla("comando", tecla, acao, corpo, r.largura, ui.QuadroNormal)
				}
				n++
			}
			if texto == "" {
				texto = ui.Quadro("comando", corpo, r.largura, ui.QuadroNormal)
			}
		case b.tipo == blocoTabela:
			texto = renderizarTabela(b.linhas, r.largura)
		case b.tipo == blocoCodigo:
			texto = renderizarCodigo(b.linguagem, b.linhas, r.largura)
		default:
			t, err := r.glamour.Render(strings.Join(b.linhas, "\n"))
			if err != nil {
				return "", err
			}
			texto = t
		}
		if r.semEstilo {
			texto = ansi.Strip(texto)
		}
		if texto = aparar(texto); texto != "" {
			saida = append(saida, texto)
		}
	}
	return strings.Join(saida, "\n\n"), nil
}

// corpoCodigo quebra as linhas do bloco na largura de dentro do quadro, como
// renderizarCodigo.
func corpoCodigo(linhas []string, largura int) string {
	interno := largura - 4 // borda + respiro de cada lado, como no ui.Quadro
	var quebradas []string
	for _, l := range apararFim(linhas) {
		quebradas = append(quebradas, quebrarCodigo(strings.ReplaceAll(l, "\t", "    "), interno)...)
	}
	return strings.Join(quebradas, "\n")
}

package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Termo é uma linha das seções de comandos e flags da ajuda: o nome que a pessoa digita
// e o que ele faz.
type Termo struct {
	Nome, Descricao string
}

// Ajuda descreve a ajuda de um comando, montada pelo chamador (o core a partir do cobra;
// os plugins a partir das próprias flags).
type Ajuda struct {
	// Comando é o caminho completo, como "yby env create"; o primeiro termo é a marca.
	Comando   string
	Descricao string
	Uso       []string
	Apelidos  []string
	Comandos  []Termo
	// Exemplos é o bloco como está no código: linhas que começam com # são comentários.
	Exemplos string
	Flags    []Termo
	Globais  []Termo
	Rodape   string
	// SemTitulo omite o cabeçalho "◆ YBY …", quando a logo já abriu a tela.
	SemTitulo bool
}

// maxColunaTermo limita a coluna dos nomes: um nome mais longo fica sozinho na linha e a
// descrição desce para a de baixo.
const maxColunaTermo = 30

// EscreverAjuda escreve a ajuda no formato das saídas de comando: o título "◆ YBY  ENV
// CREATE" com "ajuda" à direita, a descrição e as seções USO, COMANDOS, EXEMPLOS, FLAGS e
// FLAGS GLOBAIS (as vazias não aparecem), com recuo de 2 e quebra na largura do terminal.
func EscreverAjuda(w io.Writer, a Ajuda) {
	s := S()
	larg := larguraDe(w)
	if !a.SemTitulo {
		instrumento, direita := "ajuda", ""
		if _, resto, ok := strings.Cut(a.Comando, " "); ok {
			instrumento, direita = resto, "ajuda"
		}
		Title(w, instrumento, direita)
		fmt.Fprintln(w)
	}
	for _, par := range strings.Split(strings.TrimSpace(a.Descricao), "\n") {
		paragrafo(w, s.Tinta, strings.TrimSpace(par), larg)
	}
	if len(a.Uso) > 0 {
		Section(w, "uso")
		for _, u := range a.Uso {
			fmt.Fprintln(w, "  "+s.Tinta.Render(u))
		}
		if len(a.Apelidos) > 0 {
			pai := a.Comando
			if i := strings.LastIndex(a.Comando, " "); i > 0 {
				pai = a.Comando[:i]
			}
			nomes := make([]string, len(a.Apelidos))
			for i, ap := range a.Apelidos {
				nomes[i] = pai + " " + ap
			}
			fmt.Fprintln(w, "  "+s.TintaSuave.Render("também "+strings.Join(nomes, ", ")))
		}
	}
	termos(w, "comandos", a.Comandos, larg)
	if ex := exemplos(a.Exemplos); len(ex) > 0 {
		Section(w, "exemplos")
		for _, l := range ex {
			// num terminal estreito, o comentário continua com "# " e o comando com recuo
			estilo, recuo, marca := s.Tinta, "    ", ""
			if strings.HasPrefix(strings.TrimSpace(l), "#") {
				estilo, recuo, marca = s.TintaSuave, "  ", "# "
			}
			partes := []string{l}
			if larg > 0 {
				partes = Quebrar(l, larg-4)
			}
			fmt.Fprintln(w, "  "+estilo.Render(partes[0]))
			for _, p := range partes[1:] {
				fmt.Fprintln(w, recuo+estilo.Render(marca+p))
			}
		}
	}
	termos(w, "flags", a.Flags, larg)
	termos(w, "flags globais", a.Globais, larg)
	if a.Rodape != "" {
		fmt.Fprintln(w)
		paragrafo(w, s.TintaSuave, a.Rodape, larg)
	}
}

// paragrafo escreve texto recuado em 2, quebrando entre palavras quando w é um terminal.
// Linha vazia continua vazia, para separar parágrafos.
func paragrafo(w io.Writer, estilo lipgloss.Style, texto string, larg int) {
	if texto == "" {
		fmt.Fprintln(w)
		return
	}
	partes := []string{texto}
	if larg > 0 {
		partes = Quebrar(texto, larg-2)
	}
	for _, p := range partes {
		fmt.Fprintln(w, "  "+estilo.Render(p))
	}
}

// termos escreve uma seção de nome e descrição: os nomes em Tinta numa coluna com a largura
// do mais largo (até maxColunaTermo) mais 2, e as descrições em TintaSuave, quebradas sob
// o começo da descrição.
func termos(w io.Writer, secao string, lista []Termo, larg int) {
	if len(lista) == 0 {
		return
	}
	s := S()
	col := 0
	for _, t := range lista {
		if l := lipgloss.Width(t.Nome); l <= maxColunaTermo {
			col = max(col, l)
		}
	}
	col += 2
	Section(w, secao)
	for _, t := range lista {
		partes := []string{t.Descricao}
		if larg > 0 {
			partes = Quebrar(t.Descricao, larg-2-col)
		}
		recuo := strings.Repeat(" ", 2+col)
		if lipgloss.Width(t.Nome) > col-2 {
			fmt.Fprintln(w, "  "+s.Tinta.Render(t.Nome))
			for _, p := range partes {
				fmt.Fprintln(w, recuo+s.TintaSuave.Render(p))
			}
			continue
		}
		fmt.Fprintln(w, "  "+s.Tinta.Render(padDireita(t.Nome, col))+s.TintaSuave.Render(partes[0]))
		for _, p := range partes[1:] {
			fmt.Fprintln(w, recuo+s.TintaSuave.Render(p))
		}
	}
}

// exemplos tira o recuo comum do bloco e as linhas vazias das pontas.
func exemplos(bloco string) []string {
	linhas := strings.Split(strings.Trim(strings.ReplaceAll(bloco, "\r", ""), "\n"), "\n")
	recuo := -1
	for _, l := range linhas {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if recuo < 0 || n < recuo {
			recuo = n
		}
	}
	if recuo < 0 {
		return nil
	}
	out := make([]string, 0, len(linhas))
	for _, l := range linhas {
		if len(l) >= recuo {
			l = l[recuo:]
		}
		out = append(out, strings.TrimRight(l, " \t"))
	}
	return out
}

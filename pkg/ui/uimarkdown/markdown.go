// Package uimarkdown renderiza Markdown na carta do terminal: o texto passa pelo glamour
// com um estilo derivado dos tokens de pkg/ui, e tabelas e blocos de código saem com os
// componentes da carta (ui.Tabela, ou registros quando a tabela não cabe, e ui.Quadro).
//
// Fica num pacote à parte para que só quem renderiza Markdown (o Bard) leve o glamour e
// o chroma para o binário; o core usa pkg/ui sem eles.
package uimarkdown

import (
	"cmp"
	"regexp"
	"strings"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/charmbracelet/glamour"
	gansi "github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// larguraMinima é a menor largura aceita; abaixo disso o texto quebraria a cada palavra.
const larguraMinima = 20

// Renderizador converte Markdown em texto da carta numa largura fixa, em células.
// Cada linha devolvida cabe na largura e não tem espaços sobrando no fim.
type Renderizador struct {
	largura int
	glamour *glamour.TermRenderer
	// semEstilo tira todo ANSI da saída no perfil Ascii, como faz o lipgloss: o glamour
	// só descarta as cores e manteria negrito e sublinhado.
	semEstilo bool
	// aberto desenha os blocos de código sem as bordas dos lados (ui.QuadroAberto), para a
	// seleção com o mouse copiar só o código.
	aberto bool
}

// Novo cria um renderizador para a largura dada, com o tema, o perfil de cor e o modo
// de glifos atuais de pkg/ui. Crie outro quando a largura ou o tema mudarem.
func Novo(largura int) (*Renderizador, error) {
	largura = max(largura, larguraMinima)
	perfil := lipgloss.ColorProfile()
	perfilCor := perfil
	if ui.Atual().SemCor {
		// Sem cor, mas com negrito e itálico: o renderer segue no perfil do lipgloss.
		perfilCor = termenv.Ascii
	}
	g, err := glamour.NewTermRenderer(
		glamour.WithStyles(Estilo(perfilCor, lipgloss.HasDarkBackground(), largura)),
		glamour.WithColorProfile(perfil),
		glamour.WithWordWrap(largura),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		return nil, err
	}
	return &Renderizador{largura: largura, glamour: g, semEstilo: perfil == termenv.Ascii}, nil
}

// Largura devolve a largura, em células, para a qual o renderizador foi criado.
func (r *Renderizador) Largura() int { return r.largura }

// AbrirQuadros troca o desenho dos blocos de código: aberto, sem as bordas dos lados
// (ui.QuadroAberto), para a seleção com o mouse copiar só o código; fechado, o ui.Quadro.
func (r *Renderizador) AbrirQuadros(aberto bool) { r.aberto = aberto }

// Renderizar converte md. Tabelas no nível de cima viram ui.Tabela (ou registros, quando
// não cabem na largura); blocos de código cercados viram ui.Quadro, com o rótulo
// "COMANDO · ctrl+y copia" para shell. O resto passa pelo glamour. Os blocos ficam separados
// por uma linha em branco. Nada é cortado: o que não cabe quebra.
func (r *Renderizador) Renderizar(md string) (string, error) {
	var saida []string
	for _, b := range separarBlocos(md) {
		var texto string
		switch b.tipo {
		case blocoTabela:
			texto = renderizarTabela(b.linhas, r.largura)
		case blocoCodigo:
			texto = renderizarCodigo(b.linguagem, b.linhas, r.largura, r.aberto)
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

var (
	// sgrVazio é um estilo aberto e fechado sem texto no meio, que o glamour emite aos montes.
	sgrVazio = regexp.MustCompile(`\x1b\[[0-9;]*m\x1b\[0m`)
	// sgrNoFim são as sequências que sobram depois do último caractere visível.
	sgrNoFim = regexp.MustCompile(`(\x1b\[[0-9;]*m)+$`)
)

// aparar tira as linhas vazias do começo e do fim e os espaços que o glamour (e a
// ui.Tabela) deixam no fim de cada linha, ao completar as linhas até a largura.
func aparar(s string) string {
	linhas := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, l := range linhas {
		linhas[i] = apararLinha(l)
	}
	ini, fim := 0, len(linhas)
	for ini < fim && linhas[ini] == "" {
		ini++
	}
	for fim > ini && linhas[fim-1] == "" {
		fim--
	}
	return strings.Join(linhas[ini:fim], "\n")
}

// apararLinha corta os espaços do fim de uma linha com ANSI, sem perder o reset final.
func apararLinha(l string) string {
	visivel := strings.TrimRight(ansi.Strip(l), " ")
	if visivel == "" {
		return ""
	}
	if w := ansi.StringWidth(visivel); ansi.StringWidth(l) > w {
		l = ansi.Truncate(l, w, "")
	}
	for sgrVazio.MatchString(l) {
		l = sgrVazio.ReplaceAllString(l, "")
	}
	if strings.Contains(l, "\x1b[") {
		l = sgrNoFim.ReplaceAllString(l, "") + "\x1b[0m"
	}
	return l
}

// tipoBloco classifica um trecho do Markdown.
type tipoBloco int

const (
	blocoTexto tipoBloco = iota
	blocoTabela
	blocoCodigo
)

type bloco struct {
	tipo      tipoBloco
	linguagem string   // só em blocoCodigo
	linhas    []string // em blocoCodigo, o conteúdo sem as cercas
}

var (
	// cerca abre ou fecha um bloco de código: até 3 espaços, ``` ou ~~~ e a linguagem.
	cerca = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})\\s*([^`\\s]*)")
	// delimitador é a segunda linha de uma tabela GFM: | --- | :---: | ---: |
	delimitador = regexp.MustCompile(`^ {0,3}\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`)
)

// separarBlocos divide md em texto, tabelas e blocos de código do nível de cima.
func separarBlocos(md string) []bloco {
	linhas := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var blocos []bloco
	var texto []string
	fecharTexto := func() {
		if len(texto) > 0 {
			blocos = append(blocos, bloco{tipo: blocoTexto, linhas: texto})
			texto = nil
		}
	}
	for i := 0; i < len(linhas); i++ {
		l := linhas[i]
		if m := cerca.FindStringSubmatch(l); m != nil {
			fecharTexto()
			marca := m[1]
			b := bloco{tipo: blocoCodigo, linguagem: strings.ToLower(m[2])}
			for i++; i < len(linhas); i++ {
				if f := cerca.FindStringSubmatch(linhas[i]); f != nil && f[2] == "" &&
					f[1][0] == marca[0] && len(f[1]) >= len(marca) {
					break
				}
				b.linhas = append(b.linhas, linhas[i])
			}
			blocos = append(blocos, b)
			continue
		}
		if strings.Contains(l, "|") && i+1 < len(linhas) && strings.Contains(linhas[i+1], "-") &&
			delimitador.MatchString(linhas[i+1]) && len(celulas(l)) == len(celulas(linhas[i+1])) {
			fecharTexto()
			b := bloco{tipo: blocoTabela, linhas: []string{l, linhas[i+1]}}
			for i += 2; i < len(linhas) && strings.Contains(linhas[i], "|") && strings.TrimSpace(linhas[i]) != ""; i++ {
				b.linhas = append(b.linhas, linhas[i])
			}
			i--
			blocos = append(blocos, b)
			continue
		}
		texto = append(texto, l)
	}
	fecharTexto()
	return blocos
}

// celulas divide uma linha de tabela nas células, respeitando \| e tirando as bordas.
func celulas(l string) []string {
	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(l, "|")
	if strings.HasSuffix(l, "|") && !strings.HasSuffix(l, `\|`) {
		l = l[:len(l)-1]
	}
	var saida []string
	var atual strings.Builder
	for i := 0; i < len(l); i++ {
		switch {
		case l[i] == '\\' && i+1 < len(l) && l[i+1] == '|':
			atual.WriteByte('|')
			i++
		case l[i] == '|':
			saida = append(saida, strings.TrimSpace(atual.String()))
			atual.Reset()
		default:
			atual.WriteByte(l[i])
		}
	}
	return append(saida, strings.TrimSpace(atual.String()))
}

// marcasInline são as marcas de ênfase e código tiradas das células da tabela.
var marcasInline = strings.NewReplacer("**", "", "__", "", "`", "")

// statusConhecidos mapeia os estados do Kubernetes para o Status da carta: uma célula
// que é só o estado ganha glifo e cor, como na listagem do Viz.
var statusConhecidos = map[string]ui.TipoStatus{
	"running": ui.StatusRodando, "bound": ui.StatusRodando, "ready": ui.StatusRodando,
	"active": ui.StatusRodando, "available": ui.StatusRodando, "healthy": ui.StatusRodando,
	"synced":  ui.StatusRodando,
	"pending": ui.StatusAlerta, "released": ui.StatusAlerta, "terminating": ui.StatusAlerta,
	"containercreating": ui.StatusAlerta, "progressing": ui.StatusAlerta, "outofsync": ui.StatusAlerta,
	"unknown": ui.StatusAlerta, "notready": ui.StatusAlerta, "degraded": ui.StatusAlerta,
	"failed": ui.StatusFalha, "error": ui.StatusFalha, "crashloopbackoff": ui.StatusFalha,
	"imagepullbackoff": ui.StatusFalha, "errimagepull": ui.StatusFalha, "oomkilled": ui.StatusFalha,
	"lost": ui.StatusFalha, "evicted": ui.StatusFalha,
	"completed": ui.StatusConcluido, "succeeded": ui.StatusConcluido,
}

// renderizarTabela monta a ui.Tabela de uma tabela GFM (cabeçalho, delimitador, linhas)
// quando ela cabe inteira na largura. A ui.Tabela corta as células com reticências, então
// a tabela que não cabe sai em registros, com as células quebradas e nada perdido.
func renderizarTabela(linhas []string, largura int) string {
	cab := celulas(linhas[0])
	alinhamento := celulas(linhas[1])
	colunas := make([]ui.Coluna, len(cab))
	naturais := make([]int, len(cab))
	for i, titulo := range cab {
		a := alinhamento[i]
		colunas[i] = ui.Coluna{
			Titulo:     marcasInline.Replace(titulo),
			Min:        3,
			Prioridade: i,
			Direita:    strings.HasSuffix(a, ":") && !strings.HasPrefix(a, ":"),
		}
		naturais[i] = max(lipgloss.Width(colunas[i].Titulo), 3)
	}
	dados := make([]ui.Linha, 0, len(linhas)-2)
	for _, l := range linhas[2:] {
		cs := celulas(l)
		for j, c := range cs {
			c = marcasInline.Replace(c)
			if tipo, ok := statusConhecidos[strings.ToLower(c)]; ok {
				c = ui.Status(tipo, c)
			}
			cs[j] = c
			if j < len(naturais) {
				naturais[j] = max(naturais[j], lipgloss.Width(c))
			}
		}
		dados = append(dados, ui.Linha{Celulas: cs})
	}
	total := espacoColunas * (len(naturais) - 1)
	for _, w := range naturais {
		total += w
	}
	if total <= largura {
		return ui.Tabela(colunas, dados, -1, largura)
	}
	return renderizarRegistros(colunas, dados, largura)
}

// espacoColunas é o espaço entre as colunas da ui.Tabela e entre rótulo e valor nos registros.
const espacoColunas = 2

// renderizarRegistros mostra cada linha da tabela como um registro de pares "COLUNA valor",
// com o rótulo em TintaSuave e caixa-alta, como nos rótulos da ui.Tabela, e o valor em
// Tinta, quebrado na largura que sobra e alinhado sob o começo do valor. Os registros ficam
// separados por uma linha em branco; célula vazia não aparece, como em ui.Campos. Rótulo
// mais largo que metade da linha fica sozinho, com o valor abaixo.
func renderizarRegistros(colunas []ui.Coluna, dados []ui.Linha, largura int) string {
	s := ui.S()
	col := 0
	for _, c := range colunas {
		col = max(col, lipgloss.Width(c.Titulo)+espacoColunas)
	}
	col = min(col, largura/2)
	recuo := strings.Repeat(" ", col)
	registros := make([]string, 0, len(dados))
	for _, l := range dados {
		var b []string
		for i, c := range colunas {
			if i >= len(l.Celulas) || strings.TrimSpace(l.Celulas[i]) == "" {
				continue
			}
			valores := strings.Split(ansi.Wrap(l.Celulas[i], largura-col, ""), "\n")
			for j, v := range valores {
				if !strings.Contains(v, "\x1b[") {
					// Célula que já traz ANSI (o Status) não é retingida.
					v = s.Tinta.Render(v)
				}
				valores[j] = recuo + v
			}
			titulo := strings.ToUpper(c.Titulo)
			if w := lipgloss.Width(titulo); w+espacoColunas <= col {
				valores[0] = s.Rotulo.Render(titulo) + strings.Repeat(" ", col-w) + valores[0][col:]
			} else {
				for _, t := range strings.Split(ansi.Wrap(titulo, largura, ""), "\n") {
					b = append(b, s.Rotulo.Render(t))
				}
			}
			b = append(b, valores...)
		}
		if len(b) > 0 {
			registros = append(registros, strings.Join(b, "\n"))
		}
	}
	return strings.Join(registros, "\n\n")
}

// linguagensShell são as linguagens de bloco que o Bard trata como comando a rodar.
var linguagensShell = map[string]bool{
	"bash": true, "sh": true, "shell": true, "zsh": true, "console": true,
	"powershell": true, "pwsh": true, "ps1": true, "cmd": true, "bat": true,
}

// renderizarCodigo desenha o bloco de código num ui.Quadro na largura toda. Shell vira o
// quadro "COMANDO · c copia"; as outras linguagens levam o nome da linguagem. Linha mais
// larga que o quadro quebra (entre palavras ou no meio da palavra), sem perder nada: o
// Quadro cortaria o fim com reticências. Aberto, o quadro sai sem as bordas dos lados.
func renderizarCodigo(linguagem string, linhas []string, largura int, aberto bool) string {
	for len(linhas) > 0 && strings.TrimSpace(linhas[len(linhas)-1]) == "" {
		linhas = linhas[:len(linhas)-1]
	}
	interno := largura - 4 // borda + respiro de cada lado, como no ui.Quadro
	var quebradas []string
	for _, l := range linhas {
		quebradas = append(quebradas, quebrarCodigo(strings.ReplaceAll(l, "\t", "    "), interno)...)
	}
	corpo := strings.Join(quebradas, "\n")
	switch {
	case aberto && linguagensShell[linguagem]:
		return ui.QuadroAberto("comando", "c", "copia", corpo, largura)
	case aberto:
		return ui.QuadroAberto(cmp.Or(linguagem, "código"), "", "", corpo, largura)
	case linguagensShell[linguagem]:
		return ui.QuadroComTecla("comando", "c", "copia", corpo, largura, ui.QuadroNormal)
	case linguagem == "":
		return ui.Quadro("código", corpo, largura, ui.QuadroNormal)
	default:
		return ui.Quadro(linguagem, corpo, largura, ui.QuadroNormal)
	}
}

// quebrarCodigo quebra uma linha de código (sem ANSI) em pedaços de até w células: no
// último espaço que cabe, fora do recuo, ou no meio da palavra maior que a linha. O hífen
// não serve de ponto de quebra, para que "-p" não vire "-" numa linha e "p" na outra.
func quebrarCodigo(l string, w int) []string {
	var saida []string
	l = strings.TrimRight(l, " ")
	for ansi.StringWidth(l) > w {
		// Até w+1 células: um espaço logo depois da última que cabe também serve.
		cabe := ansi.Cut(l, 0, w+1)
		recuo := len(cabe) - len(strings.TrimLeft(cabe, " "))
		if i := strings.LastIndex(cabe, " "); i > recuo {
			saida = append(saida, strings.TrimRight(l[:i], " "))
			l = strings.TrimLeft(l[i:], " ")
			continue
		}
		corte := ansi.Cut(l, 0, w)
		saida = append(saida, corte)
		l = l[len(corte):]
	}
	return append(saida, l)
}

// Estilo devolve o estilo do glamour derivado dos tokens de pkg/ui para o perfil de cor
// (Ascii tira as cores), o fundo (escuro: mapa noturno; claro: kraft) e a largura.
// Nada pinta fundo: código e títulos se destacam por negrito, como na carta.
func Estilo(perfil termenv.Profile, escuro bool, largura int) gansi.StyleConfig {
	c := func(cor lipgloss.CompleteAdaptiveColor) *string {
		v := cor.Light
		if escuro {
			v = cor.Dark
		}
		var s string
		switch perfil {
		case termenv.TrueColor:
			s = v.TrueColor
		case termenv.ANSI256:
			s = v.ANSI256
		case termenv.ANSI:
			s = v.ANSI
		default:
			return nil
		}
		return &s
	}
	sim := func() *bool { b := true; return &b }
	num := func(n uint) *uint { return &n }
	texto := func(s string) *string { return &s }

	g := ui.G()
	// O marcador de lista é o fio fino da carta ("─ item"; "- item" em ASCII). Os fios
	// verticais e o cruzamento seguem os cantos do ui.Quadro, com o mesmo fallback ASCII.
	marcador := g.Linha + " "
	citacao, coluna, centro := "│ ", "│", "┼"
	if ui.Atual().ASCII {
		citacao, coluna, centro = "| ", "|", "+"
	}
	tinta, suave, fio := c(ui.Tinta), c(ui.TintaSuave), c(ui.Fio)

	titulo := gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Upper: sim()}}
	return gansi.StyleConfig{
		Document:   gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: tinta}, Margin: num(0)},
		BlockQuote: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: suave}, Indent: num(1), IndentToken: texto(citacao)},
		Paragraph:  gansi.StyleBlock{},
		List:       gansi.StyleList{LevelIndent: 2},
		Heading: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{
			BlockSuffix: "\n", Color: tinta, Bold: sim(),
		}},
		// Todo nível de título sai em negrito e caixa-alta, como o título da carta.
		H1: titulo, H2: titulo, H3: titulo, H4: titulo, H5: titulo, H6: titulo,

		Text:           gansi.StylePrimitive{},
		Strikethrough:  gansi.StylePrimitive{CrossedOut: sim()},
		Emph:           gansi.StylePrimitive{Italic: sim()},
		Strong:         gansi.StylePrimitive{Bold: sim()},
		HorizontalRule: gansi.StylePrimitive{Color: fio, Format: "\n" + strings.Repeat(g.Linha, largura) + "\n"},

		Item:        gansi.StylePrimitive{BlockPrefix: marcador},
		Enumeration: gansi.StylePrimitive{BlockPrefix: ". "},
		Task:        gansi.StyleTask{Ticked: g.Feito + " ", Unticked: g.Pendente + " "},

		Link:      gansi.StylePrimitive{Color: suave, Underline: sim()},
		LinkText:  gansi.StylePrimitive{Color: tinta, Bold: sim()},
		Image:     gansi.StylePrimitive{Color: suave, Underline: sim()},
		ImageText: gansi.StylePrimitive{Color: suave, Format: "imagem: {{.text}}"},

		Code:      gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: tinta, Bold: sim()}},
		CodeBlock: gansi.StyleCodeBlock{StyleBlock: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: tinta}, Margin: num(2)}},

		Table: gansi.StyleTable{
			StyleBlock:      gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: tinta}},
			CenterSeparator: texto(centro), ColumnSeparator: texto(coluna), RowSeparator: texto(g.Linha),
		},

		DefinitionDescription: gansi.StylePrimitive{BlockPrefix: "\n" + marcador},
		HTMLBlock:             gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: suave}},
		HTMLSpan:              gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: suave}},
	}
}

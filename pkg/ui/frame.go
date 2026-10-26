package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Limiares do layout responsivo (seção 5 da especificação).
const (
	LarguraLadoALado = 120 // a partir daqui lista e detalhe ficam lado a lado (60/40)
	LarguraColuna    = 80  // de 80 a 119: uma coluna; abaixo disso, compacto
	LarguraMinima    = 60  // abaixo disso (ou de AlturaMinima) só a tela "aumente a janela"
	AlturaMinima     = 16
)

// Modo é a variante de layout escolhida para um tamanho de janela.
type Modo int

const (
	// ModoLadoALado: lista (60%) e detalhe (40%) lado a lado, a partir de 120 colunas.
	ModoLadoALado Modo = iota
	// ModoColuna: uma coluna, com o detalhe por cima da lista (80 a 119 colunas).
	ModoColuna
	// ModoCompacto: uma coluna com abas numeradas e legenda reduzida (< 80 colunas).
	ModoCompacto
	// ModoJanelaPequena: apenas o aviso "aumente a janela" (< 60x16).
	ModoJanelaPequena
)

// ModoPara escolhe o layout para a janela largura x altura.
func ModoPara(largura, altura int) Modo {
	switch {
	case largura < LarguraMinima || altura < AlturaMinima:
		return ModoJanelaPequena
	case largura < LarguraColuna:
		return ModoCompacto
	case largura < LarguraLadoALado:
		return ModoColuna
	default:
		return ModoLadoALado
	}
}

// ajustar trunca s à largura w (em células) e completa com espaços até w.
func ajustar(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "")
	}
	if falta := w - lipgloss.Width(s); falta > 0 {
		s += strings.Repeat(" ", falta)
	}
	return s
}

// linhaEsqDir compõe uma linha de exatamente w células com esq à esquerda e dir à direita.
// Se não couber, trunca dir (com reticências) para preservar o lado esquerdo.
func linhaEsqDir(esq, dir string, w int) string {
	if w <= 0 {
		return ""
	}
	le, ld := lipgloss.Width(esq), lipgloss.Width(dir)
	if le+1+ld > w {
		resto := w - le - 1
		if resto < 2 {
			return ajustar(esq, w)
		}
		dir = ansi.Truncate(dir, resto, G().Reticencias)
		ld = lipgloss.Width(dir)
	}
	return esq + strings.Repeat(" ", w-le-ld) + dir
}

// Masthead monta a linha de topo: "◆ YBY  <INSTRUMENTO>" em negrito, o rótulo em TintaSuave
// (omitido abaixo de 80 colunas) e o contexto à direita. Sempre ocupa exatamente largura células.
func Masthead(largura int, instrumento, rotulo, contexto string) string {
	s := S()
	marca := s.Negrito.Render(G().Ativo + " YBY  " + strings.ToUpper(instrumento))
	esq := marca
	if rotulo != "" && largura >= LarguraColuna {
		candidato := esq + "  " + s.Rotulo.Render(strings.ToUpper(rotulo))
		if lipgloss.Width(candidato)+2+lipgloss.Width(contexto) <= largura {
			esq = candidato
		}
	}
	return linhaEsqDir(esq, contexto, largura)
}

// CaminhoContexto formata o contexto do masthead: segmentos separados por ▸, todos em
// TintaSuave exceto o último, em Tinta.
func CaminhoContexto(segmentos ...string) string {
	s := S()
	sep := s.TintaSuave.Render(" " + G().Caminho + " ")
	partes := make([]string, len(segmentos))
	for i, seg := range segmentos {
		if i == len(segmentos)-1 {
			partes[i] = s.Tinta.Render(seg)
		} else {
			partes[i] = s.TintaSuave.Render(seg)
		}
	}
	return strings.Join(partes, sep)
}

// IndicadorVivo devolve o indicador de atualização ao vivo, por exemplo "● ao vivo · 2 s".
func IndicadorVivo(texto string) string {
	return Colorir(lipgloss.NewStyle(), Ok).Render(G().Rodando + " " + texto)
}

// Rule devolve o fio horizontal em Fio, com largura células.
func Rule(largura int) string {
	if largura <= 0 {
		return ""
	}
	return S().Fio.Render(strings.Repeat(G().Linha, largura))
}

// StatusLine devolve a linha de status: a mensagem (já formatada, com glifo de status) à
// esquerda e "atualizado HH:MM:SS" em TintaSuave à direita. Hora zero omite o carimbo.
func StatusLine(largura int, mensagem string, hora time.Time) string {
	dir := ""
	if !hora.IsZero() {
		dir = S().TintaSuave.Render("atualizado " + hora.Format("15:04:05"))
	}
	if dir == "" {
		return ajustar(mensagem, largura)
	}
	// A mensagem cede espaço ao carimbo, que tem prioridade.
	if lipgloss.Width(mensagem)+1+lipgloss.Width(dir) > largura {
		mensagem = ansi.Truncate(mensagem, largura-lipgloss.Width(dir)-1, G().Reticencias)
	}
	return linhaEsqDir(mensagem, dir, largura)
}

// itemTecla renderiza "tecla ação": tecla em negrito, ação em TintaSuave.
func itemTecla(tecla, acao string) string {
	s := S()
	return s.Negrito.Render(tecla) + " " + s.TintaSuave.Render(acao)
}

// KeyHints devolve a legenda de teclas. Considera só os bindings habilitados e com ajuda;
// "q sair" (tecla Sair do keymap) fica sempre no canto direito. Abaixo de 80 colunas mostra
// 3 teclas + "? mais"; em qualquer largura, descarta teclas do fim até caber.
func KeyHints(largura int, teclas ...key.Binding) string {
	return legendaDeTeclas(largura, DefaultKeys().Global.Sair, true, teclas...)
}

// TeclasDoPainel devolve as teclas de um painel (o rodapé do detalhe do Viz: "l logs  y yaml
// e eventos"), no formato da legenda mas sem "q sair" nem "? mais"; descarta teclas do fim
// até caber em largura.
func TeclasDoPainel(largura int, teclas ...key.Binding) string {
	return legendaDeTeclas(largura, key.Binding{}, false, teclas...)
}

// Partes reúne o conteúdo já renderizado de uma tela para Moldura.
type Partes struct {
	Instrumento, Rotulo, Contexto string // masthead (Contexto já formatado, ver CaminhoContexto)
	Abas                          string // abas completas (>= 80 colunas)
	AbasCompactas                 string // abas numeradas (< 80 colunas); se vazio, usa Abas
	Filtros                       string // linha(s) de filtros ativos; vazio omite
	Lista                         string // corpo principal
	Detalhe                       string // detalhe do item; vazio usa a largura toda para a lista
	Mensagem                      string // frase de status, já com glifo
	Hora                          time.Time
	Teclas                        []key.Binding // teclas válidas na tela, em ordem de importância
	// Sair é a tecla de saída no canto direito da legenda; vazia, "q sair". Nas camadas em
	// que q é letra ou não sai (campo, menu, ajuda), DefaultKeys().Global.SairForcado.
	Sair key.Binding
	// SemAjuda tira o "? mais" da legenda nas camadas em que o ? não abre a ajuda (campo
	// de texto, confirmação, a própria tela de ajuda).
	SemAjuda bool
}

// linhasDe divide s em linhas; string vazia vira nenhuma linha.
func linhasDe(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

// bloco ajusta s a uma caixa w x h: trunca cada linha, completa com espaços e corta/preenche
// as linhas até h.
func bloco(s string, w, h int) []string {
	ls := linhasDe(s)
	saida := make([]string, h)
	for i := range saida {
		if i < len(ls) {
			saida[i] = ajustar(ls[i], w)
		} else {
			saida[i] = strings.Repeat(" ", w)
		}
	}
	return saida
}

// telaPequena é a tela única exibida abaixo de 60x16.
func telaPequena(largura, altura int) string {
	if altura < 1 || largura < 1 {
		return ""
	}
	s := S()
	centro := func(txt string, estilo lipgloss.Style) string {
		if lipgloss.Width(txt) > largura {
			txt = ansi.Truncate(txt, largura, G().Reticencias)
		}
		esq := (largura - lipgloss.Width(txt)) / 2
		return ajustar(strings.Repeat(" ", esq)+estilo.Render(txt), largura)
	}
	linhas := make([]string, altura)
	for i := range linhas {
		linhas[i] = strings.Repeat(" ", largura)
	}
	meio := (altura - 1) / 2
	if altura >= 2 {
		meio = (altura - 2) / 2
		linhas[meio+1] = centro("mínimo 60x16", s.TintaSuave)
	}
	linhas[meio] = centro("Aumente a janela", s.Negrito)
	return strings.Join(linhas, "\n")
}

// Moldura compõe a tela inteira (largura x altura células): masthead, régua, abas, filtros,
// corpo, régua, status e legenda, escolhendo o layout por ModoPara. O resultado tem
// exatamente altura linhas de largura células, sem quebra final.
func Moldura(largura, altura int, p Partes) string {
	modo := ModoPara(largura, altura)
	if modo == ModoJanelaPequena {
		return telaPequena(largura, altura)
	}

	// O detalhe é dado: sai em Tinta (linha a linha, para o estilo não atravessar quebras).
	detalhe := linhasDe(p.Detalhe)
	for i, l := range detalhe {
		detalhe[i] = S().Tinta.Render(l)
	}
	p.Detalhe = strings.Join(detalhe, "\n")

	abas := p.Abas
	if modo == ModoCompacto && p.AbasCompactas != "" {
		abas = p.AbasCompactas
	}
	var topo []string
	topo = append(topo, Masthead(largura, p.Instrumento, p.Rotulo, p.Contexto), Rule(largura))
	for _, l := range linhasDe(abas) {
		topo = append(topo, ajustar(l, largura))
	}
	for _, l := range linhasDe(p.Filtros) {
		topo = append(topo, ajustar(l, largura))
	}
	sair := p.Sair
	if sair.Help().Key == "" {
		sair = DefaultKeys().Global.Sair
	}
	legenda := legendaDeTeclas(largura, sair, !p.SemAjuda, p.Teclas...)
	rodape := []string{
		Rule(largura),
		ajustar(StatusLine(largura, p.Mensagem, p.Hora), largura),
		legenda,
	}

	h := altura - len(topo) - len(rodape)
	if h < 1 {
		h = 1
	}

	var corpo []string
	switch {
	case p.Detalhe == "":
		corpo = bloco(p.Lista, largura, h)
	case modo == ModoLadoALado:
		corpo = ladoALado(p.Lista, p.Detalhe, largura, h)
	default:
		corpo = empilhado(p.Lista, p.Detalhe, largura, h)
	}

	todas := append(append(topo, corpo...), rodape...)
	if len(todas) > altura { // abas/filtros demais: corta o excesso do topo para manter a altura
		todas = append(todas[:altura-len(rodape)], rodape...)
	}
	return strings.Join(todas, "\n")
}

// LargurasLadoALado devolve as larguras da lista (60%) e do detalhe (40%) no corpo lado a
// lado de largura células, descontado o fio vertical entre eles: quem desenha a lista e o
// detalhe usa estas larguras para que nada seja cortado pela moldura.
func LargurasLadoALado(largura int) (lista, detalhe int) {
	const sepLargura = 3 // " │ "
	util := max(0, largura-sepLargura)
	lista = util * 60 / 100
	return lista, util - lista
}

// LimiteDetalheEmpilhado é quantas linhas o detalhe empilhado (abaixo de 120 colunas) pode
// ocupar num corpo de h linhas: até metade, menos o fio; abaixo de 6 linhas, o corpo todo.
func LimiteDetalheEmpilhado(h int) int {
	if h < 6 {
		return max(h, 0)
	}
	return (h - 1) / 2
}

// ladoALado divide o corpo em lista (60%) e detalhe (40%) com um fio vertical entre eles.
func ladoALado(lista, detalhe string, largura, h int) []string {
	wLista, wDetalhe := LargurasLadoALado(largura)
	sep := " " + S().Fio.Render(separadorVertical()) + " "
	esq, dir := bloco(lista, wLista, h), bloco(detalhe, wDetalhe, h)
	saida := make([]string, h)
	for i := range saida {
		saida[i] = esq[i] + sep + dir[i]
	}
	return saida
}

func separadorVertical() string {
	if Atual().ASCII {
		return "|"
	}
	return "│"
}

// empilhado põe o detalhe por cima da lista (até metade da altura), separados por um fio.
func empilhado(lista, detalhe string, largura, h int) []string {
	if h < 6 { // sem espaço para os dois: o detalhe prevalece
		return bloco(detalhe, largura, h)
	}
	hDet := len(linhasDe(detalhe))
	if limite := LimiteDetalheEmpilhado(h); hDet > limite {
		hDet = limite
	}
	if hDet < 1 {
		hDet = 1
	}
	hLista := h - hDet - 1
	saida := bloco(detalhe, largura, hDet)
	saida = append(saida, Rule(largura))
	return append(saida, bloco(lista, largura, hLista)...)
}

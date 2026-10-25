package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// linhasFixasTexto são as linhas da MolduraTexto fora do corpo e do campo: masthead,
// régua, régua acima do campo, régua abaixo dele, estado e legenda.
const linhasFixasTexto = 6

// PartesTexto reúne o conteúdo já renderizado de uma tela de texto (Bard, assistentes)
// para MolduraTexto.
type PartesTexto struct {
	Instrumento, Rotulo, Contexto string // masthead (Contexto já formatado, ver CaminhoContexto)
	Corpo                         string // conversa ou conteúdo principal
	Campo                         string // campo de texto (já com o prompt), entre duas réguas
	Mensagem                      string // frase de estado, já com glifo
	Direita                       string // apoio à direita da linha de estado; sai em TintaSuave
	// Sair é a tecla de saída fixa no canto direito da legenda. Nas telas de texto a letra
	// q é digitada, então a saída costuma ser ctrl+c (DefaultKeys().Global.SairForcado).
	Sair   key.Binding
	Teclas []key.Binding // teclas válidas na tela, em ordem de importância
}

// AlturaCorpoTexto devolve quantas linhas sobram para o corpo da MolduraTexto numa janela
// de altura linhas com um campo de linhasCampo linhas. Nunca devolve menos que 1.
func AlturaCorpoTexto(altura, linhasCampo int) int {
	return max(altura-linhasFixasTexto-max(linhasCampo, 0), 1)
}

// MolduraTexto compõe uma tela de texto inteira (largura x altura células): masthead,
// régua, corpo, régua, campo, régua, estado e legenda. O corpo ocupa o que sobra e fica
// alinhado ao topo. Abaixo de 60x16 mostra só "aumente a janela". O resultado tem
// exatamente altura linhas de largura células, sem quebra final.
func MolduraTexto(largura, altura int, p PartesTexto) string {
	if ModoPara(largura, altura) == ModoJanelaPequena {
		return telaPequena(largura, altura)
	}
	campo := linhasDe(p.Campo)
	topo := []string{Masthead(largura, p.Instrumento, p.Rotulo, p.Contexto), Rule(largura)}
	meio := []string{Rule(largura)}
	for _, l := range campo {
		meio = append(meio, ajustar(l, largura))
	}
	rodape := []string{
		Rule(largura),
		StatusLineTexto(largura, p.Mensagem, p.Direita),
		KeyHintsCom(largura, p.Sair, p.Teclas...),
	}
	corpo := bloco(p.Corpo, largura, AlturaCorpoTexto(altura, len(campo)))

	todas := append(append(append(topo, corpo...), meio...), rodape...)
	if len(todas) > altura { // campo alto demais: corta o excesso do campo para manter a altura
		todas = append(todas[:altura-len(rodape)], rodape...)
	}
	return strings.Join(todas, "\n")
}

// StatusLineTexto devolve a linha de estado com a mensagem (já formatada, com glifo) à
// esquerda e direita em TintaSuave no canto direito, sempre com largura células. Quando
// não cabem as duas, a mensagem cede espaço; direita vazia deixa a linha só com a mensagem.
func StatusLineTexto(largura int, mensagem, direita string) string {
	if direita == "" {
		return ajustar(mensagem, largura)
	}
	dir := S().TintaSuave.Render(direita)
	if lipgloss.Width(mensagem)+1+lipgloss.Width(dir) > largura {
		mensagem = ansi.Truncate(mensagem, max(largura-lipgloss.Width(dir)-1, 0), G().Reticencias)
	}
	return linhaEsqDir(mensagem, dir, largura)
}

// KeyHintsCom é a legenda de teclas (ver KeyHints) com a tecla de saída dada no canto
// direito em vez de "q sair". Bindings com a mesma tecla de sair, desabilitados ou sem
// ajuda ficam de fora. Abaixo de 80 colunas mostra 3 teclas + "? mais"; em qualquer
// largura, descarta teclas do fim até caber.
func KeyHintsCom(largura int, sair key.Binding, teclas ...key.Binding) string {
	return legendaDeTeclas(largura, sair, true, teclas...)
}

// legendaDeTeclas monta a legenda de KeyHintsCom. Sem comMais (camadas em que o ? é
// letra ou fecha a camada), o "? mais" nunca aparece: quando não cabe, as teclas do fim
// somem sem aviso.
func legendaDeTeclas(largura int, sair key.Binding, comMais bool, teclas ...key.Binding) string {
	ajuda := DefaultKeys().Global.Ajuda
	compacto := largura < LarguraColuna

	var itens []key.Binding
	for _, b := range teclas {
		h := b.Help()
		if !b.Enabled() || h.Key == "" || h.Desc == "" || h.Key == sair.Help().Key {
			continue
		}
		if comMais && compacto && h.Key == ajuda.Help().Key {
			continue // vira o "? mais" fixo
		}
		itens = append(itens, b)
	}
	mais := comMais && compacto
	if compacto && len(itens) > 3 {
		itens = itens[:3]
	}

	dir := ""
	if h := sair.Help(); h.Key != "" {
		dir = itemTecla(h.Key, h.Desc)
	}
	for {
		partes := make([]string, 0, len(itens)+1)
		for _, b := range itens {
			partes = append(partes, itemTecla(b.Help().Key, b.Help().Desc))
		}
		if mais {
			partes = append(partes, itemTecla(ajuda.Help().Key, "mais"))
		}
		esq := strings.Join(partes, "  ")
		if lipgloss.Width(esq)+2+lipgloss.Width(dir) <= largura || len(itens) == 0 {
			return linhaEsqDir(esq, dir, largura)
		}
		itens = itens[:len(itens)-1]
		mais = comMais
	}
}

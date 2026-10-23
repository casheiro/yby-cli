package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// LarguraCompacta é a largura abaixo da qual as abas viram números (`1 Pods · 2 Deploy…`).
const LarguraCompacta = 80

// Aba descreve uma aba de recurso. Contagem negativa oculta o número.
type Aba struct {
	Titulo   string
	Contagem int
}

// separadores das abas.
const (
	sepAbas        = "   "
	sepAbasCompact = " · "
)

// marcaMais devolve o indicador de abas ocultas.
func marcaMais() string {
	if Atual().ASCII {
		return "-> mais"
	}
	return "⇥ mais"
}

// Abas renderiza a linha de abas em no máximo largura células. A aba ativa aparece como
// `◆ Pods 8` em negrito (◆ em Ok) e as demais como `◇ Deployments 4` em TintaSuave. Quando
// não cabem todas, mostra uma janela que contém a ativa e termina com `⇥ mais`. Com
// largura < LarguraCompacta usa o modo compacto numerado (`1 Pods · 2 Deploy…`).
// Função pura: depende apenas dos argumentos e do tema atual.
func Abas(abas []Aba, ativa, largura int) string {
	if len(abas) == 0 || largura <= 0 {
		return ""
	}
	if ativa < 0 || ativa >= len(abas) {
		ativa = 0
	}
	var linha string
	if largura < LarguraCompacta {
		linha = abasCompactas(abas, ativa, largura)
	} else {
		linha = abasNormais(abas, ativa, largura)
	}
	if lipgloss.Width(linha) > largura {
		linha = ansi.Truncate(linha, largura, G().Reticencias)
	}
	return linha
}

// itemAba é uma aba já formatada: texto puro (para medir) e versão estilizada.
type itemAba struct {
	plano, estilizado string
}

func textoAba(a Aba) string {
	if a.Contagem >= 0 {
		return a.Titulo + " " + strconv.Itoa(a.Contagem)
	}
	return a.Titulo
}

func abasNormais(abas []Aba, ativa, largura int) string {
	g, s := G(), S()
	itens := make([]itemAba, len(abas))
	for i, a := range abas {
		txt := textoAba(a)
		if i == ativa {
			itens[i] = itemAba{
				plano:      g.Ativo + " " + txt,
				estilizado: s.Ok.Render(g.Ativo) + " " + s.Negrito.Render(s.Tinta.Render(txt)),
			}
		} else {
			itens[i] = itemAba{
				plano:      g.Pendente + " " + txt,
				estilizado: s.TintaSuave.Render(g.Pendente + " " + txt),
			}
		}
	}
	return juntarJanela(itens, ativa, largura, sepAbas)
}

func abasCompactas(abas []Aba, ativa, largura int) string {
	s := S()
	montar := func(maxTitulo int) []itemAba {
		itens := make([]itemAba, len(abas))
		for i, a := range abas {
			titulo := a.Titulo
			if lipgloss.Width(titulo) > maxTitulo {
				titulo = ansi.Truncate(titulo, maxTitulo, G().Reticencias)
			}
			txt := strconv.Itoa(i+1) + " " + titulo
			if i == ativa {
				itens[i] = itemAba{plano: txt, estilizado: s.Negrito.Render(s.Tinta.Render(txt))}
			} else {
				itens[i] = itemAba{plano: txt, estilizado: s.TintaSuave.Render(txt)}
			}
		}
		return itens
	}
	// Encurta os títulos aos poucos até caberem todas as abas; senão, usa a janela com "mais".
	maior := 0
	for _, a := range abas {
		if w := lipgloss.Width(a.Titulo); w > maior {
			maior = w
		}
	}
	const minimo = 5
	for m := maior; m >= minimo; m-- {
		itens := montar(m)
		if larguraItens(itens, sepAbasCompact) <= largura {
			return juntar(itens, sepAbasCompact)
		}
	}
	return juntarJanela(montar(minimo), ativa, largura, sepAbasCompact)
}

func larguraItens(itens []itemAba, sep string) int {
	total := 0
	for i, it := range itens {
		if i > 0 {
			total += lipgloss.Width(sep)
		}
		total += lipgloss.Width(it.plano)
	}
	return total
}

func juntar(itens []itemAba, sep string) string {
	partes := make([]string, len(itens))
	for i, it := range itens {
		partes[i] = it.estilizado
	}
	return strings.Join(partes, S().Fio.Render(sep))
}

// juntarJanela mostra o maior trecho de itens que cabe em largura, contendo a aba ativa,
// e acrescenta o indicador "mais" quando algum item ficou de fora.
func juntarJanela(itens []itemAba, ativa, largura int, sep string) string {
	if larguraItens(itens, sep) <= largura {
		return juntar(itens, sep)
	}
	mais := marcaMais()
	reserva := lipgloss.Width(sep) + lipgloss.Width(mais)
	for inicio := 0; inicio <= ativa; inicio++ {
		fim := inicio
		usado := 0
		for fim < len(itens) {
			extra := lipgloss.Width(itens[fim].plano)
			if fim > inicio {
				extra += lipgloss.Width(sep)
			}
			if usado+extra+reserva > largura {
				break
			}
			usado += extra
			fim++
		}
		if ativa < fim {
			return juntar(itens[inicio:fim], sep) + S().Fio.Render(sep) + S().TintaSuave.Render(mais)
		}
	}
	// Nem a ativa sozinha cabe: devolve só ela (Abas trunca).
	return itens[ativa].estilizado
}

package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Chip é um filtro ativo, exibido como `[chave: valor ✕]`. Com Valor vazio mostra só a
// Chave (útil para filtros já no formato `app=api`).
type Chip struct {
	Chave, Valor string
}

func (c Chip) texto() string {
	if c.Valor == "" {
		return c.Chave
	}
	return c.Chave + ": " + c.Valor
}

// Filtros renderiza a linha de filtros em no máximo largura células: `filtros  [status:
// com problema ✕]  [app=api ✕]` à esquerda e `N de M` (mostrando de total) à direita em
// TintaSuave. Chips que não cabem viram `+N`. Com total <= 0 o contador é omitido.
// Função pura: depende apenas dos argumentos e do tema atual.
func Filtros(chips []Chip, mostrando, total, largura int) string {
	if largura <= 0 {
		return ""
	}
	g, s := G(), S()

	direita := ""
	orcamento := largura
	if total > 0 {
		direita = strconv.Itoa(mostrando) + " de " + strconv.Itoa(total)
		orcamento = largura - lipgloss.Width(direita) - 2
	}

	const rotulo = "filtros"
	if len(chips) == 0 {
		return montarFiltros(s.TintaSuave.Render(rotulo+"  nenhum"), rotulo+"  nenhum", direita, orcamento, largura)
	}

	// Escolhe o maior número de chips que cabe, reservando espaço para o "+N" dos ocultos.
	chip := func(c Chip) (plano, estilizado string) {
		plano = "[" + c.texto() + " " + g.Falha + "]"
		estilizado = s.Tinta.Render("["+c.texto()+" ") + s.TintaSuave.Render(g.Falha) + s.Tinta.Render("]")
		return
	}
	planoTotal := rotulo
	estilTotal := s.TintaSuave.Render(rotulo)
	melhorPlano, melhorEstil := planoTotal, estilTotal
	for i, c := range chips {
		p, e := chip(c)
		planoTotal += "  " + p
		estilTotal += "  " + e
		sufixo := ""
		if ocultos := len(chips) - i - 1; ocultos > 0 {
			sufixo = "  +" + strconv.Itoa(ocultos)
		}
		if lipgloss.Width(planoTotal+sufixo) > orcamento {
			break
		}
		melhorPlano, melhorEstil = planoTotal+sufixo, estilTotal
		if sufixo != "" {
			melhorEstil += s.TintaSuave.Render(sufixo)
		}
	}
	if melhorPlano == rotulo {
		// Nenhum chip coube: indica quantos há.
		suf := "  +" + strconv.Itoa(len(chips))
		melhorPlano += suf
		melhorEstil += s.TintaSuave.Render(suf)
	}
	return montarFiltros(melhorEstil, melhorPlano, direita, orcamento, largura)
}

// montarFiltros posiciona o contador à direita e garante que nada passe da largura.
func montarFiltros(esquerda, plana, direita string, orcamento, largura int) string {
	reticencias := G().Reticencias
	if direita == "" {
		return ansi.Truncate(esquerda, largura, reticencias)
	}
	if lipgloss.Width(plana) > orcamento {
		esquerda = ansi.Truncate(esquerda, max(orcamento, 0), reticencias)
	}
	folga := largura - lipgloss.Width(esquerda) - lipgloss.Width(direita)
	if folga < 1 {
		folga = 1
	}
	linha := esquerda + strings.Repeat(" ", folga) + S().TintaSuave.Render(direita)
	if lipgloss.Width(linha) > largura {
		linha = ansi.Truncate(linha, largura, reticencias)
	}
	return linha
}

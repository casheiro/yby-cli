package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// espacoColunas é o espaço entre colunas, em células.
const espacoColunas = 2

// Coluna descreve uma coluna da tabela. Larguras em células.
// Prioridade menor é mais importante: em terminal estreito somem primeiro as colunas de
// maior Prioridade (empate: a mais à direita). A coluna de menor Prioridade nunca some.
type Coluna struct {
	Titulo     string
	Min, Max   int // Max <= 0 significa sem limite
	Prioridade int
	Direita    bool // alinha o conteúdo à direita
}

// Linha é uma linha de dados. As células podem conter ANSI (por exemplo, de Status).
// Concluida deixa a linha em TintaSuave.
type Linha struct {
	Celulas   []string
	Concluida bool
}

// TipoStatus escolhe o glifo e a cor de um Status.
type TipoStatus int

const (
	// StatusRodando é ● em Ok (Running, Ready).
	StatusRodando TipoStatus = iota
	// StatusFalha é ✕ em Erro (CrashLoopBackOff, Failed).
	StatusFalha
	// StatusAlerta é ▲ em Aviso (Pending).
	StatusAlerta
	// StatusConcluido é ✓ em TintaSuave (Completed).
	StatusConcluido
	// StatusParado é ○ em TintaSuave (Parado).
	StatusParado
)

// Status devolve glifo + cor + palavra, como `● Running`. Nunca depende só da cor.
func Status(tipo TipoStatus, texto string) string {
	g, s := G(), S()
	var glifo string
	var estilo lipgloss.Style
	switch tipo {
	case StatusFalha:
		glifo, estilo = g.Falha, s.Erro
	case StatusAlerta:
		glifo, estilo = g.Alerta, s.Aviso
	case StatusConcluido:
		glifo, estilo = g.Feito, s.TintaSuave
	case StatusParado:
		glifo, estilo = g.Parado, s.TintaSuave
	default:
		glifo, estilo = g.Rodando, s.Ok
	}
	return estilo.Render(glifo + " " + texto)
}

// Tabela renderiza o cabeçalho (rótulos em TintaSuave, caixa-alta) e as linhas em no máximo
// largura células. selecionada é o índice da linha em tinta invertida (-1 para nenhuma).
// Colunas de menor prioridade somem quando não cabem; o texto é truncado com `…` sem
// quebrar caracteres UTF-8 nem glifos largos. Função pura.
func Tabela(colunas []Coluna, linhas []Linha, selecionada, largura int) string {
	if len(colunas) == 0 || largura <= 0 {
		return ""
	}
	visiveis, larguras := ajustarColunas(colunas, linhas, largura)
	s := S()

	saida := make([]string, 0, len(linhas)+1)
	cab := make([]string, len(visiveis))
	for i, idx := range visiveis {
		cab[i] = celula(strings.ToUpper(colunas[idx].Titulo), larguras[i], colunas[idx].Direita)
	}
	saida = append(saida, s.Rotulo.Render(strings.Join(cab, strings.Repeat(" ", espacoColunas))))

	sep := strings.Repeat(" ", espacoColunas)
	for n, l := range linhas {
		sel := n == selecionada
		partes := make([]string, len(visiveis))
		for i, idx := range visiveis {
			txt := ""
			if idx < len(l.Celulas) {
				txt = l.Celulas[idx]
			}
			if sel || l.Concluida {
				txt = ansi.Strip(txt)
			}
			partes[i] = celula(txt, larguras[i], colunas[idx].Direita)
		}
		switch {
		case sel:
			// Nome (primeira coluna) em negrito; o resto na tinta invertida sem negrito.
			resto := s.Selecionado.Bold(false)
			linha := s.Selecionado.Render(partes[0])
			for _, p := range partes[1:] {
				linha += resto.Render(sep + p)
			}
			saida = append(saida, linha+resto.Render(strings.Repeat(" ", max(0, largura-lipgloss.Width(linha)))))
		case l.Concluida:
			saida = append(saida, s.TintaSuave.Render(strings.Join(partes, sep)))
		default:
			for i := range partes {
				partes[i] = s.Tinta.Render(partes[i])
			}
			saida = append(saida, strings.Join(partes, sep))
		}
	}
	return strings.Join(saida, "\n")
}

// celula ajusta txt a exatamente w células (trunca com reticências ou preenche).
func celula(txt string, w int, direita bool) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(txt) > w {
		txt = ansi.Truncate(txt, w, G().Reticencias)
	}
	falta := w - lipgloss.Width(txt)
	if falta <= 0 {
		return txt
	}
	if direita {
		return strings.Repeat(" ", falta) + txt
	}
	return txt + strings.Repeat(" ", falta)
}

// ajustarColunas decide quais colunas aparecem (índices em ordem original) e a largura de cada.
func ajustarColunas(colunas []Coluna, linhas []Linha, largura int) ([]int, []int) {
	minimo := func(c Coluna) int { return max(c.Min, 1) }

	// Ordem de importância: prioridade menor primeiro; empate, a mais à esquerda primeiro.
	ordem := make([]int, len(colunas))
	for i := range ordem {
		ordem[i] = i
	}
	sort.SliceStable(ordem, func(a, b int) bool { return colunas[ordem[a]].Prioridade < colunas[ordem[b]].Prioridade })

	// Mantém colunas, da mais importante à menos, enquanto o mínimo delas couber.
	manter := map[int]bool{}
	usado := 0
	for _, idx := range ordem {
		extra := minimo(colunas[idx])
		if len(manter) > 0 {
			extra += espacoColunas
		}
		if len(manter) > 0 && usado+extra > largura {
			continue
		}
		manter[idx] = true
		usado += extra
	}
	visiveis := make([]int, 0, len(manter))
	for i := range colunas {
		if manter[i] {
			visiveis = append(visiveis, i)
		}
	}

	// Largura ideal: maior conteúdo, limitado por Min e Max.
	larguras := make([]int, len(visiveis))
	total := espacoColunas * (len(visiveis) - 1)
	for i, idx := range visiveis {
		c := colunas[idx]
		w := lipgloss.Width(c.Titulo)
		for _, l := range linhas {
			if idx < len(l.Celulas) {
				w = max(w, lipgloss.Width(l.Celulas[idx]))
			}
		}
		w = max(w, minimo(c))
		if c.Max > 0 {
			w = min(w, max(c.Max, minimo(c)))
		}
		larguras[i] = w
		total += w
	}
	// Encolhe a coluna com mais folga acima do mínimo até caber.
	for total > largura {
		melhor, folga := -1, 0
		for i, idx := range visiveis {
			if f := larguras[i] - minimo(colunas[idx]); f > folga {
				melhor, folga = i, f
			}
		}
		if melhor < 0 {
			break
		}
		larguras[melhor]--
		total--
	}
	// Mínimos somados maiores que a largura (uma coluna única larga): corta a última.
	if total > largura {
		larguras[len(larguras)-1] = max(1, larguras[len(larguras)-1]-(total-largura))
	}
	return visiveis, larguras
}

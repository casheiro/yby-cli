package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// EstadoEtapa descreve em que ponto uma etapa está.
type EstadoEtapa int

const (
	// EtapaPendente ainda não começou (◇).
	EtapaPendente EstadoEtapa = iota
	// EtapaAtual está em andamento (◈).
	EtapaAtual
	// EtapaFeita terminou com sucesso (✓).
	EtapaFeita
	// EtapaFalha terminou com erro (✕).
	EtapaFalha
)

// Etapa é uma linha da lista de etapas de um processo.
type Etapa struct {
	Rotulo  string
	Estado  EstadoEtapa
	Duracao time.Duration // exibida à direita quando maior que zero
	Nota    string        // complemento em TintaSuave após o rótulo (ex.: "criado"); opcional
	Fracao  float64       // progresso (0 a 1) da etapa atual; maior que zero troca o rótulo por uma cota
}

// FormatarDuracao formata um tempo em pt-BR: "0,3 s", "38 s", "1 min 12 s".
// Abaixo de 10 s usa uma casa decimal com vírgula; acima, segundos inteiros.
func FormatarDuracao(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < 10*time.Second {
		decimos := int((d + 50*time.Millisecond) / (100 * time.Millisecond))
		if decimos >= 100 {
			return "10 s"
		}
		return fmt.Sprintf("%d,%d s", decimos/10, decimos%10)
	}
	seg := int((d + 500*time.Millisecond) / time.Second)
	if seg < 60 {
		return fmt.Sprintf("%d s", seg)
	}
	min, resto := seg/60, seg%60
	if resto == 0 {
		return fmt.Sprintf("%d min", min)
	}
	return fmt.Sprintf("%d min %d s", min, resto)
}

// truncarCelulas corta s para caber em n células, terminando em reticências.
func truncarCelulas(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	return ansi.Truncate(s, n, G().Reticencias)
}

// Etapas renderiza a lista de etapas, uma por linha, com o tempo alinhado à direita.
// largura é a largura total em células; com largura <= 0 usa o tamanho natural.
func Etapas(etapas []Etapa, largura int) string {
	s, g := S(), G()
	tempos := make([]string, len(etapas))
	maxTempo, maxRotulo := 0, 0
	for i, e := range etapas {
		if e.Duracao > 0 && e.Estado != EtapaPendente {
			tempos[i] = FormatarDuracao(e.Duracao)
		}
		maxTempo = max(maxTempo, lipgloss.Width(tempos[i]))
		maxRotulo = max(maxRotulo, lipgloss.Width(e.Rotulo))
	}
	comLargura := largura > 0
	if largura <= 0 {
		largura = 2 + maxRotulo + 3 + maxTempo
		for _, e := range etapas {
			largura = max(largura, 2+lipgloss.Width(e.Rotulo)+3+lipgloss.Width(e.Nota)+2+maxTempo)
		}
	}
	// Reserva 2 células do glifo, 2 de respiro e a coluna de tempo.
	reserva := 2
	if maxTempo > 0 {
		reserva += maxTempo + 2
	}
	linhas := make([]string, 0, len(etapas))
	for i, e := range etapas {
		var glifo string
		switch e.Estado {
		case EtapaFeita:
			glifo = s.Ok.Render(g.Feito)
		case EtapaAtual:
			glifo = s.Negrito.Render(g.Andamento)
		case EtapaFalha:
			glifo = s.Erro.Render(g.Falha)
		default:
			glifo = s.TintaSuave.Render(g.Pendente)
		}
		var rotulo string
		if e.Estado == EtapaAtual && e.Fracao > 0 {
			wCota := 0 // tamanho padrão quando a largura é natural
			if largura > 0 && comLargura {
				wCota = max(largura-reserva, 1)
			}
			rotulo = Cota(e.Rotulo, e.Fracao, wCota)
		} else {
			rotulo = truncarCelulas(e.Rotulo, max(largura-reserva, 1))
			switch e.Estado {
			case EtapaAtual:
				rotulo = s.Negrito.Render(rotulo)
			case EtapaPendente:
				rotulo = s.TintaSuave.Render(rotulo)
			}
			if e.Nota != "" {
				if sobra := largura - reserva - lipgloss.Width(e.Rotulo) - 3; sobra > 0 {
					rotulo += "   " + s.TintaSuave.Render(truncarCelulas(e.Nota, sobra))
				}
			}
		}
		esquerda := glifo + " " + rotulo
		if tempos[i] != "" {
			folga := largura - lipgloss.Width(esquerda) - lipgloss.Width(tempos[i])
			esquerda += strings.Repeat(" ", max(folga, 2)) + s.TintaSuave.Render(tempos[i])
		}
		linhas = append(linhas, esquerda)
	}
	return strings.Join(linhas, "\n")
}

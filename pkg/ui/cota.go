package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// larguraBarraPadrao é a largura da barra quando a largura total não é informada.
const larguraBarraPadrao = 24

// limiteCotaPerto é a fração a partir da qual o tempo restante vira alerta.
const limiteCotaPerto = 0.85

func limitarFracao(f float64) float64 {
	switch {
	case f < 0:
		return 0
	case f > 1:
		return 1
	}
	return f
}

// Cota renderiza uma barra de cota com percentual à direita:
// "argo cd  ━━━━━━┄┄┄┄  64%". fracao vai de 0 a 1; largura é a total em células
// (<= 0 usa um tamanho padrão).
func Cota(rotulo string, fracao float64, largura int) string {
	direita := fmt.Sprintf("%d%%", int(limitarFracao(fracao)*100+0.5))
	return montarCota(rotulo, fracao, largura, S().TintaSuave.Render(direita), lipgloss.Width(direita))
}

// CotaTempo é como Cota, mas mostra o tempo restante à direita; perto do fim
// (fracao >= 85%) o tempo aparece em Aviso.
func CotaTempo(rotulo string, fracao float64, restante time.Duration, largura int) string {
	texto := FormatarDuracao(restante)
	estilo := S().TintaSuave
	if limitarFracao(fracao) >= limiteCotaPerto {
		estilo = S().Aviso
	}
	return montarCota(rotulo, fracao, largura, estilo.Render(texto), lipgloss.Width(texto))
}

// CotaRestante é a Cota de uma validade (token, certificado): a barra mostra a fração que
// ainda resta e, à direita, o texto pronto do tempo ("11 min", "3 min"). Quem chama decide
// o aviso, porque aqui a barra vazia é que está perto do fim (na CotaTempo é a cheia); com
// aviso o texto sai em Aviso. Rótulo vazio começa direto na barra.
func CotaRestante(rotulo string, fracao float64, texto string, aviso bool, largura int) string {
	estilo := S().TintaSuave
	if aviso {
		estilo = S().Aviso
	}
	return montarCota(rotulo, fracao, largura, estilo.Render(texto), lipgloss.Width(texto))
}

func montarCota(rotulo string, fracao float64, largura int, direita string, wDireita int) string {
	s, g := S(), G()
	barra := larguraBarraPadrao
	if largura > 0 {
		// rótulo, 2 espaços, barra, 2 espaços, valor (sem rótulo, sem os 2 primeiros).
		wRotulo := lipgloss.Width(rotulo) + 2
		if rotulo == "" {
			wRotulo = 0
		}
		barra = largura - wRotulo - wDireita - 2
		if barra < 4 {
			barra = 4
			if rotulo != "" {
				rotulo = truncarCelulas(rotulo, max(largura-wDireita-4-barra, 1))
			}
		}
	}
	cheio := int(limitarFracao(fracao)*float64(barra) + 0.5)
	inicio := ""
	if rotulo != "" {
		inicio = s.Tinta.Render(rotulo) + "  "
	}
	return inicio +
		s.Tinta.Render(strings.Repeat(g.Cheio, cheio)) +
		s.Fio.Render(strings.Repeat(g.Tracejado, barra-cheio)) +
		"  " + direita
}

// Package ui reúne a identidade visual das TUIs e saídas do Yby CLI ("carta do terminal"):
// paleta, glifos, tema e estilos base compartilhados.
package ui

import (
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// cor monta um CompleteAdaptiveColor com as variantes truecolor, ANSI 256 e ANSI 16
// para o mapa noturno (Dark) e para o kraft (Light).
func cor(escuroTC, escuro256, escuro16, claroTC, claro256, claro16 string) lipgloss.CompleteAdaptiveColor {
	return lipgloss.CompleteAdaptiveColor{
		Dark:  lipgloss.CompleteColor{TrueColor: escuroTC, ANSI256: escuro256, ANSI: escuro16},
		Light: lipgloss.CompleteColor{TrueColor: claroTC, ANSI256: claro256, ANSI: claro16},
	}
}

// Papéis de cor da paleta. O fundo é sempre o do terminal: nenhum componente pinta fundo.
var (
	// Tinta é a cor de texto e títulos.
	Tinta = cor("#DDD0B5", "187", "15", "#33261A", "236", "0")
	// TintaSuave é a cor de rótulos e apoio.
	TintaSuave = cor("#A1937A", "144", "7", "#5C4A36", "240", "8")
	// Fio é a cor de bordas, réguas e trechos restantes (decorativa, fora do teste de contraste).
	Fio = cor("#4A4232", "238", "8", "#A88E6A", "137", "7")
	// Ok indica sucesso, ativo e rodando.
	Ok = cor("#A7BC72", "143", "10", "#4E5F2E", "58", "2")
	// Aviso indica alerta.
	Aviso = cor("#D9AE5F", "179", "11", "#7D5A10", "94", "3")
	// Erro indica falha e ações destrutivas.
	Erro = cor("#D98361", "173", "9", "#86331A", "88", "1")
)

// luminancia calcula a luminância relativa WCAG 2.x de uma cor "#RRGGBB".
func luminancia(hex string) float64 {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return math.NaN()
	}
	var canais [3]float64
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		if err != nil {
			return math.NaN()
		}
		c := float64(v) / 255
		if c <= 0.03928 {
			canais[i] = c / 12.92
		} else {
			canais[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*canais[0] + 0.7152*canais[1] + 0.0722*canais[2]
}

// Contraste devolve a razão de contraste WCAG 2.x (1 a 21) entre duas cores "#RRGGBB".
// Devolve NaN se alguma das cores for inválida.
func Contraste(hexA, hexB string) float64 {
	a, b := luminancia(hexA), luminancia(hexB)
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}

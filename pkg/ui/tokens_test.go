package ui

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestContraste_Referencias(t *testing.T) {
	if c := Contraste("#000000", "#FFFFFF"); math.Abs(c-21) > 0.01 {
		t.Errorf("preto/branco = %.2f, esperado 21", c)
	}
	if c := Contraste("#777777", "#777777"); math.Abs(c-1) > 0.001 {
		t.Errorf("mesma cor = %.2f, esperado 1", c)
	}
	if c := Contraste("#FFFFFF", "#000000"); math.Abs(c-21) > 0.01 {
		t.Errorf("simetria quebrada: %.2f", c)
	}
	if c := Contraste("xyz", "#000000"); !math.IsNaN(c) {
		t.Errorf("cor inválida deveria dar NaN, deu %.2f", c)
	}
	if c := Contraste("#GGGGGG", "#000000"); !math.IsNaN(c) {
		t.Errorf("hex inválido deveria dar NaN, deu %.2f", c)
	}
}

type papel struct {
	nome string
	cor  lipgloss.CompleteAdaptiveColor
}

var papeis = []papel{
	{"Tinta", Tinta}, {"TintaSuave", TintaSuave}, {"Fio", Fio},
	{"Ok", Ok}, {"Aviso", Aviso}, {"Erro", Erro},
}

func TestTokens_CodigosPorPerfilETema(t *testing.T) {
	// Sequências SGR esperadas de primeiro plano, por papel, na ordem de `papeis`.
	casos := []struct {
		nome   string
		escuro bool
		perfil termenv.Profile
		esp    []string
	}{
		{"escuro truecolor", true, termenv.TrueColor, []string{
			"38;2;221;208;181", "38;2;161;147;122", "38;2;74;66;50", "38;2;167;188;114", "38;2;217;174;95", "38;2;217;131;97"}},
		{"claro truecolor", false, termenv.TrueColor, []string{
			"38;2;51;38;26", "38;2;92;74;54", "38;2;168;142;106", "38;2;78;95;46", "38;2;125;90;16", "38;2;134;51;26"}},
		{"escuro 256", true, termenv.ANSI256, []string{
			"38;5;187", "38;5;144", "38;5;238", "38;5;143", "38;5;179", "38;5;173"}},
		{"claro 256", false, termenv.ANSI256, []string{
			"38;5;236", "38;5;240", "38;5;137", "38;5;58", "38;5;94", "38;5;88"}},
		{"escuro 16", true, termenv.ANSI, []string{"97", "37", "90", "92", "93", "91"}},
		{"claro 16", false, termenv.ANSI, []string{"30", "90", "37", "32", "33", "31"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarTema(t, c.escuro, c.perfil, false)
			for i, p := range papeis {
				got := lipgloss.NewStyle().Foreground(p.cor).Render("x")
				if !contemSGR(got, c.esp[i]) {
					t.Errorf("%s: saída %q não contém SGR %q", p.nome, got, c.esp[i])
				}
			}
		})
	}
}

func TestTokens_SemCorNaoEmiteEscape(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	Configurar(Opcoes{Tema: TemaEscuro, SemCor: true})
	for _, p := range papeis {
		if got := Colorir(lipgloss.NewStyle(), p.cor).Render("x"); got != "x" {
			t.Errorf("%s: esperado texto puro, obtido %q", p.nome, got)
		}
	}
}

func TestTokens_TodosOsCamposPreenchidos(t *testing.T) {
	for _, p := range papeis {
		for nome, c := range map[string]lipgloss.CompleteColor{"Dark": p.cor.Dark, "Light": p.cor.Light} {
			if c.TrueColor == "" || c.ANSI256 == "" || c.ANSI == "" {
				t.Errorf("%s.%s com variante vazia: %+v", p.nome, nome, c)
			}
		}
	}
}

// Fundos típicos de terminal (seção 1 da especificação).
var (
	fundosEscuros = []string{"#000000", "#0C0C0C", "#1E1E1E", "#1F1C15"}
	fundosClaros  = []string{"#FFFFFF", "#FAFAFA", "#F2F2F2"}
)

func TestTokens_ContrasteWCAG(t *testing.T) {
	const minimo = 4.5
	// Fio é decorativo e fica fora.
	for _, p := range papeis {
		if p.nome == "Fio" {
			continue
		}
		for _, f := range fundosEscuros {
			if c := Contraste(p.cor.Dark.TrueColor, f); c < minimo {
				t.Errorf("%s escuro %s sobre %s: contraste %.2f < %.1f", p.nome, p.cor.Dark.TrueColor, f, c, minimo)
			}
		}
		for _, f := range fundosClaros {
			if c := Contraste(p.cor.Light.TrueColor, f); c < minimo {
				t.Errorf("%s claro %s sobre %s: contraste %.2f < %.1f", p.nome, p.cor.Light.TrueColor, f, c, minimo)
			}
		}
	}
}

// contemSGR verifica se a saída contém a sequência SGR esperada. Em truecolor aceita
// diferença de 1 por canal, pois o termenv converte hex por ponto flutuante e arredonda para baixo.
func contemSGR(saida, esperado string) bool {
	if strings.Contains(saida, "["+esperado+"m") {
		return true
	}
	if !strings.HasPrefix(esperado, "38;2;") {
		return false
	}
	var r, g, b int
	if _, err := fmt.Sscanf(esperado, "38;2;%d;%d;%d", &r, &g, &b); err != nil {
		return false
	}
	for dr := -1; dr <= 1; dr++ {
		for dg := -1; dg <= 1; dg++ {
			for db := -1; db <= 1; db++ {
				alt := fmt.Sprintf("[38;2;%d;%d;%dm", r+dr, g+dg, b+db)
				if strings.Contains(saida, alt) {
					return true
				}
			}
		}
	}
	return false
}

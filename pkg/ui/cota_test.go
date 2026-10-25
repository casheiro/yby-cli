package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestCotaRestante_SemRotuloComecaNaBarra(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	got := CotaRestante("", 0.5, "30 min", false, 20)
	if w := lipgloss.Width(got); w != 20 {
		t.Fatalf("largura %d, esperado 20: %q", w, got)
	}
	// 20 = barra 12 + 2 espaços + "30 min"; metade da barra cheia.
	if want := "======------  30 min"; got != want {
		t.Errorf("obtido %q, esperado %q", got, want)
	}
}

func TestCotaRestante_ComRotuloIgualACota(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	got := CotaRestante("token", 0.25, "15 min", false, 40)
	if w := lipgloss.Width(got); w != 40 {
		t.Fatalf("largura %d, esperado 40: %q", w, got)
	}
	if !strings.HasPrefix(got, "token  ") || !strings.HasSuffix(got, "  15 min") {
		t.Errorf("formato inesperado: %q", got)
	}
}

func TestCotaRestante_AvisoPintaOTempo(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	com := CotaRestante("", 0.05, "3 min", true, 16)
	sem := CotaRestante("", 0.05, "3 min", false, 16)
	if !strings.Contains(com, S().Aviso.Render("3 min")) {
		t.Errorf("com aviso, o tempo deveria sair em Aviso: %q", com)
	}
	if strings.Contains(sem, S().Aviso.Render("3 min")) {
		t.Errorf("sem aviso, o tempo não deveria sair em Aviso: %q", sem)
	}
	if ansi.Strip(com) != ansi.Strip(sem) {
		t.Errorf("o aviso só muda a cor: %q x %q", ansi.Strip(com), ansi.Strip(sem))
	}
}

func TestCotaRestante_LarguraPequenaMantemBarraMinima(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	got := CotaRestante("", 1, "59 min", false, 6)
	if want := "====  59 min"; got != want {
		t.Errorf("obtido %q, esperado %q", got, want)
	}
}

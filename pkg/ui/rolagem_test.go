package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestAvisoRolagem(t *testing.T) {
	casos := []struct {
		ascii    bool
		seta     Seta
		esperado string
	}{
		{false, SetaAcima, "↑ 2 mensagens anteriores"},
		{false, SetaAbaixo, "↓ 2 mensagens anteriores"},
		{true, SetaAcima, "^ 2 mensagens anteriores"},
		{true, SetaAbaixo, "v 2 mensagens anteriores"},
	}
	for _, c := range casos {
		usarTema(t, true, termenv.Ascii, c.ascii)
		got := AvisoRolagem(c.seta, "2 mensagens anteriores")
		if got != c.esperado {
			t.Errorf("ascii=%v seta=%d: %q, esperado %q", c.ascii, c.seta, got, c.esperado)
		}
		if lipgloss.Width(got) != lipgloss.Width(c.esperado) {
			t.Errorf("a seta deve ocupar 1 célula: %q", got)
		}
	}
}

func TestAvisoRolagemTintaSuave(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	got := AvisoRolagem(SetaAbaixo, "1 mensagem nova")
	if got != S().TintaSuave.Render("↓ 1 mensagem nova") {
		t.Errorf("o aviso deve sair em TintaSuave: %q", got)
	}
	if ansi.Strip(got) != "↓ 1 mensagem nova" {
		t.Errorf("texto inesperado: %q", ansi.Strip(got))
	}
}

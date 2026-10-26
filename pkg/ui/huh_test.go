package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestHuhThemeEstilos(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	th := HuhTheme()
	if got := th.Focused.Title.Render("nome"); !strings.Contains(got, "NOME") {
		t.Errorf("título deveria ir em caixa-alta: %q", got)
	}
	if got := ansi.Strip(th.Focused.TextInput.Prompt.String()); got != "› " {
		t.Errorf("prompt = %q", got)
	}
	if got := th.Focused.SelectSelector.String(); !strings.Contains(got, "◆") {
		t.Errorf("seletor = %q", got)
	}
	if !th.Focused.Title.GetBold() {
		t.Error("título deveria ser negrito")
	}
	if th.Focused.ErrorMessage.GetForeground() == (lipgloss.NoColor{}) {
		t.Error("erro deveria ter cor")
	}
}

func TestHuhThemeASCIIeSemCor(t *testing.T) {
	usarTema(t, true, termenv.Ascii, true)
	Configurar(Opcoes{Tema: TemaEscuro, ASCII: true, SemCor: true})
	th := HuhTheme()
	if got := th.Focused.SelectSelector.String(); !strings.Contains(got, "*") {
		t.Errorf("seletor ASCII = %q", got)
	}
	if th.Focused.ErrorMessage.GetForeground() != (lipgloss.NoColor{}) {
		t.Error("NO_COLOR não deveria ter cor")
	}
	if th.Blurred.Title.GetBold() {
		t.Error("título desfocado não deveria ser negrito")
	}
}

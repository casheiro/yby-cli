package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/pkg/config"
	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// isolarModoVisual aponta a home para um diretório temporário (com o config.yaml dado,
// se houver) e restaura o modo visual, o perfil do lipgloss e o config no fim do teste.
func isolarModoVisual(t *testing.T, configYAML string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, k := range []string{"YBY_THEME", "YBY_ASCII", "NO_COLOR"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	if configYAML != "" {
		dir := filepath.Join(home, ".yby")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(configYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	opcoes, escuro, perfil := yui.Atual(), lipgloss.HasDarkBackground(), lipgloss.ColorProfile()
	config.ResetGlobal()
	t.Cleanup(func() {
		config.ResetGlobal()
		yui.Configurar(opcoes)
		lipgloss.SetHasDarkBackground(escuro)
		lipgloss.SetColorProfile(perfil)
	})
}

func ambiente(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

// TestConfigurarVisual_Ambiente: YBY_THEME, YBY_ASCII e NO_COLOR valem no
// processo do plugin, como no core.
func TestConfigurarVisual_Ambiente(t *testing.T) {
	isolarModoVisual(t, "")
	lipgloss.SetColorProfile(termenv.TrueColor)
	configurarVisual(ambiente(map[string]string{"YBY_THEME": "light", "YBY_ASCII": "1", "NO_COLOR": "1"}))

	if o := yui.Atual(); o.Tema != yui.TemaClaro || !o.ASCII || !o.SemCor {
		t.Errorf("YBY_THEME, YBY_ASCII e NO_COLOR valem no plugin: %+v", o)
	}
	if lipgloss.HasDarkBackground() {
		t.Error("YBY_THEME=light deixa o fundo claro")
	}
	if yui.G().Ativo != yui.ASCII.Ativo {
		t.Error("YBY_ASCII=1 troca os glifos")
	}
}

// TestConfigurarVisual_ConfigGlobal: sem env, valem ui.theme e ui.ascii do
// ~/.yby/config.yaml.
func TestConfigurarVisual_ConfigGlobal(t *testing.T) {
	isolarModoVisual(t, "ui:\n  theme: dark\n  ascii: true\n")
	lipgloss.SetHasDarkBackground(false)
	configurarVisual(ambiente(map[string]string{"LANG": "pt_BR.UTF-8"}))

	if o := yui.Atual(); o.Tema != yui.TemaEscuro || !o.ASCII || o.SemCor {
		t.Errorf("ui.theme e ui.ascii do config.yaml valem sem env: %+v", o)
	}
	if !lipgloss.HasDarkBackground() {
		t.Error("ui.theme=dark deixa o fundo escuro")
	}
}

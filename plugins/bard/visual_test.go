package main

import (
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/config"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// isolarModoVisual aponta a home para um diretório temporário (com o config.yaml dado,
// se houver) e restaura o modo visual, o perfil do lipgloss e o config no fim do teste.
func isolarModoVisual(t *testing.T, configYAML string) {
	t.Helper()
	terminalAnterior := ui.SaidaNoTerminal
	ui.SaidaNoTerminal = func() bool { return true }
	t.Cleanup(func() { ui.SaidaNoTerminal = terminalAnterior })
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
	opcoes, escuro, perfil := ui.Atual(), lipgloss.HasDarkBackground(), lipgloss.ColorProfile()
	config.ResetGlobal()
	t.Cleanup(func() {
		config.ResetGlobal()
		ui.Configurar(opcoes)
		lipgloss.SetHasDarkBackground(escuro)
		lipgloss.SetColorProfile(perfil)
	})
}

func ambiente(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestConfigurarModoVisual_Ambiente(t *testing.T) {
	isolarModoVisual(t, "")
	lipgloss.SetColorProfile(termenv.TrueColor)
	configurarModoVisual(ambiente(map[string]string{"YBY_THEME": "light", "YBY_ASCII": "1", "NO_COLOR": "1"}))

	o := ui.Atual()
	if o.Tema != ui.TemaClaro || !o.ASCII || !o.SemCor {
		t.Errorf("YBY_THEME, YBY_ASCII e NO_COLOR valem no plugin: %+v", o)
	}
	if lipgloss.HasDarkBackground() {
		t.Error("YBY_THEME=light deixa o fundo claro (kraft)")
	}
	// Sem cor é o perfil ANSI, não o Ascii: negrito e Reverse continuam.
	if p := lipgloss.ColorProfile(); p != termenv.ANSI {
		t.Errorf("NO_COLOR no perfil ANSI, obtido %v", p)
	}
	if ui.G().Prompt != ui.ASCII.Prompt {
		t.Error("YBY_ASCII=1 troca os glifos")
	}
}

func TestConfigurarModoVisual_ConfigGlobal(t *testing.T) {
	isolarModoVisual(t, "ui:\n  theme: dark\n  ascii: true\n")
	lipgloss.SetHasDarkBackground(false)
	configurarModoVisual(ambiente(map[string]string{"LANG": "pt_BR.UTF-8"}))

	if o := ui.Atual(); o.Tema != ui.TemaEscuro || !o.ASCII || o.SemCor {
		t.Errorf("ui.theme e ui.ascii do config.yaml valem sem env: %+v", o)
	}
	if !lipgloss.HasDarkBackground() {
		t.Error("ui.theme=dark deixa o fundo escuro")
	}
}

func TestResolverTerminal(t *testing.T) {
	isolarModoVisual(t, "")
	ui.Configurar(ui.Opcoes{Tema: ui.TemaClaro})
	lipgloss.SetHasDarkBackground(false)
	resolverTerminal()
	if lipgloss.HasDarkBackground() {
		t.Error("tema forçado não é trocado pela detecção")
	}
	ui.Configurar(ui.Opcoes{Tema: ui.TemaAuto})
	lipgloss.SetHasDarkBackground(true)
	resolverTerminal()
	if !lipgloss.HasDarkBackground() {
		t.Error("no tema auto o fundo fica o já detectado")
	}
}

func TestLogDaConversa_VaiParaArquivoEVolta(t *testing.T) {
	isolarModoVisual(t, "")
	home, _ := os.UserHomeDir()
	restaurar := logDaConversa()
	slog.Warn("aviso durante a conversa")
	log.Print("linha do pacote log")
	restaurar()

	conteudo, err := os.ReadFile(filepath.Join(home, ".yby", "logs", "bard.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, esperado := range []string{"aviso durante a conversa", "linha do pacote log"} {
		if !strings.Contains(string(conteudo), esperado) {
			t.Errorf("o log da conversa deveria ter %q, veio %q", esperado, conteudo)
		}
	}
	if log.Writer() == nil {
		t.Error("o pacote log deveria voltar ao destino anterior")
	}
}

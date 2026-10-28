package ui

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/v5/pkg/config"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolverOpcoes(t *testing.T) {
	utf8 := map[string]string{"LANG": "pt_BR.UTF-8"}
	com := func(extra map[string]string) map[string]string {
		m := map[string]string{}
		for k, v := range utf8 {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	casos := []struct {
		nome     string
		goos     string
		flag     Opcoes
		flagDef  bool
		env      map[string]string
		cfg      config.UIConfig
		esperado Opcoes
	}{
		{"padrão com UTF-8", "linux", Opcoes{}, false, utf8, config.UIConfig{}, Opcoes{}},
		{"NO_COLOR vazio é ignorado", "linux", Opcoes{}, false, com(map[string]string{"NO_COLOR": ""}), config.UIConfig{}, Opcoes{}},
		{"NO_COLOR qualquer valor", "linux", Opcoes{}, false, com(map[string]string{"NO_COLOR": "1"}), config.UIConfig{}, Opcoes{SemCor: true}},
		{"YBY_THEME dark", "linux", Opcoes{}, false, com(map[string]string{"YBY_THEME": "dark"}), config.UIConfig{}, Opcoes{Tema: TemaEscuro}},
		{"YBY_THEME light", "linux", Opcoes{}, false, com(map[string]string{"YBY_THEME": "LIGHT"}), config.UIConfig{}, Opcoes{Tema: TemaClaro}},
		{"YBY_THEME auto", "linux", Opcoes{}, false, com(map[string]string{"YBY_THEME": "auto"}), config.UIConfig{Theme: "dark"}, Opcoes{Tema: TemaAuto}},
		{"YBY_THEME inválido cai na config", "linux", Opcoes{}, false, com(map[string]string{"YBY_THEME": "roxo"}), config.UIConfig{Theme: "light"}, Opcoes{Tema: TemaClaro}},
		{"config theme dark", "linux", Opcoes{}, false, utf8, config.UIConfig{Theme: "dark"}, Opcoes{Tema: TemaEscuro}},
		{"config theme inválido", "linux", Opcoes{}, false, utf8, config.UIConfig{Theme: "x"}, Opcoes{}},
		{"env vence config no tema", "linux", Opcoes{}, false, com(map[string]string{"YBY_THEME": "light"}), config.UIConfig{Theme: "dark"}, Opcoes{Tema: TemaClaro}},
		{"flag vence env no tema", "linux", Opcoes{Tema: TemaEscuro}, true, com(map[string]string{"YBY_THEME": "light"}), config.UIConfig{}, Opcoes{Tema: TemaEscuro}},
		{"flag não definida é ignorada", "linux", Opcoes{Tema: TemaEscuro, ASCII: true, SemCor: true}, false, utf8, config.UIConfig{}, Opcoes{}},
		{"flag SemCor", "linux", Opcoes{SemCor: true}, true, utf8, config.UIConfig{}, Opcoes{SemCor: true}},
		{"YBY_ASCII=1", "linux", Opcoes{}, false, com(map[string]string{"YBY_ASCII": "1"}), config.UIConfig{}, Opcoes{ASCII: true}},
		{"YBY_ASCII=true", "linux", Opcoes{}, false, com(map[string]string{"YBY_ASCII": "true"}), config.UIConfig{}, Opcoes{ASCII: true}},
		{"YBY_ASCII=0 força Unicode mesmo sem UTF-8", "linux", Opcoes{}, false, map[string]string{"YBY_ASCII": "0", "LANG": "C"}, config.UIConfig{}, Opcoes{}},
		{"YBY_ASCII=0 vence config", "linux", Opcoes{}, false, com(map[string]string{"YBY_ASCII": "0"}), config.UIConfig{ASCII: true}, Opcoes{}},
		{"config ascii", "linux", Opcoes{}, false, utf8, config.UIConfig{ASCII: true}, Opcoes{ASCII: true}},
		{"flag ASCII", "linux", Opcoes{ASCII: true}, true, utf8, config.UIConfig{}, Opcoes{ASCII: true}},
		{"LANG sem UTF-8", "linux", Opcoes{}, false, map[string]string{"LANG": "C"}, config.UIConfig{}, Opcoes{ASCII: true}},
		{"sem locale algum", "linux", Opcoes{}, false, map[string]string{}, config.UIConfig{}, Opcoes{ASCII: true}},
		{"LC_ALL UTF-8 vence LANG C", "linux", Opcoes{}, false, map[string]string{"LC_ALL": "en_US.utf8", "LANG": "C"}, config.UIConfig{}, Opcoes{}},
		{"LC_ALL C vence LANG UTF-8", "linux", Opcoes{}, false, map[string]string{"LC_ALL": "C", "LANG": "pt_BR.UTF-8"}, config.UIConfig{}, Opcoes{ASCII: true}},
		{"LC_CTYPE UTF-8", "linux", Opcoes{}, false, map[string]string{"LC_CTYPE": "UTF-8"}, config.UIConfig{}, Opcoes{}},
		{"Windows sem terminal moderno", "windows", Opcoes{}, false, map[string]string{}, config.UIConfig{}, Opcoes{ASCII: true}},
		{"Windows Terminal", "windows", Opcoes{}, false, map[string]string{"WT_SESSION": "abc"}, config.UIConfig{}, Opcoes{}},
		{"Windows VS Code", "windows", Opcoes{}, false, map[string]string{"TERM_PROGRAM": "vscode"}, config.UIConfig{}, Opcoes{}},
		{"Windows outro TERM_PROGRAM", "windows", Opcoes{}, false, map[string]string{"TERM_PROGRAM": "mintty"}, config.UIConfig{}, Opcoes{ASCII: true}},
		{"Windows com locale UTF-8", "windows", Opcoes{}, false, utf8, config.UIConfig{}, Opcoes{}},
		{"WT_SESSION fora do Windows não conta", "linux", Opcoes{}, false, map[string]string{"WT_SESSION": "abc"}, config.UIConfig{}, Opcoes{ASCII: true}},
		{"combinação total", "linux", Opcoes{}, false,
			map[string]string{"NO_COLOR": "1", "YBY_THEME": "light", "YBY_ASCII": "1"}, config.UIConfig{},
			Opcoes{Tema: TemaClaro, SemCor: true, ASCII: true}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := resolverOpcoes(c.flag, c.flagDef, env(c.env), c.cfg, c.goos)
			if got != c.esperado {
				t.Errorf("obtido %+v, esperado %+v", got, c.esperado)
			}
		})
	}
}

func TestResolverOpcoes_Publica(t *testing.T) {
	got := ResolverOpcoes(Opcoes{}, false, env(map[string]string{"LANG": "C.UTF-8", "YBY_THEME": "dark"}), config.UIConfig{})
	if got.Tema != TemaEscuro || got.ASCII || got.SemCor {
		t.Errorf("inesperado: %+v", got)
	}
}

func TestConfigurar_AplicaTemaEPerfil(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)

	Configurar(Opcoes{Tema: TemaClaro})
	if lipgloss.HasDarkBackground() {
		t.Error("tema claro deveria desligar fundo escuro")
	}
	if Atual().Tema != TemaClaro {
		t.Errorf("Atual() = %+v", Atual())
	}
	Configurar(Opcoes{Tema: TemaEscuro})
	if !lipgloss.HasDarkBackground() {
		t.Error("tema escuro deveria ligar fundo escuro")
	}

	// Auto não mexe no fundo detectado.
	Configurar(Opcoes{Tema: TemaAuto})
	if !lipgloss.HasDarkBackground() {
		t.Error("tema auto não deveria alterar a detecção")
	}

	Configurar(Opcoes{SemCor: true})
	if lipgloss.ColorProfile() != termenv.ANSI {
		t.Error("SemCor deveria forçar o perfil ANSI")
	}
}

func TestEstilos_RespeitamTema(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	s := S()
	if got := s.Ok.Render("x"); !contemSGR(got, "38;2;167;188;114") {
		t.Errorf("Ok escuro: %q", got)
	}
	if got := s.Rotulo.Render("x"); !contemSGR(got, "38;2;161;147;122") {
		t.Errorf("Rotulo deveria ser TintaSuave: %q", got)
	}
	if got := s.Selecionado.Render("x"); !strings.Contains(got, "\x1b[1;7m") && !strings.Contains(got, "7") {
		t.Errorf("Selecionado sem reverse: %q", got)
	}
	if got := s.Negrito.Render("x"); !strings.Contains(got, "[1;38;2;221;208;181m") {
		t.Errorf("Negrito: %q", got)
	}
	Configurar(Opcoes{Tema: TemaClaro})
	if got := S().Ok.Render("x"); !contemSGR(got, "38;2;78;95;46") {
		t.Errorf("Ok claro: %q", got)
	}
	for _, e := range []lipgloss.Style{s.Tinta, s.TintaSuave, s.Fio, s.Aviso, s.Erro} {
		if e.Render("x") == "x" {
			t.Error("estilo sem cor com perfil TrueColor")
		}
	}
}

func TestSemCor_MantemNegritoEReverse(t *testing.T) {
	usarTema(t, true, termenv.TrueColor, false)
	Configurar(Opcoes{Tema: TemaEscuro, SemCor: true})
	e := S()
	sel := e.Selecionado.Render("x")
	if !strings.Contains(sel, ";7m") && !strings.Contains(sel, "[7m") && !strings.Contains(sel, "[7;") {
		t.Errorf("Selecionado sem reverse: %q", sel)
	}
	for nome, got := range map[string]string{
		"Selecionado": sel, "Negrito": e.Negrito.Render("x"), "Ok": e.Ok.Render("x"),
		"Rotulo": e.Rotulo.Render("x"), "Erro": Colorir(lipgloss.NewStyle(), Erro).Render("x"),
	} {
		if strings.Contains(got, "38;") || strings.Contains(got, "48;") || strings.Contains(got, "[3") || strings.Contains(got, ";3") {
			t.Errorf("%s com código de cor: %q", nome, got)
		}
	}
	if !strings.Contains(e.Negrito.Render("x"), "1m") {
		t.Errorf("Negrito perdido: %q", e.Negrito.Render("x"))
	}
}

// Fora do terminal (pipe, arquivo), NO_COLOR não força o perfil ANSI: o negrito e o
// Reverse virariam escape no meio do texto.
func TestConfigurar_SemCorForaDoTerminalSemEscape(t *testing.T) {
	usarTema(t, true, termenv.Ascii, false)
	SaidaNoTerminal = func() bool { return false }
	Configurar(Opcoes{SemCor: true})
	if p := lipgloss.ColorProfile(); p != termenv.Ascii {
		t.Errorf("perfil = %v, quer Ascii fora do terminal", p)
	}
	var b strings.Builder
	Title(&b, "cloud", "status")
	if strings.Contains(b.String(), "\x1b[") {
		t.Errorf("escape na saída fora do terminal: %q", b.String())
	}
}

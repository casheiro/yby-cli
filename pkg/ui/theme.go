package ui

import (
	"runtime"
	"strings"
	"sync"

	"github.com/casheiro/yby-cli/v5/pkg/config"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Tema escolhe entre o mapa noturno e o kraft.
type Tema int

const (
	// TemaAuto usa a detecção do lipgloss (na dúvida, escuro).
	TemaAuto Tema = iota
	// TemaEscuro força o mapa noturno.
	TemaEscuro
	// TemaClaro força o kraft.
	TemaClaro
)

// Opcoes descreve o modo visual resolvido.
type Opcoes struct {
	Tema   Tema
	ASCII  bool // glifos ASCII
	SemCor bool // sem cores (NO_COLOR)
}

var (
	mu    sync.RWMutex
	atual Opcoes
)

func parseTema(s string) (Tema, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "auto":
		return TemaAuto, true
	case "dark", "escuro":
		return TemaEscuro, true
	case "light", "claro":
		return TemaClaro, true
	}
	return TemaAuto, false
}

func parseBool(s string) (valor, definido bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on", "sim":
		return true, true
	case "0", "false", "no", "off", "nao", "não":
		return false, true
	}
	return false, false
}

// terminalUTF8 informa se o ambiente declara UTF-8 (locale) ou, no Windows sem locale,
// é um terminal conhecido por suportá-lo (Windows Terminal, VS Code).
func terminalUTF8(getenv func(string) string, goos string) bool {
	for _, k := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := getenv(k); v != "" {
			u := strings.ToUpper(v)
			return strings.Contains(u, "UTF-8") || strings.Contains(u, "UTF8")
		}
	}
	if goos == "windows" {
		return getenv("WT_SESSION") != "" || getenv("TERM_PROGRAM") == "vscode"
	}
	return false
}

// ResolverOpcoes resolve o modo visual com a precedência flags > env > config > padrão.
// flagDefinida indica se o usuário passou flags (campos zero de flag são ignorados).
// É pura: lê o ambiente apenas por getenv.
func ResolverOpcoes(flag Opcoes, flagDefinida bool, getenv func(string) string, cfg config.UIConfig) Opcoes {
	return resolverOpcoes(flag, flagDefinida, getenv, cfg, runtime.GOOS)
}

func resolverOpcoes(flag Opcoes, flagDefinida bool, getenv func(string) string, cfg config.UIConfig, goos string) Opcoes {
	var o Opcoes

	// Tema: flag > YBY_THEME > config > auto.
	if flagDefinida && flag.Tema != TemaAuto {
		o.Tema = flag.Tema
	} else if t, ok := parseTema(getenv("YBY_THEME")); ok && strings.TrimSpace(getenv("YBY_THEME")) != "" {
		o.Tema = t
	} else if t, ok := parseTema(cfg.Theme); ok {
		o.Tema = t
	}

	// Sem cor: flag ou NO_COLOR não vazio.
	o.SemCor = (flagDefinida && flag.SemCor) || getenv("NO_COLOR") != ""

	// ASCII: flag > YBY_ASCII > config > detecção de UTF-8.
	if flagDefinida && flag.ASCII {
		o.ASCII = true
	} else if v, ok := parseBool(getenv("YBY_ASCII")); ok {
		o.ASCII = v
	} else if cfg.ASCII {
		o.ASCII = true
	} else {
		o.ASCII = !terminalUTF8(getenv, goos)
	}
	return o
}

// SaidaNoTerminal diz se o stdout é um terminal. Os testes a trocam para ver o perfil que
// Configurar aplica num terminal.
var SaidaNoTerminal = ttyStdout

// Configurar aplica as opções no renderer padrão do lipgloss e no conjunto de glifos.
func Configurar(o Opcoes) {
	mu.Lock()
	atual = o
	mu.Unlock()

	switch o.Tema {
	case TemaEscuro:
		lipgloss.SetHasDarkBackground(true)
	case TemaClaro:
		lipgloss.SetHasDarkBackground(false)
	}
	if o.SemCor && SaidaNoTerminal() {
		// ANSI (não Ascii): o perfil Ascii descartaria também negrito e Reverse. Fora do
		// terminal fica o Ascii que o lipgloss detecta, sem escape nenhum no pipe.
		lipgloss.SetColorProfile(termenv.ANSI)
	}
}

// Atual devolve as opções aplicadas por Configurar.
func Atual() Opcoes {
	mu.RLock()
	defer mu.RUnlock()
	return atual
}

// Estilos reúne os estilos base por papel. São resolvidos na hora de renderizar,
// então respeitam tema e perfil de cor configurados.
type Estilos struct {
	Tinta, TintaSuave, Fio, Ok, Aviso, Erro lipgloss.Style
	Negrito                                 lipgloss.Style
	// Rotulo é TintaSuave; a caixa-alta é feita pelo chamador (strings.ToUpper).
	Rotulo lipgloss.Style
	// Selecionado é a tinta invertida (Reverse + negrito), que funciona com NO_COLOR.
	Selecionado lipgloss.Style
}

// S devolve os estilos base.
func S() Estilos {
	fg := func(c lipgloss.TerminalColor) lipgloss.Style { return Colorir(lipgloss.NewStyle(), c) }
	return Estilos{
		Tinta:       fg(Tinta),
		TintaSuave:  fg(TintaSuave),
		Fio:         fg(Fio),
		Ok:          fg(Ok),
		Aviso:       fg(Aviso),
		Erro:        fg(Erro),
		Negrito:     Colorir(lipgloss.NewStyle().Bold(true), Tinta),
		Rotulo:      fg(TintaSuave),
		Selecionado: Colorir(lipgloss.NewStyle().Reverse(true).Bold(true), Tinta),
	}
}

// Colorir aplica a cor de primeiro plano em s apenas quando há cor (sem SemCor).
// Os componentes devem usá-la em vez de chamar .Foreground diretamente.
func Colorir(s lipgloss.Style, c lipgloss.TerminalColor) lipgloss.Style {
	if Atual().SemCor {
		return s
	}
	return s.Foreground(c)
}

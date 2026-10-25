package main

import (
	"github.com/casheiro/yby-cli/pkg/config"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/lipgloss"
)

// configurarModoVisual aplica no processo do plugin o modo visual do core: YBY_THEME,
// YBY_ASCII e NO_COLOR > ui.theme e ui.ascii do ~/.yby/config.yaml > padrão. O Bard roda
// em outro processo, então o ui.Configurar do cmd/root.go não chega até aqui; sem isto a
// TUI ficaria sempre no tema detectado, com glifos Unicode e, com NO_COLOR, no perfil
// Ascii do lipgloss, que tira também o negrito e o cursor invertido.
func configurarModoVisual(getenv func(string) string) {
	ui.Configurar(ui.ResolverOpcoes(ui.Opcoes{}, false, getenv, config.Get().UI))
}

// resolverTerminal fixa o fundo e o perfil de cor antes de a TUI assumir o terminal: no
// tema auto, a primeira consulta do lipgloss ao fundo (OSC 11) aconteceria com o Bubble
// Tea já lendo a entrada.
func resolverTerminal() {
	if ui.Atual().Tema == ui.TemaAuto {
		lipgloss.SetHasDarkBackground(lipgloss.HasDarkBackground())
	}
	lipgloss.SetColorProfile(lipgloss.ColorProfile())
}

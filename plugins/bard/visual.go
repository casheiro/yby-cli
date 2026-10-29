package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/casheiro/yby-cli/v5/pkg/config"
	"github.com/casheiro/yby-cli/v5/pkg/logger"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
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

// logDaConversa manda o log para ~/.yby/logs/bard.log enquanto a TUI está na tela: uma
// linha de log no terminal (o download do modelo de embeddings, um aviso do Synapstor)
// empurraria a moldura uma linha para cima e deixaria restos na tela. Se o arquivo não
// abrir, o log some até a TUI fechar. Devolve a função que volta o log para o stderr.
func logDaConversa() (restaurar func()) {
	cfg := config.Get()
	logger.InitGlobal(logger.Config{Level: cfg.Log.Level, Format: cfg.Log.Format})
	var destino io.Writer = io.Discard
	var arquivo *os.File
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".yby", "logs")
		if err := os.MkdirAll(dir, 0o755); err == nil {
			if f, err := os.OpenFile(filepath.Join(dir, "bard.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
				destino, arquivo = f, f
			}
		}
	}
	devolver := logger.Desviar(destino)
	anterior := log.Writer()
	log.SetOutput(destino)
	return func() {
		devolver()
		log.SetOutput(anterior)
		if arquivo != nil {
			_ = arquivo.Close()
		}
	}
}

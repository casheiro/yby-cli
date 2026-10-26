package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/config"
	"github.com/casheiro/yby-cli/pkg/plugin"
	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	"github.com/casheiro/yby-cli/plugins/viz/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// Tipos do protocolo de plugins, os mesmos do core.
type (
	PluginManifest = plugin.PluginManifest
	PluginRequest  = plugin.PluginRequest
	PluginResponse = plugin.PluginResponse
)

// configurarVisual aplica no processo do plugin o modo visual do core: YBY_THEME,
// YBY_ASCII e NO_COLOR > ui.theme e ui.ascii do ~/.yby/config.yaml > padrão. O Viz
// roda em outro processo, então o ui.Configurar do cmd/root.go não chega até aqui.
// Depois fixa o fundo e o perfil de cor antes de a TUI assumir o terminal: no tema
// auto, a primeira consulta do lipgloss ao fundo (OSC 11) aconteceria com o Bubble
// Tea já lendo a entrada.
func configurarVisual(getenv func(string) string) {
	yui.Configurar(yui.ResolverOpcoes(yui.Opcoes{}, false, getenv, config.Get().UI))
	if yui.Atual().Tema == yui.TemaAuto {
		lipgloss.SetHasDarkBackground(lipgloss.HasDarkBackground())
	}
	lipgloss.SetColorProfile(lipgloss.ColorProfile())
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var rootCmd = &cobra.Command{
		Use:   "viz",
		Short: "Yby Viz: observabilidade do cluster no terminal",
		RunE: func(cmd *cobra.Command, args []string) error {
			configurarVisual(os.Getenv)
			client, err := monitor.NewK8sClient()
			if err != nil {
				// Passa nil para mostrar a falha de conexão na TUI
				p := tea.NewProgram(ui.NewModelComErro(err), tea.WithAltScreen())
				if _, err := p.Run(); err != nil {
					return fmt.Errorf("ops, ocorreu um erro: %w", err)
				}
				return nil
			}
			// O client-go (o watch, os avisos da API) escreve pelo klog no stderr,
			// por cima da TUI: no Viz, ele não escreve nada.
			klog.SetLogger(logr.Discard())
			retryClient := monitor.NewRetryClient(client)
			p := tea.NewProgram(ui.NewModel(retryClient), tea.WithAltScreen(), tea.WithMouseCellMotion())
			final, err := p.Run()
			if m, ok := final.(ui.Model); ok {
				m.Parar() // o watch para com o programa, mesmo sem passar pelo q
			}
			if err != nil {
				return fmt.Errorf("ops, ocorreu um erro: %w", err)
			}
			return nil
		},
	}

	var manifestCmd = &cobra.Command{
		Use:    "manifest",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifest := PluginManifest{
				Name:        "viz",
				Description: "Observabilidade visual no terminal (Dashboards TUI)",
				Version:     "1.0.0",
				Hooks:       []string{"command"},
			}
			if err := json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"data": manifest,
			}); err != nil {
				return fmt.Errorf("falha ao codificar manifest: %w", err)
			}
			return nil
		},
	}

	rootCmd.AddCommand(manifestCmd)

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		var req PluginRequest
		if err := json.NewDecoder(os.Stdin).Decode(&req); err == nil {
			if req.Hook == "manifest" {
				manifest := PluginManifest{
					Name:        "viz",
					Description: "Observabilidade visual no terminal (Dashboards TUI)",
					Version:     "1.0.0",
					Hooks:       []string{"command"},
				}
				if err := json.NewEncoder(os.Stdout).Encode(PluginResponse{Data: manifest}); err != nil {
					return fmt.Errorf("falha ao codificar resposta: %w", err)
				}
				return nil
			}
		}
	}

	return rootCmd.Execute()
}

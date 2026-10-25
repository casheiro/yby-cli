package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	"github.com/casheiro/yby-cli/plugins/viz/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// Tipos do protocolo de plugins, os mesmos do core.
type (
	PluginManifest = plugin.PluginManifest
	PluginRequest  = plugin.PluginRequest
	PluginResponse = plugin.PluginResponse
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var rootCmd = &cobra.Command{
		Use:   "viz",
		Short: "Yby Viz - Observability TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := monitor.NewK8sClient()
			if err != nil {
				// Passa nil para mostrar mensagem de erro na TUI
				p := tea.NewProgram(ui.NewModel(nil), tea.WithAltScreen())
				if _, err := p.Run(); err != nil {
					return fmt.Errorf("ops, ocorreu um erro: %w", err)
				}
				return nil
			}
			retryClient := monitor.NewRetryClient(client)
			p := tea.NewProgram(ui.NewModel(retryClient), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
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

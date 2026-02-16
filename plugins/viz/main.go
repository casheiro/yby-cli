package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/plugins/viz/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type PluginManifest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Hooks       []string `json:"hooks"`
}

type PluginRequest struct {
	Hook string `json:"hook"`
}

type PluginResponse struct {
	Data interface{} `json:"data"`
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "viz",
		Short: "Yby Viz - Observability TUI",
		Run: func(cmd *cobra.Command, args []string) {
			// Inicia o programa Bubbletea com o modelo definido em internal/ui
			p := tea.NewProgram(ui.NewModel())
			if _, err := p.Run(); err != nil {
				fmt.Printf("Ops, ocorreu um erro: %v", err)
				os.Exit(1)
			}
		},
	}

	var manifestCmd = &cobra.Command{
		Use:    "manifest",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			manifest := PluginManifest{
				Name:        "viz",
				Description: "Observabilidade visual no terminal (Dashboards TUI)",
				Version:     "0.1.0",
				Hooks:       []string{"command"},
			}
			if err := json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"data": manifest,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "failed to encode manifest: %v\n", err)
				os.Exit(1)
			}
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
					Version:     "0.1.0",
					Hooks:       []string{"command"},
				}
				if err := json.NewEncoder(os.Stdout).Encode(PluginResponse{Data: manifest}); err != nil {
					fmt.Fprintf(os.Stderr, "failed to encode response: %v\n", err)
					os.Exit(1)
				}
				return
			}
		}
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

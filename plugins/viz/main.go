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
				fmt.Printf("Alas, there's been an error: %v", err)
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
				Description: "Observability TUI Plugin",
				Version:     "0.1.0",
				Hooks:       []string{"command"},
			}
			json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"data": manifest,
			})
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
					Description: "Observability TUI Plugin",
					Version:     "0.1.0",
					Hooks:       []string{"command"},
				}
				json.NewEncoder(os.Stdout).Encode(PluginResponse{Data: manifest})
				return
			}
		}
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

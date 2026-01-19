package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/plugins/forge/internal/engine"
	"github.com/casheiro/yby-cli/plugins/forge/internal/mods"
	"github.com/spf13/cobra"
)

type PluginManifest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Hooks       []string `json:"hooks"`
}

type PluginRequest struct {
	Hook    string                 `json:"hook"`
	Args    []string               `json:"args"`
	Context map[string]interface{} `json:"context"`
}

type PluginResponse struct {
	Data interface{} `json:"data"`
	Err  string      `json:"err,omitempty"`
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "forge",
		Short: "Yby Forge - Engineering Automation Plugin",
		Long:  `Automação de refatoração e manutenção de código para projetos Yby.`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	var applyCmd = &cobra.Command{
		Use:   "apply [dir]",
		Short: "Aplica refatorações automáticas",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			fmt.Printf("Forge: Iniciando engine em %s...\n", dir)
			eng := engine.NewEngine()

			// Registro dos mods
			eng.Register(&mods.LogMod{})

			if err := eng.Run(dir); err != nil {
				fmt.Printf("Erro na execução: %v\n", err)
				os.Exit(1)
			}
		},
	}

	var manifestCmd = &cobra.Command{
		Use:    "manifest",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			manifest := PluginManifest{
				Name:        "forge",
				Description: "Engineering Automation Plugin",
				Version:     "0.1.0",
				Hooks:       []string{"command"},
			}
			json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"data": manifest,
			})
		},
	}

	rootCmd.AddCommand(applyCmd, manifestCmd)

	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		var req PluginRequest
		if err := json.NewDecoder(os.Stdin).Decode(&req); err == nil {
			if req.Hook == "manifest" {
				manifest := PluginManifest{
					Name:        "forge",
					Description: "Engineering Automation Plugin",
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

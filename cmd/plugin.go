/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/spf13/cobra"
)

// pluginCmd represents the plugin command
var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Gerencia plugins do YBY CLI",
	Long:  `Lista, instala e gerencia plugins que estendem as funcionalidades da CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista plugins instalados",
	Run: func(cmd *cobra.Command, args []string) {
		pm := plugin.NewManager()
		if err := pm.Discover(); err != nil {
			fmt.Printf("Erro ao descobrir plugins: %v\n", err)
			os.Exit(1)
		}

		plugins := pm.ListPlugins()
		if len(plugins) == 0 {
			fmt.Println("Nenhum plugin encontrado.")
			return
		}

		fmt.Println("🔌 Plugins Instalados:")
		for _, p := range plugins {
			fmt.Printf("- %s (v%s): Hooks [%v]\n", p.Name, p.Version, p.Hooks)
		}
	},
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install [path|name]",
	Short: "Instala um plugin a partir de um arquivo local ou nome (atlas, bard, sentinel)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pm := plugin.NewManager()
		// Version resolution
		targetVersion := Version
		if v, _ := cmd.Flags().GetString("version"); v != "" {
			targetVersion = v
		}

		force, _ := cmd.Flags().GetBool("force")

		if err := pm.Install(args[0], targetVersion, force); err != nil {
			fmt.Printf("❌ Erro ao instalar plugin: %v\n", err)
			os.Exit(1)
		}
	},
}

var pluginRemoveCmd = &cobra.Command{
	Use:     "remove [name]",
	Aliases: []string{"rm", "uninstall", "delete"},
	Short:   "Remove um plugin instalado",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pm := plugin.NewManager()
		if err := pm.Remove(args[0]); err != nil {
			fmt.Printf("❌ Erro ao remover plugin: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✅ Plugin '%s' removido com sucesso.\n", args[0])
	},
}

var pluginUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Atualiza um ou todos os plugins instalados",
	Run: func(cmd *cobra.Command, args []string) {
		pm := plugin.NewManager()
		// Ensure discovery happens
		if err := pm.Discover(); err != nil {
			fmt.Printf("⚠️  Erro ao descobrir plugins: %v\n", err)
		}

		targets := args
		if len(args) == 0 {
			// Update all
			for _, p := range pm.ListPlugins() {
				targets = append(targets, p.Name)
			}
		}

		if len(targets) == 0 {
			fmt.Println("Nenhum plugin instalado para atualizar.")
			return
		}

		hasError := false
		for _, name := range targets {
			if err := pm.Update(name); err != nil {
				fmt.Printf("❌ Falha ao atualizar '%s': %v\n", name, err)
				hasError = true
			} else {
				fmt.Printf("✅ Plugin '%s' atualizado.\n", name)
			}
		}

		if hasError {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginRemoveCmd)
	pluginCmd.AddCommand(pluginUpdateCmd)

	// Flags for Install
	pluginInstallCmd.Flags().String("version", "", "Versão específica para instalar (ex: v1.0.0)")
	pluginInstallCmd.Flags().BoolP("force", "f", false, "Forçar reinstalação se já existir")
}

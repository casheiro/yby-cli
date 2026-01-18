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
		// Version is defined in version.go within the same package
		if err := pm.Install(args[0], Version); err != nil {
			fmt.Printf("❌ Erro ao instalar plugin: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
}

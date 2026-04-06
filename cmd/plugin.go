/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/spf13/cobra"
)

// newPluginManager é a factory para criação do gerenciador de plugins (mockável em testes)
var newPluginManager = func() *plugin.Manager {
	return plugin.NewManager()
}

// pluginCmd represents the plugin command
var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Gerencia plugins do YBY CLI",
	Long:  `Lista, instala e gerencia plugins que estendem as funcionalidades da CLI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = cmd.Help()
		return nil
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "Lista plugins instalados",
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := newPluginManager()
		if err := pm.Discover(); err != nil {
			return errors.Wrap(err, errors.ErrCodePlugin, "Erro ao descobrir plugins")
		}

		plugins := pm.ListPlugins()
		if len(plugins) == 0 {
			fmt.Println("Nenhum plugin encontrado.")
			return nil
		}

		fmt.Println("🔌 Plugins Instalados:")
		for _, p := range plugins {
			fmt.Printf("- %s (v%s): Hooks [%s]\n", p.Name, p.Version, strings.Join(p.Hooks, ", "))
		}
		return nil
	},
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install [path|name]",
	Short: "Instala um plugin a partir de um arquivo local ou nome (atlas, bard, sentinel)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := newPluginManager()
		// Version resolution
		targetVersion := Version
		if v, _ := cmd.Flags().GetString("version"); v != "" {
			targetVersion = v
		}

		force, _ := cmd.Flags().GetBool("force")

		if err := pm.Install(args[0], targetVersion, force); err != nil {
			return errors.Wrap(err, errors.ErrCodePlugin, "Erro ao instalar plugin")
		}
		return nil
	},
}

var pluginRemoveCmd = &cobra.Command{
	Use:     "remove [name]",
	Aliases: []string{"rm", "uninstall", "delete"},
	Short:   "Remove um plugin instalado",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := newPluginManager()
		if err := pm.Remove(args[0]); err != nil {
			return errors.Wrap(err, errors.ErrCodePlugin, "Erro ao remover plugin")
		}
		fmt.Printf("✅ Plugin '%s' removido com sucesso.\n", args[0])
		return nil
	},
}

var pluginUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Atualiza um ou todos os plugins instalados",
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := newPluginManager()
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
			return nil
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
			return errors.New(errors.ErrCodePlugin, "Ocorreram erros durante a atualização de um ou mais plugins")
		}
		return nil
	},
}

var pluginTrustCmd = &cobra.Command{
	Use:   "trust [name]",
	Short: "Registra um plugin como confiável na whitelist",
	Long:  `Calcula o SHA256 do binário do plugin e o registra como confiável. Plugins não registrados não podem ser executados.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := newPluginManager()
		if err := pm.Discover(); err != nil {
			return errors.Wrap(err, errors.ErrCodePlugin, "Erro ao descobrir plugins")
		}

		p, found := pm.GetPlugin(args[0])
		if !found {
			return errors.New(errors.ErrCodePlugin, fmt.Sprintf("Plugin '%s' não encontrado. Instale-o primeiro com 'yby plugin install %s'", args[0], args[0]))
		}

		if err := plugin.TrustPlugin(p.Path); err != nil {
			return errors.Wrap(err, errors.ErrCodePlugin, "Erro ao registrar confiança do plugin")
		}

		fmt.Printf("Plugin '%s' registrado como confiável.\n", args[0])
		return nil
	},
}

var pluginUntrustCmd = &cobra.Command{
	Use:   "untrust [name]",
	Short: "Remove um plugin da whitelist de confiança",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pluginName := args[0]
		if !strings.HasPrefix(pluginName, "yby-plugin-") {
			pluginName = "yby-plugin-" + pluginName
		}

		if err := plugin.UntrustPlugin(pluginName); err != nil {
			return errors.Wrap(err, errors.ErrCodePlugin, "Erro ao remover confiança do plugin")
		}

		fmt.Printf("Plugin '%s' removido da whitelist de confiança.\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginRemoveCmd)
	pluginCmd.AddCommand(pluginUpdateCmd)
	pluginCmd.AddCommand(pluginTrustCmd)
	pluginCmd.AddCommand(pluginUntrustCmd)

	// Flags for Install
	pluginInstallCmd.Flags().String("version", "", "Versão específica para instalar (ex: v1.0.0)")
	pluginInstallCmd.Flags().BoolP("force", "f", false, "Forçar reinstalação se já existir")
}

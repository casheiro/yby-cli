package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/spf13/cobra"
)

var contextFlag string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yby",
	Short: "Yby - Zero-Touch Kubernetes Automation",
	Long: `Yby CLI: Ferramenta oficial para automação e gerenciamento de clusters
seguindo os princípios GitOps.`,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
	PersistentPreRun: initConfig,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Dynamic Plugin Discovery
	// We scan for plugins before executing the root command so they are available as subcommands.
	pm := plugin.NewManager()
	if err := pm.Discover(); err == nil {
		for _, p := range pm.ListPlugins() {
			// Check if plugin supports "command" hook
			hasCommandHook := false
			for _, h := range p.Hooks {
				if h == "command" {
					hasCommandHook = true
					break
				}
			}

			if hasCommandHook {
				// Avoid collision with existing commands
				if _, _, err := rootCmd.Find([]string{p.Name}); err == nil {
					continue
				}

				// Register dynamic command
				pluginName := p.Name
				desc := p.Description
				if desc == "" {
					desc = fmt.Sprintf("Executa o plugin %s", pluginName)
				}
				cmd := &cobra.Command{
					Use:                pluginName,
					Short:              desc,
					DisableFlagParsing: true, // Pass flags directly to plugin
					Run: func(cmd *cobra.Command, args []string) {
						if err := pm.ExecuteCommandHook(pluginName, args); err != nil {
							fmt.Printf("Erro ao executar plugin %s: %v\n", pluginName, err)
							os.Exit(1)
						}
					},
				}
				rootCmd.AddCommand(cmd)
			}
		}
	}

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&contextFlag, "context", "c", "", "Define o contexto de execução (ex: local, staging, prod)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Mensagem de ajuda para alternância")
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cmd *cobra.Command, args []string) {
	// If context flag is set, we override using the standard Env Var mechanism
	// capable of being read by pkg/context
	if contextFlag != "" {
		os.Setenv("YBY_ENV", contextFlag)
	}
}

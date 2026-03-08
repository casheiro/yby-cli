package cmd

import (
	stdErr "errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/logger"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/telemetry"
	"github.com/spf13/cobra"
)

var (
	contextFlag   string
	logLevelFlag  string
	logFormatFlag string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yby",
	Short: "Yby - Developer Experience & Infrastructure Assistant",
	Long: `Yby CLI: Plataforma de Engenharia e Assistente Inteligente para clusters Kubernetes.
Atua no bootstrap, governança e operação assistida, complementando o uso do kubectl.`,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
	SilenceErrors:    true,
	SilenceUsage:     true,
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
					RunE: func(cmd *cobra.Command, args []string) error {
						if err := pm.ExecuteCommandHook(pluginName, args); err != nil {
							return errors.Wrap(err, errors.ErrCodePlugin, fmt.Sprintf("Erro ao executar plugin %s", pluginName))
						}
						return nil
					},
				}
				rootCmd.AddCommand(cmd)
			}
		}
	}

	start := time.Now()
	err := rootCmd.Execute()

	telemetry.Record("yby-cli", time.Since(start), err)
	telemetry.Flush()

	if err != nil {
		var yerr *errors.YbyError
		if stdErr.As(err, &yerr) {
			if logLevelFlag == "debug" {
				// Na flag verbose/debug, printa o stack trace verboso %+v
				slog.Error("Falha na execução", "code", yerr.Code, "details", fmt.Sprintf("%+v", yerr))
			} else {
				// Se for normal, printa só a mensagem controlada
				slog.Error("Falha na execução", "code", yerr.Code, "message", yerr.Message)
			}
		} else {
			slog.Error("Falha inesperada", "erro", err)
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&contextFlag, "context", "c", "", "Define o contexto de execução (ex: local, staging, prod)")
	rootCmd.PersistentFlags().StringVar(&logLevelFlag, "log-level", "info", "Nível de log (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormatFlag, "log-format", "text", "Formato de log (text, json)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Mensagem de ajuda para alternância")
}

// initConfig reads in config file and ENV variables if set.
func initConfig(cmd *cobra.Command, args []string) {
	// Initialize Global Logger
	logger.InitGlobal(logger.Config{
		Level:  logLevelFlag,
		Format: logFormatFlag,
	})

	// If context flag is set, we override using the standard Env Var mechanism
	// capable of being read by pkg/context
	if contextFlag != "" {
		os.Setenv("YBY_ENV", contextFlag)
	}
}

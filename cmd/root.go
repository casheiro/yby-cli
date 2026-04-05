package cmd

import (
	stdErr "errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/casheiro/yby-cli/pkg/config"
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

// newRootPluginManager é uma factory mockável para testes
var newRootPluginManager = plugin.NewManager

// discoverPlugins escaneia e registra plugins como subcomandos do rootCmd.
func discoverPlugins(cmd *cobra.Command, pm *plugin.Manager) {
	if err := pm.Discover(); err == nil {
		for _, p := range pm.ListPlugins() {
			// Verifica se o plugin suporta o hook "command"
			hasCommandHook := false
			for _, h := range p.Hooks {
				if h == "command" {
					hasCommandHook = true
					break
				}
			}

			if hasCommandHook {
				// Evita colisão com comandos existentes
				// Find retorna o root cmd sem erro quando subcomando não existe,
				// então comparamos o cmd retornado com o root para detectar colisão real
				if foundCmd, _, err := cmd.Find([]string{p.Name}); err == nil && foundCmd != cmd {
					continue
				}

				// Registra comando dinâmico
				pluginName := p.Name
				desc := p.Description
				if desc == "" {
					desc = fmt.Sprintf("Executa o plugin %s", pluginName)
				}
				pluginCmd := &cobra.Command{
					Use:                pluginName,
					Short:              desc,
					DisableFlagParsing: true, // Passa flags diretamente ao plugin
					RunE: func(c *cobra.Command, args []string) error {
						if err := pm.ExecuteCommandHook(pluginName, args); err != nil {
							return errors.Wrap(err, errors.ErrCodePlugin, fmt.Sprintf("Erro ao executar plugin %s", pluginName))
						}
						return nil
					},
				}
				cmd.AddCommand(pluginCmd)
			}
		}
	}
}

// handleExecutionError trata erros de execução, diferenciando YbyError de erros genéricos.
// Exibe hints (sugestões de correção) quando disponíveis.
func handleExecutionError(err error) {
	var yerr *errors.YbyError
	if stdErr.As(err, &yerr) {
		if logLevelFlag == "debug" {
			slog.Error("Falha na execução", "code", yerr.Code, "details", fmt.Sprintf("%+v", yerr))
		} else {
			slog.Error("Falha na execução", "code", yerr.Code, "message", yerr.Message)
		}
		// Exibe hint do erro ou do registry padrão
		if hint := yerr.GetHint(); hint != "" {
			slog.Info("Dica", "sugestão", hint)
		}
	} else {
		slog.Error("Falha inesperada", "erro", err)
		slog.Info("Dica", "sugestão", errors.GenericHint)
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	pm := newRootPluginManager()
	discoverPlugins(rootCmd, pm)

	start := time.Now()
	err := rootCmd.Execute()

	telemetry.Record("yby-cli", time.Since(start), err)
	telemetry.Flush()

	cfg := config.Get()
	if flushErr := telemetry.FlushToFile(cfg.Telemetry.Enabled); flushErr != nil {
		slog.Debug("Falha ao persistir telemetria", "erro", flushErr)
	}

	if err != nil {
		handleExecutionError(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&contextFlag, "context", "c", "", "Define o contexto de execução (ex: local, staging, prod)")
	rootCmd.PersistentFlags().StringVar(&logLevelFlag, "log-level", "info", "Nível de log (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormatFlag, "log-format", "text", "Formato de log (text, json)")

}

// initConfig carrega a configuração global e inicializa o logger.
// Precedência: flags > env vars > config file > defaults.
func initConfig(cmd *cobra.Command, args []string) {
	// Carrega configuração global (~/.yby/config.yaml + env vars + defaults)
	cfg, err := config.Load()
	if err != nil {
		slog.Warn("Falha ao carregar configuração global, usando defaults", "erro", err)
		cfg = config.DefaultConfig()
	}

	// Flags sobrescrevem config/env (só se foram explicitamente informadas)
	logLevel := cfg.Log.Level
	if cmd.Flags().Changed("log-level") {
		logLevel = logLevelFlag
	}

	logFormat := cfg.Log.Format
	if cmd.Flags().Changed("log-format") {
		logFormat = logFormatFlag
	}

	// Inicializa logger global
	logger.InitGlobal(logger.Config{
		Level:  logLevel,
		Format: logFormat,
	})

	// Se a flag --context foi informada, propaga via env var para pkg/context
	if contextFlag != "" {
		os.Setenv("YBY_ENV", contextFlag)
	}
}

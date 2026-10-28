package cmd

import (
	stdErr "errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/casheiro/yby-cli/pkg/config"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/logger"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/telemetry"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	contextFlag   string
	logLevelFlag  string
	logFormatFlag string

	// logFormatEfetivo é o formato de log resolvido em initConfig (flags > env > config).
	logFormatEfetivo string

	// saidaErro recebe o erro renderizado por handleExecutionError.
	saidaErro io.Writer = os.Stderr
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
	// só "yby", sem subcomando: a logo antes da ajuda (o --help não passa por aqui)
	RunE: func(cmd *cobra.Command, _ []string) error {
		out := cmd.OutOrStdout()
		a := ajudaDe(cmd)
		if ui.Largura(out) > 0 {
			ui.Logo(out)
			a.SemTitulo = true
		}
		ui.EscreverAjuda(out, a)
		return nil
	},
}

// newRootPluginManager é uma factory mockável para testes
var newRootPluginManager = plugin.NewManager

// anotacaoNaoConfiavel marca o comando de um plugin que a descoberta não executou por não
// ser confiável: ele aparece no yby --help com a dica do trust, mas fica fora do gen-docs.
const anotacaoNaoConfiavel = "yby/nao-confiavel"

// nomesDoCobra são os comandos que o cobra cria só no ExecuteC, depois da descoberta: o
// cmd.Find ainda não os acha, e um plugin com o nome deles tomaria o lugar (um
// yby-plugin-help num .yby/plugins clonado viraria o yby help).
var nomesDoCobra = map[string]bool{
	"help":                          true,
	"completion":                    true,
	cobra.ShellCompRequestCmd:       true,
	cobra.ShellCompNoDescRequestCmd: true,
}

// nomeDoCore diz se nome é de um comando do core (ou do cobra), que plugin nenhum toma.
func nomeDoCore(cmd *cobra.Command, nome string) bool {
	if nomesDoCobra[nome] {
		return true
	}
	// Find devolve o próprio cmd, sem erro, quando o subcomando não existe
	foundCmd, _, err := cmd.Find([]string{nome})
	return err == nil && foundCmd != cmd
}

// discoverPlugins escaneia e registra plugins como subcomandos do rootCmd. Plugin com o
// nome de um comando do core, confiável ou não, fica de fora.
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
				if nomeDoCore(cmd, p.Name) {
					slog.Debug("Plugin com o nome de um comando do core ignorado", "nome", p.Name)
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
						ambiente, args, err := argsDoPlugin(linhaDoYby(), pluginName, args)
						if err != nil {
							return err
						}
						pm.DefinirAmbiente(ambiente)
						if err := pm.ExecuteCommandHook(pluginName, args); err != nil {
							var saida *plugin.SaidaDoPlugin
							if stdErr.As(err, &saida) {
								// o plugin já mostrou o erro: só o código segue
								return saida
							}
							// a dica do executor (ex.: a do yby plugin trust) segue
							return falhaComHint(err, errors.ErrCodePlugin, fmt.Sprintf("o plugin %s falhou", pluginName),
								errors.GetDefaultHint(errors.ErrCodePlugin))
						}
						return nil
					},
				}
				if p.NaoConfiavel {
					pluginCmd.Annotations = map[string]string{anotacaoNaoConfiavel: "sim"}
				}
				cmd.AddCommand(pluginCmd)
			}
		}
	}
}

// handleExecutionError trata erros de execução, diferenciando YbyError de erros genéricos.
// No formato text, escreve o erro em saidaErro com ui.RenderError (causa, hint e
// detalhes); no formato json, mantém os registros slog de sempre.
func handleExecutionError(err error) {
	formato := logFormatEfetivo
	if formato == "" {
		// initConfig não rodou (ex.: erro no parse das flags).
		formato = logFormatFlag
	}
	if strings.ToLower(formato) == "json" {
		logExecutionError(err)
		return
	}

	var yerr *errors.YbyError
	if !stdErr.As(err, &yerr) {
		yerr = errors.Wrap(err, "", "Falha inesperada").WithHint(errors.GenericHint)
	}
	slog.Debug("Falha na execução", "code", yerr.Code, "details", fmt.Sprintf("%+v", yerr))
	// a linha em branco separa o erro do que o comando já escreveu
	fmt.Fprintln(saidaErro)
	corDoErroPeloStderr()
	ui.WriteError(saidaErro, yerr)
}

// stderrTerminal informa se a saída de erro é um terminal. Variável para os testes.
var stderrTerminal = func() bool { return term.IsTerminal(int(os.Stderr.Fd())) }

// perfilDoStderr é o perfil de cor do terminal da saída de erro. Variável para os testes.
var perfilDoStderr = func() termenv.Profile { return termenv.NewOutput(os.Stderr).EnvColorProfile() }

// corDoErroPeloStderr decide a cor do erro pelo stderr, que é para onde ele vai: o lipgloss
// decide pelo stdout, e com a saída num pipe (yby cloud dashboard | more) o erro sairia sem
// cor num terminal. Com NO_COLOR, nada muda.
func corDoErroPeloStderr() {
	if ui.Atual().SemCor || saidaErro != io.Writer(os.Stderr) || stdoutTerminal() || !stderrTerminal() {
		return
	}
	lipgloss.SetColorProfile(perfilDoStderr())
}

var (
	reComandoDesconhecido = regexp.MustCompile(`^unknown command "([^"]*)" for "([^"]*)"`)
	reArgumentos          = regexp.MustCompile(`^((accepts|requires at least|requires at most|accepts between) .*arg\(s\), received \d+|invalid argument "[^"]*" for ")`)
)

// traduzirErroDeUso troca os erros de uso do cobra (comando desconhecido, número de
// argumentos) por um YbyError em português com a dica de ajuda; cmd é o comando que o
// cobra encontrou. Os demais erros voltam como estão.
func traduzirErroDeUso(err error, cmd *cobra.Command) error {
	if err == nil {
		return nil
	}
	var yerr *errors.YbyError
	if stdErr.As(err, &yerr) {
		return err
	}
	msg := err.Error()
	if m := reComandoDesconhecido.FindStringSubmatch(msg); m != nil {
		hint := "rode " + m[2] + " --help para ver os comandos"
		if sugestoes := sugestoesDoCobra(msg); len(sugestoes) > 0 {
			hint = "você quis dizer " + m[2] + " " + strings.Join(sugestoes, " ou "+m[2]+" ") + "?"
		}
		return errors.New(errors.ErrCodeValidation, "comando desconhecido: "+m[2]+" "+m[1]).WithHint(hint)
	}
	if cmd != nil && reArgumentos.MatchString(msg) {
		return errors.Wrap(err, errors.ErrCodeValidation, "argumentos inválidos para "+cmd.CommandPath()).
			WithHint("rode " + cmd.CommandPath() + " --help para ver o uso")
	}
	return err
}

// sugestoesDoCobra extrai os nomes listados depois de "Did you mean this?".
func sugestoesDoCobra(msg string) []string {
	_, depois, ok := strings.Cut(msg, "Did you mean this?")
	if !ok {
		return nil
	}
	return strings.Fields(depois)
}

// erroDeFlag é o FlagErrorFunc do rootCmd (herdado pelos subcomandos): flag desconhecida
// ou sem valor vira YbyError com a dica de ajuda do comando.
func erroDeFlag(cmd *cobra.Command, err error) error {
	return errors.Wrap(err, errors.ErrCodeValidation, "flag inválida para "+cmd.CommandPath()).
		WithHint("rode " + cmd.CommandPath() + " --help para ver as flags")
}

// logExecutionError registra o erro via slog, um registro por campo (formato json).
func logExecutionError(err error) {
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
	pm.EnableTrustCheck()
	discoverPlugins(rootCmd, pm)
	pluginsDescobertos = pm // o ↵ do yby cloud dashboard abre o Viz por ele

	start := time.Now()
	cmd, err := rootCmd.ExecuteC()
	err = traduzirErroDeUso(err, cmd)

	telemetry.Record("yby-cli", time.Since(start), err)
	telemetry.Flush()

	cfg := config.Get()
	if flushErr := telemetry.FlushToFile(cfg.Telemetry.Enabled); flushErr != nil {
		slog.Debug("Falha ao persistir telemetria", "erro", flushErr)
	}

	if codigo := codigoDeSaida(err); codigo != 0 {
		os.Exit(codigo)
	}
}

// codigoDeSaida devolve o código com que o yby sai depois de err e escreve o erro com
// handleExecutionError. Um plugin que saiu com código próprio já escreveu o erro dele:
// o código segue (ex.: 130 do ctrl+c) e nada mais sai.
func codigoDeSaida(err error) int {
	if err == nil {
		return 0
	}
	var saida *plugin.SaidaDoPlugin
	if stdErr.As(err, &saida) {
		slog.Debug("o plugin saiu com erro", "codigo", saida.Codigo)
		return saida.Codigo
	}
	handleExecutionError(err)
	return 1
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&contextFlag, "context", "c", "", "Define o contexto de execução (ex: local, staging, prod)")
	rootCmd.PersistentFlags().StringVar(&logLevelFlag, "log-level", "info", "Nível de log (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormatFlag, "log-format", "text", "Formato de log (text, json)")
	rootCmd.SetFlagErrorFunc(erroDeFlag)
	configurarAjuda(rootCmd)

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
	logFormatEfetivo = logFormat

	// Modo visual das saídas: YBY_THEME, YBY_ASCII e NO_COLOR > ui.* do config
	ui.Configurar(ui.ResolverOpcoes(ui.Opcoes{}, false, os.Getenv, cfg.UI))

	// Primeiro uso: se o arquivo de config não existe, pergunta sobre telemetria
	if home, err := os.UserHomeDir(); err == nil {
		configPath := filepath.Join(home, ".yby", "config.yaml")
		if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
			askTelemetryOptIn(configPath)
		}
	}

	// Se a flag --context foi informada, propaga via env var para pkg/context
	if contextFlag != "" {
		os.Setenv("YBY_ENV", contextFlag)
	}
}

// askTelemetryOptIn pergunta ao usuário se deseja habilitar telemetria no primeiro uso.
// Sem terminal (pipe, CI) a pergunta não abre: nada é gravado, a telemetria fica
// desligada (padrão do config) e a pergunta volta no próximo uso num terminal.
func askTelemetryOptIn(configPath string) {
	enabled, err := prompter.Confirm("Deseja habilitar telemetria anônima para ajudar a melhorar o Yby CLI?", false)
	if err != nil {
		slog.Debug("Falha ao solicitar opt-in de telemetria", "erro", err)
		return
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		slog.Debug("Falha ao criar diretório de config", "erro", err)
		return
	}

	val := "false"
	if enabled {
		val = "true"
	}
	content := fmt.Sprintf("telemetry:\n  enabled: %s\n", val)
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		slog.Debug("Falha ao salvar preferência de telemetria", "erro", err)
	}
}

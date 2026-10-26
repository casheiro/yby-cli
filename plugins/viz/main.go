package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/casheiro/yby-cli/pkg/config"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/plugin"
	yui "github.com/casheiro/yby-cli/pkg/ui"
	"github.com/casheiro/yby-cli/plugins/viz/internal/monitor"
	"github.com/casheiro/yby-cli/plugins/viz/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

// Tipos do protocolo de plugins, os mesmos do core.
type (
	PluginManifest = plugin.PluginManifest
	PluginRequest  = plugin.PluginRequest
	PluginResponse = plugin.PluginResponse
)

// comandoViz é como a pessoa chama o Viz, pelo core.
const comandoViz = "yby viz"

// manifesto é o manifesto do Viz.
var manifesto = PluginManifest{
	Name:        "viz",
	Description: "Painel do cluster ao vivo no terminal",
	Version:     "1.0.0",
	Hooks:       []string{"command"},
}

// configurarVisual aplica no processo do plugin o modo visual do core: YBY_THEME,
// YBY_ASCII e NO_COLOR > ui.theme e ui.ascii do ~/.yby/config.yaml > padrão. O Viz
// roda em outro processo, então o ui.Configurar do cmd/root.go não chega até aqui.
// Depois fixa o fundo e o perfil de cor antes de a TUI assumir o terminal: no tema
// auto, a primeira consulta do lipgloss ao fundo (OSC 11) aconteceria com o Bubble
// Tea já lendo a entrada.
func configurarVisual(getenv func(string) string) {
	yui.Configurar(yui.ResolverOpcoes(yui.Opcoes{}, false, getenv, config.Get().UI))
	if yui.Atual().Tema == yui.TemaAuto {
		lipgloss.SetHasDarkBackground(lipgloss.HasDarkBackground())
	}
	lipgloss.SetColorProfile(lipgloss.ColorProfile())
}

func main() {
	os.Exit(executar(os.Stderr))
}

// executar roda o plugin e devolve o código de saída; um erro sai em w no formato de
// ui.WriteError (um erro comum vira ERR_PLUGIN).
func executar(w io.Writer) int {
	err := run()
	if errors.Is(err, errInterrompido) {
		return codigoCtrlC // a pessoa saiu com ctrl+c: nada a mostrar
	}
	if err != nil {
		var ye *yerrors.YbyError
		if !errors.As(err, &ye) {
			ye = yerrors.Wrap(err, yerrors.ErrCodePlugin, "o Viz falhou")
		}
		configurarVisual(os.Getenv) // o erro de flag sai antes do RunE, que configura
		yui.WriteError(w, ye)
		return 1
	}
	return 0
}

func run() error {
	cmd := novoComando(abrirTUI)

	// O core manda o pedido pela env YBY_PLUGIN_REQUEST (yby viz, RunInteractive), com os
	// args que vieram depois de "viz"; o manifesto ele pede pelo stdin. Sem pedido, é o
	// binário chamado direto, com os args da linha de comando.
	req, temPedido, err := pedidoDoCore(os.Getenv, os.Stdin)
	if err != nil {
		return err
	}
	if temPedido {
		switch req.Hook {
		case "manifest":
			return json.NewEncoder(os.Stdout).Encode(PluginResponse{Data: manifesto})
		case "command":
			cmd.SetArgs(req.Args)
		default:
			return nil
		}
	}
	if err := cmd.Execute(); err != nil {
		if errors.As(err, new(*yerrors.YbyError)) {
			return err
		}
		// o que sobra é erro de uso do cobra (argumento a mais, comando desconhecido)
		return yerrors.Wrap(err, yerrors.ErrCodeValidation, "argumentos inválidos para "+comandoViz).
			WithHint("rode " + comandoViz + " --help para ver o uso")
	}
	return nil
}

// pedidoDoCore lê o pedido do core: primeiro a env YBY_PLUGIN_REQUEST, depois um JSON
// no stdin quando ele não é um terminal.
func pedidoDoCore(getenv func(string) string, stdin *os.File) (PluginRequest, bool, error) {
	var req PluginRequest
	if env := getenv("YBY_PLUGIN_REQUEST"); env != "" {
		if err := json.Unmarshal([]byte(env), &req); err != nil {
			return req, false, yerrors.Wrap(err, yerrors.ErrCodePluginRPC, "YBY_PLUGIN_REQUEST inválido")
		}
		return req, true, nil
	}
	if stdin == nil {
		return req, false, nil
	}
	if stat, err := stdin.Stat(); err == nil && stat.Mode()&os.ModeCharDevice == 0 {
		if err := json.NewDecoder(stdin).Decode(&req); err == nil {
			return req, true, nil
		}
	}
	return req, false, nil
}

// novoComando monta o yby viz: as flags escolhem o contexto e o namespace, e abrir
// recebe a escolha.
func novoComando(abrir func(monitor.Conexao) error) *cobra.Command {
	var conexao monitor.Conexao
	cmd := &cobra.Command{
		Use:   "viz",
		Short: "Yby Viz: observabilidade do cluster no terminal",
		Long: "Observa o cluster no terminal, ao vivo: pods, deployments, statefulsets, serviços, " +
			"nós, jobs, ingresses, configmaps e eventos.\n" +
			"Abre no contexto atual do kubeconfig e no namespace dele; as flags escolhem outros.",
		Example: "  " + comandoViz + "\n" +
			"  " + comandoViz + " --context k3d-yby-local --namespace kube-system",
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(*cobra.Command, []string) error {
			return abrir(conexao)
		},
	}
	cmd.Flags().StringVar(&conexao.Kubeconfig, "kubeconfig", "", "arquivo do kubeconfig (padrão: KUBECONFIG ou ~/.kube/config)")
	cmd.Flags().StringVar(&conexao.Contexto, "context", "", "contexto do kubeconfig (padrão: o atual)")
	cmd.Flags().StringVarP(&conexao.Namespace, "namespace", "n", "", "namespace inicial (padrão: o do contexto; sem ele, todos)")
	cmd.Flags().BoolP("help", "h", false, "mostra esta ajuda")

	cmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		configurarVisual(os.Getenv)
		yui.EscreverAjuda(c.OutOrStdout(), ajudaDe(c))
	})
	cmd.SetUsageFunc(func(c *cobra.Command) error {
		yui.EscreverAjuda(c.OutOrStderr(), ajudaDe(c))
		return nil
	})
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return yerrors.Wrap(err, yerrors.ErrCodeValidation, "flag inválida para "+comandoViz).
			WithHint("rode " + comandoViz + " --help para ver as flags")
	})
	cmd.AddCommand(&cobra.Command{
		Use:    "manifest",
		Hidden: true,
		RunE: func(*cobra.Command, []string) error {
			if err := json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"data": manifesto}); err != nil {
				return fmt.Errorf("falha ao codificar manifest: %w", err)
			}
			return nil
		},
	})
	cmd.SetHelpCommand(&cobra.Command{Use: "sem-ajuda", Hidden: true})
	return cmd
}

// ajudaDe é a ajuda do yby viz no formato do core (ui.EscreverAjuda).
func ajudaDe(c *cobra.Command) yui.Ajuda {
	return yui.Ajuda{
		Comando:   comandoViz,
		Descricao: c.Long,
		Uso:       []string{comandoViz + " [flags]"},
		Exemplos:  c.Example,
		Flags:     termosDeFlags(c.Flags()),
		Rodape:    "dentro do Viz, ? mostra todas as teclas",
	}
}

// termosDeFlags lista as flags como "-n, --namespace texto", como a ajuda do core.
func termosDeFlags(fs *pflag.FlagSet) []yui.Termo {
	var termos []yui.Termo
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		nome := "    --" + f.Name
		if f.Shorthand != "" {
			nome = "-" + f.Shorthand + ", --" + f.Name
		}
		if tipo, _ := pflag.UnquoteUsage(f); tipo == "string" {
			nome += " texto"
		}
		termos = append(termos, yui.Termo{Nome: nome, Descricao: f.Usage})
	})
	return termos
}

// codigoCtrlC é o código de saída do ctrl+c (128 + SIGINT, como o Bard): quem abriu o Viz
// (o painel do yby cloud dashboard) sabe que a pessoa quer sair de tudo.
const codigoCtrlC = 130

// errInterrompido é a TUI fechada com ctrl+c.
var errInterrompido = errors.New("interrompido com ctrl+c")

// abrirTUI conecta ao cluster e abre a TUI. Um contexto pedido que não existe sai como
// erro do comando, antes da TUI; as outras falhas de conexão a TUI mostra.
func abrirTUI(c monitor.Conexao) error {
	configurarVisual(os.Getenv)
	client, err := conectar(c)
	var p *tea.Program
	switch {
	case errors.As(err, new(*yerrors.YbyError)):
		return err
	case err != nil:
		p = tea.NewProgram(ui.NewModelComErro(err), tea.WithAltScreen())
	default:
		// O client-go (o watch, os avisos da API) escreve pelo klog no stderr,
		// por cima da TUI: no Viz, ele não escreve nada.
		klog.SetLogger(logr.Discard())
		p = tea.NewProgram(ui.NewModel(monitor.NewRetryClient(client)), tea.WithAltScreen(), tea.WithMouseCellMotion())
	}
	final, err := p.Run()
	if m, ok := final.(ui.Model); ok {
		m.Parar() // o watch para com o programa, mesmo sem passar pelo q
	}
	if err != nil {
		return yerrors.Wrap(err, yerrors.ErrCodeIO, "a TUI do Viz falhou").
			WithHint("rode de novo num terminal interativo")
	}
	if m, ok := final.(ui.Model); ok && m.Interrompido() {
		return errInterrompido
	}
	return nil
}

// conectar cria o client do cluster. O contexto pedido que não existe vira YbyError,
// com os contextos que existem na dica.
func conectar(c monitor.Conexao) (*monitor.K8sClient, error) {
	client, err := monitor.NovoK8sClient(c)
	var inexistente *monitor.ContextoInexistenteError
	if errors.As(err, &inexistente) {
		return nil, erroContextoInexistente(inexistente)
	}
	return client, err
}

// maxContextosNaDica é quantos contextos a dica do contexto inexistente lista.
const maxContextosNaDica = 5

func erroContextoInexistente(e *monitor.ContextoInexistenteError) *yerrors.YbyError {
	hint := "o kubeconfig não tem contextos; conecte um cluster com yby cloud connect ou yby up"
	if n := len(e.Disponiveis); n > 0 {
		nomes := e.Disponiveis[:min(n, maxContextosNaDica)]
		hint = "contextos do kubeconfig: " + strings.Join(nomes, ", ")
		if n > maxContextosNaDica {
			hint += fmt.Sprintf(" e mais %d (kubectl config get-contexts)", n-maxContextosNaDica)
		}
	}
	return yerrors.New(yerrors.ErrCodeConfig, fmt.Sprintf("o contexto %s não existe no kubeconfig", e.Nome)).
		WithContext("contexto", e.Nome).
		WithHint(hint)
}

/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/casheiro/yby-cli/pkg/confianca"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/plugin"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
)

// gerenciadorPlugins é o que os comandos de plugin usam de plugin.Manager (um dublê nos
// testes, já que a descoberta executa os binários).
type gerenciadorPlugins interface {
	Discover() error
	ListPlugins() []plugin.PluginManifest
	GetPlugin(name string) (*plugin.LoadedPlugin, bool)
	Install(pluginSource, version string, force bool) error
	Remove(name string) error
	Update(name string) error
}

// newPluginManager é a factory para criação do gerenciador de plugins (mockável em testes)
var newPluginManager = func() gerenciadorPlugins {
	return plugin.NewManager()
}

// hintListaPlugins aponta para a lista de plugins quando o nome dado não confere.
const hintListaPlugins = "confira o nome com yby plugin list"

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
			return errors.Wrap(err, errors.ErrCodePlugin, "falha ao procurar os plugins").
				WithHint("confira se ~/.yby/plugins e .yby/plugins podem ser lidos")
		}

		renderPluginList(cmd.OutOrStdout(), pm.ListPlugins())
		return nil
	},
}

// renderPluginList escreve os plugins instalados em colunas, em ordem alfabética.
func renderPluginList(w io.Writer, plugins []plugin.PluginManifest) {
	tituloComando(w, "plugin", "list · "+contagem(len(plugins), "instalado", "instalados"))
	if len(plugins) == 0 {
		ui.Check(w, ui.CheckNeutro, "nenhum plugin instalado · instale com yby plugin install <nome>", "", "")
		return
	}
	ordenados := append([]plugin.PluginManifest(nil), plugins...)
	sort.Slice(ordenados, func(i, j int) bool { return ordenados[i].Name < ordenados[j].Name })
	linhas := make([]ui.Linha, len(ordenados))
	for i, p := range ordenados {
		linhas[i] = ui.Linha{Celulas: []string{p.Name, "v" + strings.TrimPrefix(p.Version, "v"), strings.Join(p.Hooks, ", ")}}
	}
	ui.Listagem(w, []ui.Coluna{
		{Titulo: "plugin"},
		{Titulo: "versão", Prioridade: 1},
		{Titulo: "hooks", Prioridade: 2},
	}, linhas)
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
			return falhaComHint(err, errors.ErrCodePlugin, "erro ao instalar plugin",
				"informe um arquivo local, uma URL ou um plugin nativo: atlas, bard, sentinel, synapstor ou viz")
		}

		// Install só avisa no log quando não consegue registrar o checksum: a linha confere
		// o registro de confiança antes de dizer que o plugin é confiável.
		nome, binario := binarioInstalado(args[0])
		linha := linhaCheck{ui.CheckOk, nome, "instalado", ""}
		if binario != "" {
			if confiavel, _ := plugin.IsTrusted(binario); confiavel {
				linha.nota = "registrado como confiável"
			} else {
				linha.tipo = ui.CheckAviso
				linha.nota = "não registrado como confiável · rode yby plugin trust " + strings.TrimPrefix(nome, "yby-plugin-")
			}
		}
		w := cmd.OutOrStdout()
		tituloComando(w, "plugin", "install")
		checksAlinhados(w, linha)
		return nil
	},
}

// binarioInstalado devolve o nome a mostrar para a origem de plugin install e o caminho
// do binário instalado em ~/.yby/plugins (vazio quando não dá para saber, como numa URL).
// Um arquivo local vira o nome do arquivo; um plugin nativo é instalado como yby-plugin-<nome>.
func binarioInstalado(origem string) (nome, caminho string) {
	if strings.HasPrefix(origem, "http://") || strings.HasPrefix(origem, "https://") {
		return origem, ""
	}
	nome = filepath.Base(strings.TrimPrefix(origem, "file://"))
	home, err := os.UserHomeDir()
	if err != nil {
		return nome, ""
	}
	dir := filepath.Join(home, ".yby", "plugins")
	for _, candidato := range []string{nome, "yby-plugin-" + nome} {
		if info, err := os.Stat(filepath.Join(dir, candidato)); err == nil && !info.IsDir() {
			return nome, filepath.Join(dir, candidato)
		}
	}
	return nome, ""
}

var pluginRemoveCmd = &cobra.Command{
	Use:     "remove [name]",
	Aliases: []string{"rm", "uninstall", "delete"},
	Short:   "Remove um plugin instalado",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := newPluginManager()
		if err := pm.Remove(args[0]); err != nil {
			return falhaComHint(err, errors.ErrCodePlugin, "erro ao remover plugin", hintListaPlugins)
		}
		w := cmd.OutOrStdout()
		tituloComando(w, "plugin", "remove")
		checksAlinhados(w, linhaCheck{ui.CheckOk, args[0], "removido", ""})
		return nil
	},
}

var pluginUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Atualiza um ou todos os plugins instalados",
	RunE: func(cmd *cobra.Command, args []string) error {
		pm := newPluginManager()
		w := cmd.OutOrStdout()
		tituloComando(w, "plugin", "update")

		// Ensure discovery happens
		var linhas []linhaCheck
		if err := pm.Discover(); err != nil {
			linhas = append(linhas, linhaCheck{ui.CheckAviso, "descoberta", "falhou", mensagemDoErro(err)})
		}

		targets := args
		if len(args) == 0 {
			// Update all
			for _, p := range pm.ListPlugins() {
				targets = append(targets, p.Name)
			}
		}

		if len(targets) == 0 {
			checksAlinhados(w, linhas...)
			ui.Check(w, ui.CheckNeutro, "nenhum plugin instalado para atualizar", "", "")
			return nil
		}

		falhas := 0
		for _, name := range targets {
			if err := pm.Update(name); err != nil {
				linhas = append(linhas, linhaCheck{ui.CheckErro, name, "falhou", mensagemDoErro(err)})
				falhas++
			} else {
				linhas = append(linhas, linhaCheck{ui.CheckOk, name, "atualizado", ""})
			}
		}
		checksAlinhados(w, linhas...)

		if falhas > 0 {
			msg := fmt.Sprintf("%d plugins de %d não foram atualizados", falhas, len(targets))
			if falhas == 1 {
				msg = fmt.Sprintf("1 plugin de %d não foi atualizado", len(targets))
			}
			return errors.New(errors.ErrCodePlugin, msg).
				WithHint("confira as linhas acima; rode de novo com --log-level=debug para detalhes")
		}
		return nil
	},
}

// ehArquivoDoProjeto diz se o argumento do trust e do untrust é um arquivo do projeto
// (ferramenta do Bard em .yby/tools, prompt em .yby/prompts) e não o nome de um plugin.
func ehArquivoDoProjeto(arg string) bool {
	switch strings.ToLower(filepath.Ext(arg)) {
	case ".yaml", ".yml", ".txt":
		return true
	}
	return false
}

var pluginTrustCmd = &cobra.Command{
	Use:   "trust [name|arquivo]",
	Short: "Registra um plugin, ou uma ferramenta ou prompt do projeto, como confiável",
	Long: `Calcula o SHA256 do binário do plugin e o registra como confiável. Plugins não registrados não podem ser executados.

Com um arquivo do projeto (.yby/tools/*.yaml, as ferramentas do Bard, ou .yby/prompts/*.txt), registra o SHA256 do arquivo naquele caminho: sem isso, o yby não o usa, porque um repositório clonado pode trazê-lo. Se o arquivo mudar, a confiança cai e é preciso confirmar de novo.`,
	Example: `  yby plugin trust sentinel
  yby plugin trust .yby/tools/backup.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if ehArquivoDoProjeto(args[0]) {
			if err := confianca.Confiar(args[0]); err != nil {
				return errors.Wrap(err, errors.ErrCodeIO, "erro ao registrar confiança do arquivo").
					WithHint("confira o caminho do arquivo e a permissão de escrita em ~/.yby")
			}
			w := cmd.OutOrStdout()
			tituloComando(w, "plugin", "trust")
			checksAlinhados(w, linhaCheck{ui.CheckOk, filepath.ToSlash(args[0]), "confiável", "SHA256 do arquivo registrado"})
			return nil
		}

		pm := newPluginManager()
		if err := pm.Discover(); err != nil {
			return errors.Wrap(err, errors.ErrCodePlugin, "falha ao procurar os plugins").
				WithHint("confira se ~/.yby/plugins e .yby/plugins podem ser lidos")
		}

		p, found := pm.GetPlugin(args[0])
		if !found {
			return errors.New(errors.ErrCodePluginNotFound, fmt.Sprintf("plugin '%s' não encontrado", args[0])).
				WithHint(fmt.Sprintf("instale-o primeiro com yby plugin install %s", args[0]))
		}

		if err := plugin.TrustPlugin(p.Path); err != nil {
			return errors.Wrap(err, errors.ErrCodePlugin, "erro ao registrar confiança do plugin").
				WithHint("confira a permissão de escrita em ~/.yby")
		}

		w := cmd.OutOrStdout()
		tituloComando(w, "plugin", "trust")
		checksAlinhados(w, linhaCheck{ui.CheckOk, args[0], "confiável", "SHA256 do binário registrado"})
		return nil
	},
}

var pluginUntrustCmd = &cobra.Command{
	Use:   "untrust [name|arquivo]",
	Short: "Remove um plugin, ou uma ferramenta ou prompt do projeto, da whitelist de confiança",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if ehArquivoDoProjeto(args[0]) {
			if err := confianca.Remover(args[0]); err != nil {
				return errors.Wrap(err, errors.ErrCodePlugin, "erro ao remover confiança do arquivo").
					WithHint("confira o caminho: é o mesmo usado no yby plugin trust")
			}
			w := cmd.OutOrStdout()
			tituloComando(w, "plugin", "untrust")
			checksAlinhados(w, linhaCheck{ui.CheckOk, filepath.ToSlash(args[0]), "removido", "fora da lista de confiança"})
			return nil
		}

		pluginName := args[0]
		if !strings.HasPrefix(pluginName, "yby-plugin-") {
			pluginName = "yby-plugin-" + pluginName
		}

		if err := plugin.UntrustPlugin(pluginName); err != nil {
			return errors.Wrap(err, errors.ErrCodePlugin, "erro ao remover confiança do plugin").
				WithHint(hintListaPlugins)
		}

		w := cmd.OutOrStdout()
		tituloComando(w, "plugin", "untrust")
		checksAlinhados(w, linhaCheck{ui.CheckOk, args[0], "removido", "fora da lista de confiança"})
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

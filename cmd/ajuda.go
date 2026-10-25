package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/casheiro/yby-cli/pkg/config"
	"github.com/casheiro/yby-cli/pkg/ui"
)

// configurarAjuda troca a ajuda e o uso padrão do cobra (em inglês) pela ajuda da carta.
// Vale para todos os subcomandos, que herdam as funções do root.
func configurarAjuda(root *cobra.Command) {
	root.SetHelpFunc(func(c *cobra.Command, _ []string) {
		// com --help o cobra pula o PersistentPreRun: o modo visual é resolvido aqui
		configurarVisual()
		ui.EscreverAjuda(c.OutOrStdout(), ajudaDe(c))
	})
	root.SetUsageFunc(func(c *cobra.Command) error {
		ui.EscreverAjuda(c.OutOrStderr(), ajudaDe(c))
		return nil
	})
}

// ajudaDe monta a ajuda de c a partir dos campos do cobra.
func ajudaDe(c *cobra.Command) ui.Ajuda {
	a := ui.Ajuda{
		Comando:   c.CommandPath(),
		Descricao: c.Long,
		Apelidos:  c.Aliases,
		Exemplos:  c.Example,
		Flags:     termosDeFlags(c.LocalFlags()),
		Globais:   termosDeFlags(c.InheritedFlags()),
	}
	if a.Descricao == "" {
		a.Descricao = c.Short
	}
	if c.Name() == "help" && strings.HasPrefix(c.Long, "Help provides help") {
		a.Descricao = "Mostra a ajuda de qualquer comando: " + c.Root().Name() + " help <comando>"
	}
	// o root e os grupos sem argumentos (bootstrap, cloud, generate, plugin) só rodam para
	// mostrar a ajuda: a linha de uso deles é a dos subcomandos
	_, temArgs, _ := strings.Cut(c.Use, " ")
	soAjuda := !c.HasParent() || (c.HasAvailableSubCommands() && temArgs == "")
	if c.Runnable() && !soAjuda {
		a.Uso = append(a.Uso, linhaDeUso(c))
	}
	if c.HasAvailableSubCommands() {
		a.Uso = append(a.Uso, c.CommandPath()+" <comando>")
		a.Rodape = c.CommandPath() + " <comando> --help mostra os detalhes de cada um"
	}
	for _, sub := range c.Commands() {
		if !sub.IsAvailableCommand() && sub.Name() != "help" {
			continue
		}
		a.Comandos = append(a.Comandos, ui.Termo{Nome: sub.Name(), Descricao: descricaoCurta(sub)})
	}
	return a
}

// linhaDeUso devolve o caminho do comando com os argumentos do Use e "[flags]" quando há
// flags, como o cobra faz, com o "help [command]" padrão traduzido.
func linhaDeUso(c *cobra.Command) string {
	linha := c.CommandPath()
	if _, args, ok := strings.Cut(c.Use, " "); ok {
		linha += " " + strings.ReplaceAll(args, "[command]", "[comando]")
	}
	if c.HasAvailableFlags() && !c.DisableFlagsInUseLine && !strings.Contains(linha, "[flags]") {
		linha += " [flags]"
	}
	return linha
}

// descricaoCurta devolve o Short do comando; o help padrão do cobra vem em inglês.
func descricaoCurta(c *cobra.Command) string {
	if c.Name() == "help" && c.Short == "Help about any command" {
		return "Mostra a ajuda de um comando"
	}
	return c.Short
}

// tiposDeFlag traduz o nome do valor que o pflag mostra ao lado da flag.
var tiposDeFlag = map[string]string{
	"string":         "texto",
	"strings":        "lista",
	"stringArray":    "texto",
	"stringToString": "chave=valor",
	"int":            "n",
	"int32":          "n",
	"int64":          "n",
	"uint":           "n",
	"float":          "n",
	"float64":        "n",
	"duration":       "duração",
}

// padroesVazios são os valores padrão que não vale mostrar.
var padroesVazios = map[string]bool{"": true, "false": true, "0": true, "[]": true, "0s": true, "map[]": true}

// termosDeFlags lista as flags visíveis de fs como "-c, --context texto", com o padrão no
// fim da descrição quando ele diz algo.
func termosDeFlags(fs *pflag.FlagSet) []ui.Termo {
	var termos []ui.Termo
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		nome := "    --" + f.Name
		if f.Shorthand != "" {
			nome = "-" + f.Shorthand + ", --" + f.Name
		}
		tipo, uso := pflag.UnquoteUsage(f)
		if t, ok := tiposDeFlag[tipo]; ok {
			tipo = t
		}
		if tipo != "" {
			nome += " " + tipo
		}
		if f.Name == "help" {
			uso = "mostra esta ajuda"
		}
		if !padroesVazios[f.DefValue] {
			uso += fmt.Sprintf(" (padrão %s)", f.DefValue)
		}
		termos = append(termos, ui.Termo{Nome: nome, Descricao: uso})
	})
	return termos
}

// configurarVisual aplica o modo visual (YBY_THEME, YBY_ASCII e NO_COLOR > ui.* do config).
func configurarVisual() {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	ui.Configurar(ui.ResolverOpcoes(ui.Opcoes{}, false, os.Getenv, cfg.UI))
}

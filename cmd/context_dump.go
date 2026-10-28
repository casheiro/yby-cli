/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"io"
	"os"
	"sort"
	"strings"

	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/spf13/cobra"
)

// contextDumpCmd represents the context dump command
var contextDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Exibe o contexto atual e variáveis carregadas",
	Long: `Mostra as variáveis de ambiente que foram injetadas pelo Motor de Contextos.
Útil para debugar se o .env correto está sendo carregado.`,
	Run: func(cmd *cobra.Command, args []string) {
		renderContextDump(cmd.OutOrStdout(), os.Environ())
	},
}

// renderContextDump escreve o contexto atual (YBY_ENV) e as variáveis YBY_* de environ,
// em ordem alfabética, seguidas da legenda das principais.
func renderContextDump(w io.Writer, environ []string) {
	vars := append([]string(nil), environ...)
	sort.Strings(vars)

	env := "padrão"
	var campos []ui.Campo
	for _, v := range vars {
		if !strings.HasPrefix(v, "YBY_") {
			continue
		}
		key, val, _ := strings.Cut(v, "=")
		if key == "YBY_ENV" && val != "" {
			env = val
		}
		// Obfuscate potential secrets if needed, but for now show raw
		campos = append(campos, ui.Campo{Rotulo: key, Valor: val})
	}

	ui.Title(w, "env", "dump · "+env)
	ui.Section(w, "variáveis YBY_*")
	if len(campos) == 0 {
		ui.Check(w, ui.CheckNeutro, "nenhuma variável YBY_* no ambiente", "", "")
	} else {
		ui.Campos(w, campos...)
	}

	ui.Section(w, "legenda")
	ui.Campos(w,
		ui.Campo{Rotulo: "YBY_ENV", Valor: "define o modo (local, staging, prod); sem ele, local se configurado"},
		ui.Campo{Rotulo: "YBY_GIT_REPOURL", Valor: "injetado em git.repoURL"},
		ui.Campo{Rotulo: "YBY_K3D_AGENTS", Valor: "configura o cluster local"},
	)
}

func init() {
	envCmd.AddCommand(contextDumpCmd)
}

/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// contextDumpCmd represents the context dump command
var contextDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Exibe o contexto atual e variáveis carregadas",
	Long: `Mostra as variáveis de ambiente que foram injetadas pelo Motor de Contextos.
Útil para debugar se o .env correto está sendo carregado.`,
	Run: func(cmd *cobra.Command, args []string) {
		env := os.Getenv("YBY_ENV")
		if env == "" {
			env = "padrão (local se configurado ou simples)"
		}

		fmt.Println(headerStyle.Render(fmt.Sprintf("🔍 Contexto Atual: %s", env)))
		fmt.Println("------------------------------------------------")

		// List interesting vars (YBY_*)
		vars := os.Environ()
		sort.Strings(vars)

		found := false
		for _, v := range vars {
			if strings.HasPrefix(v, "YBY_") {
				parts := strings.SplitN(v, "=", 2)
				key := parts[0]
				val := parts[1]
				// Obfuscate potential secrets if needed, but for now show raw
				fmt.Printf("%s: %s\n", key, val)
				found = true
			}
		}

		if !found {
			fmt.Println("ℹ️  Nenhuma variável YBY_* encontrada no ambiente.")
		}

		fmt.Println("------------------------------------------------")
		fmt.Println("Legenda:")
		fmt.Println("  YBY_ENV          -> Define o modo (local, staging, prod)")
		fmt.Println("  YBY_GIT_REPOURL  -> Injetado em git.repoURL")
		fmt.Println("  YBY_K3D_AGENTS   -> Configura cluster local")
	},
}

func init() {
	envCmd.AddCommand(contextDumpCmd)
}

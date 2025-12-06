/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Valida charts Helm e manifests",
	Long: `Executa linting e renderização de templates (dry-run) para garantir
que os charts Helm estão válidos antes do commit.

Equivalente ao antigo 'make validate'.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🔍 Yby Validate - Validação de Charts"))
		fmt.Println("---------------------------------------")

		charts := []string{"charts/system", "charts/bootstrap", "charts/cluster-config"}

		fmt.Println(headerStyle.Render("0️⃣  Resolvendo Dependências..."))
		for _, chart := range charts {
			runCommand("helm", "dependency", "build", chart)
		}

		fmt.Println("\n" + headerStyle.Render("1️⃣  Helm Lint..."))
		for _, chart := range charts {
			fmt.Printf("Linting %s... ", chart)
			if err := exec.Command("helm", "lint", chart).Run(); err != nil {
				fmt.Printf("%s\n", crossStyle.String())
				os.Exit(1)
			}
			fmt.Printf("%s\n", checkStyle.String())
		}

		fmt.Println("\n" + headerStyle.Render("2️⃣  Helm Template Check (Dry Run)..."))
		valuesFile := "config/cluster-values.yaml"
		// Fallback for location (if running from cli/)
		if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
			valuesFile = "../config/cluster-values.yaml"
		}

		for _, chart := range charts {
			name := "release-name" // dummy name
			fmt.Printf("Templating %s... ", chart)
			// Silent output unless error
			cmd := exec.Command("helm", "template", name, chart, "-f", valuesFile)
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("%s\n", crossStyle.String())
				fmt.Println(string(out))
				os.Exit(1)
			}
			fmt.Printf("%s\n", checkStyle.String())
		}

		fmt.Println("\n" + checkStyle.Render("✨ Validação concluída com sucesso!"))
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

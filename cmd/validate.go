/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Valida charts Helm e manifests",
	Long: `Executa linting e renderização de templates (dry-run) para garantir
que os charts Helm estão válidos antes do commit.

Equivalente ao antigo 'make validate'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(titleStyle.Render("🔍 Yby Validate - Validação de Charts"))
		fmt.Println("---------------------------------------")

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}

		charts := []string{
			JoinInfra(root, "charts/system"),
			JoinInfra(root, "charts/bootstrap"),
			JoinInfra(root, "charts/cluster-config"),
		}

		fmt.Println(headerStyle.Render("0️⃣  Resolvendo Dependências..."))
		for _, chart := range charts {
			fmt.Printf("%s Executando: helm dependency build %s\n", grayStyle.Render("Exec >"), chart)
			runCmd := execCommand("helm", "dependency", "build", chart)
			runCmd.Stdout = os.Stdout
			runCmd.Stderr = os.Stderr
			if err := runCmd.Run(); err != nil {
				return errors.Wrap(err, errors.ErrCodeExec, fmt.Sprintf("Erro ao atualizar subcharts de %s", chart))
			}
		}

		fmt.Println("\n" + headerStyle.Render("1️⃣  Helm Lint..."))
		for _, chart := range charts {
			fmt.Printf("Linting em %s... ", chart)
			if err := execCommand("helm", "lint", chart).Run(); err != nil {
				fmt.Printf("%s\n", crossStyle.String())
				return errors.Wrap(err, errors.ErrCodeExec, fmt.Sprintf("Erro no lint do chart %s", chart))
			}
			fmt.Printf("%s\n", checkStyle.String())
		}

		fmt.Println("\n" + headerStyle.Render("2️⃣  Verificação de Template Helm (Simulação)..."))
		valuesFile := JoinInfra(root, "config/cluster-values.yaml")
		// Fallback for location (if running from cli/)
		if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
			valuesFile = "../config/cluster-values.yaml"
		}

		for _, chart := range charts {
			name := "release-name" // dummy name
			fmt.Printf("Gerando template de %s... ", chart)
			// Silent output unless error
			cmd := execCommand("helm", "template", name, chart, "-f", valuesFile)
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("%s\n", crossStyle.String())
				fmt.Println(string(out))
				return errors.Wrap(err, errors.ErrCodeManifest, fmt.Sprintf("Erro na renderização do template %s", chart))
			}
			fmt.Printf("%s\n", checkStyle.String())
		}

		fmt.Println("\n" + checkStyle.Render("✨ Validação concluída com sucesso!"))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

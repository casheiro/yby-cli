/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/services/validate"
	"github.com/spf13/cobra"
)

// newValidateService permite substituicao em testes.
var newValidateService = func(r shared.Runner) validate.Service {
	helm := &validate.RealHelmRunner{Runner: r}
	return validate.NewService(helm)
}

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

		valuesFile := JoinInfra(root, "config/cluster-values.yaml")
		// Fallback para localizacao alternativa (ex: executando de cli/)
		if _, err := os.Stat(valuesFile); os.IsNotExist(err) {
			valuesFile = "../config/cluster-values.yaml"
		}

		runner := &shared.RealRunner{}
		svc := newValidateService(runner)

		report, err := svc.Run(cmd.Context(), charts, valuesFile)

		// Renderizar resultados por etapa
		renderValidateReport(report)

		if err != nil {
			return err
		}

		fmt.Println("\n" + checkStyle.Render("✨ Validação concluída com sucesso!"))
		return nil
	},
}

// renderValidateReport exibe o resultado de cada etapa de validacao.
func renderValidateReport(report *validate.ValidateReport) {
	if report == nil {
		return
	}

	fmt.Println(headerStyle.Render("0️⃣  Resolvendo Dependências..."))
	for _, c := range report.Charts {
		if c.DepsOK {
			fmt.Printf("%s Dependências resolvidas: %s %s\n", grayStyle.Render("Exec >"), c.Chart, checkStyle.String())
		} else if c.Error != "" {
			fmt.Printf("%s Dependências: %s %s\n", grayStyle.Render("Exec >"), c.Chart, crossStyle.String())
			return
		}
	}

	fmt.Println("\n" + headerStyle.Render("1️⃣  Helm Lint..."))
	for _, c := range report.Charts {
		if c.LintOK {
			fmt.Printf("Linting em %s... %s\n", c.Chart, checkStyle.String())
		} else if c.Error != "" {
			fmt.Printf("Linting em %s... %s\n", c.Chart, crossStyle.String())
			return
		}
	}

	fmt.Println("\n" + headerStyle.Render("2️⃣  Verificação de Template Helm (Simulação)..."))
	for _, c := range report.Charts {
		if c.TemplateOK {
			fmt.Printf("Gerando template de %s... %s\n", c.Chart, checkStyle.String())
		} else if c.Error != "" {
			fmt.Printf("Gerando template de %s... %s\n", c.Chart, crossStyle.String())
			fmt.Println(c.Error)
			return
		}
	}
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

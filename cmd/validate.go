/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"io"
	"os"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/casheiro/yby-cli/v5/pkg/services/validate"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
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
		w := cmd.OutOrStdout()

		if _, err := lookPath("helm"); err != nil {
			return errors.New(errors.ErrCodeCmdNotFound, "helm não encontrado").
				WithHint("instale o helm (https://helm.sh/docs/intro/install) e rode yby validate de novo")
		}

		root, err := FindInfraRoot()
		if err != nil {
			root = "."
		}
		ui.Title(w, "validate", root)

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
		renderValidateReport(w, report)

		if err != nil {
			return err
		}

		ui.Summary(w, len(report.Charts), 0, 0, "charts válidos")
		return nil
	},
}

// renderValidateReport escreve o resultado de cada etapa de validação (dependências,
// lint e template), uma linha por chart; para na etapa e no chart que falhou.
func renderValidateReport(w io.Writer, report *validate.ValidateReport) {
	if report == nil {
		return
	}

	// saida: a etapa traz a saída do helm (o erro devolvido pelo comando não a inclui).
	etapas := []struct {
		secao string
		ok    func(validate.ChartResult) bool
		saida bool
	}{
		{"dependências", func(c validate.ChartResult) bool { return c.DepsOK }, false},
		{"helm lint", func(c validate.ChartResult) bool { return c.LintOK }, false},
		{"template · simulação", func(c validate.ChartResult) bool { return c.TemplateOK }, true},
	}
	for _, e := range etapas {
		ui.Section(w, e.secao)
		for _, c := range report.Charts {
			if e.ok(c) {
				ui.Check(w, ui.CheckOk, c.Chart, "", "")
			} else if c.Error != "" {
				ui.Check(w, ui.CheckErro, c.Chart, "", "")
				if e.saida {
					saidaRecuada(w, c.Error, true)
				}
				return
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

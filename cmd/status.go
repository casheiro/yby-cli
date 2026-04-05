/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/services/status"
	"github.com/spf13/cobra"
)

// newStatusService permite substituição em testes.
var newStatusService = func(r shared.Runner) status.Service {
	inspector := &status.KubectlInspector{Runner: r}
	return status.NewService(inspector)
}

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Verifica status do cluster e apps",
	Long: `Exibe informações sobre os nós do cluster, pods do Argo CD e Ingresses configurados.
Equivalente ao antigo 'make status'.`,
	Example: `  yby status`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := lookPath("kubectl"); err != nil {
			return errors.New(errors.ErrCodeCmdNotFound, "kubectl não encontrado")
		}

		runner := &shared.RealRunner{}
		svc := newStatusService(runner)
		report := svc.Check(cmd.Context())

		renderStatusReport(report)
		return nil
	},
}

// renderStatusReport exibe o relatório de status usando os estilos do CLI.
func renderStatusReport(report *status.StatusReport) {
	fmt.Println(titleStyle.Render("📊 Status do Cluster"))
	fmt.Println("---------------------------------------")

	// Nodes
	fmt.Println(headerStyle.Render("🖥️  Nodes"))
	if !report.Nodes.Available {
		fmt.Println(crossStyle.Render(report.Nodes.Message))
		if report.Nodes.Output != "" {
			fmt.Println(grayStyle.Render(report.Nodes.Output))
		}
	} else {
		fmt.Println(report.Nodes.Output)
	}

	// Argo CD
	fmt.Println("")
	fmt.Println(headerStyle.Render("🐙 Argo CD Pods"))
	if !report.ArgoCD.Available {
		fmt.Println(warningStyle.Render(report.ArgoCD.Message))
	} else {
		fmt.Println(report.ArgoCD.Output)
	}

	// Ingresses
	fmt.Println("")
	fmt.Println(headerStyle.Render("🔗 Ingresses"))
	if !report.Ingress.Available {
		fmt.Println(warningStyle.Render(report.Ingress.Message))
	} else if report.Ingress.Output == "" {
		fmt.Println(report.Ingress.Message)
	} else {
		fmt.Println(report.Ingress.Output)
	}

	// KEDA
	fmt.Println("")
	fmt.Println(headerStyle.Render("⚡ Autoscaling (KEDA)"))
	if !report.KEDA.Available {
		fmt.Println(warningStyle.Render(report.KEDA.Message))
	} else if report.KEDA.Output == "" {
		fmt.Println(report.KEDA.Message)
	} else {
		fmt.Println(report.KEDA.Output)
	}

	// Kepler
	fmt.Println("")
	fmt.Println(headerStyle.Render("🍃 Eficiência Energética (Kepler)"))
	if !report.Kepler.Available {
		fmt.Println(crossStyle.Render(report.Kepler.Message))
	} else if report.Kepler.Message != "" {
		// Kepler disponível — verificar se está ativo ou só instalado
		if report.Kepler.Output != "" && report.Kepler.Message == "Sensor Kepler ATIVO e monitorando o cluster." {
			fmt.Println(checkStyle.Render(report.Kepler.Message))
		} else {
			fmt.Println(warningStyle.Render(report.Kepler.Message))
		}
	}
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

package cmd

import (
	"github.com/spf13/cobra"
)

// chartCmd represents the chart command
var chartCmd = &cobra.Command{
	Use:   "chart",
	Short: "Gerenciamento de Helm Charts (Workload Abstraction)",
	Long:  `Utilitários para criar, validar e gerenciar Charts Helm seguindo os padrões do Yby.`,
	Example: `  yby chart create my-service
  yby chart create my-api --dir infra/charts
  # Validar chart existente
  yby chart validate charts/my-service`,
}

func init() {
	rootCmd.AddCommand(chartCmd)
}

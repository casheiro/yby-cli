package cmd

import (
	"github.com/spf13/cobra"
)

// chartCmd represents the chart command
var chartCmd = &cobra.Command{
	Use:   "chart",
	Short: "Gerenciamento de Helm Charts (Workload Abstraction)",
	Long:  `Utilitários para criar, validar e gerenciar Charts Helm seguindo os padrões do Yby.`,
}

func init() {
	rootCmd.AddCommand(chartCmd)
}

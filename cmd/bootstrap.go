/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// bootstrapCmd represents the bootstrap command
var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Comandos para bootstrap (VPS ou Cluster)",
	Long: `Grupo de comandos para inicializar a infraestrutura.

Disponível:
  yby bootstrap vps      - Provisiona um servidor Linux do zero (K3s, Docker, Firewall)
  yby bootstrap cluster  - Instala a stack GitOps (ArgoCD, Workflows) no cluster conectado
`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)
}

/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/network"
	"github.com/spf13/cobra"
)

// accessCmd represents the access command
var accessCmd = &cobra.Command{
	Use:   "access",
	Short: "Abre túneis de acesso aos serviços do cluster",
	Long: `Estabelece conexões seguras (port-forward) para os serviços disponíveis:
- Argo CD
- MinIO (se detectado)
- Prometheus (para alimentar Grafana)
- Grafana Local (via Docker)
- Headlamp (Token)

Você pode especificar um contexto (local/prod) com --context.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🚀 Iniciando Acesso Unificado ao Cluster...")

		// Setup context with cancellation
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Setup signal handling for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		go func() {
			<-sigChan
			fmt.Println("\n🛑 Encerrando túneis...")
			cancel()
		}()

		targetContext, _ := cmd.Flags().GetString("context")

		// Dependency Injection
		netAdapter := network.NewClusterNetworkAdapter()
		containerAdapter := network.NewContainerAdapter()
		accessSvc := network.NewAccessService(netAdapter, containerAdapter)

		opts := network.AccessOptions{
			TargetContext: targetContext,
		}

		if err := accessSvc.Run(ctx, opts); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "Erro na execução de acesso")
		}

		fmt.Println("✅ Túneis encerrados.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(accessCmd)
	accessCmd.Flags().StringP("context", "c", "", "Nome do contexto Kubernetes")
}

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
	Run: func(cmd *cobra.Command, args []string) {
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
			fmt.Printf("⚠️  Erro na execução: %v\n", err)
			if osExit != nil {
				osExit(1)
			} else {
				os.Exit(1)
			}
		}

		fmt.Println("✅ Túneis encerrados.")
	},
}

func init() {
	rootCmd.AddCommand(accessCmd)
	accessCmd.Flags().StringP("context", "c", "", "Nome do contexto Kubernetes")
}

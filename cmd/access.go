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
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/ui"
	"github.com/spf13/cobra"
)

// newNetworkAdapter cria o adaptador de rede do cluster (mockável em testes). A saída do
// kubectl (as tabelas do get svc, o "Forwarding from…" e o "Handling connection for…" do
// port-forward) vai para o log em debug, linha a linha, em vez de cair no meio das linhas
// do access; nada fica acumulado durante a sessão.
var newNetworkAdapter = func() network.ClusterNetworkManager {
	return &network.RealClusterNetworkManager{Runner: runnerAoVivo{Runner: &shared.RealRunner{}}}
}

// newContainerAdapter cria o adaptador de containers locais (mockável em testes), com a
// saída do docker no log em debug, como a do kubectl.
var newContainerAdapter = func() network.LocalContainerManager {
	return &network.DockerContainerManager{Runner: runnerAoVivo{Runner: &shared.RealRunner{}}}
}

// accessCmd represents the access command
var accessCmd = &cobra.Command{
	Use:   "access",
	Short: "Abre túneis de acesso aos serviços do cluster",
	Example: `  yby access
  yby access --context prod
  # Abre túnel para o ArgoCD
  yby access --context local`,
	Long: `Estabelece conexões seguras (port-forward) para os serviços disponíveis:
- Argo CD
- MinIO (se detectado)
- Prometheus (para alimentar Grafana)
- Grafana Local (via Docker)
- Headlamp (Token)

Você pode especificar um contexto (local/prod) com --context.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cmd.OutOrStdout()
		ui.Title(w, "access", agora().Format("02/01 15:04"))

		// Setup context with cancellation
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Setup signal handling for graceful shutdown: ctrl+c encerra os túneis
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigChan)
		go func() {
			select {
			case <-sigChan:
				cancel()
			case <-ctx.Done():
			}
		}()

		targetContext, _ := cmd.Flags().GetString("context")

		// Dependency Injection (factories mockáveis)
		netAdapter := newNetworkAdapter()
		containerAdapter := newContainerAdapter()
		accessSvc := network.NewAccessService(netAdapter, containerAdapter)
		accessSvc.Saida = w

		opts := network.AccessOptions{
			TargetContext: targetContext,
		}

		if err := accessSvc.Run(ctx, opts); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao abrir os túneis").
				WithHint("confira o contexto do kubeconfig com yby doctor ou escolha um com --context")
		}

		fmt.Fprintln(w)
		ui.Check(w, ui.CheckNeutro, "túneis encerrados", "", "")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(accessCmd)
	accessCmd.Flags().StringP("context", "c", "", "Nome do contexto Kubernetes")
}

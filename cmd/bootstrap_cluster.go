/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/bootstrap"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

var bootstrapClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Instala a stack GitOps (ArgoCD, Workflows) no cluster conectado",
	Long: `Executa o bootstrap completo do cluster, instalando:
1. Argo CD, Argo Workflows e Argo Events (Infraestrutura)
2. System Charts (CRDs, Cert-Manager, Monitoring)
3. Configuração de Secrets (Git Credentials, Tokens)
4. Aplicação Root (App of Apps) para início do GitOps
5. Versions são lidas de .yby/blueprint.yaml se disponível.`,
	Example: `  # Bootstrap padrão (lê variáveis GITHUB_REPO e TOKEN do ambiente)
  yby bootstrap cluster

  # Forçar uso do blueprint para versões
  yby bootstrap cluster --context prod`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(titleStyle.Render("🚀 Yby Bootstrap - Cluster GitOps"))
		fmt.Println("---------------------------------------")

		// 0. Resolve Infra Root
		root, err := FindInfraRoot()
		if err != nil {
			fmt.Println(warningStyle.Render("⚠️  Raiz da infraestrutura não encontrada (.yby/). Assumindo diretório atual '.'"))
			root = "."
		} else {
			fmt.Printf("📂 Infraestrutura detectada em: %s\n", root)
		}

		// Inject Dependencies
		runner := &shared.RealRunner{}
		filesystem := &shared.RealFilesystem{}
		k8s := &bootstrap.RealK8sClient{Runner: runner}

		svc := bootstrap.NewService(runner, filesystem, k8s)

		opts := bootstrap.BootstrapOptions{
			Root:        root,
			RepoURL:     os.Getenv("GITHUB_REPO"),
			Context:     contextFlag,
			Environment: os.Getenv("YBY_ENV"),
		}

		if err := svc.Run(cmd.Context(), opts); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "Erro no bootstrap")
		}

		fmt.Println("\n" + checkStyle.Render("🎉 Bootstrap do Cluster concluído!"))
		fmt.Println("👉 Execute 'yby access' para acessar os dashboards.")
		return nil
	},
}

func init() {
	bootstrapCmd.AddCommand(bootstrapClusterCmd)
}

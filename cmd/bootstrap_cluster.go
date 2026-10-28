/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/services/bootstrap"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
	"github.com/spf13/cobra"
)

// newBootstrapClusterService permite substituição em testes
var newBootstrapClusterService = func(runner shared.Runner, fs shared.Filesystem) *bootstrap.BootstrapService {
	k8s := &bootstrap.RealK8sClient{Runner: runner}
	return bootstrap.NewService(runner, fs, k8s)
}

// acharRaizInfra localiza a raiz da infraestrutura (substituível em testes).
var acharRaizInfra = FindInfraRoot

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
		w := cmd.OutOrStdout()

		// 0. Resolve Infra Root
		root, err := acharRaizInfra()
		if err != nil {
			root = "."
		}
		ui.Title(w, "bootstrap cluster", agora().Format("02/01 15:04"))
		if err != nil {
			ui.Check(w, ui.CheckAviso, "raiz .yby/ não encontrada · usando o diretório atual", "", "")
		}

		// Inject Dependencies: a saída dos comandos externos não atropela as etapas, mas o
		// que um plugin exec do kubeconfig pede no stderr (login de um cluster gerenciado)
		// aparece abaixo delas
		prog := novoProgresso(w, bootstrap.Rotulos...)
		runner := runnerAoVivo{Runner: &shared.RealRunner{}, aviso: prog.Detalhe, atraso: atrasoAviso}
		filesystem := &shared.RealFilesystem{}
		svc := newBootstrapClusterService(runner, filesystem)

		opts := bootstrap.BootstrapOptions{
			Root:        root,
			RepoURL:     os.Getenv("GITHUB_REPO"),
			Context:     contextFlag,
			Environment: os.Getenv("YBY_ENV"),
			Etapas:      prog,
		}

		parar := animar(prog)
		err = svc.Run(cmd.Context(), opts)
		parar()
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "falha no bootstrap do cluster").
				WithHint(dicaDe(err, "confira o acesso ao cluster com yby doctor e rode o bootstrap de novo; ele retoma da última fase"))
		}

		fmt.Fprintln(w)
		ui.Check(w, ui.CheckOk, "stack GitOps instalada", "", "")
		ui.Check(w, ui.CheckNeutro, "yby access abre os dashboards", "", "")
		return nil
	},
}

func init() {
	bootstrapCmd.AddCommand(bootstrapClusterCmd)
}

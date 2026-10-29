package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/services/environment"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/casheiro/yby-cli/v5/pkg/ui"

	"github.com/spf13/cobra"
)

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroi o ambiente (cluster k3d)",
	Long: `Remove o cluster Yby e limpa recursos associados.
ATENÇÃO: Este comando é destrutivo e removerá o cluster criado pelo 'yby up'.
Para ambientes não-locais (dev/staging/prod), o cluster k3d vem de YBY_CLUSTER_NAME
(sem ela, o comando recusa em vez de apagar o cluster local) e exige a flag
--yes-destroy-production e confirmação interativa digitando o nome do cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		w := cmd.OutOrStdout()
		env := os.Getenv("YBY_ENV")
		if env == "" {
			env = contextFlag
		}

		ambiente := env
		if ambiente == "" {
			ambiente = "local"
		}
		ui.Title(w, "destroy", "ambiente "+ambiente)

		// o cluster padrão (yby-local) é o do ambiente local: outro ambiente precisa dizer
		// qual cluster k3d é o dele, senão o destroy apagaria o cluster local
		clusterName := os.Getenv("YBY_CLUSTER_NAME")
		if clusterName == "" {
			if ambiente != "local" {
				return errors.New(errors.ErrCodeValidation,
					fmt.Sprintf("o ambiente %s não tem cluster k3d definido", env)).
					WithHint("o yby destroy remove clusters k3d: defina o do ambiente em YBY_CLUSTER_NAME; um cluster remoto se remove pelo provedor")
			}
			clusterName = "yby-local"
		}

		// Para ambientes não-locais, exigir double-confirm
		if env != "" && env != "local" {
			yesDestroy, _ := cmd.Flags().GetBool("yes-destroy-production")
			if !yesDestroy {
				return errors.New(errors.ErrCodeValidation,
					fmt.Sprintf("destruir o ambiente %s pede --yes-destroy-production", env)).
					WithHint(fmt.Sprintf("confirme com yby destroy --context %s --yes-destroy-production", env))
			}

			// Confirmação interativa: ↵ só libera depois de digitado o nome do cluster
			confirmado, err := prompter.ConfirmarNome("destruir cluster", clusterName)
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeIO, "falha ao ler confirmação").
					WithHint("rode yby destroy num terminal interativo para digitar o nome do cluster")
			}
			if !confirmado {
				ui.Check(w, ui.CheckNeutro, "destruição cancelada", "", "")
				return nil
			}
		}

		rotulo := "cluster " + clusterName
		prog := novoProgresso(w, rotulo)
		parar := animar(prog)
		defer parar()
		prog.Iniciar(rotulo)
		cluster := newDestroyClusterManager()
		if err := cluster.Delete(cmd.Context(), clusterName); err != nil {
			prog.Falhar("")
			return errors.Wrap(err, errors.ErrCodeExec, "falha ao destruir o cluster").
				WithHint("confira o nome com k3d cluster list e se o Docker está rodando")
		}
		prog.Concluir("removido")
		return nil
	},
}

// newDestroyClusterManager cria o ClusterManager para o comando destroy (mockável em
// testes); os comandos externos rodam quietos para não atropelar a etapa na tela.
var newDestroyClusterManager = func() environment.ClusterManager {
	return &environment.K3dClusterManager{Runner: runnerQuieto{&shared.RealRunner{}}}
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().Bool("yes-destroy-production", false, "Confirma destruição de ambientes não-locais (requerido para dev/staging/prod)")
}

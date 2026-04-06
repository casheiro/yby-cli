package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/environment"
	"github.com/casheiro/yby-cli/pkg/services/shared"

	"github.com/spf13/cobra"
)

// newDestroyClusterManager cria o ClusterManager para o comando destroy (mockável em testes)
var newDestroyClusterManager = func() environment.ClusterManager {
	return &environment.K3dClusterManager{Runner: &shared.RealRunner{}}
}

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroi o ambiente (cluster k3d)",
	Long: `Remove o cluster Yby e limpa recursos associados.
ATENÇÃO: Este comando é destrutivo e removerá o cluster criado pelo 'yby up'.
Para ambientes não-locais (dev/staging/prod), exige a flag --yes-destroy-production
e confirmação interativa digitando o nome do cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		env := os.Getenv("YBY_ENV")
		if env == "" {
			env = contextFlag
		}

		clusterName := os.Getenv("YBY_CLUSTER_NAME")
		if clusterName == "" {
			clusterName = "yby-local"
		}

		// Para ambientes não-locais, exigir double-confirm
		if env != "" && env != "local" {
			yesDestroy, _ := cmd.Flags().GetBool("yes-destroy-production")
			if !yesDestroy {
				return errors.New(errors.ErrCodeValidation,
					fmt.Sprintf("destruir ambiente '%s' requer a flag --yes-destroy-production", env)).
					WithHint("Use: yby destroy --yes-destroy-production")
			}

			// Confirmação interativa: digitar o nome do cluster
			typed, err := prompter.Input(
				fmt.Sprintf("Digite o nome do cluster '%s' para confirmar a destruição:", clusterName),
				"",
			)
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeIO, "falha ao ler confirmação")
			}
			if typed != clusterName {
				return errors.New(errors.ErrCodeValidation,
					fmt.Sprintf("nome digitado '%s' não confere com o cluster '%s'. Destruição cancelada", typed, clusterName))
			}
		}

		fmt.Printf("💣 Destruindo cluster '%s'...\n", clusterName)

		cluster := newDestroyClusterManager()
		if err := cluster.Delete(cmd.Context(), clusterName); err != nil {
			return errors.Wrap(err, errors.ErrCodeExec, "Erro ao destruir cluster")
		}

		fmt.Println("✅ Cluster removido com sucesso.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
	destroyCmd.Flags().Bool("yes-destroy-production", false, "Confirma destruição de ambientes não-locais (requerido para dev/staging/prod)")
}

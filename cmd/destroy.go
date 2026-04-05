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
	Short: "Destroi o ambiente local (cluster k3d)",
	Long: `Remove o cluster Yby local e limpa recursos associados.
ATENÇÃO: Este comando é destrutivo e removerá o cluster criado pelo 'yby up' no modo local.
Não afeta ambientes remotos (dev/staging/prod) por segurança.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Safety Check: Only local allowed
		env := os.Getenv("YBY_ENV")
		if env == "" {
			env = contextFlag
		}
		if env != "" && env != "local" {
			return errors.New(errors.ErrCodeValidation, fmt.Sprintf("'yby destroy' só é permitido no ambiente local. Ambiente atual: %s", env))
		}

		clusterName := os.Getenv("YBY_CLUSTER_NAME")
		if clusterName == "" {
			clusterName = "yby-local"
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
}

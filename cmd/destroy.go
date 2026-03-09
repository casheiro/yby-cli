package cmd

import (
	"fmt"
	"os"

	"github.com/casheiro/yby-cli/pkg/errors"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroi o ambiente local (cluster k3d)",
	Long: `Remove o cluster Yby local e limpa recursos associados.
ATENÇÃO: Este comando é destrutivo e removerá o cluster criado pelo 'yby up' no modo local.
Não afeta ambientes remotos (dev/staging/prod) por segurança.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Safety Check: Only local allowed
		env := viper.GetString("environment")
		if env != "" && env != "local" {
			return errors.New(errors.ErrCodeValidation, fmt.Sprintf("'yby destroy' só é permitido no ambiente local. Ambiente atual: %s", env))
		}

		clusterName := os.Getenv("YBY_CLUSTER_NAME")
		if clusterName == "" {
			clusterName = "yby-local"
		}

		fmt.Printf("💣 Destruindo cluster '%s'...\n", clusterName)

		// Run k3d delete
		c := execCommand("k3d", "cluster", "delete", clusterName)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			fmt.Printf("❌ Erro ao destruir cluster: %v\n", err)
			return errors.Wrap(err, errors.ErrCodeExec, "Erro ao destruir cluster")
		} else {
			fmt.Println("✅ Cluster removido com sucesso.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}

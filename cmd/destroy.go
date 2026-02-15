package cmd

import (
	"fmt"
	"os"
	"os/exec"

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
	Run: func(cmd *cobra.Command, args []string) {
		// Safety Check: Only local allowed
		env := viper.GetString("environment")
		if env != "" && env != "local" {
			fmt.Printf("❌ 'yby destroy' só é permitido no ambiente local. Ambiente atual: %s\n", env)
			os.Exit(1)
		}

		clusterName := os.Getenv("YBY_CLUSTER_NAME")
		if clusterName == "" {
			clusterName = "yby-local"
		}

		fmt.Printf("💣 Destruindo cluster '%s'...\n", clusterName)

		// Run k3d delete
		c := exec.Command("k3d", "cluster", "delete", clusterName)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			fmt.Printf("❌ Erro ao destruir cluster: %v\n", err)
			// Don't exit 1, maybe partial success?
		} else {
			fmt.Println("✅ Cluster removido com sucesso.")
		}
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)
}

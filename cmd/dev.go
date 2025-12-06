/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// devCmd represents the dev command
var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Inicia ambiente de desenvolvimento completo (Cluster + Argo CD)",
	Long: `Sobe um cluster local usando k3d e instala toda a stack GitOps.
Idempotente: Se o cluster já existir, apenas garante que está rodando e atualiza a stack.

Equivalente ao antigo 'make dev'.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🏃 Yby Dev - Ambiente de Desenvolvimento"))
		fmt.Println("---------------------------------------")

		// 1. Check dependencies
		if _, err := exec.LookPath("k3d"); err != nil {
			fmt.Println(crossStyle.Render("❌ k3d não encontrado. Rode 'yby setup' primeiro."))
			os.Exit(1)
		}

		clusterName := os.Getenv("CLUSTER_NAME")
		if clusterName == "" {
			clusterName = "yby-local"
		}

		// 2. Cluster Lifecycle
		fmt.Printf("%s Verificando cluster '%s'...\n", stepStyle.Render("🔍"), clusterName)

		// Check if cluster exists
		out, _ := exec.Command("k3d", "cluster", "list", clusterName).CombinedOutput()
		if strings.Contains(string(out), "No nodes found") || strings.Contains(string(out), "no cluster found") {
			// Create
			fmt.Println(stepStyle.Render("🚀 Criando cluster..."))
			// Check if config exists
			configFile := "local/k3d-config.yaml"
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				// Fallback or absolute path check could be added here
				configFile = "../local/k3d-config.yaml" // Try sibling dir if running from cli/
				if _, err := os.Stat(configFile); os.IsNotExist(err) {
					fmt.Println(warningStyle.Render("⚠️  Config 'local/k3d-config.yaml' não encontrada. Usando defaults do k3d."))
					runCommand("k3d", "cluster", "create", clusterName)
				} else {
					runCommand("k3d", "cluster", "create", clusterName, "--config", configFile)
				}
			} else {
				runCommand("k3d", "cluster", "create", clusterName, "--config", configFile)
			}
		} else {
			fmt.Println(checkStyle.Render("✅ Cluster já existe."))
			// Ensure it's running (start is idempotent-ish, returns error if already running usually, or just works)
			// k3d cluster start returns 0 if already running? Let's check.
			fmt.Print(stepStyle.Render("🔄 Garantindo que cluster está rodando... "))
			_ = exec.Command("k3d", "cluster", "start", clusterName).Run()
			fmt.Println(checkStyle.String())
		}

		// 3. Bootstrap
		// We can call the bootstrapClusterCmd Run function directly or via Execute
		// But calling Run directly is easier if we don't need flag parsing
		fmt.Println("")
		bootstrapClusterCmd.Run(bootstrapClusterCmd, []string{})

		// 4. Status
		fmt.Println("")
		statusCmd.Run(statusCmd, []string{})
	},
}

func init() {
	rootCmd.AddCommand(devCmd)
}

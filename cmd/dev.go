/*
Copyright © 2025 Yby Team
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/casheiro/yby-cli/pkg/config"
	ybyctx "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/mirror"
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

		// 0. Context & Env Init
		wd, _ := os.Getwd()
		ctxManager := ybyctx.NewManager(wd)
		cfg, _ := config.Load()

		activeCtx, err := ctxManager.ResolveActive(contextFlag, cfg)
		if err != nil {
			fmt.Printf("❌ Erro resolvendo contexto: %v\n", err)
			os.Exit(1)
		}
		if err := ctxManager.LoadContext(activeCtx); err != nil {
			fmt.Printf("❌ Erro carregando ambiente: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("🌍 Contexto Ativo: %s\n", activeCtx)

		// 1. Check dependencies
		if _, err := exec.LookPath("k3d"); err != nil {
			fmt.Println(crossStyle.Render("❌ k3d não encontrado. Rode 'yby setup' primeiro."))
			os.Exit(1)
		}

		// Env Vars now populated by LoadContext
		clusterName := os.Getenv("YBY_CLUSTER_NAME")
		if clusterName == "" {
			clusterName = os.Getenv("CLUSTER_NAME") // Backward compat
		}
		if clusterName == "" {
			clusterName = "yby-local"
		}

		// 2. Cluster Lifecycle
		fmt.Printf("%s Verificando cluster '%s'...\n", stepStyle.Render("🔍"), clusterName)

		// Check if cluster exists
		out, _ := exec.Command("k3d", "cluster", "list", clusterName).CombinedOutput()
		if strings.Contains(string(out), "No nodes found") || strings.Contains(string(out), "no cluster found") {
			// Create logic (omitted full logic for brevity, relying on previous robust logic or simply calling create)
			// Re-using previous logic exactly:
			fmt.Println(stepStyle.Render("🚀 Criando cluster..."))
			configFile := "local/k3d-config.yaml" // Default
			if cfgVal := os.Getenv("YBY_K3D_CONFIG"); cfgVal != "" {
				configFile = cfgVal
			}

			if _, err := os.Stat(configFile); os.IsNotExist(err) && !strings.HasSuffix(configFile, ".yaml") {
				// Naive check
				configFile = ""
			}

			k3dArgs := []string{"cluster", "create", clusterName}
			if configFile != "" {
				if _, err := os.Stat(configFile); err == nil {
					k3dArgs = append(k3dArgs, "--config", configFile)
				}
			}
			runCommand("k3d", k3dArgs...)
		} else {
			fmt.Println(checkStyle.Render("✅ Cluster já existe."))
			fmt.Print(stepStyle.Render("🔄 Garantindo que cluster está rodando... "))
			_ = exec.Command("k3d", "cluster", "start", clusterName).Run()
			fmt.Println(checkStyle.String())
		}

		// 3. Mirror Setup (Hybrid GitOps)
		var mirrorMgr *mirror.MirrorManager
		if activeCtx == "local" || os.Getenv("YBY_MODE") == "mirror" {
			fmt.Println("🪞 Inicializando Local Mirror (Hybrid GitOps)...")
			mirrorMgr = mirror.NewManager(".")
			if err := mirrorMgr.EnsureGitServer(); err != nil {
				fmt.Printf(warningStyle.Render("⚠️ Falha ao garantir Git Server: %v\n"), err)
				mirrorMgr = nil // Disable sync if init failed
			} else {
				fmt.Println(checkStyle.Render("✅ Git Server Operacional."))
			}
		}

		// 4. Boostrap
		fmt.Println("")
		bootstrapClusterCmd.Run(bootstrapClusterCmd, []string{})

		// 5. Status
		fmt.Println("")
		statusCmd.Run(statusCmd, []string{})

		// 6. Blocking Sync Loop
		if mirrorMgr != nil {
			fmt.Println("")
			fmt.Println(titleStyle.Render("🔄 Yby Dev Mirror Active"))
			fmt.Println("   Press Ctrl+C to stop syncing.")
			mirrorMgr.StartSyncLoop(context.Background())
		}
	},
}

func init() {
	rootCmd.AddCommand(devCmd)
}

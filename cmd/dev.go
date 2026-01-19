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
	Example: `  # Iniciar ambiente local
  yby dev

  # Resetar ambiente local (deleta cluster e recria)
  yby destroy && yby dev`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(titleStyle.Render("🏃 Yby Dev - Ambiente de Desenvolvimento"))
		fmt.Println("---------------------------------------")

		// 0. Resolve Infra Root
		root, err := FindInfraRoot()
		if err != nil {
			// If not found, try current directory if it has environments.yaml
			// Or just assume "." if it's a new project but maybe failed check?
			// The error usually means we are too far up or down.
			// Let's stick to "." but warn.
			fmt.Printf("⚠️  Infra root não detectada automaticamente: %v. Usando '.'\n", err)
			root = "."
		}

		// 1. Enforce Local Semantics
		if contextFlag != "" && contextFlag != "local" {
			fmt.Println(crossStyle.Render("❌ Erro: 'yby dev' é exclusivo para ambiente local."))
			fmt.Printf("Para contextos remotos (%s), utilize 'yby bootstrap cluster' e GitOps.\n", contextFlag)
			os.Exit(1)
		}
		// Force local
		contextFlag = "local"
		os.Setenv("YBY_ENV", "local")

		// 2. Context & Env Init
		// Phase 5 Fix: Use discovered root for context manager
		// wd, _ := os.Getwd() -> replaced by root
		ctxManager := ybyctx.NewManager(root)

		// Validate if 'local' exists in manifest
		activeCtx, env, err := ctxManager.GetCurrent()
		if err != nil {
			fmt.Printf("❌ Erro resolvendo contexto: %v\n", err)
			fmt.Println("Dica: Certifique-se que o environments.yaml existe (yby init) e possui um ambiente 'local'.")
			os.Exit(1)
		}

		if env.Type != "local" {
			fmt.Printf("❌ Erro: Ambiente '%s' não é do tipo 'local' (tipo atual: %s)\n", activeCtx, env.Type)
			os.Exit(1)
		}

		fmt.Printf("🌍 Contexto Ativo: %s (Forçado)\n", activeCtx)

		// 3. Check dependencies
		if _, err := exec.LookPath("k3d"); err != nil {
			fmt.Println(crossStyle.Render("❌ k3d não encontrado. Rode 'yby setup' primeiro."))
			os.Exit(1)
		}

		// Env Vars now populated by explicit env mgmt? NO.
		// pkg/context removed LoadContext which loaded .env.
		// So we must rely on explicit config or defaults.
		clusterName := os.Getenv("YBY_CLUSTER_NAME")
		if clusterName == "" {
			clusterName = os.Getenv("CLUSTER_NAME") // Backward compat
		}
		if clusterName == "" {
			clusterName = "yby-local"
		}

		// 4. Cluster Lifecycle
		fmt.Printf("%s Verificando cluster '%s'...\n", stepStyle.Render("🔍"), clusterName)

		// Check if cluster exists
		out, _ := exec.Command("k3d", "cluster", "list", clusterName).CombinedOutput()
		if strings.Contains(string(out), "No nodes found") || strings.Contains(string(out), "no cluster found") {
			fmt.Println(stepStyle.Render("🚀 Criando cluster..."))
			configFile := JoinInfra(root, "local/k3d-config.yaml") // Default
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

		// 5. Mirror Setup (Hybrid GitOps)
		var mirrorMgr *mirror.MirrorManager
		// Always run mirror in dev mode
		fmt.Println("🪞 Inicializando Local Mirror (Hybrid GitOps)...")
		mirrorMgr = mirror.NewManager(root)
		if err := mirrorMgr.EnsureGitServer(); err != nil {
			fmt.Printf(warningStyle.Render("⚠️ Falha ao garantir Git Server: %v\n"), err)
			mirrorMgr = nil // Disable sync if init failed
		} else {
			fmt.Println(checkStyle.Render("✅ Git Server Operacional."))
			// Initial Sync to populate the repo BEFORE bootstrap
			fmt.Print(stepStyle.Render("🔄 Sincronizando código inicial para o Mirror... "))
			if err := mirrorMgr.Sync(); err != nil {
				fmt.Printf(crossStyle.Render("❌ Falha no Sync inicial: %v\n"), err)
				// We proceed, but ArgoCD might fail to sync app
			} else {
				fmt.Println(checkStyle.Render("✅ Código sincronizado."))
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
			fmt.Println(titleStyle.Render("🔄 Espelho de Desenvolvimento Yby Ativo"))
			fmt.Println("   Pressione Ctrl+C para parar a sincronização.")
			mirrorMgr.StartSyncLoop(context.Background())
		}
	},
}

func init() {
	rootCmd.AddCommand(devCmd)
}

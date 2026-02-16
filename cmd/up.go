package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	ybyctx "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/mirror"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// upCmd represents the up command
var upCmd = &cobra.Command{
	Use:     "up",
	Aliases: []string{"dev"}, // Backward compatibility
	Short:   "Inicia ou verifica o ambiente (Local = Sync, Remoto = Check)",
	Long: `O comando 'up' coloca o ambiente no estado desejado.

Comportamento por Ambiente:
  - local: Inicia cluster (se necessário), configura Git Mirror, cria Túnel de Sync e mantém sincronização automática.
  - dev/staging/prod: Verifica acesso ao cluster e estado do GitOps. NÃO inicia sincronização local (use 'git push').`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 1. Setup Signal Handling
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println("\n🛑 Encerrando graciosamente...")
			cancel()
		}()

		// 2. Resolve Infra Root
		root, err := FindInfraRoot()
		if err != nil {
			// Try current dir
			root = "."
		}

		// 3. Detect Environment Strategy
		// We use pkg/context to detect the *configured* environment for this root
		ctxManager := ybyctx.NewManager(root)
		activeCtx, envDef, err := ctxManager.GetCurrent()

		// Fallback specific for 'yby init' flow
		targetEnv := viper.GetString("environment")
		if targetEnv == "" {
			if err == nil {
				targetEnv = envDef.Type // Use type as env logic
			} else {
				targetEnv = "local" // Default to local if no context
			}
		}

		fmt.Printf("🚀 Iniciando ambiente '%s' (Contexto: %s)...\n", targetEnv, activeCtx)

		// 4. Logic Branch
		if targetEnv == "local" {
			// Force YBY_ENV for subcommands
			os.Setenv("YBY_ENV", "local")
			runLocalUp(ctx, root)
		} else {
			runRemoteUp(ctx, targetEnv)
		}
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runLocalUp(ctx context.Context, root string) {
	// A. Check dependencies
	if _, err := exec.LookPath("k3d"); err != nil {
		fmt.Println("❌ k3d não encontrado. Rode 'yby setup' primeiro.")
		os.Exit(1)
	}

	clusterName := os.Getenv("YBY_CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "yby-local"
	}

	// B. Cluster Lifecycle
	fmt.Printf("🔍 Verificando cluster '%s'...\n", clusterName)
	checkCmd := exec.Command("k3d", "cluster", "list", clusterName)
	if err := checkCmd.Run(); err != nil {
		// Cluster doesn't exist (k3d returns error if not found? or just empty list?)
		// standard k3d list returns exit code 0 usually unless error.
		// grep it?
		// Let's rely on standard logic: Try verify, if fail create.
		// Simplified:
		fmt.Println("🚀 Criando cluster...")

		k3dArgs := []string{"cluster", "create", clusterName}
		// Config logic
		configFile := JoinInfra(root, "local/k3d-config.yaml")
		if _, err := os.Stat(configFile); err == nil {
			k3dArgs = append(k3dArgs, "--config", configFile)
		}

		runCommand("k3d", k3dArgs...)
	} else {
		fmt.Println("✅ Cluster já existe. Garantindo start...")
		if err := execCommand("k3d", "cluster", "start", clusterName).Run(); err != nil {
			fmt.Printf("❌ Falha ao iniciar cluster '%s': %v\n", clusterName, err)
			osExit(1)
		}
	}

	// C. Mirror & Sync
	fmt.Println("🪞 Inicializando Local Mirror (Hybrid GitOps)...")
	mirrorMgr := mirror.NewManager(root)

	if err := mirrorMgr.EnsureGitServer(); err != nil {
		fmt.Printf("❌ Falha no Git Server: %v\n", err)
		return
	}

	// Tunnel (CRITICAL FIX)
	fmt.Println("🔌 Estabelecendo Túnel Seguro...")
	if err := mirrorMgr.SetupTunnel(ctx); err != nil {
		fmt.Printf("❌ Falha no Túnel: %v\n", err)
		return
	}

	// Initial Sync
	fmt.Print("🔄 Sincronizando código inicial... ")
	if err := mirrorMgr.Sync(); err != nil {
		fmt.Printf("❌ Falha no Sync inicial: %v\n", err)
		// Proceed anyway? Bootstrap might fail if repo empty.
	} else {
		fmt.Println("✅ Código sincronizado.")
	}

	// D. Bootstrap
	fmt.Println("🛠️  Executando Bootstrap...")
	// We call the existing bootstrap command logic
	bootstrapClusterCmd.Run(bootstrapClusterCmd, []string{})

	// E. Status
	statusCmd.Run(statusCmd, []string{})

	// F. Sync Loop
	fmt.Println("\n🔄 Espelho de Desenvolvimento Yby Ativo (Ctrl+C para parar)")
	mirrorMgr.StartSyncLoop(ctx)
}

func runRemoteUp(ctx context.Context, env string) {
	fmt.Printf("🌍 Ambiente Remoto Detectado: %s\n", env)
	fmt.Println("ℹ️  Modo de Operação: Observação (Sync Local Desativado)")
	fmt.Println("📝 Para atualizar o cluster, faça commit e push para o repositório remoto:")
	fmt.Println("   git push origin main")

	// Delegate to status
	statusCmd.Run(statusCmd, []string{})
}

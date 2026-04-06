package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	ybyctx "github.com/casheiro/yby-cli/pkg/context"
	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/bootstrap"
	"github.com/casheiro/yby-cli/pkg/services/environment"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/spf13/cobra"
)

var plainSecrets bool

// upCmd represents the up command
var upCmd = &cobra.Command{
	Use:     "up",
	Aliases: []string{"dev"}, // Backward compatibility
	Short:   "Inicia ou verifica o ambiente (Local = Sync, Remoto = Check)",
	Long: `O comando 'up' coloca o ambiente no estado desejado.

Comportamento por Ambiente:
  - local: Inicia cluster (se necessário), configura Git Mirror, cria Túnel de Sync e mantém sincronização automática.
  - dev/staging/prod: Verifica acesso ao cluster e estado do GitOps. NÃO inicia sincronização local (use 'git push').`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
		targetEnv := os.Getenv("YBY_ENV")
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
			return runLocalUp(ctx, root)
		} else {
			return runRemoteUp(ctx, targetEnv)
		}
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
	upCmd.Flags().BoolVar(&plainSecrets, "plain-secrets", false,
		"Usa secrets simples (não encriptados) no ambiente local. Evita problemas ao recriar clusters.")
}

// newLocalEnvironmentService cria o serviço de ambiente local com todas as dependências (mockável em testes)
var newLocalEnvironmentService = func(root string) *environment.EnvironmentService {
	runner := &shared.RealRunner{}
	fs := &shared.RealFilesystem{}
	cluster := &environment.K3dClusterManager{Runner: runner}
	mirrorAdapter := environment.NewGitMirrorAdapter(root, runner)
	k8s := &bootstrap.RealK8sClient{Runner: runner}
	bs := bootstrap.NewService(runner, fs, k8s)
	return environment.NewEnvironmentService(runner, fs, cluster, mirrorAdapter, bs)
}

func runLocalUp(ctx context.Context, root string) error {
	// 1. Dependency Injection Setup
	envSvc := newLocalEnvironmentService(root)

	// 2. Options Resolution
	clusterName := os.Getenv("YBY_CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "yby-local"
	}

	opts := environment.UpOptions{
		Root:         root,
		Environment:  "local",
		ClusterName:  clusterName,
		PlainSecrets: plainSecrets,
	}

	// 3. Execution
	if err := envSvc.Up(ctx, opts); err != nil {
		return errors.Wrap(err, errors.ErrCodeExec, "Falha ao iniciar ambiente local")
	}

	// 4. Final status report
	fmt.Println("")
	_ = statusCmd.RunE(statusCmd, []string{})

	// Maintain sync loop if context is active
	// Note: StartSyncLoop is already started as a goroutine in Up() if local
	<-ctx.Done()
	return nil
}

func runRemoteUp(ctx context.Context, env string) error {
	fmt.Printf("🌍 Ambiente Remoto Detectado: %s\n", env)
	fmt.Println("ℹ️  Modo de Operação: Observação (Sync Local Desativado)")
	fmt.Println("📝 Para atualizar o cluster, faça commit e push para o repositório remoto:")
	fmt.Println("   git push origin main")

	// Delegate to status
	_ = statusCmd.RunE(statusCmd, []string{})
	return nil
}

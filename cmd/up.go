package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	ybyctx "github.com/casheiro/yby-cli/v5/pkg/context"
	"github.com/casheiro/yby-cli/v5/pkg/errors"
	"github.com/casheiro/yby-cli/v5/pkg/services/bootstrap"
	"github.com/casheiro/yby-cli/v5/pkg/services/environment"
	"github.com/casheiro/yby-cli/v5/pkg/services/shared"
	"github.com/casheiro/yby-cli/v5/pkg/ui"
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
		w := cmd.OutOrStdout()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 1. Setup Signal Handling: ctrl+c encerra o auto-sync
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		go func() {
			select {
			case <-sigCh:
				cancel()
			case <-ctx.Done():
			}
		}()

		// 2. Resolve Infra Root
		root, err := acharRaizInfra()
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

		direita := "ambiente " + targetEnv
		if activeCtx != "" && activeCtx != targetEnv {
			direita += " · contexto " + activeCtx
		}
		ui.Title(w, "up", direita)

		// 4. Logic Branch
		if targetEnv == "local" {
			// Force YBY_ENV for subcommands
			os.Setenv("YBY_ENV", "local")
			return runLocalUp(ctx, w, root)
		}
		return runRemoteUp(ctx, w, targetEnv)
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
	upCmd.Flags().BoolVar(&plainSecrets, "plain-secrets", false,
		"Usa secrets simples (não encriptados) no ambiente local. Evita problemas ao recriar clusters.")
}

// newLocalEnvironmentService cria o serviço de ambiente local com todas as dependências
// (mockável em testes); w recebe as linhas do auto-sync. Os comandos externos rodam
// quietos para não atropelar as etapas na tela.
var newLocalEnvironmentService = func(root string, w io.Writer) *environment.EnvironmentService {
	runner := runnerQuieto{&shared.RealRunner{}}
	fs := &shared.RealFilesystem{}
	cluster := &environment.K3dClusterManager{Runner: runner}
	mirrorAdapter := environment.NewGitMirrorAdapter(root, runner).ComSaida(w)
	k8s := &bootstrap.RealK8sClient{Runner: runner}
	bs := bootstrap.NewService(runner, fs, k8s)
	return environment.NewEnvironmentService(runner, fs, cluster, mirrorAdapter, bs)
}

// statusDepoisDoUp escreve o relatório do yby status ao fim do up; sem kubectl, não há
// o que mostrar.
func statusDepoisDoUp(ctx context.Context, w io.Writer) {
	if _, err := lookPath("kubectl"); err != nil {
		return
	}
	fmt.Fprintln(w)
	renderStatusReport(w, newStatusService(&shared.RealRunner{}).Check(ctx), agora())
}

func runLocalUp(ctx context.Context, w io.Writer, root string) error {
	// 1. Dependency Injection Setup
	envSvc := newLocalEnvironmentService(root, w)

	// 2. Options Resolution
	clusterName := os.Getenv("YBY_CLUSTER_NAME")
	if clusterName == "" {
		clusterName = "yby-local"
	}

	opts := environment.UpOptions{
		Root:             root,
		Environment:      "local",
		ClusterName:      clusterName,
		PlainSecrets:     plainSecrets,
		SyncPeloChamador: true,
	}

	// 3. Execution, uma etapa por fase
	prog := novoProgresso(w, environment.PlanoLocal(clusterName)...)
	envSvc.Etapas = prog
	parar := animar(prog)
	err := envSvc.Up(ctx, opts)
	parar()
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeExec, "falha ao iniciar o ambiente local").
			WithHint(dicaDe(err, "rode yby doctor para conferir docker, k3d e kubectl"))
	}

	// 4. Final status report
	statusDepoisDoUp(ctx, w)

	// 5. Auto-sync até o ctrl+c
	fmt.Fprintln(w)
	envSvc.Mirror.StartSyncLoop(ctx)
	return nil
}

func runRemoteUp(ctx context.Context, w io.Writer, env string) error {
	ui.Check(w, ui.CheckNeutro, "só observação: o sync local fica desligado em "+env, "", "")
	ui.Check(w, ui.CheckNeutro, "para atualizar o cluster, faça commit e git push origin main", "", "")

	// Delegate to status
	statusDepoisDoUp(ctx, w)
	return nil
}

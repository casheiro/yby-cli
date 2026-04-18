package environment

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/services/bootstrap"
	"github.com/casheiro/yby-cli/pkg/services/shared"
)

// validEnvironmentTypes lista todos os tipos de ambiente reconhecidos.
var validEnvironmentTypes = []string{"local", "remote", "eks", "aks", "gke"}

// IsValidEnvironmentType informa se t é um tipo de ambiente reconhecido.
func IsValidEnvironmentType(t string) bool {
	for _, v := range validEnvironmentTypes {
		if v == t {
			return true
		}
	}
	return false
}

// EnvironmentService orchestrates the 'up' command logic
type EnvironmentService struct {
	Runner    shared.Runner
	FS        shared.Filesystem
	Cluster   ClusterManager
	Mirror    MirrorService
	Bootstrap BootstrapService
}

func NewEnvironmentService(
	runner shared.Runner,
	fs shared.Filesystem,
	cluster ClusterManager,
	mirror MirrorService,
	bs BootstrapService,
) *EnvironmentService {
	return &EnvironmentService{
		Runner:    runner,
		FS:        fs,
		Cluster:   cluster,
		Mirror:    mirror,
		Bootstrap: bs,
	}
}

func (s *EnvironmentService) Up(ctx context.Context, opts UpOptions) error {
	if opts.Environment == "local" {
		return s.runLocalUp(ctx, opts)
	}
	return s.runRemoteUp(ctx, opts)
}

func (s *EnvironmentService) runLocalUp(ctx context.Context, opts UpOptions) error {
	// 1. Check dependencies
	if _, err := s.Runner.LookPath("k3d"); err != nil {
		return fmt.Errorf("k3d não encontrado. Rode 'yby setup' primeiro")
	}

	// 2. Cluster Lifecycle
	exists, err := s.Cluster.Exists(ctx, opts.ClusterName)
	if err != nil || !exists {
		slog.Info("Cluster não encontrado, criando", "cluster", opts.ClusterName)
		// Try to find config file
		configFile := "" // Logic for config path could be injected or resolved
		if err := s.Cluster.Create(ctx, opts.ClusterName, configFile); err != nil {
			return fmt.Errorf("falha ao criar cluster: %w", err)
		}
	} else {
		slog.Info("Cluster já existe, garantindo start", "cluster", opts.ClusterName)
		if err := s.Cluster.Start(ctx, opts.ClusterName); err != nil {
			return fmt.Errorf("falha ao iniciar cluster: %w", err)
		}
	}

	// 3. Mirror & Sync
	slog.Info("Inicializando Git Mirror")
	if err := s.Mirror.EnsureGitServer(); err != nil {
		return err
	}

	slog.Info("Estabelecendo túnel")
	if err := s.Mirror.SetupTunnel(ctx); err != nil {
		return err
	}

	slog.Info("Sincronizando código")
	if err := s.Mirror.Sync(); err != nil {
		slog.Warn("Falha no sync inicial", "error", err)
	}

	// 4. Bootstrap
	slog.Info("Executando Bootstrap")
	bsOpts := bootstrap.BootstrapOptions{
		Root:         opts.Root,
		Context:      "local",
		Environment:  "local",
		PlainSecrets: opts.PlainSecrets,
	}
	if err := s.Bootstrap.Run(ctx, bsOpts); err != nil {
		return fmt.Errorf("falha no bootstrap: %w", err)
	}

	// 4.5 Aguardar convergência do ArgoCD
	slog.Info("Aguardando ArgoCD convergir")
	if err := s.Bootstrap.WaitHealthy(ctx, "root-app", "argocd", 180); err != nil {
		slog.Warn("Timeout aguardando convergência do ArgoCD", "error", err,
			"dica", "O cluster pode levar mais tempo para convergir. Verifique com 'yby status'.")
		// NÃO retorna erro — o cluster pode convergir depois
	}

	// 5. Start Sync Loop (Async)
	go s.Mirror.StartSyncLoop(ctx)

	slog.Info("Ambiente pronto e sincronizado")
	return nil
}

func (s *EnvironmentService) runRemoteUp(ctx context.Context, opts UpOptions) error {
	slog.Info("Ambiente remoto detectado", "ambiente", opts.Environment)

	// 1. Verificar conectividade com o cluster
	slog.Info("Verificando conexão com o cluster remoto")
	output, err := s.Runner.RunCombinedOutput(ctx, "kubectl", "cluster-info")
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeClusterOffline,
			"falha ao conectar com cluster remoto").
			WithHint("Verifique se o kubeconfig está configurado corretamente com 'yby doctor'")
	}
	slog.Debug("Cluster info obtido", "output", strings.TrimSpace(string(output)))

	// 2. Verificar namespace existe (criar se necessário)
	ns := opts.Namespace
	if ns == "" {
		ns = "default"
	}
	slog.Info("Verificando namespace", "namespace", ns)
	_, err = s.Runner.RunCombinedOutput(ctx, "kubectl", "get", "namespace", ns)
	if err != nil {
		slog.Info("Namespace não encontrado, criando", "namespace", ns)
		if createErr := s.Runner.Run(ctx, "kubectl", "create", "namespace", ns); createErr != nil {
			return errors.Wrap(createErr, errors.ErrCodeExec,
				fmt.Sprintf("falha ao criar namespace '%s'", ns)).
				WithHint("Verifique se você tem permissão para criar namespaces no cluster")
		}
	}

	// 3. Verificar Argo CD está rodando
	slog.Info("Verificando Argo CD")
	argoOutput, err := s.Runner.RunCombinedOutput(ctx,
		"kubectl", "get", "pods", "-n", "argocd", "-l", "app.kubernetes.io/name=argocd-server", "--no-headers")
	if err != nil || strings.TrimSpace(string(argoOutput)) == "" {
		slog.Warn("Argo CD não detectado no cluster",
			"dica", "Execute 'yby bootstrap cluster' para instalar a stack GitOps")
	} else {
		slog.Info("Argo CD detectado no cluster")
	}

	slog.Info("Ambiente remoto verificado com sucesso", "ambiente", opts.Environment)
	return nil
}

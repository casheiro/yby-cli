package environment

import (
	"context"
	"fmt"

	"github.com/casheiro/yby-cli/pkg/services/bootstrap"
	"github.com/casheiro/yby-cli/pkg/services/shared"
)

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
		fmt.Printf("🚀 Cluster '%s' não encontrado. Criando...\n", opts.ClusterName)
		// Try to find config file
		configFile := "" // Logic for config path could be injected or resolved
		if err := s.Cluster.Create(ctx, opts.ClusterName, configFile); err != nil {
			return fmt.Errorf("falha ao criar cluster: %w", err)
		}
	} else {
		fmt.Printf("✅ Cluster '%s' já existe. Garantindo start...\n", opts.ClusterName)
		if err := s.Cluster.Start(ctx, opts.ClusterName); err != nil {
			return fmt.Errorf("falha ao iniciar cluster: %w", err)
		}
	}

	// 3. Mirror & Sync
	fmt.Println("🪞 Inicializando Git Mirror...")
	if err := s.Mirror.EnsureGitServer(); err != nil {
		return err
	}

	fmt.Println("🔌 Estabelecendo Túnel...")
	if err := s.Mirror.SetupTunnel(ctx); err != nil {
		return err
	}

	fmt.Println("🔄 Sincronizando código...")
	if err := s.Mirror.Sync(); err != nil {
		fmt.Printf("⚠️ Falha no sync inicial: %v\n", err)
	}

	// 4. Bootstrap
	fmt.Println("🛠️  Executando Bootstrap...")
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
	fmt.Println("⏳ Aguardando ArgoCD convergir...")
	if err := s.Bootstrap.WaitHealthy(ctx, "root-app", "argocd", 180); err != nil {
		fmt.Printf("⚠️  Timeout aguardando convergência: %v\n", err)
		fmt.Println("   O cluster pode levar mais tempo para convergir. Verifique com 'yby status'.")
		// NÃO retorna erro — o cluster pode convergir depois
	}

	// 5. Start Sync Loop (Async)
	go s.Mirror.StartSyncLoop(ctx)

	fmt.Println("\n✅ Ambiente pronto e sincronizado.")
	return nil
}

func (s *EnvironmentService) runRemoteUp(ctx context.Context, opts UpOptions) error {
	fmt.Printf("🌍 Ambiente Remoto Detectado: %s\n", opts.Environment)
	fmt.Println("ℹ️  Modo de Operação: Observação (Sync Local Desativado)")
	// In the future, this could trigger status checks via K8s client
	return nil
}

package environment

import (
	"context"
	"errors"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/bootstrap"
)

func TestEnvironmentService_Up_ClusterExistsAndStarts(t *testing.T) {
	cluster := &MockClusterManager{
		ExistsFunc: func(ctx context.Context, name string) (bool, error) {
			return true, nil // Cluster already exists
		},
		StartFunc: func(ctx context.Context, name string) error {
			return nil
		},
	}
	mirror := &MockMirrorService{}
	bs := &MockBootstrapService{}
	runner := &MockRunner{}

	svc := NewEnvironmentService(runner, nil, cluster, mirror, bs)
	err := svc.Up(context.Background(), UpOptions{
		Root:        "/tmp/infra",
		Environment: "local",
		ClusterName: "yby-existing",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestEnvironmentService_Up_ClusterStartError(t *testing.T) {
	cluster := &MockClusterManager{
		ExistsFunc: func(ctx context.Context, name string) (bool, error) {
			return true, nil
		},
		StartFunc: func(ctx context.Context, name string) error {
			return errors.New("failed to start cluster")
		},
	}
	runner := &MockRunner{}
	svc := NewEnvironmentService(runner, nil, cluster, nil, nil)
	err := svc.Up(context.Background(), UpOptions{Environment: "local", ClusterName: "yby-test"})
	if err == nil {
		t.Error("expected error when cluster start fails, got nil")
	}
}

func TestEnvironmentService_Up_ClusterCreateError(t *testing.T) {
	cluster := &MockClusterManager{
		ExistsFunc: func(ctx context.Context, name string) (bool, error) {
			return false, nil
		},
		CreateFunc: func(ctx context.Context, name string, configFile string) error {
			return errors.New("cluster creation failed")
		},
	}
	runner := &MockRunner{}
	svc := NewEnvironmentService(runner, nil, cluster, nil, nil)
	err := svc.Up(context.Background(), UpOptions{Environment: "local", ClusterName: "yby-test"})
	if err == nil {
		t.Error("expected error when cluster create fails, got nil")
	}
}

func TestEnvironmentService_Up_MirrorEnsureError(t *testing.T) {
	cluster := &MockClusterManager{
		ExistsFunc: func(ctx context.Context, name string) (bool, error) { return false, nil },
		CreateFunc: func(ctx context.Context, name string, configFile string) error { return nil },
	}
	mirror := &MockMirrorService{
		EnsureGitServerFunc: func() error {
			return errors.New("git server failed")
		},
	}
	runner := &MockRunner{}
	svc := NewEnvironmentService(runner, nil, cluster, mirror, nil)
	err := svc.Up(context.Background(), UpOptions{Environment: "local", ClusterName: "yby-test"})
	if err == nil {
		t.Error("expected error when EnsureGitServer fails, got nil")
	}
}

func TestEnvironmentService_Up_MirrorSetupTunnelError(t *testing.T) {
	cluster := &MockClusterManager{
		ExistsFunc: func(ctx context.Context, name string) (bool, error) { return false, nil },
		CreateFunc: func(ctx context.Context, name string, configFile string) error { return nil },
	}
	mirror := &MockMirrorService{
		EnsureGitServerFunc: func() error { return nil },
		SetupTunnelFunc: func(ctx context.Context) error {
			return errors.New("tunnel setup failed")
		},
	}
	runner := &MockRunner{}
	svc := NewEnvironmentService(runner, nil, cluster, mirror, nil)
	err := svc.Up(context.Background(), UpOptions{Environment: "local", ClusterName: "yby-test"})
	if err == nil {
		t.Error("expected error when SetupTunnel fails, got nil")
	}
}

func TestEnvironmentService_Up_BootstrapError(t *testing.T) {
	cluster := &MockClusterManager{
		ExistsFunc: func(ctx context.Context, name string) (bool, error) { return false, nil },
		CreateFunc: func(ctx context.Context, name string, configFile string) error { return nil },
	}
	mirror := &MockMirrorService{}
	bs := &MockBootstrapService{
		RunFunc: func(ctx context.Context, opts bootstrap.BootstrapOptions) error {
			return errors.New("bootstrap failed")
		},
	}
	runner := &MockRunner{}
	svc := NewEnvironmentService(runner, nil, cluster, mirror, bs)
	err := svc.Up(context.Background(), UpOptions{
		Root: "/tmp/infra", Environment: "local", ClusterName: "yby-test",
	})
	if err == nil {
		t.Error("expected error when Bootstrap fails, got nil")
	}
}

func TestEnvironmentService_Up_PlainSecrets_PropagarParaBootstrap(t *testing.T) {
	var receivedPlainSecrets bool
	cluster := &MockClusterManager{
		ExistsFunc: func(ctx context.Context, name string) (bool, error) { return true, nil },
		StartFunc:  func(ctx context.Context, name string) error { return nil },
	}
	mirror := &MockMirrorService{}
	bs := &MockBootstrapService{
		RunFunc: func(ctx context.Context, opts bootstrap.BootstrapOptions) error {
			receivedPlainSecrets = opts.PlainSecrets
			return nil
		},
	}
	runner := &MockRunner{}
	svc := NewEnvironmentService(runner, nil, cluster, mirror, bs)

	err := svc.Up(context.Background(), UpOptions{
		Root:         "/tmp/infra",
		Environment:  "local",
		ClusterName:  "yby-test",
		PlainSecrets: true,
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !receivedPlainSecrets {
		t.Error("PlainSecrets deveria ser propagado para BootstrapOptions")
	}
}

func TestEnvironmentService_Up_Remote_Sucesso(t *testing.T) {
	runner := &MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("Kubernetes control plane is running"), nil
		},
	}
	svc := NewEnvironmentService(runner, nil, nil, nil, nil)
	err := svc.Up(context.Background(), UpOptions{Environment: "staging", Namespace: "default"})
	if err != nil {
		t.Errorf("esperado sem erro para ambiente remoto com cluster acessível, obtido: %v", err)
	}
}

func TestEnvironmentService_Up_Remote_ClusterOffline(t *testing.T) {
	runner := &MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, errors.New("connection refused")
		},
	}
	svc := NewEnvironmentService(runner, nil, nil, nil, nil)
	err := svc.Up(context.Background(), UpOptions{Environment: "staging"})
	if err == nil {
		t.Error("esperado erro quando cluster remoto está offline, obtido nil")
	}
}

func TestEnvironmentService_Up_Remote_NamespaceNaoExiste_CriadoComSucesso(t *testing.T) {
	calls := []string{}
	runner := &MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "cluster-info" {
				return []byte("Kubernetes control plane is running"), nil
			}
			if len(args) > 1 && args[0] == "get" && args[1] == "namespace" {
				return nil, errors.New("not found")
			}
			// get pods argocd
			return []byte("argocd-server-abc123 1/1 Running"), nil
		},
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			calls = append(calls, name+" "+args[0])
			return nil
		},
	}
	svc := NewEnvironmentService(runner, nil, nil, nil, nil)
	err := svc.Up(context.Background(), UpOptions{Environment: "staging", Namespace: "my-app"})
	if err != nil {
		t.Errorf("esperado sem erro, obtido: %v", err)
	}
	found := false
	for _, c := range calls {
		if c == "kubectl create" {
			found = true
		}
	}
	if !found {
		t.Error("esperado que namespace fosse criado via kubectl create")
	}
}

func TestEnvironmentService_Up_Remote_NamespaceCriacaoFalha(t *testing.T) {
	runner := &MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "cluster-info" {
				return []byte("Kubernetes control plane is running"), nil
			}
			// namespace get falha
			return nil, errors.New("not found")
		},
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return errors.New("forbidden")
		},
	}
	svc := NewEnvironmentService(runner, nil, nil, nil, nil)
	err := svc.Up(context.Background(), UpOptions{Environment: "staging", Namespace: "restricted-ns"})
	if err == nil {
		t.Error("esperado erro quando criação de namespace falha, obtido nil")
	}
}

func TestEnvironmentService_Up_Remote_NamespacePadrao(t *testing.T) {
	runner := &MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
	}
	svc := NewEnvironmentService(runner, nil, nil, nil, nil)
	// Namespace vazio deve usar "default"
	err := svc.Up(context.Background(), UpOptions{Environment: "production"})
	if err != nil {
		t.Errorf("esperado sem erro, obtido: %v", err)
	}
}

func TestIsValidEnvironmentType_TiposValidos(t *testing.T) {
	validos := []string{"local", "remote", "eks", "aks", "gke"}
	for _, tipo := range validos {
		if !IsValidEnvironmentType(tipo) {
			t.Errorf("tipo '%s' deveria ser válido", tipo)
		}
	}
}

func TestIsValidEnvironmentType_TiposInvalidos(t *testing.T) {
	invalidos := []string{"", "aws", "azure", "gcp", "k3d", "unknown"}
	for _, tipo := range invalidos {
		if IsValidEnvironmentType(tipo) {
			t.Errorf("tipo '%s' não deveria ser válido", tipo)
		}
	}
}

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

func TestEnvironmentService_Up_Remote(t *testing.T) {
	runner := &MockRunner{}
	svc := NewEnvironmentService(runner, nil, nil, nil, nil)
	// Remote environment should not fail (just prints message)
	err := svc.Up(context.Background(), UpOptions{Environment: "staging"})
	if err != nil {
		t.Errorf("expected no error for remote environment, got: %v", err)
	}
}

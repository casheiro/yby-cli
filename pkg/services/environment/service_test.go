package environment

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/bootstrap"
)

// Mocks

type MockClusterManager struct {
	ExistsFunc func(ctx context.Context, name string) (bool, error)
	CreateFunc func(ctx context.Context, name string, configFile string) error
	StartFunc  func(ctx context.Context, name string) error
	DeleteFunc func(ctx context.Context, name string) error
}

func (m *MockClusterManager) Exists(ctx context.Context, name string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, name)
	}
	return true, nil
}

func (m *MockClusterManager) Create(ctx context.Context, name string, configFile string) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, name, configFile)
	}
	return nil
}

func (m *MockClusterManager) Start(ctx context.Context, name string) error {
	if m.StartFunc != nil {
		return m.StartFunc(ctx, name)
	}
	return nil
}

func (m *MockClusterManager) Delete(ctx context.Context, name string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, name)
	}
	return nil
}

type MockMirrorService struct {
	EnsureGitServerFunc func() error
	SetupTunnelFunc     func(ctx context.Context) error
	SyncFunc            func() error
}

func (m *MockMirrorService) EnsureGitServer() error {
	if m.EnsureGitServerFunc != nil {
		return m.EnsureGitServerFunc()
	}
	return nil
}

func (m *MockMirrorService) SetupTunnel(ctx context.Context) error {
	if m.SetupTunnelFunc != nil {
		return m.SetupTunnelFunc(ctx)
	}
	return nil
}

func (m *MockMirrorService) Sync() error {
	if m.SyncFunc != nil {
		return m.SyncFunc()
	}
	return nil
}

func (m *MockMirrorService) StartSyncLoop(ctx context.Context) {}

type MockBootstrapService struct {
	RunFunc func(ctx context.Context, opts bootstrap.BootstrapOptions) error
}

func (m *MockBootstrapService) Run(ctx context.Context, opts bootstrap.BootstrapOptions) error {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, opts)
	}
	return nil
}

type MockRunner struct {
	LookPathFunc func(file string) (string, error)
}

func (m *MockRunner) Run(ctx context.Context, name string, args ...string) error { return nil }
func (m *MockRunner) RunCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return nil, nil
}
func (m *MockRunner) RunStdin(ctx context.Context, stdin string, name string, args ...string) error {
	return nil
}
func (m *MockRunner) RunStdinOutput(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
	return nil, nil
}
func (m *MockRunner) LookPath(file string) (string, error) {
	if m.LookPathFunc != nil {
		return m.LookPathFunc(file)
	}
	return "/usr/bin/" + file, nil
}

// Tests

func TestEnvironmentService_Up_LocalSuccess(t *testing.T) {
	cluster := &MockClusterManager{
		ExistsFunc: func(ctx context.Context, name string) (bool, error) {
			return false, nil // Simulate cluster needs creation
		},
	}
	mirror := &MockMirrorService{}
	bs := &MockBootstrapService{}
	runner := &MockRunner{}

	svc := NewEnvironmentService(runner, nil, cluster, mirror, bs)

	opts := UpOptions{
		Root:        "/tmp/infra",
		Environment: "local",
		ClusterName: "yby-test",
	}

	err := svc.Up(context.Background(), opts)
	if err != nil {
		t.Fatalf("Expected nil error, got %v", err)
	}
}

func TestEnvironmentService_Up_LocalDependencyError(t *testing.T) {
	runner := &MockRunner{
		LookPathFunc: func(file string) (string, error) {
			if file == "k3d" {
				return "", fmt.Errorf("not found")
			}
			return "/usr/bin/" + file, nil
		},
	}
	svc := NewEnvironmentService(runner, nil, nil, nil, nil)

	err := svc.Up(context.Background(), UpOptions{Environment: "local"})
	if err == nil {
		t.Fatal("Expected error due to missing k3d, got nil")
	}
}

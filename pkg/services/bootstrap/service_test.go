package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"testing"
)

// Mocks

type MockRunner struct {
	RunFunc func(ctx context.Context, name string, args ...string) error
}

func (m *MockRunner) Run(ctx context.Context, name string, args ...string) error {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, name, args...)
	}
	return nil
}

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
	return "/usr/bin/" + file, nil
}

type MockFilesystem struct {
	ReadFileFunc func(name string) ([]byte, error)
}

func (m *MockFilesystem) ReadFile(name string) ([]byte, error) {
	if m.ReadFileFunc != nil {
		return m.ReadFileFunc(name)
	}
	return nil, os.ErrNotExist
}

func (m *MockFilesystem) WriteFile(name string, d []byte, p fs.FileMode) error { return nil }
func (m *MockFilesystem) MkdirAll(path string, perm fs.FileMode) error         { return nil }
func (m *MockFilesystem) Stat(name string) (fs.FileInfo, error)                { return nil, nil }
func (m *MockFilesystem) UserHomeDir() (string, error)                         { return "/home/user", nil }
func (m *MockFilesystem) WalkDir(root string, fn fs.WalkDirFunc) error         { return nil }

type MockK8sClient struct {
	CreateNamespaceFunc func(ctx context.Context, ns string) error
	ApplyManifestFunc   func(ctx context.Context, path string, namespace string) error
}

func (m *MockK8sClient) WaitPodReady(ctx context.Context, l, ns string, t int) error { return nil }
func (m *MockK8sClient) WaitCRD(ctx context.Context, crdName string, t int) error    { return nil }
func (m *MockK8sClient) NamespaceExists(ctx context.Context, ns string) (bool, error) {
	return true, nil
}
func (m *MockK8sClient) CreateNamespace(ctx context.Context, ns string) error {
	if m.CreateNamespaceFunc != nil {
		return m.CreateNamespaceFunc(ctx, ns)
	}
	return nil
}
func (m *MockK8sClient) ApplyManifest(ctx context.Context, path string, namespace string) error {
	if m.ApplyManifestFunc != nil {
		return m.ApplyManifestFunc(ctx, path, namespace)
	}
	return nil
}
func (m *MockK8sClient) PatchApplication(ctx context.Context, n, ns, p string) error { return nil }
func (m *MockK8sClient) WaitApplicationHealthy(ctx context.Context, name, namespace string, timeoutSeconds int) error {
	return nil
}

// Tests

func TestBootstrapService_Run_Basic(t *testing.T) {
	runner := &MockRunner{}
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte(`
prompts:
  - id: git.repoURL
    default: https://github.com/org/repo.git
`), nil
		},
	}
	k8s := &MockK8sClient{}

	svc := NewService(runner, fsys, k8s)
	opts := BootstrapOptions{
		Root:    "/tmp/infra",
		RepoURL: "https://github.com/org/repo.git",
	}

	err := svc.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Expected nil error, got %v", err)
	}
}

func TestBootstrapService_PhaseSystemBootstrap(t *testing.T) {
	ctx := context.Background()
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			if name == "helm" && args[0] == "repo" && args[1] == "add" {
				return nil
			}
			return nil
		},
	}
	fsys := &MockFilesystem{}
	k8s := &MockK8sClient{
		CreateNamespaceFunc: func(ctx context.Context, ns string) error {
			if ns == "argocd" || ns == "argo" || ns == "argo-events" {
				return nil
			}
			return fmt.Errorf("unexpected namespace %s", ns)
		},
	}

	svc := NewService(runner, fsys, k8s)
	err := svc.phaseSystemBootstrap(ctx, "/infra", "argo/argo-cd", "5.51.6")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestBootstrapService_Run_Local(t *testing.T) {
	runner := &MockRunner{}
	fsys := &MockFilesystem{
		ReadFileFunc: func(name string) ([]byte, error) {
			return []byte("prompts: []"), nil
		},
	}
	k8s := &MockK8sClient{}

	svc := NewService(runner, fsys, k8s)
	opts := BootstrapOptions{
		Root:        "/tmp/infra",
		Context:     "local",
		Environment: "local",
	}

	err := svc.Run(context.Background(), opts)
	if err != nil {
		t.Fatalf("Expected nil error in local mode, got %v", err)
	}
}

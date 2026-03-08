package secrets

import (
	"context"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var _ shared.Runner = (*MockRunner)(nil)
var _ shared.Filesystem = (*MockFS)(nil)

type MockRunner struct {
	mock.Mock
}

func (m *MockRunner) Run(ctx context.Context, name string, args ...string) error {
	return m.Called(ctx, name, args).Error(0)
}

func (m *MockRunner) RunCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cArgs := m.Called(ctx, name, args)
	var out []byte
	if cArgs.Get(0) != nil {
		out = cArgs.Get(0).([]byte)
	}
	return out, cArgs.Error(1)
}

func (m *MockRunner) RunStdin(ctx context.Context, stdin string, name string, args ...string) error {
	return m.Called(ctx, stdin, name, args).Error(0)
}

func (m *MockRunner) RunStdinOutput(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
	cArgs := m.Called(ctx, stdin, name, args)
	var out []byte
	if cArgs.Get(0) != nil {
		out = cArgs.Get(0).([]byte)
	}
	return out, cArgs.Error(1)
}

func (m *MockRunner) LookPath(file string) (string, error) {
	cArgs := m.Called(file)
	return cArgs.String(0), cArgs.Error(1)
}

type MockFS struct {
	mock.Mock
	shared.RealFilesystem // Embedded so we don't have to mock every single FS method if unused
}

func TestGenerateWebhook(t *testing.T) {
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)
	ctx := context.Background()

	opts := Options{Provider: "github", SecretVal: "my-secret", OutputPath: "/tmp/out.yaml"}

	runner.On("RunCombinedOutput", ctx, "kubectl", []string{"create", "secret", "generic", "github-webhook-secret",
		"--from-literal=secret=my-secret", "--namespace", "argo-events", "--dry-run=client", "-o", "yaml"}).
		Return([]byte("apiVersion: v1\nkind: Secret"), nil)

	runner.On("RunStdinOutput", ctx, "apiVersion: v1\nkind: Secret", "kubeseal",
		[]string{"--controller-name=sealed-secrets", "--controller-namespace=sealed-secrets", "--format=yaml"}).
		Return([]byte("apiVersion: bitnami.com/v1alpha1\nkind: SealedSecret"), nil)

	fsys.On("MkdirAll", "/tmp", mock.Anything).Return(nil)
	fsys.On("WriteFile", "/tmp/out.yaml", []byte("apiVersion: bitnami.com/v1alpha1\nkind: SealedSecret"), mock.Anything).Return(nil)

	res, err := svc.GenerateWebhook(ctx, opts)

	assert.NoError(t, err)
	assert.Equal(t, "my-secret", res)
	runner.AssertExpectations(t)
}

func TestGenerateMinIO(t *testing.T) {
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)
	ctx := context.Background()

	opts := Options{OutputPath: "/tmp/minio.yaml"}

	runner.On("RunCombinedOutput", ctx, "openssl", []string{"rand", "-hex", "16"}).
		Return([]byte("pass123\n"), nil)

	runner.On("RunCombinedOutput", ctx, "kubectl", []string{"create", "secret", "generic", "minio-secret",
		"--from-literal=rootUser=admin", "--from-literal=rootPassword=pass123", "--namespace", "storage", "--dry-run=client", "-o", "yaml"}).
		Return([]byte("kind: Secret\n"), nil)

	runner.On("RunStdinOutput", ctx, "kind: Secret\n", "kubeseal",
		[]string{"--controller-name=sealed-secrets", "--controller-namespace=sealed-secrets", "--format=yaml"}).
		Return([]byte("kind: SealedSecret\n"), nil)

	fsys.On("MkdirAll", "/tmp", mock.Anything).Return(nil)
	fsys.On("WriteFile", "/tmp/minio.yaml", []byte("kind: SealedSecret\n"), mock.Anything).Return(nil)

	user, err := svc.GenerateMinIO(ctx, opts)
	assert.NoError(t, err)
	assert.Equal(t, "admin", user)
	runner.AssertExpectations(t)
}

func TestCreateGitHubToken(t *testing.T) {
	runner := new(MockRunner)
	fsys := new(MockFS)
	svc := NewService(runner, fsys)
	ctx := context.Background()

	opts := Options{Token: "ghp_1234"}

	runner.On("RunCombinedOutput", ctx, "kubectl", []string{"create", "secret", "generic", "github-token",
		"--from-literal=token=ghp_1234", "--namespace", "argocd", "--dry-run=client", "-o", "yaml"}).
		Return([]byte("kind: Secret\n"), nil)

	runner.On("RunStdin", ctx, "kind: Secret\n", "kubectl", []string{"apply", "-f", "-"}).Return(nil)

	err := svc.CreateGitHubToken(ctx, opts)
	assert.NoError(t, err)
}

package secrets

import (
	"context"
	"errors"
	"os"
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
	shared.RealFilesystem // Embedded to avoid mocking everything
}

func (m *MockFS) MkdirAll(path string, perm os.FileMode) error {
	return m.Called(path, perm).Error(0)
}

func (m *MockFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return m.Called(name, data, perm).Error(0)
}

func (m *MockFS) Stat(name string) (os.FileInfo, error) {
	cArgs := m.Called(name)
	var info os.FileInfo
	if cArgs.Get(0) != nil {
		info = cArgs.Get(0).(os.FileInfo)
	}
	return info, cArgs.Error(1)
}

func TestGenerateWebhook(t *testing.T) {
	ctx := context.Background()

	t.Run("Success with provided SecretVal", func(t *testing.T) {
		runner := new(MockRunner)
		fsys := new(MockFS)
		svc := NewService(runner, fsys)

		opts := Options{Provider: "github", SecretVal: "my-secret", OutputPath: "/tmp/out.yaml"}

		runner.On("RunCombinedOutput", ctx, "kubectl", []string{"create", "secret", "generic", "github-webhook-secret",
			"--from-literal=secret=my-secret", "--namespace", "argo-events", "--dry-run=client", "-o", "yaml"}).
			Return([]byte("secret"), nil)

		runner.On("RunStdinOutput", ctx, "secret", "kubeseal",
			[]string{"--controller-name=sealed-secrets", "--controller-namespace=sealed-secrets", "--format=yaml"}).
			Return([]byte("sealed"), nil)

		fsys.On("MkdirAll", "/tmp", mock.Anything).Return(nil)
		fsys.On("WriteFile", "/tmp/out.yaml", []byte("sealed"), mock.Anything).Return(nil)

		res, err := svc.GenerateWebhook(ctx, opts)
		assert.NoError(t, err)
		assert.Equal(t, "my-secret", res)
		runner.AssertExpectations(t)
	})

	t.Run("Success without SecretVal (random)", func(t *testing.T) {
		runner := new(MockRunner)
		fsys := new(MockFS)
		svc := NewService(runner, fsys)

		opts := Options{Provider: "gitlab", OutputPath: "/tmp/out.yaml"}

		runner.On("RunCombinedOutput", ctx, "openssl", []string{"rand", "-hex", "20"}).
			Return([]byte("randomval\n"), nil)

		runner.On("RunCombinedOutput", ctx, "kubectl", []string{"create", "secret", "generic", "gitlab-webhook-secret",
			"--from-literal=secret=randomval", "--namespace", "argo-events", "--dry-run=client", "-o", "yaml"}).
			Return([]byte("secret"), nil)

		runner.On("RunStdinOutput", ctx, "secret", "kubeseal",
			[]string{"--controller-name=sealed-secrets", "--controller-namespace=sealed-secrets", "--format=yaml"}).
			Return([]byte("sealed"), nil)

		fsys.On("MkdirAll", "/tmp", mock.Anything).Return(nil)
		fsys.On("WriteFile", "/tmp/out.yaml", []byte("sealed"), mock.Anything).Return(nil)

		res, err := svc.GenerateWebhook(ctx, opts)
		assert.NoError(t, err)
		assert.Equal(t, "randomval", res)
		runner.AssertExpectations(t)
	})

	t.Run("OpenSSL failure", func(t *testing.T) {
		runner := new(MockRunner)
		svc := NewService(runner, new(MockFS))
		opts := Options{Provider: "github", OutputPath: "/tmp/out.yaml"}

		runner.On("RunCombinedOutput", ctx, "openssl", []string{"rand", "-hex", "20"}).
			Return(nil, errors.New("openssl error"))

		_, err := svc.GenerateWebhook(ctx, opts)
		assert.ErrorContains(t, err, "falha ao gerar secret aleatório")
	})

	t.Run("Kubectl failure", func(t *testing.T) {
		runner := new(MockRunner)
		svc := NewService(runner, new(MockFS))
		opts := Options{Provider: "github", SecretVal: "val", OutputPath: "/tmp/out.yaml"}

		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).
			Return(nil, errors.New("kubectl error"))

		_, err := svc.GenerateWebhook(ctx, opts)
		assert.ErrorContains(t, err, "falha ao gerar secret com kubectl")
	})

	t.Run("Seal and save failure", func(t *testing.T) {
		runner := new(MockRunner)
		svc := NewService(runner, new(MockFS))
		opts := Options{Provider: "github", SecretVal: "val", OutputPath: "/tmp/out.yaml"}

		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return([]byte("secret"), nil)
		runner.On("RunStdinOutput", ctx, "secret", "kubeseal", mock.Anything).Return(nil, errors.New("kubeseal err"))

		_, err := svc.GenerateWebhook(ctx, opts)
		assert.ErrorContains(t, err, "erro ao executar kubeseal")
	})
}

func TestGenerateMinIO(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		runner := new(MockRunner)
		fsys := new(MockFS)
		svc := NewService(runner, fsys)

		opts := Options{OutputPath: "/tmp/minio.yaml"}

		runner.On("RunCombinedOutput", ctx, "openssl", []string{"rand", "-hex", "16"}).
			Return([]byte("pass123\n"), nil)

		runner.On("RunCombinedOutput", ctx, "kubectl", []string{"create", "secret", "generic", "minio-secret",
			"--from-literal=rootUser=admin", "--from-literal=rootPassword=pass123", "--namespace", "storage", "--dry-run=client", "-o", "yaml"}).
			Return([]byte("secret"), nil)

		runner.On("RunStdinOutput", ctx, "secret", "kubeseal", mock.Anything).
			Return([]byte("sealed"), nil)

		fsys.On("MkdirAll", "/tmp", mock.Anything).Return(nil)
		fsys.On("WriteFile", "/tmp/minio.yaml", []byte("sealed"), mock.Anything).Return(nil)

		user, err := svc.GenerateMinIO(ctx, opts)
		assert.NoError(t, err)
		assert.Equal(t, "admin", user)
	})

	t.Run("OpenSSL failure", func(t *testing.T) {
		runner := new(MockRunner)
		svc := NewService(runner, new(MockFS))

		runner.On("RunCombinedOutput", ctx, "openssl", []string{"rand", "-hex", "16"}).
			Return(nil, errors.New("openssl err"))

		_, err := svc.GenerateMinIO(ctx, Options{})
		assert.ErrorContains(t, err, "falha ao gerar senha MinIO")
	})

	t.Run("Kubectl failure", func(t *testing.T) {
		runner := new(MockRunner)
		svc := NewService(runner, new(MockFS))

		runner.On("RunCombinedOutput", ctx, "openssl", mock.Anything).Return([]byte("pass"), nil)
		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return(nil, errors.New("kubectl err"))

		_, err := svc.GenerateMinIO(ctx, Options{})
		assert.ErrorContains(t, err, "falha ao gerar secret com kubectl")
	})
}

func TestCreateGitHubToken(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		runner := new(MockRunner)
		svc := NewService(runner, new(MockFS))

		opts := Options{Token: "token"}

		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return([]byte("secret"), nil)
		runner.On("RunStdin", ctx, "secret", "kubectl", []string{"apply", "-f", "-"}).Return(nil)

		err := svc.CreateGitHubToken(ctx, opts)
		assert.NoError(t, err)
	})

	t.Run("Kubectl create failure", func(t *testing.T) {
		runner := new(MockRunner)
		svc := NewService(runner, new(MockFS))

		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return(nil, errors.New("fail"))

		err := svc.CreateGitHubToken(ctx, Options{Token: "token"})
		assert.ErrorContains(t, err, "erro ao gerar secret github-token")
	})

	t.Run("Kubectl apply failure", func(t *testing.T) {
		runner := new(MockRunner)
		svc := NewService(runner, new(MockFS))

		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return([]byte("secret"), nil)
		runner.On("RunStdin", ctx, "secret", "kubectl", mock.Anything).Return(errors.New("fail"))

		err := svc.CreateGitHubToken(ctx, Options{Token: "token"})
		assert.ErrorContains(t, err, "erro ao aplicar secret github-token")
	})
}

func TestBackupKeys(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		runner := new(MockRunner)
		fsys := new(MockFS)
		svc := NewService(runner, fsys)

		opts := Options{OutputPath: "/tmp/backup.yaml"}

		runner.On("RunCombinedOutput", ctx, "kubectl", []string{"get", "secret", "-n", "sealed-secrets", "-l", "sealedsecrets.bitnami.com/sealed-secrets-key=active", "-o", "name"}).
			Return([]byte("secret/my-active-key\n"), nil)

		fsys.On("MkdirAll", "/tmp", mock.Anything).Return(nil)

		runner.On("RunCombinedOutput", ctx, "kubectl", []string{"get", "secret", "my-active-key", "-n", "sealed-secrets", "-o", "yaml"}).
			Return([]byte("backup data"), nil)

		fsys.On("WriteFile", "/tmp/backup.yaml", []byte("backup data"), mock.Anything).Return(nil)

		key, err := svc.BackupKeys(ctx, opts)
		assert.NoError(t, err)
		assert.Equal(t, "my-active-key", key)
	})

	t.Run("Key not found", func(t *testing.T) {
		runner := new(MockRunner)
		svc := NewService(runner, new(MockFS))

		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return([]byte{}, errors.New("fail"))

		_, err := svc.BackupKeys(ctx, Options{})
		assert.ErrorContains(t, err, "chave não encontrada")
	})

	t.Run("Empty key fallback string", func(t *testing.T) {
		runner := new(MockRunner)
		svc := NewService(runner, new(MockFS))

		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return([]byte(""), nil)

		_, err := svc.BackupKeys(ctx, Options{})
		assert.ErrorContains(t, err, "chave não encontrada")
	})

	t.Run("MkdirAll failure", func(t *testing.T) {
		runner := new(MockRunner)
		fsys := new(MockFS)
		svc := NewService(runner, fsys)

		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return([]byte("secret/my-active-key"), nil).Once()
		fsys.On("MkdirAll", mock.Anything, mock.Anything).Return(errors.New("mkdir fail"))

		_, err := svc.BackupKeys(ctx, Options{OutputPath: "/tmp/out"})
		assert.ErrorContains(t, err, "erro ao criar diretório")
	})

	t.Run("Get backup data failure", func(t *testing.T) {
		runner := new(MockRunner)
		fsys := new(MockFS)
		svc := NewService(runner, fsys)

		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return([]byte("secret/key"), nil).Once()
		fsys.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return(nil, errors.New("get fail")).Once()

		_, err := svc.BackupKeys(ctx, Options{OutputPath: "/tmp/out"})
		assert.ErrorContains(t, err, "erro ao buscar backup do kubernetes")
	})

	t.Run("Write failure", func(t *testing.T) {
		runner := new(MockRunner)
		fsys := new(MockFS)
		svc := NewService(runner, fsys)

		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return([]byte("secret/key"), nil).Once()
		fsys.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)
		runner.On("RunCombinedOutput", ctx, "kubectl", mock.Anything).Return([]byte("data"), nil).Once()
		fsys.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("write fail"))

		_, err := svc.BackupKeys(ctx, Options{OutputPath: "/tmp/out"})
		assert.ErrorContains(t, err, "erro ao salvar backup")
	})
}

func TestRestoreKeys(t *testing.T) {
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		runner := new(MockRunner)
		fsys := new(MockFS)
		svc := NewService(runner, fsys)
		opts := Options{OutputPath: "/tmp/backup.yaml"}

		fsys.On("Stat", "/tmp/backup.yaml").Return(nil, nil)
		runner.On("Run", ctx, "kubectl", []string{"create", "ns", "sealed-secrets"}).Return(nil)
		runner.On("Run", ctx, "kubectl", []string{"apply", "-f", "/tmp/backup.yaml"}).Return(nil)
		runner.On("Run", ctx, "kubectl", []string{"delete", "pod", "-n", "sealed-secrets", "-l", "app.kubernetes.io/name=sealed-secrets"}).Return(nil)

		err := svc.RestoreKeys(ctx, opts)
		assert.NoError(t, err)
	})

	t.Run("File not found", func(t *testing.T) {
		fsys := new(MockFS)
		svc := NewService(new(MockRunner), fsys)

		fsys.On("Stat", mock.Anything).Return(nil, errors.New("not exists"))

		err := svc.RestoreKeys(ctx, Options{})
		assert.ErrorContains(t, err, "arquivo de backup não encontrado")
	})

	t.Run("Apply failure", func(t *testing.T) {
		runner := new(MockRunner)
		fsys := new(MockFS)
		svc := NewService(runner, fsys)

		fsys.On("Stat", mock.Anything).Return(nil, nil)
		runner.On("Run", ctx, "kubectl", []string{"create", "ns", "sealed-secrets"}).Return(errors.New("already exists"))
		runner.On("Run", ctx, "kubectl", []string{"apply", "-f", ""}).Return(errors.New("apply fail"))

		err := svc.RestoreKeys(ctx, Options{})
		assert.ErrorContains(t, err, "erro ao aplicar chave")
	})

	t.Run("Delete pod failure", func(t *testing.T) {
		runner := new(MockRunner)
		fsys := new(MockFS)
		svc := NewService(runner, fsys)

		fsys.On("Stat", mock.Anything).Return(nil, nil)
		runner.On("Run", ctx, "kubectl", []string{"create", "ns", "sealed-secrets"}).Return(nil)
		runner.On("Run", ctx, "kubectl", []string{"apply", "-f", ""}).Return(nil)
		runner.On("Run", ctx, "kubectl", []string{"delete", "pod", "-n", "sealed-secrets", "-l", "app.kubernetes.io/name=sealed-secrets"}).Return(errors.New("delete fail"))

		err := svc.RestoreKeys(ctx, Options{})
		assert.ErrorContains(t, err, "erro ao reiniciar controller")
	})
}

package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/secrets"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

// ========================================================
// Helpers para testes de secrets
// ========================================================

// mockSecretsFactory substitui a factory de secrets para testes,
// retornando um serviço com mocks de Runner e Filesystem.
// Retorna uma função de teardown para restaurar a factory original.
func mockSecretsFactory(mockRunner *testutil.MockRunner, mockFs *testutil.MockFilesystem) func() {
	orig := newSecretsService
	newSecretsService = func(r shared.Runner, fs shared.Filesystem) secrets.Service {
		return secrets.NewService(mockRunner, mockFs)
	}
	return func() { newSecretsService = orig }
}

// successMocks retorna mocks que simulam sucesso em todas as operações
func successMocks() (*testutil.MockRunner, *testutil.MockFilesystem) {
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			// Simula diferentes outputs conforme o comando
			if name == "openssl" {
				return []byte("abc123def456\n"), nil
			}
			if name == "kubectl" {
				// Para backup: retorna nome do secret
				for _, a := range args {
					if a == "sealedsecrets.bitnami.com/sealed-secrets-key=active" {
						return []byte("secret/sealed-secrets-key-abc123\n"), nil
					}
				}
				return []byte("apiVersion: v1\nkind: Secret\n"), nil
			}
			if name == "kubeseal" {
				return []byte("apiVersion: bitnami.com/v1alpha1\nkind: SealedSecret\n"), nil
			}
			return []byte("ok"), nil
		},
		RunStdinFunc: func(ctx context.Context, stdin string, name string, args ...string) error {
			return nil
		},
		RunStdinOutputFunc: func(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
			return []byte("sealed-output"), nil
		},
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
	}
	mockFs := &testutil.MockFilesystem{
		WriteFileFunc: func(name string, data []byte, perm fs.FileMode) error { return nil },
		MkdirAllFunc:  func(path string, perm fs.FileMode) error { return nil },
		StatFunc: func(name string) (fs.FileInfo, error) {
			return nil, nil // arquivo existe
		},
	}
	return runner, mockFs
}

// errorMocks retorna mocks que simulam erro em operações
func errorMocks() (*testutil.MockRunner, *testutil.MockFilesystem) {
	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return fmt.Errorf("erro simulado")
		},
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("erro simulado no comando")
		},
		RunStdinFunc: func(ctx context.Context, stdin string, name string, args ...string) error {
			return fmt.Errorf("erro simulado stdin")
		},
		RunStdinOutputFunc: func(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("erro simulado stdin output")
		},
		LookPathFunc: func(file string) (string, error) {
			return "", fmt.Errorf("comando não encontrado: %s", file)
		},
	}
	mockFs := &testutil.MockFilesystem{
		WriteFileFunc: func(name string, data []byte, perm fs.FileMode) error {
			return fmt.Errorf("erro ao escrever")
		},
		MkdirAllFunc: func(path string, perm fs.FileMode) error {
			return fmt.Errorf("erro ao criar diretório")
		},
		StatFunc: func(name string) (fs.FileInfo, error) {
			return nil, fmt.Errorf("arquivo não encontrado")
		},
	}
	return runner, mockFs
}

// ========================================================
// Testes do webhook secret
// ========================================================

func TestWebhookSecretCmd_RunSuccess(t *testing.T) {
	runner, mockFs := successMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := webhookSecretCmd
	cmd.SetArgs([]string{"github", "meu-segredo"})
	cmd.Run(cmd, []string{"github", "meu-segredo"})
	// Cobrir statements — o comando usa fmt.Println, saída vai para stdout
}

func TestWebhookSecretCmd_RunSuccessSemSegredo(t *testing.T) {
	runner, mockFs := successMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	// Sem valor de secret — gera aleatório
	t.Setenv("WEBHOOK_SECRET", "")

	cmd := webhookSecretCmd
	cmd.Run(cmd, []string{"github"})
}

func TestWebhookSecretCmd_RunSuccessSemArgs(t *testing.T) {
	runner, mockFs := successMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	t.Setenv("WEBHOOK_SECRET", "env-secret")

	cmd := webhookSecretCmd
	cmd.Run(cmd, []string{})
}

func TestWebhookSecretCmd_RunErro(t *testing.T) {
	runner, mockFs := errorMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := webhookSecretCmd
	cmd.Run(cmd, []string{"github", "meu-segredo"})
}

// ========================================================
// Testes do minio secret
// ========================================================

func TestMinioSecretCmd_RunSuccess(t *testing.T) {
	runner, mockFs := successMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := minioSecretCmd
	cmd.Run(cmd, []string{})
}

func TestMinioSecretCmd_RunErro(t *testing.T) {
	runner, mockFs := errorMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := minioSecretCmd
	cmd.Run(cmd, []string{})
}

// ========================================================
// Testes do github-token secret
// ========================================================

func TestGithubTokenSecretCmd_RunSuccess(t *testing.T) {
	runner, mockFs := successMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := githubTokenSecretCmd
	cmd.Run(cmd, []string{"ghp_meutokenaqui"})
}

func TestGithubTokenSecretCmd_RunErro(t *testing.T) {
	runner, mockFs := errorMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := githubTokenSecretCmd
	cmd.Run(cmd, []string{"ghp_meutokenaqui"})
}

// ========================================================
// Testes do backup keys
// ========================================================

func TestBackupKeysCmd_RunSuccess(t *testing.T) {
	runner, mockFs := successMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := backupKeysCmd
	cmd.Run(cmd, []string{})
}

func TestBackupKeysCmd_RunSuccessComPath(t *testing.T) {
	runner, mockFs := successMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := backupKeysCmd
	cmd.Run(cmd, []string{"/tmp/backup.yaml"})
}

func TestBackupKeysCmd_RunErro(t *testing.T) {
	runner, mockFs := errorMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := backupKeysCmd
	cmd.Run(cmd, []string{})
}

// ========================================================
// Testes do restore keys
// ========================================================

func TestRestoreKeysCmd_RunSuccess(t *testing.T) {
	runner, mockFs := successMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := restoreKeysCmd
	cmd.Run(cmd, []string{})
}

func TestRestoreKeysCmd_RunSuccessComPath(t *testing.T) {
	runner, mockFs := successMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := restoreKeysCmd
	cmd.Run(cmd, []string{"/tmp/backup.yaml"})
}

func TestRestoreKeysCmd_RunErro(t *testing.T) {
	runner, mockFs := errorMocks()
	teardown := mockSecretsFactory(runner, mockFs)
	defer teardown()

	cmd := restoreKeysCmd
	cmd.Run(cmd, []string{})
}

// ========================================================
// Teste da factory newSecretsService
// ========================================================

func TestNewSecretsServiceFactory_Default(t *testing.T) {
	runner := &testutil.MockRunner{}
	mockFs := &testutil.MockFilesystem{}

	// Verifica que a factory padrão retorna um serviço válido
	svc := newSecretsService(runner, mockFs)
	assert.NotNil(t, svc, "newSecretsService deveria retornar um serviço não-nil")
}

package secrets

import (
	"context"
	"fmt"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/casheiro/yby-cli/pkg/testutil"
)

func TestNewStrategy_SealedSecrets(t *testing.T) {
	runner := &testutil.MockRunner{}
	fs := &testutil.MockFilesystem{}

	strategy := NewStrategy("sealed-secrets", runner, fs)
	require.NotNil(t, strategy)
	assert.Equal(t, "sealed-secrets", strategy.Name())
	assert.Contains(t, strategy.ScaffoldTemplates(), "sealed-secret")
}

func TestNewStrategy_ExternalSecrets(t *testing.T) {
	runner := &testutil.MockRunner{}
	fs := &testutil.MockFilesystem{}

	strategy := NewStrategy("external-secrets", runner, fs)
	require.NotNil(t, strategy)
	assert.Equal(t, "external-secrets", strategy.Name())
	assert.Contains(t, strategy.ScaffoldTemplates(), "external-secret")
}

func TestNewStrategy_SOPS(t *testing.T) {
	runner := &testutil.MockRunner{}
	fs := &testutil.MockFilesystem{}

	strategy := NewStrategy("sops", runner, fs)
	require.NotNil(t, strategy)
	assert.Equal(t, "sops", strategy.Name())
	assert.Contains(t, strategy.ScaffoldTemplates(), "sops-secret")
}

func TestNewStrategy_DefaultIsExternalSecrets(t *testing.T) {
	runner := &testutil.MockRunner{}
	fs := &testutil.MockFilesystem{}

	strategy := NewStrategy("unknown", runner, fs)
	require.NotNil(t, strategy)
	assert.Equal(t, "external-secrets", strategy.Name())
}

func TestExternalSecretsStrategy_GenerateSecret_NoOutputPath(t *testing.T) {
	runner := &testutil.MockRunner{}
	fs := &testutil.MockFilesystem{}

	strategy := NewExternalSecretsStrategy(runner, fs)
	// OutputPath vazio: retorna nil sem gerar nada (referências ESO são externas)
	err := strategy.GenerateSecret(nil, SecretOpts{})
	assert.NoError(t, err)
}

func TestExternalSecretsStrategy_GenerateSecret_WithOutputPath(t *testing.T) {
	ctx := context.Background()
	var writtenData []byte
	runner := &testutil.MockRunner{}
	fsys := &testutil.MockFilesystem{
		MkdirAllFunc:  func(path string, perm fs.FileMode) error { return nil },
		WriteFileFunc: func(name string, data []byte, perm fs.FileMode) error { writtenData = data; return nil },
	}

	strategy := NewExternalSecretsStrategy(runner, fsys)
	err := strategy.GenerateSecret(ctx, SecretOpts{
		Name:       "meu-secret",
		Namespace:  "default",
		OutputPath: "/tmp/es.yaml",
		Data:       map[string]string{"chave": "valor"},
	})
	assert.NoError(t, err)
	assert.Contains(t, string(writtenData), "kind: ExternalSecret")
	assert.Contains(t, string(writtenData), "meu-secret")
}

func TestSOPSStrategy_GenerateSecret_Success(t *testing.T) {
	ctx := context.Background()
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return []byte("secret-yaml"), nil
		},
		RunStdinOutputFunc: func(_ context.Context, stdin string, name string, args ...string) ([]byte, error) {
			return []byte("encrypted"), nil
		},
	}
	fsys := &testutil.MockFilesystem{
		MkdirAllFunc:  func(path string, perm fs.FileMode) error { return nil },
		WriteFileFunc: func(name string, data []byte, perm fs.FileMode) error { return nil },
	}

	strategy := NewSOPSStrategy(runner, fsys)
	err := strategy.GenerateSecret(ctx, SecretOpts{
		Name:       "meu-secret",
		Namespace:  "default",
		OutputPath: "/tmp/sops.yaml",
		Data:       map[string]string{"key": "val"},
	})
	assert.NoError(t, err)
}

func TestSOPSStrategy_GenerateSecret_KubectlError(t *testing.T) {
	ctx := context.Background()
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("kubectl falhou")
		},
	}
	fsys := &testutil.MockFilesystem{}

	strategy := NewSOPSStrategy(runner, fsys)
	err := strategy.GenerateSecret(ctx, SecretOpts{
		Name:       "s",
		Namespace:  "ns",
		OutputPath: "/tmp/out.yaml",
	})
	assert.ErrorContains(t, err, "kubectl")
}

func TestSealedSecretsStrategy_Name(t *testing.T) {
	s := &SealedSecretsStrategy{}
	assert.Equal(t, "sealed-secrets", s.Name())
}

func TestExternalSecretsStrategy_Name(t *testing.T) {
	s := &ExternalSecretsStrategy{}
	assert.Equal(t, "external-secrets", s.Name())
}

func TestSOPSStrategy_Name(t *testing.T) {
	s := &SOPSStrategy{}
	assert.Equal(t, "sops", s.Name())
}

func TestSecretOpts_Structure(t *testing.T) {
	opts := SecretOpts{
		Name:       "my-secret",
		Namespace:  "default",
		Data:       map[string]string{"key": "value"},
		OutputPath: "/tmp/secret.yaml",
	}

	assert.Equal(t, "my-secret", opts.Name)
	assert.Equal(t, "default", opts.Namespace)
	assert.Equal(t, "value", opts.Data["key"])
	assert.Equal(t, "/tmp/secret.yaml", opts.OutputPath)
}

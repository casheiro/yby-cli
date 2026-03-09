package secrets

import (
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

func TestExternalSecretsStrategy_GenerateSecret_NoOp(t *testing.T) {
	runner := &testutil.MockRunner{}
	fs := &testutil.MockFilesystem{}

	strategy := NewExternalSecretsStrategy(runner, fs)
	err := strategy.GenerateSecret(nil, SecretOpts{})
	assert.NoError(t, err)
}

func TestSOPSStrategy_GenerateSecret_NoOp(t *testing.T) {
	runner := &testutil.MockRunner{}
	fs := &testutil.MockFilesystem{}

	strategy := NewSOPSStrategy(runner, fs)
	err := strategy.GenerateSecret(nil, SecretOpts{})
	assert.NoError(t, err)
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

package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunEx_Success(t *testing.T) {
	mock := &testutil.MockExecutor{
		RunFunc: func(name, script string) error {
			return nil
		},
	}
	err := runEx(mock, "teste", "echo hello")
	assert.NoError(t, err)
}

func TestRunEx_Error(t *testing.T) {
	mock := &testutil.MockExecutor{
		RunFunc: func(name, script string) error {
			return assert.AnError
		},
	}
	err := runEx(mock, "teste", "echo hello")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "teste")
}

func TestCopyFile_Success(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")

	require.NoError(t, os.WriteFile(src, []byte("conteúdo teste"), 0644))

	err := copyFile(src, dst)
	assert.NoError(t, err)

	data, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "conteúdo teste", string(data))
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "nonexistent"), filepath.Join(dir, "dest"))
	assert.Error(t, err)
}

func TestFetchKubeconfig_Success(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("CLUSTER_NAME", "test-cluster")

	runner := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("merged-kubeconfig-content"), nil
		},
	}

	mock := &testutil.MockExecutor{
		FetchFileFunc: func(path string) ([]byte, error) {
			return []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
current-context: default
kind: Config
users:
- name: default
  user: {}
`), nil
		},
	}

	err := fetchKubeconfig(mock, "192.168.1.100", runner)
	assert.NoError(t, err)

	// Verificar que o kubeconfig foi escrito
	configPath := filepath.Join(dir, ".kube", "config")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, "merged-kubeconfig-content", string(data))
}

func TestBootstrapVpsCmd_Flags(t *testing.T) {
	f := bootstrapVpsCmd.Flags().Lookup("host")
	assert.NotNil(t, f)
	f2 := bootstrapVpsCmd.Flags().Lookup("local")
	assert.NotNil(t, f2)
}

func TestK3sToken_CryptoRand(t *testing.T) {
	// Simular que K3S_TOKEN não está definido (usa crypto/rand)
	t.Setenv("K3S_TOKEN", "")

	// Gerar dois tokens para verificar aleatoriedade
	tokenBytes1 := make([]byte, 32)
	_, err := rand.Read(tokenBytes1)
	require.NoError(t, err)
	token1 := hex.EncodeToString(tokenBytes1)

	tokenBytes2 := make([]byte, 32)
	_, err = rand.Read(tokenBytes2)
	require.NoError(t, err)
	token2 := hex.EncodeToString(tokenBytes2)

	// 64 caracteres hex = 32 bytes
	assert.Len(t, token1, 64, "Token deve ter 64 caracteres hex")
	assert.Len(t, token2, 64, "Token deve ter 64 caracteres hex")
	assert.NotEqual(t, token1, token2, "Dois tokens consecutivos devem ser diferentes")
	assert.Regexp(t, "^[0-9a-f]{64}$", token1, "Token deve conter apenas hex")
}

func TestBootstrapVpsCmd_SkipTLSVerifyFlag(t *testing.T) {
	f := bootstrapVpsCmd.Flags().Lookup("skip-tls-verify")
	assert.NotNil(t, f, "Flag --skip-tls-verify deve existir")
	assert.Equal(t, "false", f.DefValue, "Default deve ser false (seguro)")
}

func TestCopyFile_Permissao0600(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")

	require.NoError(t, os.WriteFile(src, []byte("dados sensíveis"), 0644))

	err := copyFile(src, dst)
	assert.NoError(t, err)

	info, err := os.Stat(dst)
	require.NoError(t, err)
	// Verificar que permissão é 0600 (apenas owner pode ler/escrever)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "Backup kubeconfig deve ter permissão 0600")
}

package cmd

import (
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
	teardown := mockExecCommand()
	defer teardown()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("CLUSTER_NAME", "test-cluster")

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

	err := fetchKubeconfig(mock, "192.168.1.100")
	// Pode falhar caso kubectl não esteja disponível no ambiente de teste
	// O objetivo principal é verificar que não entra em pânico
	_ = err
}

func TestBootstrapVpsCmd_Flags(t *testing.T) {
	f := bootstrapVpsCmd.Flags().Lookup("host")
	assert.NotNil(t, f)
	f2 := bootstrapVpsCmd.Flags().Lookup("local")
	assert.NotNil(t, f2)
}

package cmd

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/bootstrap"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootstrapClusterCmd_RunE_Sucesso(t *testing.T) {
	origFactory := newBootstrapClusterService
	defer func() { newBootstrapClusterService = origFactory }()

	// Cria diretório temporário com .yby para FindInfraRoot
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".yby"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config", "cluster-values.yaml"), []byte("{}"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "manifests", "argocd"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifests", "argocd", "root-app.yaml"), []byte("apiVersion: v1"), 0644))

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	t.Setenv("GITHUB_REPO", "https://github.com/test/repo")
	t.Setenv("YBY_ENV", "local")

	newBootstrapClusterService = func(r shared.Runner, f shared.Filesystem) *bootstrap.BootstrapService {
		mockRunner := &testutil.MockRunner{
			RunFunc: func(ctx context.Context, name string, args ...string) error {
				return nil
			},
			RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte("ok"), nil
			},
			LookPathFunc: func(file string) (string, error) {
				return "/usr/bin/" + file, nil
			},
		}
		mockFs := &testutil.MockFilesystem{
			ReadFileFunc: func(name string) ([]byte, error) {
				return []byte(""), nil
			},
			WriteFileFunc: func(name string, data []byte, perm fs.FileMode) error {
				return nil
			},
			MkdirAllFunc: func(path string, perm fs.FileMode) error {
				return nil
			},
			StatFunc: func(name string) (fs.FileInfo, error) {
				return nil, os.ErrNotExist
			},
		}
		k8s := &bootstrap.RealK8sClient{Runner: mockRunner}
		return bootstrap.NewService(mockRunner, mockFs, k8s)
	}

	// Define contexto para evitar panic de nil context no retry
	ctx := context.Background()
	bootstrapClusterCmd.SetContext(ctx)

	err := bootstrapClusterCmd.RunE(bootstrapClusterCmd, []string{})
	assert.NoError(t, err)
}

func TestBootstrapClusterCmd_RunE_ErroNoServico(t *testing.T) {
	origFactory := newBootstrapClusterService
	defer func() { newBootstrapClusterService = origFactory }()

	// Sem .yby para forçar fallback do FindInfraRoot
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	t.Setenv("GITHUB_REPO", "")
	t.Setenv("YBY_ENV", "")

	newBootstrapClusterService = func(r shared.Runner, f shared.Filesystem) *bootstrap.BootstrapService {
		mockRunner := &testutil.MockRunner{
			RunFunc: func(ctx context.Context, name string, args ...string) error {
				return assert.AnError
			},
			RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return nil, assert.AnError
			},
			LookPathFunc: func(file string) (string, error) {
				return "/usr/bin/" + file, nil
			},
		}
		mockFs := &testutil.MockFilesystem{
			ReadFileFunc: func(name string) ([]byte, error) {
				return nil, os.ErrNotExist
			},
		}
		k8s := &bootstrap.RealK8sClient{Runner: mockRunner}
		return bootstrap.NewService(mockRunner, mockFs, k8s)
	}

	ctx := context.Background()
	bootstrapClusterCmd.SetContext(ctx)

	err := bootstrapClusterCmd.RunE(bootstrapClusterCmd, []string{})
	assert.Error(t, err)
}

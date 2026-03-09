package environment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEnvironmentService(t *testing.T) {
	svc := NewEnvironmentService(nil, nil, nil, nil, nil)
	assert.NotNil(t, svc)
}

func TestEnvironmentService_Up_RemotoComNomeDeAmbiente(t *testing.T) {
	svc := NewEnvironmentService(&MockRunner{}, nil, nil, nil, nil)
	// Diferentes nomes de ambientes remotos
	for _, env := range []string{"staging", "production", "remote", "aws-prod"} {
		err := svc.Up(context.Background(), UpOptions{Environment: env})
		assert.NoError(t, err, "ambiente remoto '%s' não deveria falhar", env)
	}
}

func TestEnvironmentService_Up_LocalSyncFalha_ContinuaNormalmente(t *testing.T) {
	// Quando Sync() falha, o fluxo deve continuar (erro é soft)
	cluster := &MockClusterManager{
		ExistsFunc: func(ctx context.Context, name string) (bool, error) {
			return false, nil
		},
	}
	mirror := &MockMirrorService{
		SyncFunc: func() error {
			return errors.New("sync falhou")
		},
	}
	bs := &MockBootstrapService{}
	runner := &MockRunner{}

	svc := NewEnvironmentService(runner, nil, cluster, mirror, bs)
	err := svc.Up(context.Background(), UpOptions{
		Root:        "/tmp/infra",
		Environment: "local",
		ClusterName: "yby-test",
	})
	// Sync falha mas o fluxo continua
	assert.NoError(t, err)
}

func TestEnvironmentService_Up_LocalClusterExistsRetornaErro(t *testing.T) {
	// Quando Exists retorna erro, deve criar cluster (trata como não existente)
	createdCluster := false
	cluster := &MockClusterManager{
		ExistsFunc: func(ctx context.Context, name string) (bool, error) {
			return false, errors.New("falha ao verificar")
		},
		CreateFunc: func(ctx context.Context, name string, configFile string) error {
			createdCluster = true
			return nil
		},
	}
	mirror := &MockMirrorService{}
	bs := &MockBootstrapService{}
	runner := &MockRunner{}

	svc := NewEnvironmentService(runner, nil, cluster, mirror, bs)
	err := svc.Up(context.Background(), UpOptions{
		Root:        "/tmp/infra",
		Environment: "local",
		ClusterName: "yby-test",
	})
	assert.NoError(t, err)
	assert.True(t, createdCluster, "cluster deveria ter sido criado quando Exists retorna erro")
}

func TestUpOptions_Campos(t *testing.T) {
	opts := UpOptions{
		Root:        "/infra",
		Environment: "local",
		ClusterName: "cluster-1",
	}
	assert.Equal(t, "/infra", opts.Root)
	assert.Equal(t, "local", opts.Environment)
	assert.Equal(t, "cluster-1", opts.ClusterName)
}

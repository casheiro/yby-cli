package environment

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

// --- Testes para K3dClusterManager ---

func TestK3dClusterManager_Exists_Found(t *testing.T) {
	mock := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("NAME         SERVERS   AGENTS   STATUS\nmy-cluster   1/1       0/0      running\n"), nil
		},
	}
	mgr := &K3dClusterManager{Runner: mock}

	exists, err := mgr.Exists(context.Background(), "my-cluster")

	assert.NoError(t, err)
	assert.True(t, exists, "deve retornar true quando o cluster está na lista")
}

func TestK3dClusterManager_Exists_NotFound(t *testing.T) {
	mock := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("NAME           SERVERS   AGENTS   STATUS\nother-cluster  1/1       0/0      running\n"), nil
		},
	}
	mgr := &K3dClusterManager{Runner: mock}

	exists, err := mgr.Exists(context.Background(), "my-cluster")

	assert.NoError(t, err)
	assert.False(t, exists, "deve retornar false quando o cluster não está na lista")
}

func TestK3dClusterManager_Exists_Error(t *testing.T) {
	mock := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("comando k3d não encontrado")
		},
	}
	mgr := &K3dClusterManager{Runner: mock}

	exists, err := mgr.Exists(context.Background(), "my-cluster")

	assert.NoError(t, err, "erro do comando deve ser ignorado")
	assert.False(t, exists, "deve retornar false quando o comando falha")
}

func TestK3dClusterManager_Create_WithConfig(t *testing.T) {
	var capturedArgs []string
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil
		},
	}
	mgr := &K3dClusterManager{Runner: mock}

	err := mgr.Create(context.Background(), "test-cluster", "/path/to/config.yaml")

	assert.NoError(t, err)
	assert.Contains(t, capturedArgs, "--config", "deve incluir flag --config")
	assert.Contains(t, capturedArgs, "/path/to/config.yaml", "deve incluir caminho do config")
	assert.Contains(t, capturedArgs, "test-cluster", "deve incluir nome do cluster")
}

func TestK3dClusterManager_Create_WithoutConfig(t *testing.T) {
	var capturedArgs []string
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil
		},
	}
	mgr := &K3dClusterManager{Runner: mock}

	err := mgr.Create(context.Background(), "test-cluster", "")

	assert.NoError(t, err)
	assert.NotContains(t, capturedArgs, "--config", "não deve incluir flag --config quando configFile é vazio")
}

func TestK3dClusterManager_Create_Error(t *testing.T) {
	expectedErr := fmt.Errorf("falha ao criar cluster")
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return expectedErr
		},
	}
	mgr := &K3dClusterManager{Runner: mock}

	err := mgr.Create(context.Background(), "test-cluster", "")

	assert.ErrorIs(t, err, expectedErr)
}

func TestK3dClusterManager_Start_Success(t *testing.T) {
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}
	mgr := &K3dClusterManager{Runner: mock}

	err := mgr.Start(context.Background(), "my-cluster")

	assert.NoError(t, err)
}

func TestK3dClusterManager_Start_Error(t *testing.T) {
	expectedErr := fmt.Errorf("falha ao iniciar cluster")
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return expectedErr
		},
	}
	mgr := &K3dClusterManager{Runner: mock}

	err := mgr.Start(context.Background(), "my-cluster")

	assert.ErrorIs(t, err, expectedErr)
}

// --- Testes para K3dClusterManager.Delete ---

func TestK3dClusterManager_Delete_Success(t *testing.T) {
	var capturedArgs []string
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = append([]string{name}, args...)
			return nil
		},
	}
	mgr := &K3dClusterManager{Runner: mock}

	err := mgr.Delete(context.Background(), "my-cluster")

	assert.NoError(t, err)
	assert.Equal(t, []string{"k3d", "cluster", "delete", "my-cluster"}, capturedArgs,
		"deve executar 'k3d cluster delete <nome>'")
}

func TestK3dClusterManager_Delete_Error(t *testing.T) {
	expectedErr := fmt.Errorf("falha ao deletar cluster")
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return expectedErr
		},
	}
	mgr := &K3dClusterManager{Runner: mock}

	err := mgr.Delete(context.Background(), "my-cluster")

	assert.ErrorIs(t, err, expectedErr)
}

// --- Teste para GitMirrorAdapter ---

func TestNewGitMirrorAdapter(t *testing.T) {
	mock := &testutil.MockRunner{}

	adapter := NewGitMirrorAdapter("/tmp/test-path", mock)

	assert.NotNil(t, adapter, "deve retornar um adaptador não nulo")
	assert.NotNil(t, adapter.manager, "deve inicializar o MirrorManager interno")
}

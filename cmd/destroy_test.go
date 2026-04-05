package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/environment"
	"github.com/stretchr/testify/assert"
)

// mockDestroyClusterManager substitui a factory do destroy para testes
func mockDestroyClusterManager(deleteErr error) func() {
	original := newDestroyClusterManager
	newDestroyClusterManager = func() environment.ClusterManager {
		return &mockClusterManager{
			deleteFunc: func(ctx context.Context, name string) error {
				return deleteErr
			},
		}
	}
	return func() { newDestroyClusterManager = original }
}

func TestDestroyCmd_LocalEnv_WithMock(t *testing.T) {
	teardown := mockDestroyClusterManager(nil)
	defer teardown()

	// Garantir que o ambiente é "local" ou vazio
	t.Setenv("YBY_ENV", "")
	t.Setenv("YBY_CLUSTER_NAME", "test-cluster")

	// Salvar e restaurar contextFlag
	oldCtx := contextFlag
	contextFlag = ""
	defer func() { contextFlag = oldCtx }()

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
}

func TestDestroyCmd_NonLocalEnv_Rejected(t *testing.T) {
	t.Setenv("YBY_ENV", "prod")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "local")
}

func TestDestroyCmd_DefaultClusterName(t *testing.T) {
	var capturedName string
	original := newDestroyClusterManager
	newDestroyClusterManager = func() environment.ClusterManager {
		return &mockClusterManager{
			deleteFunc: func(ctx context.Context, name string) error {
				capturedName = name
				return nil
			},
		}
	}
	defer func() { newDestroyClusterManager = original }()

	t.Setenv("YBY_ENV", "")
	t.Setenv("YBY_CLUSTER_NAME", "")

	oldCtx := contextFlag
	contextFlag = ""
	defer func() { contextFlag = oldCtx }()

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
	assert.Equal(t, "yby-local", capturedName, "deve usar 'yby-local' como nome padrão do cluster")
}

func TestDestroyCmd_YBYEnvStaging_Rejected(t *testing.T) {
	t.Setenv("YBY_ENV", "staging")
	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ERR_VALIDATION")
	assert.Contains(t, err.Error(), "staging")
}

func TestDestroyCmd_YBYEnvLocal_Allowed(t *testing.T) {
	teardown := mockDestroyClusterManager(nil)
	defer teardown()

	t.Setenv("YBY_ENV", "local")
	t.Setenv("YBY_CLUSTER_NAME", "test-cluster")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
}

func TestDestroyCmd_DeleteError(t *testing.T) {
	teardown := mockDestroyClusterManager(fmt.Errorf("falha ao deletar cluster"))
	defer teardown()

	t.Setenv("YBY_ENV", "local")
	t.Setenv("YBY_CLUSTER_NAME", "test-cluster")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Erro ao destruir cluster")
}

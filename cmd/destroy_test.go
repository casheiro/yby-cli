package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDestroyCmd_LocalEnv_WithMock(t *testing.T) {
	teardown := mockExecCommand()
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
	teardown := mockExecCommand()
	defer teardown()

	t.Setenv("YBY_ENV", "")
	t.Setenv("YBY_CLUSTER_NAME", "")

	// Salvar e restaurar contextFlag
	oldCtx := contextFlag
	contextFlag = ""
	defer func() { contextFlag = oldCtx }()

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
}

func TestDestroyCmd_YBYEnvStaging_Rejected(t *testing.T) {
	t.Setenv("YBY_ENV", "staging")
	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ERR_VALIDATION")
	assert.Contains(t, err.Error(), "staging")
}

func TestDestroyCmd_YBYEnvLocal_Allowed(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	t.Setenv("YBY_ENV", "local")
	t.Setenv("YBY_CLUSTER_NAME", "test-cluster")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
}

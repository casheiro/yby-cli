package cmd

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestDestroyCmd_LocalEnv_WithMock(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	// Garantir que o ambiente é "local" ou vazio
	viper.Set("environment", "")
	t.Setenv("YBY_CLUSTER_NAME", "test-cluster")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
}

func TestDestroyCmd_NonLocalEnv_Rejected(t *testing.T) {
	viper.Set("environment", "prod")
	defer viper.Set("environment", "")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "local")
}

func TestDestroyCmd_DefaultClusterName(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	viper.Set("environment", "")
	t.Setenv("YBY_CLUSTER_NAME", "")

	err := destroyCmd.RunE(destroyCmd, []string{})
	assert.NoError(t, err)
}

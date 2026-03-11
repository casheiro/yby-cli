package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunRemoteUp_Success(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	err := runRemoteUp(context.Background(), "prod")
	assert.NoError(t, err)
}

func TestRunRemoteUp_DevEnv(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	err := runRemoteUp(context.Background(), "staging")
	assert.NoError(t, err)
}

func TestUpCmd_Structure(t *testing.T) {
	assert.Equal(t, "up", upCmd.Use)
	assert.Contains(t, upCmd.Aliases, "dev")
	assert.NotEmpty(t, upCmd.Short)
}

package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusCmd_WithMock(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	// statusCmd usa RunE, então verificamos que não causa panic
	assert.NotPanics(t, func() {
		_ = statusCmd.RunE(statusCmd, []string{})
	})
}

func TestStatusCmd_KubectlNotFound(t *testing.T) {
	originalLookPath := lookPath
	defer func() { lookPath = originalLookPath }()

	lookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found: %s", file)
	}

	assert.NotPanics(t, func() {
		_ = statusCmd.RunE(statusCmd, []string{})
	})
}

func TestStatusCmd_RunE_Exists(t *testing.T) {
	assert.NotNil(t, statusCmd.RunE, "statusCmd deve usar RunE")
}

func TestStatusCmd_KubectlNotFound_ReturnsError(t *testing.T) {
	originalLookPath := lookPath
	defer func() { lookPath = originalLookPath }()

	lookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found: %s", file)
	}

	err := statusCmd.RunE(statusCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubectl")
}

package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusCmd_WithMock(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	// statusCmd usa Run (não RunE), então verificamos que não causa panic
	assert.NotPanics(t, func() {
		statusCmd.Run(statusCmd, []string{})
	})
}

func TestStatusCmd_KubectlNotFound(t *testing.T) {
	originalLookPath := lookPath
	defer func() { lookPath = originalLookPath }()

	lookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found: %s", file)
	}

	assert.NotPanics(t, func() {
		statusCmd.Run(statusCmd, []string{})
	})
}

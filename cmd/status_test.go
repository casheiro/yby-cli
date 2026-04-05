package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/services/status"
	"github.com/stretchr/testify/assert"
)

// mockStatusService implementa status.Service para testes do cmd.
type mockStatusService struct {
	report *status.StatusReport
}

func (m *mockStatusService) Check(_ context.Context) *status.StatusReport {
	return m.report
}

func TestStatusCmd_WithMock(t *testing.T) {
	origFactory := newStatusService
	defer func() { newStatusService = origFactory }()

	newStatusService = func(_ shared.Runner) status.Service {
		return &mockStatusService{
			report: &status.StatusReport{
				Nodes:   status.ComponentStatus{Available: true, Output: "node1  Ready"},
				ArgoCD:  status.ComponentStatus{Available: true, Output: "argocd-server Running"},
				Ingress: status.ComponentStatus{Available: true, Output: "my-ing nginx"},
				KEDA:    status.ComponentStatus{Available: true, Output: "my-scaler"},
				Kepler:  status.ComponentStatus{Available: true, Message: "Sensor Kepler ATIVO e monitorando o cluster.", Output: "kepler Running"},
			},
		}
	}

	// mockExecCommand continua necessário por conta do lookPath no cmd
	teardown := mockExecCommand()
	defer teardown()

	assert.NotPanics(t, func() {
		err := statusCmd.RunE(statusCmd, []string{})
		assert.NoError(t, err)
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

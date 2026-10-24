package cmd

import (
	"bytes"
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
				Kepler:  status.ComponentStatus{Available: true, Message: status.MensagemKeplerAtivo, Output: "kepler Running"},
			},
		}
	}

	teardown := mockLookPath()
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

func TestRenderStatusReport_Golden(t *testing.T) {
	casos := []struct {
		nome      string
		relatorio *status.StatusReport
	}{
		{"status_ativo", &status.StatusReport{
			Nodes:   status.ComponentStatus{Available: true, Output: "NAME     STATUS   ROLES           AGE\nnode1    Ready    control-plane   3d\n"},
			ArgoCD:  status.ComponentStatus{Available: true, Output: "NAME                 READY   STATUS\nargocd-server-abc    1/1     Running\n"},
			Ingress: status.ComponentStatus{Available: true, Message: "nenhum ingress encontrado"},
			KEDA:    status.ComponentStatus{Available: false, Message: "KEDA não detectado · CRDs ausentes?"},
			Kepler:  status.ComponentStatus{Available: true, Output: "kepler-x Running", Message: status.MensagemKeplerAtivo},
		}},
		{"status_sem_cluster", &status.StatusReport{
			SemCluster: true,
			Nodes: status.ComponentStatus{Available: false, Message: "sem acesso ao cluster", Output: "E1024 15:46:20.317079   39364 memcache.go:265] \"Unhandled Error\" err=\"couldn't get current server API group list\"\n" +
				"E1024 15:46:20.329701   39364 memcache.go:265] \"Unhandled Error\" err=\"couldn't get current server API group list\"\n" +
				"The connection to the server localhost:6443 was refused\n"},
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			usarSaidaTexto(t, false)
			var b bytes.Buffer
			renderStatusReport(&b, c.relatorio, quandoGolden)
			conferirGolden(t, c.nome, b.String())
			conferirSemRestos(t, b.String())
		})
	}
}

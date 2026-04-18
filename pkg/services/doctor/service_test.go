package doctor

import (
	"context"
	"errors"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var _ shared.Runner = (*MockRunner)(nil)

type MockRunner struct {
	mock.Mock
}

func (m *MockRunner) Run(ctx context.Context, name string, args ...string) error {
	calledArgs := m.Called(ctx, name, args)
	return calledArgs.Error(0)
}

func (m *MockRunner) RunCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	calledArgs := m.Called(ctx, name, args)
	return calledArgs.Get(0).([]byte), calledArgs.Error(1)
}

func (m *MockRunner) RunStdin(ctx context.Context, stdin string, name string, args ...string) error {
	calledArgs := m.Called(ctx, stdin, name, args)
	return calledArgs.Error(0)
}

func (m *MockRunner) RunStdinOutput(ctx context.Context, stdin string, name string, args ...string) ([]byte, error) {
	calledArgs := m.Called(ctx, stdin, name, args)
	var retBytes []byte
	if b := calledArgs.Get(0); b != nil {
		retBytes = b.([]byte)
	}
	return retBytes, calledArgs.Error(1)
}

func (m *MockRunner) LookPath(file string) (string, error) {
	calledArgs := m.Called(file)
	return calledArgs.String(0), calledArgs.Error(1)
}

func TestDoctorService_Run(t *testing.T) {
	mockRunner := new(MockRunner)
	svc := NewService(mockRunner)

	ctx := context.Background()

	// Setup Mocks
	mockRunner.On("RunCombinedOutput", ctx, "grep", []string{"MemTotal", "/proc/meminfo"}).Return([]byte("MemTotal: 16000000 kB\n"), nil)

	// Tools exist
	tools := []string{"kubectl", "helm", "argocd", "git", "direnv"}
	for _, tool := range tools {
		mockRunner.On("LookPath", tool).Return("/usr/bin/"+tool, nil)
	}

	// docker info success
	mockRunner.On("Run", ctx, "docker", []string{"info"}).Return(nil)

	// Ferramentas opcionais
	mockRunner.On("LookPath", "kubeseal").Return("/usr/bin/kubeseal", nil)
	mockRunner.On("LookPath", "sops").Return("/usr/bin/sops", nil)
	mockRunner.On("LookPath", "age").Return("/usr/bin/age", nil)

	// kubectl get nodes success
	mockRunner.On("Run", ctx, "kubectl", []string{"--insecure-skip-tls-verify", "get", "nodes"}).Return(nil)

	// CRDs success
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "servicemonitors.monitoring.coreos.com"}).Return(nil)
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "clusterissuers.cert-manager.io"}).Return(nil)
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "scaledobjects.keda.sh"}).Return(nil)

	// Cloud CLIs não instalados (cloud.Detect usa LookPath)
	mockRunner.On("LookPath", "aws").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "az").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "gcloud").Return("", errors.New("not found"))

	report := svc.Run(ctx)

	assert.NotNil(t, report)
	assert.Len(t, report.System, 1)
	assert.True(t, report.System[0].Status)
	assert.Contains(t, report.System[0].Message, "16000000 kB")

	assert.Len(t, report.Tools, 9)         // 5 obrigatórios + 1 docker + 3 opcionais
	assert.True(t, report.Tools[0].Status) // kubectl
	assert.True(t, report.Tools[5].Status) // docker

	assert.Len(t, report.Cluster, 1)
	assert.True(t, report.Cluster[0].Status)

	assert.Len(t, report.CRDs, 3)
	assert.True(t, report.CRDs[0].Status)

	// Cloud: nenhum provider detectado
	assert.Len(t, report.Cloud, 1)
	assert.Equal(t, "Nenhum provider cloud detectado", report.Cloud[0].Message)
}

func TestDoctorReport_CloudFieldExists(t *testing.T) {
	report := &DoctorReport{}
	assert.NotNil(t, report)
	assert.Nil(t, report.Cloud)

	report.Cloud = []CheckResult{
		{Name: "aws", Status: true, Message: "CLI v2.15.0 instalado"},
	}
	assert.Len(t, report.Cloud, 1)
	assert.Equal(t, "aws", report.Cloud[0].Name)
}

func TestDoctor_NoCloudProviders(t *testing.T) {
	// Quando nenhum provider cloud é detectado (cloud.Detect retorna slice vazio),
	// o report deve conter uma mensagem informativa.
	mockRunner := new(MockRunner)
	svc := NewService(mockRunner)
	ctx := context.Background()

	// Setup mocks mínimos para Run() completar
	mockRunner.On("RunCombinedOutput", ctx, "grep", []string{"MemTotal", "/proc/meminfo"}).Return([]byte("MemTotal: 16000000 kB\n"), nil)
	tools := []string{"kubectl", "helm", "argocd", "git", "direnv"}
	for _, tool := range tools {
		mockRunner.On("LookPath", tool).Return("/usr/bin/"+tool, nil)
	}
	mockRunner.On("Run", ctx, "docker", []string{"info"}).Return(nil)
	mockRunner.On("LookPath", "kubeseal").Return("/usr/bin/kubeseal", nil)
	mockRunner.On("LookPath", "sops").Return("/usr/bin/sops", nil)
	mockRunner.On("LookPath", "age").Return("/usr/bin/age", nil)
	mockRunner.On("Run", ctx, "kubectl", []string{"--insecure-skip-tls-verify", "get", "nodes"}).Return(nil)
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "servicemonitors.monitoring.coreos.com"}).Return(nil)
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "clusterissuers.cert-manager.io"}).Return(nil)
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "scaledobjects.keda.sh"}).Return(nil)

	// Cloud CLIs não instalados
	mockRunner.On("LookPath", "aws").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "az").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "gcloud").Return("", errors.New("not found"))

	report := svc.Run(ctx)

	// cloud.Detect sem kubeconfig e sem CLIs cloud instalados retorna vazio,
	// então checkCloudProviders retorna mensagem informativa
	assert.NotNil(t, report.Cloud)
	assert.Len(t, report.Cloud, 1)
	assert.Equal(t, "Cloud Providers", report.Cloud[0].Name)
	assert.True(t, report.Cloud[0].Status)
	assert.Equal(t, "Nenhum provider cloud detectado", report.Cloud[0].Message)
}

func TestDoctorService_Failures(t *testing.T) {
	mockRunner := new(MockRunner)
	svc := NewService(mockRunner)

	ctx := context.Background()

	// Setup Mocks failing
	mockRunner.On("RunCombinedOutput", ctx, "grep", []string{"MemTotal", "/proc/meminfo"}).Return([]byte{}, errors.New("exit status 1"))

	mockRunner.On("LookPath", "kubectl").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "helm").Return("/usr/bin/helm", nil)
	mockRunner.On("LookPath", "argocd").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "git").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "direnv").Return("", errors.New("not found"))

	// Ferramentas opcionais
	mockRunner.On("LookPath", "kubeseal").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "sops").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "age").Return("", errors.New("not found"))

	mockRunner.On("Run", ctx, "docker", []string{"info"}).Return(errors.New("permission denied"))
	mockRunner.On("Run", ctx, "kubectl", []string{"--insecure-skip-tls-verify", "get", "nodes"}).Return(errors.New("connection refused"))
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "servicemonitors.monitoring.coreos.com"}).Return(errors.New("not found"))
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "clusterissuers.cert-manager.io"}).Return(errors.New("not found"))
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "scaledobjects.keda.sh"}).Return(errors.New("not found"))

	// Cloud CLIs não instalados
	mockRunner.On("LookPath", "aws").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "az").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "gcloud").Return("", errors.New("not found"))

	report := svc.Run(ctx)

	assert.NotNil(t, report)
	assert.Len(t, report.System, 1)
	assert.False(t, report.System[0].Status)

	assert.False(t, report.Tools[0].Status) // kubectl missing
	assert.True(t, report.Tools[1].Status)  // helm found
	assert.False(t, report.Tools[5].Status) // docker permission denied

	assert.False(t, report.Cluster[0].Status)
	assert.False(t, report.CRDs[0].Status)
}

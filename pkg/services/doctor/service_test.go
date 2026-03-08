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
	tools := []string{"kubectl", "helm", "kubeseal", "argocd", "git", "direnv"}
	for _, tool := range tools {
		mockRunner.On("LookPath", tool).Return("/usr/bin/"+tool, nil)
	}

	// docker info success
	mockRunner.On("Run", ctx, "docker", []string{"info"}).Return(nil)

	// kubectl get nodes success
	mockRunner.On("Run", ctx, "kubectl", []string{"--insecure-skip-tls-verify", "get", "nodes"}).Return(nil)

	// CRDs success
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "servicemonitors.monitoring.coreos.com"}).Return(nil)
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "clusterissuers.cert-manager.io"}).Return(nil)
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "scaledobjects.keda.sh"}).Return(nil)

	report := svc.Run(ctx)

	assert.NotNil(t, report)
	assert.Len(t, report.System, 1)
	assert.True(t, report.System[0].Status)
	assert.Contains(t, report.System[0].Message, "16000000 kB")

	assert.Len(t, report.Tools, 7)         // 6 path tools + 1 docker
	assert.True(t, report.Tools[0].Status) // kubectl
	assert.True(t, report.Tools[6].Status) // docker

	assert.Len(t, report.Cluster, 1)
	assert.True(t, report.Cluster[0].Status)

	assert.Len(t, report.CRDs, 3)
	assert.True(t, report.CRDs[0].Status)
}

func TestDoctorService_Failures(t *testing.T) {
	mockRunner := new(MockRunner)
	svc := NewService(mockRunner)

	ctx := context.Background()

	// Setup Mocks failing
	mockRunner.On("RunCombinedOutput", ctx, "grep", []string{"MemTotal", "/proc/meminfo"}).Return([]byte{}, errors.New("exit status 1"))

	mockRunner.On("LookPath", "kubectl").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "helm").Return("/usr/bin/helm", nil)
	mockRunner.On("LookPath", "kubeseal").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "argocd").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "git").Return("", errors.New("not found"))
	mockRunner.On("LookPath", "direnv").Return("", errors.New("not found"))

	mockRunner.On("Run", ctx, "docker", []string{"info"}).Return(errors.New("permission denied"))
	mockRunner.On("Run", ctx, "kubectl", []string{"--insecure-skip-tls-verify", "get", "nodes"}).Return(errors.New("connection refused"))
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "servicemonitors.monitoring.coreos.com"}).Return(errors.New("not found"))
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "clusterissuers.cert-manager.io"}).Return(errors.New("not found"))
	mockRunner.On("Run", ctx, "kubectl", []string{"get", "crd", "scaledobjects.keda.sh"}).Return(errors.New("not found"))

	report := svc.Run(ctx)

	assert.NotNil(t, report)
	assert.Len(t, report.System, 1)
	assert.False(t, report.System[0].Status)

	assert.False(t, report.Tools[0].Status) // kubectl missing
	assert.True(t, report.Tools[1].Status)  // helm found
	assert.False(t, report.Tools[6].Status) // docker permission denied

	assert.False(t, report.Cluster[0].Status)
	assert.False(t, report.CRDs[0].Status)
}

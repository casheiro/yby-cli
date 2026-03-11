package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/network"
	"github.com/stretchr/testify/assert"
)

// ========================================================
// Mocks para testes de access
// ========================================================

type mockClusterNetworkManager struct {
	getCurrentContextFunc func() (string, error)
	getSecretValueFunc    func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error)
	hasServiceFunc        func(ctx context.Context, kubeContext, ns, serviceName string) bool
	portForwardFunc       func(ctx context.Context, kubeContext, ns, resource, ports string) error
	createTokenFunc       func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error)
	killPortForwardFunc   func(port string)
}

func (m *mockClusterNetworkManager) GetCurrentContext() (string, error) {
	if m.getCurrentContextFunc != nil {
		return m.getCurrentContextFunc()
	}
	return "k3d-yby-local", nil
}

func (m *mockClusterNetworkManager) GetSecretValue(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
	if m.getSecretValueFunc != nil {
		return m.getSecretValueFunc(ctx, kubeContext, ns, secretName, jsonPathKey)
	}
	return "mock-value", nil
}

func (m *mockClusterNetworkManager) HasService(ctx context.Context, kubeContext, ns, serviceName string) bool {
	if m.hasServiceFunc != nil {
		return m.hasServiceFunc(ctx, kubeContext, ns, serviceName)
	}
	return false
}

func (m *mockClusterNetworkManager) PortForward(ctx context.Context, kubeContext, ns, resource, ports string) error {
	if m.portForwardFunc != nil {
		return m.portForwardFunc(ctx, kubeContext, ns, resource, ports)
	}
	return nil
}

func (m *mockClusterNetworkManager) CreateToken(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
	if m.createTokenFunc != nil {
		return m.createTokenFunc(ctx, kubeContext, ns, serviceAccount, duration)
	}
	return "mock-token-abc123", nil
}

func (m *mockClusterNetworkManager) KillPortForward(port string) {
	if m.killPortForwardFunc != nil {
		m.killPortForwardFunc(port)
	}
}

type mockLocalContainerManager struct {
	isAvailableFunc  func() bool
	startGrafanaFunc func(ctx context.Context) error
}

func (m *mockLocalContainerManager) IsAvailable() bool {
	if m.isAvailableFunc != nil {
		return m.isAvailableFunc()
	}
	return false
}

func (m *mockLocalContainerManager) StartGrafana(ctx context.Context) error {
	if m.startGrafanaFunc != nil {
		return m.startGrafanaFunc(ctx)
	}
	return nil
}

// ========================================================
// Testes do access command
// ========================================================

func TestAccessCmd_RunComContextoMock(t *testing.T) {
	origNet := newNetworkAdapter
	origCont := newContainerAdapter
	defer func() {
		newNetworkAdapter = origNet
		newContainerAdapter = origCont
	}()

	mockNet := &mockClusterNetworkManager{
		getCurrentContextFunc: func() (string, error) {
			return "k3d-yby-local", nil
		},
		hasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return false
		},
		createTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "token-123", nil
		},
	}
	mockCont := &mockLocalContainerManager{
		isAvailableFunc: func() bool { return false },
	}

	newNetworkAdapter = func() network.ClusterNetworkManager { return mockNet }
	newContainerAdapter = func() network.LocalContainerManager { return mockCont }

	// Usa contexto já cancelado para que g.Wait() retorne rápido
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	accessCmd.SetContext(ctx)
	err := accessCmd.RunE(accessCmd, []string{})
	// Com contexto cancelado, pode retornar nil ou erro — ambos aceitáveis
	_ = err
}

func TestAccessCmd_RunComServicosMock(t *testing.T) {
	origNet := newNetworkAdapter
	origCont := newContainerAdapter
	defer func() {
		newNetworkAdapter = origNet
		newContainerAdapter = origCont
	}()

	mockNet := &mockClusterNetworkManager{
		getCurrentContextFunc: func() (string, error) {
			return "k3d-yby-local", nil
		},
		getSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			return "admin-password", nil
		},
		hasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			// Simula todos os serviços disponíveis
			return true
		},
		portForwardFunc: func(ctx context.Context, kubeContext, ns, resource, ports string) error {
			// Retorna imediatamente, simula port-forward bem-sucedido
			return nil
		},
		createTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "headlamp-token-xyz", nil
		},
		killPortForwardFunc: func(port string) {},
	}
	mockCont := &mockLocalContainerManager{
		isAvailableFunc:  func() bool { return true },
		startGrafanaFunc: func(ctx context.Context) error { return nil },
	}

	newNetworkAdapter = func() network.ClusterNetworkManager { return mockNet }
	newContainerAdapter = func() network.LocalContainerManager { return mockCont }

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	accessCmd.SetContext(ctx)
	err := accessCmd.RunE(accessCmd, []string{})
	_ = err
}

func TestAccessCmd_RunErroContexto(t *testing.T) {
	origNet := newNetworkAdapter
	origCont := newContainerAdapter
	defer func() {
		newNetworkAdapter = origNet
		newContainerAdapter = origCont
	}()

	mockNet := &mockClusterNetworkManager{
		getCurrentContextFunc: func() (string, error) {
			return "", fmt.Errorf("erro ao detectar contexto kubernetes")
		},
	}
	mockCont := &mockLocalContainerManager{}

	newNetworkAdapter = func() network.ClusterNetworkManager { return mockNet }
	newContainerAdapter = func() network.LocalContainerManager { return mockCont }

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	accessCmd.SetContext(ctx)
	err := accessCmd.RunE(accessCmd, []string{})
	assert.Error(t, err, "deveria retornar erro quando não consegue detectar contexto")
}

func TestAccessCmd_RunComContextoFlag(t *testing.T) {
	origNet := newNetworkAdapter
	origCont := newContainerAdapter
	defer func() {
		newNetworkAdapter = origNet
		newContainerAdapter = origCont
	}()

	mockNet := &mockClusterNetworkManager{
		hasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return false
		},
		createTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "", fmt.Errorf("sem token")
		},
	}
	mockCont := &mockLocalContainerManager{}

	newNetworkAdapter = func() network.ClusterNetworkManager { return mockNet }
	newContainerAdapter = func() network.LocalContainerManager { return mockCont }

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	accessCmd.SetContext(ctx)
	// Testa com flag --context definida
	_ = accessCmd.Flags().Set("context", "meu-cluster")
	defer func() { _ = accessCmd.Flags().Set("context", "") }()

	err := accessCmd.RunE(accessCmd, []string{})
	_ = err
}

func TestAccessCmd_GrafanaFalha(t *testing.T) {
	origNet := newNetworkAdapter
	origCont := newContainerAdapter
	defer func() {
		newNetworkAdapter = origNet
		newContainerAdapter = origCont
	}()

	mockNet := &mockClusterNetworkManager{
		getCurrentContextFunc: func() (string, error) {
			return "k3d-yby-local", nil
		},
		hasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			// Simula apenas prometheus disponível
			return serviceName == "system-kube-prometheus-sta-prometheus"
		},
		portForwardFunc: func(ctx context.Context, kubeContext, ns, resource, ports string) error {
			return nil
		},
		createTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "", fmt.Errorf("sem token")
		},
		killPortForwardFunc: func(port string) {},
	}
	mockCont := &mockLocalContainerManager{
		isAvailableFunc: func() bool { return true },
		startGrafanaFunc: func(ctx context.Context) error {
			return fmt.Errorf("erro ao iniciar grafana")
		},
	}

	newNetworkAdapter = func() network.ClusterNetworkManager { return mockNet }
	newContainerAdapter = func() network.LocalContainerManager { return mockCont }

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	accessCmd.SetContext(ctx)
	err := accessCmd.RunE(accessCmd, []string{})
	_ = err
}

// ========================================================
// Testes das factories
// ========================================================

func TestNewNetworkAdapterFactory_Default(t *testing.T) {
	adapter := newNetworkAdapter()
	assert.NotNil(t, adapter, "newNetworkAdapter deveria retornar adaptador não-nil")
}

func TestNewContainerAdapterFactory_Default(t *testing.T) {
	adapter := newContainerAdapter()
	assert.NotNil(t, adapter, "newContainerAdapter deveria retornar adaptador não-nil")
}

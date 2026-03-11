package network

import (
	"context"
	"fmt"
	"testing"
)

type MockClusterNetworkManager struct {
	GetCurrentContextFunc func() (string, error)
	GetSecretValueFunc    func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error)
	HasServiceFunc        func(ctx context.Context, kubeContext, ns, serviceName string) bool
	PortForwardFunc       func(ctx context.Context, kubeContext, ns, resource, ports string) error
	CreateTokenFunc       func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error)
	KillPortForwardFunc   func(port string)
}

func (m *MockClusterNetworkManager) GetCurrentContext() (string, error) {
	if m.GetCurrentContextFunc != nil {
		return m.GetCurrentContextFunc()
	}
	return "local-ctx", nil
}

func (m *MockClusterNetworkManager) GetSecretValue(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
	if m.GetSecretValueFunc != nil {
		return m.GetSecretValueFunc(ctx, kubeContext, ns, secretName, jsonPathKey)
	}
	return "mock-secret", nil
}

func (m *MockClusterNetworkManager) HasService(ctx context.Context, kubeContext, ns, serviceName string) bool {
	if m.HasServiceFunc != nil {
		return m.HasServiceFunc(ctx, kubeContext, ns, serviceName)
	}
	return false
}

func (m *MockClusterNetworkManager) PortForward(ctx context.Context, kubeContext, ns, resource, ports string) error {
	if m.PortForwardFunc != nil {
		return m.PortForwardFunc(ctx, kubeContext, ns, resource, ports)
	}
	return nil
}

func (m *MockClusterNetworkManager) CreateToken(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
	if m.CreateTokenFunc != nil {
		return m.CreateTokenFunc(ctx, kubeContext, ns, serviceAccount, duration)
	}
	return "mock-token", nil
}

func (m *MockClusterNetworkManager) KillPortForward(port string) {
	if m.KillPortForwardFunc != nil {
		m.KillPortForwardFunc(port)
	}
}

type MockLocalContainerManager struct {
	IsAvailableFunc  func() bool
	StartGrafanaFunc func(ctx context.Context) error
}

func (m *MockLocalContainerManager) IsAvailable() bool {
	if m.IsAvailableFunc != nil {
		return m.IsAvailableFunc()
	}
	return true
}

func (m *MockLocalContainerManager) StartGrafana(ctx context.Context) error {
	if m.StartGrafanaFunc != nil {
		return m.StartGrafanaFunc(ctx)
	}
	return nil
}

func TestDefaultAccessService_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			// Fake success for some services to test that it spawns port-forwards
			if ns == "argocd" && serviceName == "argocd-server" {
				return true
			}
			if ns == "storage" && serviceName == "minio" {
				return true
			}
			return false
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			return "mock_pass", nil
		},
		PortForwardFunc: func(ctx context.Context, kubeContext, ns, resource, ports string) error {
			// Check if we can immediately return successfully to let errgroup finish
			return nil
		},
	}
	mockCont := &MockLocalContainerManager{}

	svc := NewAccessService(mockNet, mockCont)

	opts := AccessOptions{
		TargetContext: "test-ctx",
	}

	err := svc.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestDefaultAccessService_NoContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		GetCurrentContextFunc: func() (string, error) {
			return "", fmt.Errorf("no context found")
		},
	}
	mockCont := &MockLocalContainerManager{}
	svc := NewAccessService(mockNet, mockCont)

	err := svc.Run(ctx, AccessOptions{})
	if err == nil {
		t.Fatal("Expected error when no context is provided and retrieval fails")
	}
}

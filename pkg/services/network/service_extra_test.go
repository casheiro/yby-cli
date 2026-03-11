package network

import (
	"context"
	"errors"
	"testing"
)

func TestNewAccessService(t *testing.T) {
	mockNet := &MockClusterNetworkManager{}
	mockCont := &MockLocalContainerManager{}
	svc := NewAccessService(mockNet, mockCont)
	if svc == nil {
		t.Fatal("NewAccessService should not return nil")
	}
}

func TestDefaultAccessService_Run_WithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return false
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			return "", errors.New("secret not found")
		},
		CreateTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "test-token", nil
		},
	}
	mockCont := &MockLocalContainerManager{}

	svc := NewAccessService(mockNet, mockCont)
	err := svc.Run(ctx, AccessOptions{TargetContext: "test-ctx"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDefaultAccessService_Run_WithMinio(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return ns == "storage" && serviceName == "minio"
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			return "adminuser", nil
		},
		PortForwardFunc: func(ctx context.Context, kubeContext, ns, resource, ports string) error {
			return nil
		},
	}
	mockCont := &MockLocalContainerManager{}
	svc := NewAccessService(mockNet, mockCont)
	err := svc.Run(ctx, AccessOptions{TargetContext: "test-ctx"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDefaultAccessService_Run_WithPrometheus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return ns == "kube-system" && serviceName == "system-kube-prometheus-sta-prometheus"
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			return "", errors.New("no secret")
		},
		PortForwardFunc: func(ctx context.Context, kubeContext, ns, resource, ports string) error {
			return nil
		},
	}
	mockCont := &MockLocalContainerManager{
		IsAvailableFunc: func() bool { return true },
		StartGrafanaFunc: func(ctx context.Context) error {
			return nil
		},
	}
	svc := NewAccessService(mockNet, mockCont)
	err := svc.Run(ctx, AccessOptions{TargetContext: "test-ctx"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDefaultAccessService_Run_WithPrometheus_DockerUnavailable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return ns == "monitoring" && serviceName == "prometheus-server"
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			return "", errors.New("no secret")
		},
		PortForwardFunc: func(ctx context.Context, kubeContext, ns, resource, ports string) error {
			return nil
		},
	}
	mockCont := &MockLocalContainerManager{
		IsAvailableFunc: func() bool { return false }, // Docker not available
	}
	svc := NewAccessService(mockNet, mockCont)
	err := svc.Run(ctx, AccessOptions{TargetContext: "test-ctx"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDefaultAccessService_Run_TokenError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return false
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			return "", errors.New("no token")
		},
		CreateTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "", errors.New("token create failed")
		},
	}
	mockCont := &MockLocalContainerManager{}
	svc := NewAccessService(mockNet, mockCont)
	// should succeed even if token creation fails (error is soft)
	err := svc.Run(ctx, AccessOptions{TargetContext: "test-ctx"})
	if err != nil {
		t.Fatalf("expected no error (token error is soft), got: %v", err)
	}
}

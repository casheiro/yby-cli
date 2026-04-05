package network

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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

// --- Testes para maskToken ---

func TestMaskToken_LongerThan16(t *testing.T) {
	// Token longo deve mostrar primeiros 8 e últimos 8 caracteres
	token := "abcdefghijklmnopqrstuvwxyz"
	result := maskToken(token)

	assert.Equal(t, "abcdefgh***...**stuvwxyz", result, "deve mascarar o meio do token")
}

func TestMaskToken_Exactly16(t *testing.T) {
	// Token com exatamente 16 caracteres deve ser totalmente mascarado
	token := "1234567890123456"
	result := maskToken(token)

	assert.Equal(t, "***", result, "deve retornar *** para token com 16 caracteres")
}

func TestMaskToken_ShortString(t *testing.T) {
	// Token curto (< 8 chars) deve ser totalmente mascarado
	token := "abc"
	result := maskToken(token)

	assert.Equal(t, "***", result, "deve retornar *** para string curta")
}

func TestMaskToken_Empty(t *testing.T) {
	// String vazia deve ser totalmente mascarada
	result := maskToken("")

	assert.Equal(t, "***", result, "deve retornar *** para string vazia")
}

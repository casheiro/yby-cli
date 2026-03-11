package network

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultAccessService_Run_DeteccaoAutomaticaDeContexto(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		GetCurrentContextFunc: func() (string, error) {
			return "contexto-auto-detectado", nil
		},
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return false
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			return "", errors.New("sem segredo")
		},
		CreateTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "", errors.New("sem token")
		},
	}
	mockCont := &MockLocalContainerManager{}

	svc := NewAccessService(mockNet, mockCont)
	// Sem TargetContext, deve detectar automaticamente
	err := svc.Run(ctx, AccessOptions{})
	assert.NoError(t, err)
}

func TestDefaultAccessService_Run_ArgoErroSenha(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return false
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			// Falha ao obter senha do ArgoCD
			return "", errors.New("segredo não encontrado")
		},
		CreateTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "token-ok", nil
		},
	}
	mockCont := &MockLocalContainerManager{}

	svc := NewAccessService(mockNet, mockCont)
	err := svc.Run(ctx, AccessOptions{TargetContext: "test"})
	// Erro do Argo é soft, não deve falhar
	assert.NoError(t, err)
}

func TestDefaultAccessService_Run_MinioSegundoCandidato(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			// Primeiro candidato (storage/minio) não existe, segundo (default/minio) existe
			return ns == "default" && serviceName == "minio"
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			if secretName == "minio-secret" {
				return "", errors.New("não encontrado")
			}
			if secretName == "minio-creds" {
				if jsonPathKey == "rootUser" {
					return "admin", nil
				}
				return "senha123", nil
			}
			return "", errors.New("sem segredo")
		},
		PortForwardFunc: func(ctx context.Context, kubeContext, ns, resource, ports string) error {
			return nil
		},
		CreateTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "", errors.New("sem token")
		},
	}
	mockCont := &MockLocalContainerManager{}

	svc := NewAccessService(mockNet, mockCont)
	err := svc.Run(ctx, AccessOptions{TargetContext: "test"})
	assert.NoError(t, err)
}

func TestDefaultAccessService_Run_MinioSemCredenciais(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return ns == "minio" && serviceName == "minio"
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			// Todas as tentativas de busca de credenciais falham
			return "", errors.New("não encontrado")
		},
		PortForwardFunc: func(ctx context.Context, kubeContext, ns, resource, ports string) error {
			return nil
		},
		CreateTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "", errors.New("sem token")
		},
	}
	mockCont := &MockLocalContainerManager{}

	svc := NewAccessService(mockNet, mockCont)
	err := svc.Run(ctx, AccessOptions{TargetContext: "test"})
	assert.NoError(t, err)
}

func TestDefaultAccessService_Run_PrometheusComGrafanaErro(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			return ns == "monitoring" && serviceName == "prometheus-kube-prometheus-prometheus"
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			return "", errors.New("sem segredo")
		},
		PortForwardFunc: func(ctx context.Context, kubeContext, ns, resource, ports string) error {
			return nil
		},
		CreateTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "", errors.New("sem token")
		},
	}
	mockCont := &MockLocalContainerManager{
		IsAvailableFunc: func() bool { return true },
		StartGrafanaFunc: func(ctx context.Context) error {
			return errors.New("falha ao iniciar Grafana")
		},
	}

	svc := NewAccessService(mockNet, mockCont)
	err := svc.Run(ctx, AccessOptions{TargetContext: "test"})
	// Erro do Grafana é soft
	assert.NoError(t, err)
}

func TestDefaultAccessService_Run_ComTodosServicos(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	killCalled := map[string]bool{}
	mockNet := &MockClusterNetworkManager{
		HasServiceFunc: func(ctx context.Context, kubeContext, ns, serviceName string) bool {
			// Todos os serviços disponíveis
			if ns == "storage" && serviceName == "minio" {
				return true
			}
			if ns == "kube-system" && serviceName == "system-kube-prometheus-sta-prometheus" {
				return true
			}
			return false
		},
		GetSecretValueFunc: func(ctx context.Context, kubeContext, ns, secretName, jsonPathKey string) (string, error) {
			return "valor-segredo", nil
		},
		PortForwardFunc: func(ctx context.Context, kubeContext, ns, resource, ports string) error {
			return nil
		},
		CreateTokenFunc: func(ctx context.Context, kubeContext, ns, serviceAccount, duration string) (string, error) {
			return "meu-token", nil
		},
		KillPortForwardFunc: func(port string) {
			killCalled[port] = true
		},
	}
	mockCont := &MockLocalContainerManager{
		IsAvailableFunc:  func() bool { return true },
		StartGrafanaFunc: func(ctx context.Context) error { return nil },
	}

	svc := NewAccessService(mockNet, mockCont)
	err := svc.Run(ctx, AccessOptions{TargetContext: "test"})
	assert.NoError(t, err)
	// Verifica que KillPortForward foi chamado para as portas esperadas
	assert.True(t, killCalled["9000"])
	assert.True(t, killCalled["9001"])
	assert.True(t, killCalled["9090"])
}

func TestAccessOptions_Campos(t *testing.T) {
	opts := AccessOptions{TargetContext: "meu-contexto"}
	assert.Equal(t, "meu-contexto", opts.TargetContext)
}

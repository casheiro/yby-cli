package network

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

// --- Testes para RealClusterNetworkManager ---

func TestRealClusterNetworkManager_GetCurrentContext(t *testing.T) {
	mock := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("k3d-yby-local\n"), nil
		},
	}
	mgr := &RealClusterNetworkManager{Runner: mock}

	result, err := mgr.GetCurrentContext()

	assert.NoError(t, err)
	assert.Equal(t, "k3d-yby-local", result, "deve retornar o contexto sem espaços extras")
}

func TestRealClusterNetworkManager_GetCurrentContext_Error(t *testing.T) {
	mock := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("nenhum contexto configurado")
		},
	}
	mgr := &RealClusterNetworkManager{Runner: mock}

	_, err := mgr.GetCurrentContext()

	assert.Error(t, err)
}

func TestRealClusterNetworkManager_GetSecretValue_Success(t *testing.T) {
	secretValue := "minha-senha-secreta"
	encoded := base64.StdEncoding.EncodeToString([]byte(secretValue))

	mock := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte(encoded), nil
		},
	}
	mgr := &RealClusterNetworkManager{Runner: mock}

	result, err := mgr.GetSecretValue(context.Background(), "k3d-yby", "argocd", "argocd-initial-admin-secret", "password")

	assert.NoError(t, err)
	assert.Equal(t, secretValue, result, "deve decodificar o valor base64 corretamente")
}

func TestRealClusterNetworkManager_GetSecretValue_Error(t *testing.T) {
	mock := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("secret não encontrada")
		},
	}
	mgr := &RealClusterNetworkManager{Runner: mock}

	result, err := mgr.GetSecretValue(context.Background(), "k3d-yby", "argocd", "secret-inexistente", "password")

	assert.Error(t, err)
	assert.Empty(t, result)
}

func TestRealClusterNetworkManager_HasService_True(t *testing.T) {
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}
	mgr := &RealClusterNetworkManager{Runner: mock}

	result := mgr.HasService(context.Background(), "k3d-yby", "argocd", "argocd-server")

	assert.True(t, result, "deve retornar true quando o serviço existe")
}

func TestRealClusterNetworkManager_HasService_False(t *testing.T) {
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return fmt.Errorf("serviço não encontrado")
		},
	}
	mgr := &RealClusterNetworkManager{Runner: mock}

	result := mgr.HasService(context.Background(), "k3d-yby", "argocd", "svc-inexistente")

	assert.False(t, result, "deve retornar false quando o serviço não existe")
}

func TestRealClusterNetworkManager_PortForward_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancela imediatamente

	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return ctx.Err()
		},
	}
	mgr := &RealClusterNetworkManager{Runner: mock}

	err := mgr.PortForward(ctx, "k3d-yby", "argocd", "svc/argocd-server", "8080:443")

	assert.ErrorIs(t, err, context.Canceled, "deve retornar erro de contexto cancelado")
}

func TestRealClusterNetworkManager_CreateToken(t *testing.T) {
	mock := &testutil.MockRunner{
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.token\n"), nil
		},
	}
	mgr := &RealClusterNetworkManager{Runner: mock}

	token, err := mgr.CreateToken(context.Background(), "k3d-yby", "kubernetes-dashboard", "admin-user", "24h")

	assert.NoError(t, err)
	assert.Equal(t, "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.token", token, "deve retornar o token sem espaços extras")
}

func TestRealClusterNetworkManager_KillPortForward(t *testing.T) {
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}
	mgr := &RealClusterNetworkManager{Runner: mock}

	// Apenas verifica que não entra em pânico
	assert.NotPanics(t, func() {
		mgr.KillPortForward("8080")
	})
}

func TestRealClusterNetworkManager_KillPortForward_Error(t *testing.T) {
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return fmt.Errorf("processo não encontrado")
		},
	}
	mgr := &RealClusterNetworkManager{Runner: mock}

	// Mesmo com erro, não deve entrar em pânico (erro é ignorado)
	assert.NotPanics(t, func() {
		mgr.KillPortForward("8080")
	})
}

// --- Testes para DockerContainerManager ---

func TestDockerContainerManager_IsAvailable_True(t *testing.T) {
	mock := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/docker", nil
		},
	}
	mgr := &DockerContainerManager{Runner: mock}

	assert.True(t, mgr.IsAvailable(), "deve retornar true quando docker está disponível")
}

func TestDockerContainerManager_IsAvailable_False(t *testing.T) {
	mock := &testutil.MockRunner{
		LookPathFunc: func(file string) (string, error) {
			return "", fmt.Errorf("docker não encontrado no PATH")
		},
	}
	mgr := &DockerContainerManager{Runner: mock}

	assert.False(t, mgr.IsAvailable(), "deve retornar false quando docker não está disponível")
}

func TestDockerContainerManager_StartGrafana_Success(t *testing.T) {
	callCount := 0
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			callCount++
			return nil
		},
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("container-id-abc123\n"), nil
		},
	}
	mgr := &DockerContainerManager{Runner: mock}

	err := mgr.StartGrafana(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount, "deve chamar Run duas vezes (volume create e rm)")
}

func TestDockerContainerManager_StartGrafana_Error(t *testing.T) {
	mock := &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("erro: porta já em uso"), fmt.Errorf("exit status 1")
		},
	}
	mgr := &DockerContainerManager{Runner: mock}

	err := mgr.StartGrafana(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "erro: porta já em uso")
}

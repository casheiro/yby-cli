package logs

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListPods_Sucesso verifica listagem de pods com sucesso.
func TestListPods_Sucesso(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return []byte("nginx-abc123 redis-xyz789 api-server-1"), nil
		},
	}

	svc := NewService(runner)
	pods, err := svc.ListPods(context.Background(), "default")

	require.NoError(t, err)
	assert.Equal(t, []string{"nginx-abc123", "redis-xyz789", "api-server-1"}, pods)
}

// TestListPods_Vazio verifica retorno nil quando não há pods.
func TestListPods_Vazio(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}

	svc := NewService(runner)
	pods, err := svc.ListPods(context.Background(), "empty-ns")

	require.NoError(t, err)
	assert.Nil(t, pods)
}

// TestListPods_Erro verifica propagação de erro com hint.
func TestListPods_Erro(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	svc := NewService(runner)
	_, err := svc.ListPods(context.Background(), "default")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao listar pods")
}

// TestListContainers_Sucesso verifica listagem de containers.
func TestListContainers_Sucesso(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return []byte("app sidecar"), nil
		},
	}

	svc := NewService(runner)
	containers, err := svc.ListContainers(context.Background(), "default", "meu-pod")

	require.NoError(t, err)
	assert.Equal(t, []string{"app", "sidecar"}, containers)
}

// TestListContainers_Erro verifica erro ao listar containers.
func TestListContainers_Erro(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("pod not found")
		},
	}

	svc := NewService(runner)
	_, err := svc.ListContainers(context.Background(), "default", "inexistente")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao listar containers")
}

// TestGetLogs_Sucesso verifica obtenção de logs com sucesso.
func TestGetLogs_Sucesso(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return []byte("2024-01-01 INFO Starting server...\n2024-01-01 INFO Ready."), nil
		},
	}

	svc := NewService(runner)
	output, err := svc.GetLogs(context.Background(), LogOptions{
		Namespace: "default",
		Pod:       "nginx-abc123",
		Tail:      100,
	})

	require.NoError(t, err)
	assert.Contains(t, output, "Starting server")
}

// TestGetLogs_ComContainer verifica que o container é passado nos args.
func TestGetLogs_ComContainer(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			capturedArgs = args
			return []byte("log output"), nil
		},
	}

	svc := NewService(runner)
	_, err := svc.GetLogs(context.Background(), LogOptions{
		Namespace: "prod",
		Pod:       "api-pod",
		Container: "sidecar",
		Tail:      50,
	})

	require.NoError(t, err)
	assert.Contains(t, capturedArgs, "-c")
	assert.Contains(t, capturedArgs, "sidecar")
	assert.Contains(t, capturedArgs, "--tail")
	assert.Contains(t, capturedArgs, "50")
}

// TestGetLogs_Erro verifica propagação de erro.
func TestGetLogs_Erro(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("pod is not running")
		},
	}

	svc := NewService(runner)
	_, err := svc.GetLogs(context.Background(), LogOptions{
		Namespace: "default",
		Pod:       "crashed-pod",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao obter logs")
}

// TestStreamLogs_Sucesso verifica que stream chama Run com --follow.
func TestStreamLogs_Sucesso(t *testing.T) {
	var capturedArgs []string
	runner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, name string, args ...string) error {
			capturedArgs = args
			return nil
		},
	}

	svc := NewService(runner)
	err := svc.StreamLogs(context.Background(), LogOptions{
		Namespace: "default",
		Pod:       "nginx-abc123",
	})

	require.NoError(t, err)
	assert.Contains(t, capturedArgs, "--follow")
}

// TestStreamLogs_Erro verifica propagação de erro no stream.
func TestStreamLogs_Erro(t *testing.T) {
	runner := &testutil.MockRunner{
		RunFunc: func(_ context.Context, name string, args ...string) error {
			return fmt.Errorf("stream interrupted")
		},
	}

	svc := NewService(runner)
	err := svc.StreamLogs(context.Background(), LogOptions{
		Namespace: "default",
		Pod:       "meu-pod",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao acompanhar logs")
}

// TestDetectNamespace_Exato verifica detecção por nome exato.
func TestDetectNamespace_Exato(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return []byte("nginx-abc123\tdefault\nredis-xyz789\tcache\napi-server\tbackend"), nil
		},
	}

	svc := NewService(runner)
	ns, err := svc.DetectNamespace(context.Background(), "redis-xyz789")

	require.NoError(t, err)
	assert.Equal(t, "cache", ns)
}

// TestDetectNamespace_Prefixo verifica detecção por prefixo.
func TestDetectNamespace_Prefixo(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return []byte("nginx-deploy-abc123\tdefault\napi-deploy-xyz789\tbackend"), nil
		},
	}

	svc := NewService(runner)
	ns, err := svc.DetectNamespace(context.Background(), "api-deploy")

	require.NoError(t, err)
	assert.Equal(t, "backend", ns)
}

// TestDetectNamespace_NaoEncontrado verifica erro quando pod não existe.
func TestDetectNamespace_NaoEncontrado(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return []byte("nginx-abc123\tdefault"), nil
		},
	}

	svc := NewService(runner)
	_, err := svc.DetectNamespace(context.Background(), "inexistente")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "não encontrado")
}

// TestDetectNamespace_ErroKubectl verifica propagação de erro do kubectl.
func TestDetectNamespace_ErroKubectl(t *testing.T) {
	runner := &testutil.MockRunner{
		RunCombinedOutputFunc: func(_ context.Context, name string, args ...string) ([]byte, error) {
			return nil, fmt.Errorf("cluster unreachable")
		},
	}

	svc := NewService(runner)
	_, err := svc.DetectNamespace(context.Background(), "qualquer")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao buscar pods")
}

// TestBuildKubectlArgs verifica a construção de argumentos.
func TestBuildKubectlArgs(t *testing.T) {
	svc := &logsService{}

	tests := []struct {
		name string
		opts LogOptions
		want []string
	}{
		{
			name: "básico",
			opts: LogOptions{Namespace: "default", Pod: "nginx"},
			want: []string{"logs", "nginx", "-n", "default"},
		},
		{
			name: "com container",
			opts: LogOptions{Namespace: "prod", Pod: "api", Container: "sidecar"},
			want: []string{"logs", "api", "-n", "prod", "-c", "sidecar"},
		},
		{
			name: "com follow",
			opts: LogOptions{Namespace: "dev", Pod: "app", Follow: true},
			want: []string{"logs", "app", "-n", "dev", "--follow"},
		},
		{
			name: "com tail",
			opts: LogOptions{Namespace: "ns", Pod: "p", Tail: 50},
			want: []string{"logs", "p", "-n", "ns", "--tail", "50"},
		},
		{
			name: "completo",
			opts: LogOptions{Namespace: "prod", Pod: "api", Container: "app", Follow: true, Tail: 100},
			want: []string{"logs", "api", "-n", "prod", "-c", "app", "--follow", "--tail", "100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.buildKubectlArgs(tt.opts)
			assert.Equal(t, tt.want, got)
		})
	}
}

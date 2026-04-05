package cmd

import (
	"context"
	"fmt"
	"testing"

	"github.com/casheiro/yby-cli/pkg/services/logs"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/stretchr/testify/assert"
)

// mockLogsService implementa logs.Service para testes.
type mockLogsService struct {
	pods       []string
	containers []string
	logOutput  string
	namespace  string
	err        error
}

func (m *mockLogsService) ListPods(_ context.Context, _ string) ([]string, error) {
	return m.pods, m.err
}

func (m *mockLogsService) ListContainers(_ context.Context, _, _ string) ([]string, error) {
	return m.containers, m.err
}

func (m *mockLogsService) GetLogs(_ context.Context, _ logs.LogOptions) (string, error) {
	return m.logOutput, m.err
}

func (m *mockLogsService) StreamLogs(_ context.Context, _ logs.LogOptions) error {
	return m.err
}

func (m *mockLogsService) DetectNamespace(_ context.Context, _ string) (string, error) {
	if m.namespace == "" && m.err != nil {
		return "", m.err
	}
	return m.namespace, nil
}

// TestLogsCmd_ComPodArg verifica execuç��o com argumento de pod.
func TestLogsCmd_ComPodArg(t *testing.T) {
	origFactory := newLogsService
	defer func() { newLogsService = origFactory }()

	mock := &mockLogsService{
		namespace:  "default",
		pods:       []string{"nginx-abc123"},
		containers: []string{"nginx"},
		logOutput:  "log line 1\nlog line 2\n",
	}
	newLogsService = func(_ shared.Runner) logs.Service { return mock }

	teardown := mockLookPath()
	defer teardown()

	// Desabilitar TTY para evitar prompts interativos
	origTTY := isTTY
	defer func() { isTTY = origTTY }()
	isTTY = func() bool { return false }

	err := logsCmd.RunE(logsCmd, []string{"nginx-abc123"})
	assert.NoError(t, err)
}

// TestLogsCmd_KubectlNotFound verifica erro quando kubectl não está instalado.
func TestLogsCmd_KubectlNotFound(t *testing.T) {
	originalLookPath := lookPath
	defer func() { lookPath = originalLookPath }()

	lookPath = func(file string) (string, error) {
		return "", fmt.Errorf("not found: %s", file)
	}

	err := logsCmd.RunE(logsCmd, []string{"nginx"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kubectl")
}

// TestLogsCmd_SemArgNaoInterativo verifica erro quando não há TTY nem argumento.
func TestLogsCmd_SemArgNaoInterativo(t *testing.T) {
	teardown := mockLookPath()
	defer teardown()

	origFactory := newLogsService
	defer func() { newLogsService = origFactory }()
	newLogsService = func(_ shared.Runner) logs.Service {
		return &mockLogsService{}
	}

	origTTY := isTTY
	defer func() { isTTY = origTTY }()
	isTTY = func() bool { return false }

	err := logsCmd.RunE(logsCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "obrigatório")
}

// TestLogsCmd_DetectNamespaceErro verifica propagação de erro na detecção de namespace.
func TestLogsCmd_DetectNamespaceErro(t *testing.T) {
	origFactory := newLogsService
	defer func() { newLogsService = origFactory }()

	mock := &mockLogsService{
		err: fmt.Errorf("pod não encontrado"),
	}
	newLogsService = func(_ shared.Runner) logs.Service { return mock }

	teardown := mockLookPath()
	defer teardown()

	err := logsCmd.RunE(logsCmd, []string{"inexistente"})
	assert.Error(t, err)
}

// TestLogsCmd_Structure verifica a estrutura básica do comando.
func TestLogsCmd_Structure(t *testing.T) {
	assert.Equal(t, "logs [pod]", logsCmd.Use)
	assert.NotEmpty(t, logsCmd.Short)
	assert.NotNil(t, logsCmd.RunE)

	// Verificar flags
	f := logsCmd.Flags()
	assert.NotNil(t, f.Lookup("follow"))
	assert.NotNil(t, f.Lookup("container"))
	assert.NotNil(t, f.Lookup("tail"))

	// Verificar shorthand
	assert.NotNil(t, f.ShorthandLookup("f"), "shorthand 'f' deve existir")
	assert.NotNil(t, f.ShorthandLookup("t"), "shorthand 't' deve existir")
}

// TestLogsCmd_Follow verifica que a flag --follow aciona StreamLogs.
func TestLogsCmd_Follow(t *testing.T) {
	origFactory := newLogsService
	defer func() { newLogsService = origFactory }()

	streamCalled := false
	mock := &mockLogsService{
		namespace:  "default",
		pods:       []string{"nginx-abc123"},
		containers: []string{"nginx"},
	}

	// Substituir para rastrear chamadas
	svc := &trackingLogsService{
		mockLogsService: mock,
		onStreamLogs: func() {
			streamCalled = true
		},
	}
	newLogsService = func(_ shared.Runner) logs.Service { return svc }

	teardown := mockLookPath()
	defer teardown()

	origTTY := isTTY
	defer func() { isTTY = origTTY }()
	isTTY = func() bool { return false }

	// Configurar flag follow
	logsCmd.Flags().Set("follow", "true")
	defer logsCmd.Flags().Set("follow", "false")

	err := logsCmd.RunE(logsCmd, []string{"nginx-abc123"})
	assert.NoError(t, err)
	assert.True(t, streamCalled, "StreamLogs deveria ter sido chamado com --follow")
}

// trackingLogsService embeds mockLogsService e adiciona tracking.
type trackingLogsService struct {
	*mockLogsService
	onStreamLogs func()
}

func (t *trackingLogsService) StreamLogs(ctx context.Context, opts logs.LogOptions) error {
	if t.onStreamLogs != nil {
		t.onStreamLogs()
	}
	return t.mockLogsService.StreamLogs(ctx, opts)
}

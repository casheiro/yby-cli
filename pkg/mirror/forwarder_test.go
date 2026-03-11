package mirror

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPortForwarder_Stop_NaoIniciado(t *testing.T) {
	// Criar PortForwarder manualmente (sem kubeconfig) para testar Stop
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
	}

	// Stop quando não iniciado não deve causar panic
	assert.NotPanics(t, func() {
		pf.Stop()
	})

	// Verificar que o canal foi fechado
	select {
	case <-pf.stopCh:
		// OK - canal fechado como esperado
	default:
		t.Error("stopCh deveria estar fechado após Stop()")
	}
}

func TestPortForwarder_CamposPreenchidos(t *testing.T) {
	pf := &PortForwarder{
		Namespace:  "meu-namespace",
		Service:    "meu-servico",
		TargetPort: 8080,
		LocalPort:  3000,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
	}

	assert.Equal(t, "meu-namespace", pf.Namespace)
	assert.Equal(t, "meu-servico", pf.Service)
	assert.Equal(t, 8080, pf.TargetPort)
	assert.Equal(t, 3000, pf.LocalPort)
}

func TestNewPortForwarder_SemKubeconfig(t *testing.T) {
	// Sem kubeconfig configurado, deve falhar
	t.Setenv("KUBECONFIG", "/tmp/kubeconfig-inexistente-teste")

	_, err := NewPortForwarder("test-ns", "git-server", 9418)
	assert.Error(t, err, "deve falhar sem kubeconfig válido")
}

// --- Testes adicionais para MirrorManager ---

func TestSetupTunnel_FalhaCriarPortForwarder(t *testing.T) {
	// SetupTunnel chama NewPortForwarder internamente, que precisa de kubeconfig
	t.Setenv("KUBECONFIG", "/tmp/kubeconfig-inexistente-teste")

	runner := &MockRunner{}
	m := NewManager("/some/path", runner)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := m.SetupTunnel(ctx)
	assert.Error(t, err, "deve falhar quando kubeconfig não está disponível")
	assert.Contains(t, err.Error(), "port forwarder")
}

func TestSync_VerificaURLRemota(t *testing.T) {
	// Verifica que o Sync usa a URL correta baseada na porta local
	var capturedArgs []string
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedArgs = args
			return nil
		},
	}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner, localPort: 54321}

	err := m.Sync()
	assert.NoError(t, err)
	assert.Contains(t, capturedArgs, "git://localhost:54321/repo.git",
		"URL do remote deve conter a porta correta")
}

func TestSync_PropagaErroGitPush(t *testing.T) {
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return errors.New("erro de autenticação")
		},
	}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner, localPort: 12345}

	err := m.Sync()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "git push failed")
}

func TestStartSyncLoop_CancelaAposSyncInicial(t *testing.T) {
	syncCount := 0
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			syncCount++
			return nil
		},
	}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner, localPort: 12345}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelar imediatamente

	m.StartSyncLoop(ctx)
	// Deve ter feito pelo menos o sync inicial
	assert.GreaterOrEqual(t, syncCount, 1, "deve executar pelo menos o sync inicial")
}

func TestStartSyncLoop_SyncInicialComErro(t *testing.T) {
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return errors.New("falha no push")
		},
	}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner, localPort: 12345}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelar imediatamente

	// Não deve causar panic mesmo com erro no sync inicial
	assert.NotPanics(t, func() {
		m.StartSyncLoop(ctx)
	})
}

func TestStartSyncLoop_ParaForwarderAoCancelar(t *testing.T) {
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}

	forwarder := &PortForwarder{
		stopCh:  make(chan struct{}),
		readyCh: make(chan struct{}),
	}

	m := &MirrorManager{
		Namespace: "test-ns",
		Runner:    runner,
		localPort: 12345,
		forwarder: forwarder,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m.StartSyncLoop(ctx)

	// Verifica que o forwarder foi parado
	select {
	case <-forwarder.stopCh:
		// OK - canal fechado como esperado
	default:
		t.Error("forwarder.stopCh deveria estar fechado após cancelamento")
	}
}

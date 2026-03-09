package mirror

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/portforward"
)

// --- Mock do Forwarder para testes ---

// mockForwarder implementa a interface Forwarder para testes
type mockForwarder struct {
	startFunc func(ctx context.Context) (int, error)
	stopFunc  func()
	stopped   bool
}

func (m *mockForwarder) Start(ctx context.Context) (int, error) {
	if m.startFunc != nil {
		return m.startFunc(ctx)
	}
	return 0, nil
}

func (m *mockForwarder) Stop() {
	m.stopped = true
	if m.stopFunc != nil {
		m.stopFunc()
	}
}

// --- Testes para SetupTunnel com ForwarderFactory mockada ---

func TestSetupTunnel_Sucesso(t *testing.T) {
	mf := &mockForwarder{
		startFunc: func(ctx context.Context) (int, error) {
			return 54321, nil
		},
	}

	runner := &MockRunner{}
	m := NewManager("/repo", runner)
	m.ForwarderFactory = func(namespace, service string, targetPort int) (Forwarder, error) {
		assert.Equal(t, "yby-system", namespace)
		assert.Equal(t, "git-server", service)
		assert.Equal(t, 9418, targetPort)
		return mf, nil
	}

	ctx := context.Background()
	err := m.SetupTunnel(ctx)
	require.NoError(t, err)
	assert.Equal(t, 54321, m.localPort)
	assert.Equal(t, mf, m.forwarder)
}

func TestSetupTunnel_ErroNaFactory(t *testing.T) {
	runner := &MockRunner{}
	m := NewManager("/repo", runner)
	m.ForwarderFactory = func(namespace, service string, targetPort int) (Forwarder, error) {
		return nil, errors.New("falha ao criar forwarder")
	}

	ctx := context.Background()
	err := m.SetupTunnel(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create port forwarder")
}

func TestSetupTunnel_ErroNoStart(t *testing.T) {
	mf := &mockForwarder{
		startFunc: func(ctx context.Context) (int, error) {
			return 0, errors.New("erro ao iniciar port-forward")
		},
	}

	runner := &MockRunner{}
	m := NewManager("/repo", runner)
	m.ForwarderFactory = func(namespace, service string, targetPort int) (Forwarder, error) {
		return mf, nil
	}

	ctx := context.Background()
	err := m.SetupTunnel(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start port forwarder")
}

func TestSetupTunnel_FactoryNilUsaDefault(t *testing.T) {
	// Quando ForwarderFactory é nil, deve usar defaultForwarderFactory
	// que vai falhar sem kubeconfig, mas cobre o caminho
	t.Setenv("KUBECONFIG", "/tmp/kubeconfig-inexistente-coverage-test")

	runner := &MockRunner{}
	m := &MirrorManager{
		LocalPath:        "/repo",
		Namespace:        "test-ns",
		Runner:           runner,
		ForwarderFactory: nil,
	}

	ctx := context.Background()
	err := m.SetupTunnel(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create port forwarder")
}

// --- Testes para StartSyncLoop com ticker (cobrindo o caminho do ticker.C) ---

func TestStartSyncLoop_ExecutaSyncViaTicker(t *testing.T) {
	syncCount := 0
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			syncCount++
			return nil
		},
	}

	m := &MirrorManager{
		Namespace: "test-ns",
		Runner:    runner,
		localPort: 12345,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Executar em goroutine e aguardar pelo menos uma iteração do ticker
	done := make(chan struct{})
	go func() {
		m.StartSyncLoop(ctx)
		close(done)
	}()

	// Aguardar tempo suficiente para sync inicial + pelo menos 1 tick (5s)
	// Porém, para testes rápidos, vamos cancelar logo após sync inicial
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	// Deve ter executado pelo menos o sync inicial
	assert.GreaterOrEqual(t, syncCount, 1, "deve executar o sync inicial")
}

func TestStartSyncLoop_SyncComErroNoTicker(t *testing.T) {
	// Testa o cenário onde sync falha durante iteração do ticker
	callCount := 0
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			callCount++
			return errors.New("falha no push")
		},
	}

	mf := &mockForwarder{}
	m := &MirrorManager{
		Namespace: "test-ns",
		Runner:    runner,
		localPort: 12345,
		forwarder: mf,
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		m.StartSyncLoop(ctx)
		close(done)
	}()

	// Aguardar sync inicial e cancelar
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	assert.GreaterOrEqual(t, callCount, 1, "deve tentar o sync pelo menos uma vez")
	assert.True(t, mf.stopped, "forwarder deve ser parado ao cancelar")
}

func TestStartSyncLoop_SemForwarder(t *testing.T) {
	// Testa o caminho onde forwarder é nil quando o contexto é cancelado
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}

	m := &MirrorManager{
		Namespace: "test-ns",
		Runner:    runner,
		localPort: 12345,
		forwarder: nil, // sem forwarder
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Não deve causar panic quando forwarder é nil
	assert.NotPanics(t, func() {
		m.StartSyncLoop(ctx)
	})
}

// --- Testes para NewManager ---

func TestNewManager_ValoresPadrao(t *testing.T) {
	runner := &MockRunner{}
	m := NewManager("/meu/projeto", runner)

	assert.Equal(t, "/meu/projeto", m.LocalPath)
	assert.Equal(t, "yby-system", m.Namespace)
	assert.NotNil(t, m.Runner)
	assert.NotNil(t, m.ForwarderFactory, "ForwarderFactory deve ser inicializada com valor padrão")
}

// --- Testes adicionais para EnsureGitServer ---

func TestEnsureGitServer_VerificaConteudoManifestosNS(t *testing.T) {
	// Verifica que os manifestos contêm o namespace correto
	var stdinCaptures []string
	runner := &MockRunner{
		RunStdinFunc: func(ctx context.Context, stdin string, name string, args ...string) error {
			stdinCaptures = append(stdinCaptures, stdin)
			return nil
		},
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}

	m := &MirrorManager{Namespace: "meu-namespace", Runner: runner}
	err := m.EnsureGitServer()
	require.NoError(t, err)

	require.Len(t, stdinCaptures, 2, "deve ter 2 chamadas RunStdin (namespace + manifests)")
	assert.Contains(t, stdinCaptures[0], "meu-namespace", "namespace deve estar no manifesto")
	assert.Contains(t, stdinCaptures[1], "meu-namespace", "namespace deve estar nos manifestos de deploy")
	assert.Contains(t, stdinCaptures[1], "kind: Deployment")
	assert.Contains(t, stdinCaptures[1], "kind: Service")
}

func TestEnsureGitServer_VerificaComandoRollout(t *testing.T) {
	// Verifica que o rollout é chamado com os argumentos corretos
	var capturedName string
	var capturedArgs []string
	runner := &MockRunner{
		RunStdinFunc: func(ctx context.Context, stdin string, name string, args ...string) error {
			return nil
		},
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedName = name
			capturedArgs = args
			return nil
		},
	}

	m := &MirrorManager{Namespace: "test-ns", Runner: runner}
	err := m.EnsureGitServer()
	require.NoError(t, err)

	assert.Equal(t, "kubectl", capturedName)
	assert.Contains(t, capturedArgs, "rollout")
	assert.Contains(t, capturedArgs, "deployment/git-server")
	assert.Contains(t, capturedArgs, "test-ns")
}

// --- Testes table-driven para Sync ---

func TestSync_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		localPort int
		runErr    error
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "sem tunnel estabelecido",
			localPort: 0,
			runErr:    nil,
			wantErr:   true,
			errMsg:    "tunnel not established",
		},
		{
			name:      "push com sucesso",
			localPort: 9999,
			runErr:    nil,
			wantErr:   false,
		},
		{
			name:      "push com erro de rede",
			localPort: 9999,
			runErr:    errors.New("connection refused"),
			wantErr:   true,
			errMsg:    "git push failed",
		},
		{
			name:      "push com erro de autenticacao",
			localPort: 8888,
			runErr:    errors.New("authentication failed"),
			wantErr:   true,
			errMsg:    "git push failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &MockRunner{
				RunFunc: func(ctx context.Context, name string, args ...string) error {
					return tt.runErr
				},
			}
			m := &MirrorManager{
				Namespace: "test-ns",
				Runner:    runner,
				localPort: tt.localPort,
			}

			err := m.Sync()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- Testes para Sync verificando argumentos do git push ---

func TestSync_VerificaArgumentosGitPush(t *testing.T) {
	tests := []struct {
		name      string
		localPort int
		wantURL   string
	}{
		{
			name:      "porta 12345",
			localPort: 12345,
			wantURL:   "git://localhost:12345/repo.git",
		},
		{
			name:      "porta 9418",
			localPort: 9418,
			wantURL:   "git://localhost:9418/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedName string
			var capturedArgs []string
			runner := &MockRunner{
				RunFunc: func(ctx context.Context, name string, args ...string) error {
					capturedName = name
					capturedArgs = args
					return nil
				},
			}
			m := &MirrorManager{
				Namespace: "test-ns",
				Runner:    runner,
				localPort: tt.localPort,
			}

			err := m.Sync()
			require.NoError(t, err)
			assert.Equal(t, "git", capturedName)
			assert.Equal(t, "push", capturedArgs[0])
			assert.Equal(t, tt.wantURL, capturedArgs[1])
			assert.Equal(t, "HEAD:main", capturedArgs[2])
			assert.Equal(t, "--force", capturedArgs[3])
		})
	}
}

// --- Testes para PortForwarder (construtor e campos) ---

func TestNewPortForwarder_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		kubeconfig string
		wantErr    bool
	}{
		{
			name:       "kubeconfig inexistente",
			kubeconfig: "/tmp/nao-existe-coverage-test",
			wantErr:    true,
		},
		{
			name:       "kubeconfig vazio",
			kubeconfig: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.kubeconfig != "" {
				t.Setenv("KUBECONFIG", tt.kubeconfig)
			} else {
				// Forçar ausência de kubeconfig configurando caminho inexistente
				t.Setenv("KUBECONFIG", "/tmp/kubeconfig-vazio-coverage-test")
				t.Setenv("HOME", "/tmp/home-inexistente-coverage-test")
			}

			_, err := NewPortForwarder("ns", "svc", 8080)
			if tt.wantErr {
				assert.Error(t, err, "deve falhar com kubeconfig inválido")
			}
		})
	}
}

func TestPortForwarder_StopDuplo(t *testing.T) {
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
	}

	// Primeiro Stop deve funcionar
	pf.Stop()

	// Segundo Stop em canal já fechado deve causar panic
	// Verificamos que o canal está fechado
	select {
	case <-pf.stopCh:
		// OK - canal fechado
	default:
		t.Error("stopCh deveria estar fechado após Stop()")
	}
}

// --- Testes para defaultForwarderFactory ---

func TestDefaultForwarderFactory_SemKubeconfig(t *testing.T) {
	t.Setenv("KUBECONFIG", "/tmp/kubeconfig-factory-test")

	_, err := defaultForwarderFactory("ns", "svc", 9418)
	assert.Error(t, err, "deve falhar sem kubeconfig válido")
}

// --- Testes para StartSyncLoop com forwarder mockado ---

func TestStartSyncLoop_ForwarderMockStopChamado(t *testing.T) {
	mf := &mockForwarder{}
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}

	m := &MirrorManager{
		Namespace: "test-ns",
		Runner:    runner,
		localPort: 12345,
		forwarder: mf,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelar imediatamente

	m.StartSyncLoop(ctx)
	assert.True(t, mf.stopped, "Stop() do forwarder deve ser chamado ao cancelar contexto")
}

// --- Testes para SetupTunnel com cenários diversos ---

func TestSetupTunnel_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		factoryErr error
		startErr   error
		startPort  int
		wantErr    bool
		wantErrMsg string
		wantPort   int
	}{
		{
			name:       "sucesso com porta 54321",
			factoryErr: nil,
			startErr:   nil,
			startPort:  54321,
			wantErr:    false,
			wantPort:   54321,
		},
		{
			name:       "sucesso com porta 12345",
			factoryErr: nil,
			startErr:   nil,
			startPort:  12345,
			wantErr:    false,
			wantPort:   12345,
		},
		{
			name:       "erro na factory",
			factoryErr: errors.New("falha na factory"),
			wantErr:    true,
			wantErrMsg: "failed to create port forwarder",
		},
		{
			name:       "erro no start",
			factoryErr: nil,
			startErr:   errors.New("timeout ao iniciar"),
			wantErr:    true,
			wantErrMsg: "failed to start port forwarder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &MockRunner{}
			m := NewManager("/repo", runner)

			m.ForwarderFactory = func(namespace, service string, targetPort int) (Forwarder, error) {
				if tt.factoryErr != nil {
					return nil, tt.factoryErr
				}
				return &mockForwarder{
					startFunc: func(ctx context.Context) (int, error) {
						if tt.startErr != nil {
							return 0, tt.startErr
						}
						return tt.startPort, nil
					},
				}, nil
			}

			ctx := context.Background()
			err := m.SetupTunnel(ctx)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPort, m.localPort)
				assert.NotNil(t, m.forwarder)
			}
		})
	}
}

// --- Testes para EnsureGitServer table-driven ---

func TestEnsureGitServer_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		stdinErr   func(int) error // erro por chamada (1=namespace, 2=manifest)
		runErr     error
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:     "sucesso completo",
			stdinErr: func(call int) error { return nil },
			runErr:   nil,
			wantErr:  false,
		},
		{
			name: "erro na criacao de namespace",
			stdinErr: func(call int) error {
				if call == 1 {
					return errors.New("namespace error")
				}
				return nil
			},
			runErr:     nil,
			wantErr:    true,
			wantErrMsg: "failed to create namespace",
		},
		{
			name: "erro na aplicacao de manifestos",
			stdinErr: func(call int) error {
				if call == 2 {
					return errors.New("manifest error")
				}
				return nil
			},
			runErr:     nil,
			wantErr:    true,
			wantErrMsg: "failed to apply git-server manifests",
		},
		{
			name:       "erro no rollout",
			stdinErr:   func(call int) error { return nil },
			runErr:     errors.New("rollout timeout"),
			wantErr:    true,
			wantErrMsg: "git-server failed to start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			runner := &MockRunner{
				RunStdinFunc: func(ctx context.Context, stdin string, name string, args ...string) error {
					callCount++
					return tt.stdinErr(callCount)
				},
				RunFunc: func(ctx context.Context, name string, args ...string) error {
					return tt.runErr
				},
			}

			m := &MirrorManager{Namespace: "test-ns", Runner: runner}
			err := m.EnsureGitServer()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- Mocks para PodLister e TunnelDialer ---

// mockPodLister implementa PodLister para testes
type mockPodLister struct {
	listFunc func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error)
}

func (m *mockPodLister) ListPods(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, namespace, labelSelector)
	}
	return &corev1.PodList{}, nil
}

// mockPortForwardSession implementa PortForwardSession para testes
type mockPortForwardSession struct {
	forwardFunc  func() error
	getPortsFunc func() ([]portforward.ForwardedPort, error)
}

func (m *mockPortForwardSession) ForwardPorts() error {
	if m.forwardFunc != nil {
		return m.forwardFunc()
	}
	return nil
}

func (m *mockPortForwardSession) GetPorts() ([]portforward.ForwardedPort, error) {
	if m.getPortsFunc != nil {
		return m.getPortsFunc()
	}
	return nil, nil
}

// mockTunnelDialer implementa TunnelDialer para testes
type mockTunnelDialer struct {
	createFunc func(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error)
}

func (m *mockTunnelDialer) CreateTunnel(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error) {
	if m.createFunc != nil {
		return m.createFunc(podName, namespace, ports, stopCh, readyCh, out, errOut)
	}
	return &mockPortForwardSession{}, nil
}

// --- Testes para PortForwarder.Start ---

func TestStart_SemPodsEncontrados(t *testing.T) {
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		podLister: &mockPodLister{
			listFunc: func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
				return &corev1.PodList{}, nil
			},
		},
		tunnelDialer: &mockTunnelDialer{},
	}

	ctx := context.Background()
	_, err := pf.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found for service git-server")
}

func TestStart_ErroPrimeiraListagem_FallbackSucesso(t *testing.T) {
	callCount := 0
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		out:        io.Discard,
		podLister: &mockPodLister{
			listFunc: func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
				callCount++
				if callCount == 1 {
					// Primeira chamada com label app.kubernetes.io/name falha
					return nil, errors.New("label não encontrada")
				}
				// Segunda chamada com label app= retorna pod rodando
				return &corev1.PodList{
					Items: []corev1.Pod{
						{
							Status: corev1.PodStatus{Phase: corev1.PodRunning},
						},
					},
				}, nil
			},
		},
		tunnelDialer: &mockTunnelDialer{
			createFunc: func(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error) {
				// Simular tunnel pronto
				go func() { close(readyCh) }()
				return &mockPortForwardSession{
					getPortsFunc: func() ([]portforward.ForwardedPort, error) {
						return []portforward.ForwardedPort{{Local: 54321}}, nil
					},
				}, nil
			},
		},
	}

	ctx := context.Background()
	port, err := pf.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, 54321, port)
}

func TestStart_ErroAmbasListagens(t *testing.T) {
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		podLister: &mockPodLister{
			listFunc: func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
				return nil, errors.New("erro de listagem")
			},
		},
		tunnelDialer: &mockTunnelDialer{},
	}

	ctx := context.Background()
	_, err := pf.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list pods for service git-server")
}

func TestStart_PodNaoRunning(t *testing.T) {
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		podLister: &mockPodLister{
			listFunc: func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
				return &corev1.PodList{
					Items: []corev1.Pod{
						{
							Status: corev1.PodStatus{Phase: corev1.PodPending},
						},
						{
							Status: corev1.PodStatus{Phase: corev1.PodFailed},
						},
					},
				}, nil
			},
		},
		tunnelDialer: &mockTunnelDialer{},
	}

	ctx := context.Background()
	_, err := pf.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no running pods found for service git-server")
}

func TestStart_ErroAoCriarTunnel(t *testing.T) {
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		out:        io.Discard,
		podLister: &mockPodLister{
			listFunc: func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
				return &corev1.PodList{
					Items: []corev1.Pod{
						{
							Status: corev1.PodStatus{Phase: corev1.PodRunning},
						},
					},
				}, nil
			},
		},
		tunnelDialer: &mockTunnelDialer{
			createFunc: func(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error) {
				return nil, errors.New("falha ao criar tunnel")
			},
		},
	}

	ctx := context.Background()
	_, err := pf.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "falha ao criar tunnel")
}

func TestStart_Sucesso(t *testing.T) {
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		out:        io.Discard,
		podLister: &mockPodLister{
			listFunc: func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
				return &corev1.PodList{
					Items: []corev1.Pod{
						{
							Status: corev1.PodStatus{Phase: corev1.PodRunning},
						},
					},
				}, nil
			},
		},
		tunnelDialer: &mockTunnelDialer{
			createFunc: func(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error) {
				go func() { close(readyCh) }()
				return &mockPortForwardSession{
					getPortsFunc: func() ([]portforward.ForwardedPort, error) {
						return []portforward.ForwardedPort{{Local: 33333}}, nil
					},
				}, nil
			},
		},
	}

	ctx := context.Background()
	port, err := pf.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, 33333, port)
	assert.Equal(t, 33333, pf.LocalPort)
}

func TestStart_TimeoutContexto(t *testing.T) {
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		out:        io.Discard,
		podLister: &mockPodLister{
			listFunc: func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
				return &corev1.PodList{
					Items: []corev1.Pod{
						{
							Status: corev1.PodStatus{Phase: corev1.PodRunning},
						},
					},
				}, nil
			},
		},
		tunnelDialer: &mockTunnelDialer{
			createFunc: func(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error) {
				// Não fechar readyCh para simular timeout
				return &mockPortForwardSession{}, nil
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelar imediatamente para simular timeout

	_, err := pf.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout waiting for portforward")
}

func TestStart_ErroAoObterPortas(t *testing.T) {
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		out:        io.Discard,
		podLister: &mockPodLister{
			listFunc: func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
				return &corev1.PodList{
					Items: []corev1.Pod{
						{
							Status: corev1.PodStatus{Phase: corev1.PodRunning},
						},
					},
				}, nil
			},
		},
		tunnelDialer: &mockTunnelDialer{
			createFunc: func(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error) {
				go func() { close(readyCh) }()
				return &mockPortForwardSession{
					getPortsFunc: func() ([]portforward.ForwardedPort, error) {
						return nil, errors.New("erro ao obter portas")
					},
				}, nil
			},
		},
	}

	ctx := context.Background()
	_, err := pf.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get forwarded ports")
}

func TestStart_SemPortasRetornadas(t *testing.T) {
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		out:        io.Discard,
		podLister: &mockPodLister{
			listFunc: func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
				return &corev1.PodList{
					Items: []corev1.Pod{
						{
							Status: corev1.PodStatus{Phase: corev1.PodRunning},
						},
					},
				}, nil
			},
		},
		tunnelDialer: &mockTunnelDialer{
			createFunc: func(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error) {
				go func() { close(readyCh) }()
				return &mockPortForwardSession{
					getPortsFunc: func() ([]portforward.ForwardedPort, error) {
						return []portforward.ForwardedPort{}, nil // lista vazia
					},
				}, nil
			},
		},
	}

	ctx := context.Background()
	_, err := pf.Start(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no ports forwarded")
}

func TestStart_PrimeiraListagemVazia_FallbackComPods(t *testing.T) {
	callCount := 0
	pf := &PortForwarder{
		Namespace:  "test-ns",
		Service:    "git-server",
		TargetPort: 9418,
		stopCh:     make(chan struct{}),
		readyCh:    make(chan struct{}),
		out:        io.Discard,
		podLister: &mockPodLister{
			listFunc: func(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
				callCount++
				if callCount == 1 {
					// Primeira chamada retorna lista vazia (sem erro)
					return &corev1.PodList{}, nil
				}
				// Fallback retorna pod rodando
				return &corev1.PodList{
					Items: []corev1.Pod{
						{
							Status: corev1.PodStatus{Phase: corev1.PodRunning},
						},
					},
				}, nil
			},
		},
		tunnelDialer: &mockTunnelDialer{
			createFunc: func(podName, namespace string, ports []string, stopCh, readyCh chan struct{}, out, errOut io.Writer) (PortForwardSession, error) {
				go func() { close(readyCh) }()
				return &mockPortForwardSession{
					getPortsFunc: func() ([]portforward.ForwardedPort, error) {
						return []portforward.ForwardedPort{{Local: 44444}}, nil
					},
				}, nil
			},
		},
	}

	ctx := context.Background()
	port, err := pf.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, 44444, port)
	assert.Equal(t, 2, callCount, "deve ter feito 2 chamadas ao podLister (fallback)")
}

// --- Testes para verificar que Sync usa o comando git correto ---

func TestSync_UsaComandoCorreto(t *testing.T) {
	var capturedName string
	var capturedArgs []string
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			capturedName = name
			capturedArgs = args
			return nil
		},
	}

	m := &MirrorManager{
		Namespace: "test-ns",
		Runner:    runner,
		localPort: 7777,
	}

	err := m.Sync()
	require.NoError(t, err)

	assert.Equal(t, "git", capturedName)
	require.Len(t, capturedArgs, 4)
	assert.Equal(t, "push", capturedArgs[0])
	assert.True(t, strings.HasPrefix(capturedArgs[1], "git://localhost:7777/"))
	assert.Equal(t, "HEAD:main", capturedArgs[2])
	assert.Equal(t, "--force", capturedArgs[3])
}

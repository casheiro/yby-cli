package cmd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/pkg/services/bootstrap"
	"github.com/casheiro/yby-cli/pkg/services/environment"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/casheiro/yby-cli/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

// ========================================================
// Mocks para testes de up
// ========================================================

type mockClusterManager struct {
	existsFunc func(ctx context.Context, name string) (bool, error)
	createFunc func(ctx context.Context, name string, configFile string) error
	startFunc  func(ctx context.Context, name string) error
	deleteFunc func(ctx context.Context, name string) error
}

func (m *mockClusterManager) Exists(ctx context.Context, name string) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, name)
	}
	return true, nil
}

func (m *mockClusterManager) Create(ctx context.Context, name string, configFile string) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, name, configFile)
	}
	return nil
}

func (m *mockClusterManager) Start(ctx context.Context, name string) error {
	if m.startFunc != nil {
		return m.startFunc(ctx, name)
	}
	return nil
}

func (m *mockClusterManager) Delete(ctx context.Context, name string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, name)
	}
	return nil
}

type mockMirrorService struct {
	ensureGitServerFunc func() error
	setupTunnelFunc     func(ctx context.Context) error
	syncFunc            func() error
	startSyncLoopFunc   func(ctx context.Context)
}

func (m *mockMirrorService) EnsureGitServer() error {
	if m.ensureGitServerFunc != nil {
		return m.ensureGitServerFunc()
	}
	return nil
}

func (m *mockMirrorService) SetupTunnel(ctx context.Context) error {
	if m.setupTunnelFunc != nil {
		return m.setupTunnelFunc(ctx)
	}
	return nil
}

func (m *mockMirrorService) Sync() error {
	if m.syncFunc != nil {
		return m.syncFunc()
	}
	return nil
}

func (m *mockMirrorService) StartSyncLoop(ctx context.Context) {
	if m.startSyncLoopFunc != nil {
		m.startSyncLoopFunc(ctx)
	}
}

type mockBootstrapService struct {
	runFunc func(ctx context.Context, opts bootstrap.BootstrapOptions) error
}

func (m *mockBootstrapService) Run(ctx context.Context, opts bootstrap.BootstrapOptions) error {
	if m.runFunc != nil {
		return m.runFunc(ctx, opts)
	}
	return nil
}

// newUpMockRunner cria um MockRunner que simula sucesso em todas as operações
func newUpMockRunner() shared.Runner {
	return &testutil.MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
		RunCombinedOutputFunc: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("ok"), nil
		},
		LookPathFunc: func(file string) (string, error) {
			return "/usr/bin/" + file, nil
		},
	}
}

// newUpMockFs cria um MockFilesystem que simula sucesso em todas as operações
func newUpMockFs() shared.Filesystem {
	return &testutil.MockFilesystem{}
}

// ========================================================
// Testes de runLocalUp
// ========================================================

func TestRunLocalUp_ErroNoLookPath(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	orig := newLocalEnvironmentService
	defer func() { newLocalEnvironmentService = orig }()

	// Cria um mock que retorna serviço com Runner que falha no LookPath
	newLocalEnvironmentService = func(root string) *environment.EnvironmentService {
		mockRunner := &testutil.MockRunner{
			LookPathFunc: func(file string) (string, error) {
				return "", fmt.Errorf("k3d não encontrado")
			},
		}
		cluster := &mockClusterManager{}
		mirror := &mockMirrorService{}
		bs := &mockBootstrapService{}

		return environment.NewEnvironmentService(mockRunner, newUpMockFs(), cluster, mirror, bs)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runLocalUp(ctx, "/tmp/test-root")
	assert.Error(t, err, "deveria retornar erro quando k3d não está disponível")
}

func TestRunLocalUp_ClusterExisteStartFalha(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	orig := newLocalEnvironmentService
	defer func() { newLocalEnvironmentService = orig }()

	newLocalEnvironmentService = func(root string) *environment.EnvironmentService {
		cluster := &mockClusterManager{
			existsFunc: func(ctx context.Context, name string) (bool, error) {
				return true, nil
			},
			startFunc: func(ctx context.Context, name string) error {
				return fmt.Errorf("falha ao iniciar cluster")
			},
		}
		mirror := &mockMirrorService{}
		bs := &mockBootstrapService{}

		return environment.NewEnvironmentService(newUpMockRunner(), newUpMockFs(), cluster, mirror, bs)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runLocalUp(ctx, ".")
	assert.Error(t, err, "deveria retornar erro quando cluster.Start falha")
}

func TestRunLocalUp_ClusterNaoExisteCriacaoFalha(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	orig := newLocalEnvironmentService
	defer func() { newLocalEnvironmentService = orig }()

	t.Setenv("YBY_CLUSTER_NAME", "meu-cluster-custom")

	newLocalEnvironmentService = func(root string) *environment.EnvironmentService {
		cluster := &mockClusterManager{
			existsFunc: func(ctx context.Context, name string) (bool, error) {
				return false, nil
			},
			createFunc: func(ctx context.Context, name string, configFile string) error {
				return fmt.Errorf("falha ao criar cluster")
			},
		}
		mirror := &mockMirrorService{}
		bs := &mockBootstrapService{}

		return environment.NewEnvironmentService(newUpMockRunner(), newUpMockFs(), cluster, mirror, bs)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runLocalUp(ctx, ".")
	assert.Error(t, err, "deveria retornar erro quando cluster.Create falha")
}

// ========================================================
// Teste de runLocalUp com sucesso completo
// ========================================================

func TestRunLocalUp_Sucesso(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	orig := newLocalEnvironmentService
	defer func() { newLocalEnvironmentService = orig }()

	newLocalEnvironmentService = func(root string) *environment.EnvironmentService {
		cluster := &mockClusterManager{
			existsFunc: func(ctx context.Context, name string) (bool, error) {
				return true, nil
			},
			startFunc: func(ctx context.Context, name string) error {
				return nil
			},
		}
		mirror := &mockMirrorService{
			ensureGitServerFunc: func() error { return nil },
			setupTunnelFunc:     func(ctx context.Context) error { return nil },
			syncFunc:            func() error { return nil },
			startSyncLoopFunc:   func(ctx context.Context) {},
		}
		bs := &mockBootstrapService{
			runFunc: func(ctx context.Context, opts bootstrap.BootstrapOptions) error { return nil },
		}
		return environment.NewEnvironmentService(newUpMockRunner(), newUpMockFs(), cluster, mirror, bs)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancela após um curto intervalo para que <-ctx.Done() retorne
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := runLocalUp(ctx, t.TempDir())
	assert.NoError(t, err, "runLocalUp deveria completar com sucesso")
}

func TestRunLocalUp_Sucesso_ComClusterName(t *testing.T) {
	teardown := mockExecCommand()
	defer teardown()

	t.Setenv("YBY_CLUSTER_NAME", "custom-cluster")

	orig := newLocalEnvironmentService
	defer func() { newLocalEnvironmentService = orig }()

	var clusterNameCapturado string
	newLocalEnvironmentService = func(root string) *environment.EnvironmentService {
		cluster := &mockClusterManager{
			existsFunc: func(ctx context.Context, name string) (bool, error) {
				clusterNameCapturado = name
				return true, nil
			},
			startFunc: func(ctx context.Context, name string) error {
				return nil
			},
		}
		mirror := &mockMirrorService{
			ensureGitServerFunc: func() error { return nil },
			setupTunnelFunc:     func(ctx context.Context) error { return nil },
			syncFunc:            func() error { return nil },
			startSyncLoopFunc:   func(ctx context.Context) {},
		}
		bs := &mockBootstrapService{
			runFunc: func(ctx context.Context, opts bootstrap.BootstrapOptions) error { return nil },
		}
		return environment.NewEnvironmentService(newUpMockRunner(), newUpMockFs(), cluster, mirror, bs)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := runLocalUp(ctx, t.TempDir())
	assert.NoError(t, err)
	assert.Equal(t, "custom-cluster", clusterNameCapturado,
		"deveria usar o nome de cluster da variável YBY_CLUSTER_NAME")
}

// ========================================================
// Teste da factory newLocalEnvironmentService
// ========================================================

func TestNewLocalEnvironmentServiceFactory_Default(t *testing.T) {
	svc := newLocalEnvironmentService("/tmp/test")
	assert.NotNil(t, svc, "newLocalEnvironmentService deveria retornar um serviço não-nil")
}

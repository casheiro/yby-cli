package mirror

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sync"
	"time"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/casheiro/yby-cli/pkg/retry"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/charmbracelet/lipgloss"
)

//go:embed manifests/*.yaml
var manifestsFS embed.FS

var (
	stepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // Blue
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // Red
)

// Forwarder define a interface para port-forwarding de pods K8s
type Forwarder interface {
	Start(ctx context.Context) (int, error)
	Stop()
	HealthCheck(ctx context.Context) error
}

// ForwarderFactory cria instâncias de Forwarder
type ForwarderFactory func(namespace, service string, targetPort int) (Forwarder, error)

// defaultForwarderFactory usa NewPortForwarder real
func defaultForwarderFactory(namespace, service string, targetPort int) (Forwarder, error) {
	return NewPortForwarder(namespace, service, targetPort)
}

// MirrorManager handles the in-cluster Git mirror for Hybrid GitOps
type MirrorManager struct {
	LocalPath string // Path to the local git repo (e.g. ".")
	Namespace string
	Runner    shared.Runner

	// ForwarderFactory permite injetar factory customizada para testes
	ForwarderFactory ForwarderFactory

	// mu protege acesso concorrente a forwarder e localPort
	mu sync.Mutex

	// Forwarder instance for local access
	forwarder Forwarder
	localPort int

	// healthInterval define o intervalo entre verificações de saúde (padrão 10s)
	healthInterval time.Duration
}

func NewManager(localPath string, runner shared.Runner) *MirrorManager {
	return &MirrorManager{
		LocalPath:        localPath,
		Namespace:        "yby-system",
		Runner:           runner,
		ForwarderFactory: defaultForwarderFactory,
	}
}

// EnsureGitServer deploys the git-server to the cluster if not present
func (m *MirrorManager) EnsureGitServer() error {
	// 1. Create Namespace
	nsTpl, err := manifestsFS.ReadFile("manifests/namespace.yaml")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "failed to read namespace manifest")
	}
	nsManifest := fmt.Sprintf(string(nsTpl), m.Namespace)

	if err := m.Runner.RunStdin(context.Background(), nsManifest, "kubectl", "apply", "-f", "-"); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "failed to create namespace")
	}

	// 2. Apply Manifests (Deployment + Service)
	serverTpl, err := manifestsFS.ReadFile("manifests/git-server.yaml")
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeIO, "failed to read git-server manifest")
	}
	manifest := fmt.Sprintf(string(serverTpl), m.Namespace, m.Namespace)

	if err := m.Runner.RunStdin(context.Background(), manifest, "kubectl", "apply", "-f", "-"); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "failed to apply git-server manifests")
	}

	// Wait for rollout
	if err := m.Runner.Run(context.Background(), "kubectl", "rollout", "status", "deployment/git-server", "-n", m.Namespace, "--timeout=60s"); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "git-server failed to start")
	}

	return nil
}

// SetupTunnel establishes the port-forward tunnel
func (m *MirrorManager) SetupTunnel(ctx context.Context) error {
	factory := m.ForwarderFactory
	if factory == nil {
		factory = defaultForwarderFactory
	}

	pf, err := factory(m.Namespace, "git-server", 9418)
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodePortForward, "failed to create port forwarder")
	}

	port, err := pf.Start(ctx)
	if err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodePortForward, "failed to start port forwarder")
	}

	m.mu.Lock()
	m.forwarder = pf
	m.localPort = port
	m.mu.Unlock()

	fmt.Printf("📡 Tunnel established: localhost:%d -> git-server:9418\n", port)
	return nil
}

// Sync pushes local changes to the in-cluster git server via the tunnel
func (m *MirrorManager) Sync() error {
	m.mu.Lock()
	port := m.localPort
	m.mu.Unlock()

	if port == 0 {
		return ybyerrors.New(ybyerrors.ErrCodePortForward, "tunnel not established. Call SetupTunnel() first")
	}

	remoteURL := fmt.Sprintf("git://localhost:%d/repo.git", port)

	if err := m.Runner.Run(context.Background(), "git", "push", remoteURL, "HEAD:main", "--force"); err != nil {
		return ybyerrors.Wrap(err, ybyerrors.ErrCodeExec, "git push failed")
	}

	return nil
}

// reconnect reconecta o port-forward com retry exponencial
func (m *MirrorManager) reconnect(ctx context.Context) error {
	fmt.Println(stepStyle.Render("🔄 Reconectando port-forward..."))
	slog.Info("iniciando reconexão do port-forward", "namespace", m.Namespace)

	m.mu.Lock()
	if m.forwarder != nil {
		m.forwarder.Stop()
		m.forwarder = nil
		m.localPort = 0
	}
	m.mu.Unlock()

	return retry.Do(ctx, retry.Options{
		InitialInterval:     2 * time.Second,
		MaxInterval:         30 * time.Second,
		MaxElapsedTime:      5 * time.Minute,
		RandomizationFactor: 0.3,
		Multiplier:          2.0,
	}, func() error {
		err := m.SetupTunnel(ctx)
		if err != nil {
			slog.Warn("tentativa de reconexão falhou", "error", err)
			fmt.Println(errorStyle.Render(fmt.Sprintf("⚠️  Tentativa de reconexão falhou: %v", err)))
		}
		return err
	})
}

// backoffDelay calcula o delay com backoff exponencial para erros consecutivos de sync
func (m *MirrorManager) backoffDelay(errs int) time.Duration {
	base := 5 * time.Second
	shift := errs
	if shift > 5 {
		shift = 5
	}
	delay := base * time.Duration(1<<shift)
	if delay > 3*time.Minute {
		delay = 3 * time.Minute
	}
	return delay
}

// StartSyncLoop monitora mudanças e sincroniza automaticamente com health check e backoff
func (m *MirrorManager) StartSyncLoop(ctx context.Context) {
	syncInterval := 5 * time.Second
	healthInterval := m.healthInterval
	if healthInterval == 0 {
		healthInterval = 10 * time.Second
	}

	syncTicker := time.NewTicker(syncInterval)
	healthTicker := time.NewTicker(healthInterval)
	defer syncTicker.Stop()
	defer healthTicker.Stop()

	fmt.Println(stepStyle.Render("🔄 Auto-sync habilitado..."))

	// Sync inicial
	if err := m.Sync(); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Sync inicial falhou: %v", err)))
	} else {
		fmt.Println(successStyle.Render("✅ Sync inicial completo"))
	}

	var consecutiveErrs int
	for {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			if m.forwarder != nil {
				m.forwarder.Stop()
			}
			m.mu.Unlock()
			return

		case <-healthTicker.C:
			m.mu.Lock()
			fwd := m.forwarder
			m.mu.Unlock()
			if fwd != nil {
				if err := fwd.HealthCheck(ctx); err != nil {
					slog.Warn("health check falhou, iniciando reconexão", "error", err)
					fmt.Println(errorStyle.Render("⚠️  Conexão perdida, reconectando..."))
					if err := m.reconnect(ctx); err != nil {
						slog.Error("reconexão falhou definitivamente", "error", err)
						fmt.Println(errorStyle.Render("❌ Reconexão falhou: " + err.Error()))
					} else {
						fmt.Println(successStyle.Render("✅ Reconectado com sucesso"))
						consecutiveErrs = 0
					}
				}
			}

		case <-syncTicker.C:
			if err := m.Sync(); err != nil {
				consecutiveErrs++
				delay := m.backoffDelay(consecutiveErrs)
				slog.Warn("sync falhou", "erros_consecutivos", consecutiveErrs, "proximo_em", delay, "error", err)
				fmt.Println(errorStyle.Render(fmt.Sprintf("⚠️ Erro de sync (tentativa %d, próximo em %s): %v", consecutiveErrs, delay, err)))
				syncTicker.Reset(delay)
			} else {
				if consecutiveErrs > 0 {
					syncTicker.Reset(syncInterval)
				}
				consecutiveErrs = 0
			}
		}
	}
}

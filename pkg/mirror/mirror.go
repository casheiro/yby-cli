package mirror

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/casheiro/yby-cli/pkg/retry"
	"github.com/casheiro/yby-cli/pkg/services/shared"
	"github.com/charmbracelet/lipgloss"
)

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
	nsManifest := fmt.Sprintf(`
apiVersion: v1
kind: Namespace
metadata:
  name: %s
`, m.Namespace)

	if err := m.Runner.RunStdin(context.Background(), nsManifest, "kubectl", "apply", "-f", "-"); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	// 2. Apply Manifests (Deployment + Service)
	manifest := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: git-server
  namespace: %s
  labels:
    app: git-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: git-server
  template:
    metadata:
      labels:
        app: git-server
    spec:
      containers:
      - name: git-server
        image: bitnami/git:latest
        command:
        - /bin/bash
        - -c
        - |
          mkdir -p /git/repo.git
          if [ ! -d "/git/repo.git/HEAD" ]; then
            git init --bare --initial-branch=main /git/repo.git
            git config --file /git/repo.git/config http.receivepack true
            touch /git/repo.git/git-daemon-export-ok
          fi
          exec git daemon --verbose --base-path=/git --export-all --enable=receive-pack
        ports:
        - containerPort: 9418
        volumeMounts:
        - name: git-data
          mountPath: /git
      volumes:
      - name: git-data
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: git-server
  namespace: %s
  labels:
    app: git-server
spec:
  selector:
    app: git-server
  ports:
  - port: 9418
    targetPort: 9418
    protocol: TCP
`, m.Namespace, m.Namespace)

	if err := m.Runner.RunStdin(context.Background(), manifest, "kubectl", "apply", "-f", "-"); err != nil {
		return fmt.Errorf("failed to apply git-server manifests: %w", err)
	}

	// Wait for rollout
	if err := m.Runner.Run(context.Background(), "kubectl", "rollout", "status", "deployment/git-server", "-n", m.Namespace, "--timeout=60s"); err != nil {
		return fmt.Errorf("git-server failed to start: %w", err)
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
		return fmt.Errorf("failed to create port forwarder: %w", err)
	}

	port, err := pf.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start port forwarder: %w", err)
	}

	m.forwarder = pf
	m.localPort = port
	fmt.Printf("📡 Tunnel established: localhost:%d -> git-server:9418\n", port)
	return nil
}

// Sync pushes local changes to the in-cluster git server via the tunnel
func (m *MirrorManager) Sync() error {
	if m.localPort == 0 {
		return fmt.Errorf("tunnel not established. Call SetupTunnel() first")
	}

	remoteURL := fmt.Sprintf("git://localhost:%d/repo.git", m.localPort)

	if err := m.Runner.Run(context.Background(), "git", "push", remoteURL, "HEAD:main", "--force"); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	return nil
}

// reconnect reconecta o port-forward com retry exponencial
func (m *MirrorManager) reconnect(ctx context.Context) error {
	fmt.Println(stepStyle.Render("🔄 Reconectando port-forward..."))
	slog.Info("iniciando reconexão do port-forward", "namespace", m.Namespace)

	if m.forwarder != nil {
		m.forwarder.Stop()
		m.forwarder = nil
		m.localPort = 0
	}

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
			if m.forwarder != nil {
				m.forwarder.Stop()
			}
			return

		case <-healthTicker.C:
			if m.forwarder != nil {
				if err := m.forwarder.HealthCheck(ctx); err != nil {
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

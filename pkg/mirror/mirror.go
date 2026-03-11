package mirror

import (
	"context"
	"fmt"
	"time"

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

// StartSyncLoop watches for changes and syncs automatically
func (m *MirrorManager) StartSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	fmt.Println(stepStyle.Render("🔄 Auto-sync enabled. Watching for changes..."))

	if err := m.Sync(); err != nil {
		fmt.Println(errorStyle.Render(fmt.Sprintf("❌ Initial Sync Failed: %v", err)))
	} else {
		fmt.Println(successStyle.Render("✅ Initial Sync Complete"))
	}

	for {
		select {
		case <-ctx.Done():
			if m.forwarder != nil {
				m.forwarder.Stop()
			}
			return
		case <-ticker.C:
			if err := m.Sync(); err != nil {
				fmt.Println(errorStyle.Render(fmt.Sprintf("⚠️ Sync Error: %v", err)))
			}
		}
	}
}

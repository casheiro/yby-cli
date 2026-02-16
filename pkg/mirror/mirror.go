package mirror

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	stepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // Blue
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // Red

	// execCommand is a variable to allow mocking in tests
	execCommand = exec.Command
)

// MirrorManager handles the in-cluster Git mirror for Hybrid GitOps
type MirrorManager struct {
	LocalPath string // Path to the local git repo (e.g. ".")
	Namespace string

	// PortForwarder instance for local access
	forwarder *PortForwarder
	localPort int
}

func NewManager(localPath string) *MirrorManager {
	return &MirrorManager{
		LocalPath: localPath,
		Namespace: "yby-system",
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

	cmdNs := execCommand("kubectl", "apply", "-f", "-")
	stdinNs, err := cmdNs.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdinNs.Close()
		io.WriteString(stdinNs, nsManifest)
	}()

	if out, err := cmdNs.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create namespace: %s: %w", out, err)
	}

	// 2. Apply Manifests (Deployment + Service)
	// BUG-008 Fix: Service exposes port 9418, targetPort 9418
	// BUG-010 Fix: git init --bare --initial-branch=main
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
            # Allow anonymous push (safe for localhost-only access via forwarder)
            git config --file /git/repo.git/config http.receivepack true
            touch /git/repo.git/git-daemon-export-ok
          fi
          # Start git daemon with verbose logging
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

	cmd := execCommand("kubectl", "apply", "-f", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, manifest)
	}()

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to apply git-server manifests: %s: %w", out, err)
	}

	// Wait for rollout
	if err := runKubectl("rollout", "status", "deployment/git-server", "-n", m.Namespace, "--timeout=60s"); err != nil {
		return fmt.Errorf("git-server failed to start: %w", err)
	}

	return nil
}

// SetupTunnel establishes the port-forward tunnel
// MUST be called before Sync() or StartSyncLoop() in local environment
func (m *MirrorManager) SetupTunnel(ctx context.Context) error {
	pf, err := NewPortForwarder(m.Namespace, "git-server", 9418)
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

	// Git push to localhost
	// git push git://localhost:<port>/repo.git HEAD:main --force
	remoteURL := fmt.Sprintf("git://localhost:%d/repo.git", m.localPort)

	// Ensure we are pushing to main
	cmd := execCommand("git", "push", remoteURL, "HEAD:main", "--force")
	cmd.Dir = m.LocalPath

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push failed: %s: %w", out, err)
	}

	return nil
}

// StartSyncLoop watches for changes and syncs automatically
// This blocks until context is cancelled
func (m *MirrorManager) StartSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // Poll every 5s
	defer ticker.Stop()

	fmt.Println(stepStyle.Render("🔄 Auto-sync enabled. Watching for changes..."))

	// Initial sync
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
			// Optimization: Check if there are changes to commit/push?
			// For now, to keep it "Zero Config", we push whatever committed state implies?
			// The original "yby dev" promise was "commit -> sync".
			// So we blindly try to push HEAD. If it's up to date, git handles it gracefully.
			if err := m.Sync(); err != nil {
				// Don't spam terminal on "Everything up-to-date" or transient errors?
				// Actually git push returns 0 if up to date.
				// If it fails, likely connectivity or non-fast-forward (we use force).
				fmt.Println(errorStyle.Render(fmt.Sprintf("⚠️ Sync Error: %v", err)))

				// Self-healing: Try to restart tunnel if broken?
				// Implement implementation logic here if needed.
			}
		}
	}
}

func runKubectl(args ...string) error {
	cmd := execCommand("kubectl", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", string(out), err)
	}
	return nil
}

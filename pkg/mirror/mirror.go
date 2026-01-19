package mirror

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// MirrorManager handles the lifecycle of the in-cluster Git Mirror
type MirrorManager struct {
	Namespace string
	LocalPath string
}

func NewManager(localPath string) *MirrorManager {
	return &MirrorManager{
		Namespace: "yby-system",
		LocalPath: localPath,
	}
}

// EnsureGitServer checks if the git-server is running, if not deploys it
func (m *MirrorManager) EnsureGitServer() error {
	// 0. Ensure Namespace
	_ = exec.Command("kubectl", "create", "ns", m.Namespace).Run() // Ignore error if exists

	// 1. Check Service
	cmd := exec.Command("kubectl", "get", "svc", "git-server", "-n", m.Namespace)
	if err := cmd.Run(); err != nil {
		fmt.Println("📦 Implantando Servidor Git no Cluster...")
		return m.deployServer()
	}
	return nil
}

// deployServer applies the manifest for a simple git server
func (m *MirrorManager) deployServer() error {
	// Minimal Git Server Manifest (Alpine + git-daemon or HTTP)
	// Using a simple busybox with git init --bare + minimal http/ssh is tricky.
	// Let's use a known image or a simple script.
	// For MVP: We assume a 'git-server' Deployment exposing port 80/22.

	manifest := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: git-server
  namespace: yby-system
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
        command: ["/bin/sh", "-c"]
        args:
          - |
            mkdir -p /git/repo.git && \
            cd /git/repo.git && \
            git init --bare && \
            touch /git/repo.git/git-daemon-export-ok && \
            echo "Starting Git Daemon..." && \
            git daemon --reuseaddr --base-path=/git --export-all --verbose --enable=receive-pack
        ports:
        - containerPort: 9418
        volumeMounts:
        - name: git-volume
          mountPath: /git
      volumes:
      - name: git-volume
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: git-server
  namespace: yby-system
spec:
  selector:
    app: git-server
  ports:
    - protocol: TCP
      port: 80
      targetPort: 9418
`
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("falha ao implantar git-server: %s", string(output))
	}

	// Wait for rollout
	fmt.Println("⏳ Aguardando Servidor Git...")
	_ = exec.Command("kubectl", "rollout", "status", "deployment/git-server", "-n", m.Namespace, "--timeout=60s").Run()

	return nil
}

// StartSyncLoop starts the synchronization process
func (m *MirrorManager) StartSyncLoop(ctx context.Context) {
	fmt.Println("🔄 Iniciando Agendador de Sincronização (intervalo de 5s)...")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Initial Sync
	if err := m.Sync(); err != nil {
		fmt.Printf("⚠️ Erro de Sincronização: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.Sync(); err != nil {
				fmt.Printf("⚠️ Erro de Sincronização: %v\n", err)
			}
		}
	}
}

// Sync performs a one-shot synchronization
func (m *MirrorManager) Sync() error {
	// 1. Get Pod Name
	out, err := exec.Command("kubectl", "get", "pods", "-n", m.Namespace, "-l", "app=git-server", "-o", "jsonpath={.items[0].metadata.name}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("falha ao obter pod: %v", err)
	}
	podName := strings.TrimSpace(string(out))
	if podName == "" {
		return fmt.Errorf("nenhum pod git-server encontrado no namespace %s", m.Namespace)
	}

	// Note: We need to handle 'incremental' updates to avoid overhead?
	// For MVP 5s loop, full sync is fine for small repos.

	remoteScript := `
set -e
mkdir -p /tmp/workspace
rm -rf /tmp/workspace/*
tar xf - -C /tmp/workspace
cd /tmp/workspace
# if [ ! -d infra ]; then echo "Warning: infra dir not found in sync"; fi
git init -q
git config user.email "bot@yby"
git config user.name "Yby Bot"
git add .
git commit -q -m "Sync" || true
git remote add origin /git/repo.git || git remote set-url origin /git/repo.git
git push origin master --force -q
`
	// Phase 5 Logic: Sync CONTENTS of m.LocalPath to ROOT of git-server repo.
	cmdStr := fmt.Sprintf("tar cf - -C %s . | kubectl exec -i -n %s %s -- sh -c '%s'", m.LocalPath, m.Namespace, podName, remoteScript)

	// fmt.Printf("DEBUG: Executing Sync...\n")
	if err := exec.Command("sh", "-c", cmdStr).Run(); err != nil {
		return err
	}

	// fmt.Println("   ✅ Synced.")
	return nil
}

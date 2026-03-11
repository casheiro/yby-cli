package mirror

import (
	"context"
	"errors"
	"testing"
)

func TestNewManager(t *testing.T) {
	runner := &MockRunner{}
	m := NewManager("/some/path", runner)

	if m == nil {
		t.Fatal("NewManager should not return nil")
	}
	if m.LocalPath != "/some/path" {
		t.Errorf("expected LocalPath=/some/path, got %s", m.LocalPath)
	}
	if m.Namespace != "yby-system" {
		t.Errorf("expected Namespace=yby-system, got %s", m.Namespace)
	}
}

func TestEnsureGitServer_NamespaceError(t *testing.T) {
	callCount := 0
	runner := &MockRunner{
		RunStdinFunc: func(ctx context.Context, stdin string, name string, args ...string) error {
			callCount++
			if callCount == 1 {
				return errors.New("namespace creation failed")
			}
			return nil
		},
	}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner}
	err := m.EnsureGitServer()
	if err == nil {
		t.Error("expected error when namespace creation fails, got nil")
	}
}

func TestEnsureGitServer_ManifestError(t *testing.T) {
	callCount := 0
	runner := &MockRunner{
		RunStdinFunc: func(ctx context.Context, stdin string, name string, args ...string) error {
			callCount++
			if callCount == 2 {
				return errors.New("manifest application failed")
			}
			return nil
		},
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner}
	err := m.EnsureGitServer()
	if err == nil {
		t.Error("expected error when manifest application fails, got nil")
	}
}

func TestEnsureGitServer_RolloutError(t *testing.T) {
	runner := &MockRunner{
		RunStdinFunc: func(ctx context.Context, stdin string, name string, args ...string) error {
			return nil
		},
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return errors.New("rollout timeout")
		},
	}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner}
	err := m.EnsureGitServer()
	if err == nil {
		t.Error("expected error when rollout fails, got nil")
	}
}

func TestSync_WithoutTunnel(t *testing.T) {
	runner := &MockRunner{}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner, localPort: 0}
	err := m.Sync()
	if err == nil {
		t.Error("expected error when tunnel not established, got nil")
	}
}

func TestSync_WithTunnelSuccess(t *testing.T) {
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner, localPort: 12345}
	err := m.Sync()
	if err != nil {
		t.Errorf("expected Sync to succeed, got: %v", err)
	}
}

func TestSync_WithTunnelError(t *testing.T) {
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return errors.New("git push failed")
		},
	}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner, localPort: 12345}
	err := m.Sync()
	if err == nil {
		t.Error("expected error from failed git push, got nil")
	}
}

func TestStartSyncLoop_CancelImmediately(t *testing.T) {
	runner := &MockRunner{
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}
	m := &MirrorManager{Namespace: "test-ns", Runner: runner, localPort: 12345}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	// Should not block
	m.StartSyncLoop(ctx)
}

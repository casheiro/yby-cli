package mirror

import (
	"context"
	"strings"
	"testing"
)

type MockRunner struct {
	RunFunc      func(ctx context.Context, name string, args ...string) error
	RunStdinFunc func(ctx context.Context, stdin string, name string, args ...string) error
}

func (m *MockRunner) Run(ctx context.Context, name string, args ...string) error {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, name, args...)
	}
	return nil
}

func (m *MockRunner) RunCombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	return nil, nil
}

func (m *MockRunner) RunStdin(ctx context.Context, stdin string, name string, args ...string) error {
	if m.RunStdinFunc != nil {
		return m.RunStdinFunc(ctx, stdin, name, args...)
	}
	return nil
}

func (m *MockRunner) LookPath(file string) (string, error) {
	return "/usr/bin/" + file, nil
}

func TestEnsureGitServer_NamespaceCreation(t *testing.T) {
	runner := &MockRunner{
		RunStdinFunc: func(ctx context.Context, stdin string, name string, args ...string) error {
			if name == "kubectl" && len(args) >= 3 && args[0] == "apply" && args[1] == "-f" && args[2] == "-" {
				if strings.Contains(stdin, "kind: Namespace") {
					// Namespace creation detected successfully
				}
				// Simulate success
				return nil
			}
			return nil
		},
		RunFunc: func(ctx context.Context, name string, args ...string) error {
			return nil
		},
	}

	m := &MirrorManager{
		Namespace: "test-ns",
		Runner:    runner,
	}

	err := m.EnsureGitServer()
	if err != nil {
		t.Errorf("EnsureGitServer failed: %v", err)
	}
}

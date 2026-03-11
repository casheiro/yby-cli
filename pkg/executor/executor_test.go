package executor

import (
	"errors"
	"testing"
)

func TestRealCommandExecutor_LookPath(t *testing.T) {
	executor := &RealCommandExecutor{}

	// Test with a command that should exist
	path, err := executor.LookPath("sh")
	if err != nil {
		t.Errorf("LookPath(sh) failed: %v", err)
	}
	if path == "" {
		t.Error("Expected non-empty path for sh")
	}

	// Test with a command that shouldn't exist
	_, err = executor.LookPath("nonexistent-command-xyz")
	if err == nil {
		t.Error("Expected error for nonexistent command")
	}
}

func TestMockCommandExecutor(t *testing.T) {
	mock := &MockCommandExecutor{
		LookPathFunc: func(file string) (string, error) {
			if file == "kubectl" {
				return "/usr/bin/kubectl", nil
			}
			return "", errors.New("not found")
		},
		CommandFunc: func(name string, arg ...string) Command {
			return &mockCommand{
				RunFunc: func() error {
					if name == "kubectl" {
						return nil
					}
					return errors.New("command failed")
				},
			}
		},
	}

	// Test LookPath
	path, err := mock.LookPath("kubectl")
	if err != nil {
		t.Errorf("LookPath(kubectl) failed: %v", err)
	}
	if path != "/usr/bin/kubectl" {
		t.Errorf("Expected /usr/bin/kubectl, got %s", path)
	}

	_, err = mock.LookPath("helm")
	if err == nil {
		t.Error("Expected error for helm")
	}

	// Test Command
	cmd := mock.Command("kubectl", "get", "nodes")
	if err := cmd.Run(); err != nil {
		t.Errorf("kubectl command should succeed: %v", err)
	}

	cmd = mock.Command("helm", "list")
	if err := cmd.Run(); err == nil {
		t.Error("helm command should fail")
	}
}

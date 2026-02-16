package cmd

import (
	"os"
	"os/exec"
	"testing"
)

func TestExecuteHelmRepoAdd_Failure(t *testing.T) {
	// Setup mock execCommand
	mockExecCommand()
	defer func() { execCommand = exec.Command }()

	// Setup mock osExit
	var exitCode int
	osExit = func(code int) {
		exitCode = code
		panic("os.Exit called")
	}
	defer func() { osExit = os.Exit }()

	// Capture stdout/stderr? (Not strictly necessary if checking exit code, but good for cleanliness)

	defer func() {
		if r := recover(); r != nil {
			if r != "os.Exit called" {
				t.Errorf("Panicked with %v", r)
			}
		}
	}()

	// Trigger failure via specific args defined in exec_mock_test.go
	executeHelmRepoAdd("fail-repo", "https://charts.fail.com")

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
}

func TestCreateNamespace_AlreadyExists(t *testing.T) {
	mockExecCommand()
	defer func() { execCommand = exec.Command }()

	// Mock osExit to panic if called (it SHOULD NOT be called for already exists)
	osExit = func(code int) {
		t.Fatalf("os.Exit called with code %d when it should be ignored", code)
	}
	defer func() { osExit = os.Exit }()

	// "exists-ns" triggers "AlreadyExists" output in mock
	createNamespace("exists-ns")
}

func TestCreateNamespace_Failure(t *testing.T) {
	mockExecCommand()
	defer func() { execCommand = exec.Command }()

	var exitCode int
	osExit = func(code int) {
		exitCode = code
		panic("os.Exit called")
	}
	defer func() { osExit = os.Exit }()

	defer func() {
		if r := recover(); r != nil {
			if r != "os.Exit called" {
				t.Errorf("Panicked with %v", r)
			}
		}
	}()

	// "fail-ns" triggers generic failure in mock
	createNamespace("fail-ns")

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
}

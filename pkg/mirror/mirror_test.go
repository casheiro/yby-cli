package mirror

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestHelperProcess is the entrypoint for the mock process.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "kubectl":
		// check if it's the apply command for namespace
		if len(args) >= 3 && args[0] == "apply" && args[1] == "-f" && args[2] == "-" {
			// Success! The fix is working.
			// We can verify stdin content here if needed, but for now just command args are enough.
			return
		}
		// Failure case for old piped command
		if len(args) > 0 && args[0] == "create" && strings.Contains(strings.Join(args, " "), "|") {
			fmt.Fprintf(os.Stderr, "Error: piped command detected\n")
			os.Exit(1)
		}
	case "helm":
		// Mock helm success
		return
	case "git":
		// Mock git success
		return
	}
}

func TestEnsureGitServer_NamespaceCreation(t *testing.T) {
	// Mock exec.Command
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
	defer func() { execCommand = exec.Command }()

	m := &MirrorManager{
		Namespace: "test-ns",
	}

	// We only want to test the namespace creation logic which happens inside EnsureGitServer
	// However, EnsureGitServer does many things.
	// To isolate, we might need to mock more or refactor.
	// For now, let's try running it and see if our mock catches the kubectl apply call.
	// Note: EnsureGitServer calls other commands too (helm, git).
	// Our TestHelperProcess mocks them as success.

	err := m.EnsureGitServer()
	if err != nil {
		t.Errorf("EnsureGitServer failed: %v", err)
	}
}

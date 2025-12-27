package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Sandbox represents an isolated environment for testing the CLI
type Sandbox struct {
	ID          string
	WorkDir     string
	ContainerID string
}

// NewSandbox creates a temporary directory and prepares for docker run
func NewSandbox(t *testing.T) *Sandbox {
	dir, err := os.MkdirTemp("", "yby-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	return &Sandbox{
		ID:      filepath.Base(dir),
		WorkDir: dir,
	}
}

// Start launches a container with the CLI mounted
func (s *Sandbox) Start(t *testing.T) {
	// 1. Build CLI binary
	binPath := filepath.Join(s.WorkDir, "yby")

	// Determine Project Root
	wd, _ := os.Getwd()
	// Usually running from test/e2e/scenarios
	// Look for go.mod upwards
	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			t.Fatalf("Could not find project root (go.mod) from %s", wd)
		}
		projectRoot = parent
	}
	t.Logf("Project Root: %s", projectRoot)

	// Build using the specific main entrypoint
	cmdPath := "./cmd/yby" // Relative to project root

	buildCmd := exec.Command("go", "build", "-o", binPath, cmdPath)
	buildCmd.Dir = projectRoot
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0") // Ensure static binary for Alpine
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\n%s", err, string(out))
	}

	// 2. Run Container (Alpine)
	// We mount the binary and the temp dir as workspace
	cmd := exec.Command("docker", "run", "-d", "--rm",
		"-v", fmt.Sprintf("%s:/usr/local/bin/yby", binPath),
		"-v", fmt.Sprintf("%s:/workspace", s.WorkDir),
		"-w", "/workspace",
		"alpine:latest",
		"tail", "-f", "/dev/null", // Keep alive
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start container: %v\n%s", err, string(out))
	}
	s.ContainerID = strings.TrimSpace(string(out))

	// Wait a bit
	time.Sleep(2 * time.Second)
}

// Stop cleans up
func (s *Sandbox) Stop() {
	if s.ContainerID != "" {
		exec.Command("docker", "rm", "-f", s.ContainerID).Run()
	}
	os.RemoveAll(s.WorkDir)
}

// RunCLI executes a command inside the container
func (s *Sandbox) RunCLI(t *testing.T, args ...string) string {
	dockerArgs := append([]string{"exec", s.ContainerID, "yby"}, args...)

	var out []byte
	var err error

	// Retry up to 3 times for "page not found" (CI flake)
	for i := 0; i < 3; i++ {
		cmd := exec.Command("docker", dockerArgs...)
		out, err = cmd.CombinedOutput()
		outputStr := string(out)

		if err == nil {
			return outputStr
		}

		// Check for specific Docker daemon flake
		if strings.Contains(outputStr, "page not found") {
			t.Logf("Docker exec failed with 'page not found', retrying... (%d/3)", i+1)
			time.Sleep(2 * time.Second)
			continue
		}

		// Real failure
		t.Fatalf("CLI command failed: yby %s\n%s", strings.Join(args, " "), outputStr)
	}

	// If we exhausted retries
	t.Fatalf("CLI command failed after retries: yby %s\n%s", strings.Join(args, " "), string(out))
	return ""
}

// RunShell executes an arbitrary command inside the container (e.g. apk, ls)
func (s *Sandbox) RunShell(t *testing.T, args ...string) string {
	dockerArgs := append([]string{"exec", s.ContainerID}, args...)
	cmd := exec.Command("docker", dockerArgs...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Shell command failed: %s\n%s", strings.Join(args, " "), string(out))
	}
	return string(out)
}

// AssertFileExists checks if file exists inside container
func (s *Sandbox) AssertFileExists(t *testing.T, path string) {
	cmd := exec.Command("docker", "exec", s.ContainerID, "test", "-f", path)
	if err := cmd.Run(); err != nil {
		t.Errorf("File %s does not exist in container", path)
	}
}

// AssertFileContains checks content
func (s *Sandbox) AssertFileContains(t *testing.T, path, sub string) {
	cmd := exec.Command("docker", "exec", s.ContainerID, "cat", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Failed to read %s: %v", path, err)
		return
	}
	if !strings.Contains(string(out), sub) {
		t.Errorf("File %s does not contain '%s'. Content matches?", path, sub)
		// t.Logf("Content: %s", string(out))
	}
}

// AssertFileNotExists checks if file does NOT exist inside container
func (s *Sandbox) AssertFileNotExists(t *testing.T, path string) {
	cmd := exec.Command("docker", "exec", s.ContainerID, "test", "-e", path)
	// If it exists, exit code is 0 (nil error). We want it to fail (non-nil error).
	if err := cmd.Run(); err == nil {
		t.Errorf("File %s SHOULD NOT exist in container, but it does.", path)
	}
}

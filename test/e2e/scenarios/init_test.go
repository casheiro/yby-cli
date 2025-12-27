package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

func TestInit_Headless_Complete(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// 1. Run yby init (Headless)
	// --topology complete --workflow gitflow
	// We wait a bit for container to be fully up
	time.Sleep(2 * time.Second)

	output := s.RunCLI(t, "init",
		"--topology", "complete",
		"--workflow", "gitflow",
		"--git-repo", "https://github.com/test/repo",
		"--env", "dev",
		"--include-ci=true",
		"--include-devcontainer=true",
	)

	t.Logf("Output: %s", output)

	// 2. Validate Files
	// Environments Manifest
	s.AssertFileExists(t, ".yby/environments.yaml")
	s.AssertFileContains(t, ".yby/environments.yaml", "current: dev")
	s.AssertFileContains(t, ".yby/environments.yaml", "prod:")
	s.AssertFileContains(t, ".yby/environments.yaml", "staging:")

	// Workflows
	s.AssertFileExists(t, ".github/workflows/feature-pipeline.yaml")
	// s.AssertFileExists(t, ".github/workflows/release-automation.yaml") // Need to check if gitflow has this exact name

	// DevContainer
	s.AssertFileExists(t, ".devcontainer/devcontainer.json")

	// Values
	s.AssertFileExists(t, "config/values-dev.yaml")
	s.AssertFileExists(t, "config/values-prod.yaml")
	s.AssertFileExists(t, "config/values-staging.yaml")
	s.AssertFileExists(t, "config/values-staging.yaml")

	// 3. Validate Templating (Crucial Check)
	// We expect the git repo URL to be injected into cluster-values.yaml
	s.AssertFileContains(t, "config/cluster-values.yaml", "https://github.com/test/repo")

	// We expect the project name (derived from repo) to be injected into root-app.yaml
	// repo: test/repo -> project: "repo" (or "test-repo" depending on logic, let's assume default derivation)
	// Actually, deriveProjectName("https://github.com/test/repo") -> "repo"
	s.AssertFileContains(t, "manifests/argocd/root-app.yaml", "project: repo")
}

func TestEnv_Commands(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Init first
	s.RunCLI(t, "init", "--topology", "standard", "--workflow", "essential", "--git-repo", "x")

	// Test List
	out := s.RunCLI(t, "env", "list")
	if !contains(out, "local") || !contains(out, "prod") {
		t.Errorf("Env list missing envs. Got: %s", out)
	}

	// Test Use
	s.RunCLI(t, "env", "use", "prod")
	s.AssertFileContains(t, ".yby/environments.yaml", "current: prod")

	// Test Show
	outShow := s.RunCLI(t, "env", "show")
	if !contains(outShow, "Ambiente Ativo: prod") {
		t.Errorf("Env show failed. Got: %s", outShow)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

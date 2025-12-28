package scenarios

import (
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

func TestInit_Topology_Single(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Single Topology: Should only create production configs
	output := s.RunCLI(t, "init",
		"--topology", "single",
		"--workflow", "essential",
		"--git-repo", "https://github.com/test/single",
		"--env", "prod",
		"--include-ci=false", // Minimal
	)
	t.Logf("Output: %s", output)

	// Validate Environments
	s.AssertFileExists(t, ".yby/environments.yaml")
	s.AssertFileContains(t, ".yby/environments.yaml", "current: prod")

	// Should contain prod
	s.AssertFileContains(t, ".yby/environments.yaml", "prod:")

	// Should NOT contain local, dev, staging
	// sandbox.AssertFileNotContains doesn't exist, we can inspect content manually or implement it.
	// For now let's rely on file existence of config/values-*

	s.AssertFileExists(t, "config/values-prod.yaml")

	// Ensure other env values are NOT created
	s.AssertFileNotExists(t, "config/values-local.yaml")
	s.AssertFileNotExists(t, "config/values-dev.yaml")
	s.AssertFileNotExists(t, "config/values-staging.yaml")
}

func TestInit_Topology_Standard(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Standard: Local + Prod
	s.RunCLI(t, "init",
		"--topology", "standard",
		"--workflow", "essential",
		"--git-repo", "https://github.com/test/std",
	)

	// Check Environments
	s.AssertFileExists(t, "config/values-local.yaml")
	s.AssertFileExists(t, "config/values-prod.yaml")

	// Verify environments.yaml content
	s.AssertFileContains(t, ".yby/environments.yaml", "local:")
	s.AssertFileContains(t, ".yby/environments.yaml", "prod:")
}

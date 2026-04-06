//go:build e2e

package scenarios

import (
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

func TestInit_Topology_Single(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Single Topology: gera apenas ambiente local (mudança intencional em refactor anterior)
	output := s.RunCLI(t, "init",
		"--topology", "single",
		"--workflow", "essential",
		"--git-repo", "https://github.com/test/single",
		"--env", "local",
		"--include-ci=false",
	)
	t.Logf("Output: %s", output)

	// Validate Environments
	s.AssertFileExists(t, ".yby/environments.yaml")
	s.AssertFileContains(t, ".yby/environments.yaml", "current: local")
	s.AssertFileContains(t, ".yby/environments.yaml", "local:")

	// Single topology cria apenas values-local
	s.AssertFileExists(t, "config/values-local.yaml")

	// Não deve criar outros ambientes
	s.AssertFileNotExists(t, "config/values-prod.yaml")
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

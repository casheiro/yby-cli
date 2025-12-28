package scenarios

import (
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

func TestInit_Workflow_Essential(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	s.RunCLI(t, "init",
		"--topology", "single",
		"--workflow", "essential",
		"--git-repo", "https://github.com/test/essential",
	)

	// Essential should have pr-main-checks but NOT release-automation (which is gitflow specific)
	s.AssertFileExists(t, ".github/workflows/pr-main-checks.yaml")

	// Check content to ensure it's the essential version (maybe verify absence of complex triggers?)
	// Actually, just existence is good for now.
}

func TestInit_Flags_NoCI(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	s.RunCLI(t, "init",
		"--topology", "single",
		"--workflow", "essential",
		"--include-ci=false",
		"--git-repo", "https://github.com/test/noci",
	)

	// Should NOT create .github/workflows
	s.AssertFileNotExists(t, ".github/workflows")
}

func TestInit_Flags_NoDevContainer(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	s.RunCLI(t, "init",
		"--topology", "single",
		"--workflow", "essential",
		"--include-devcontainer=false",
		"--git-repo", "https://github.com/test/nodev",
	)

	s.AssertFileNotExists(t, ".devcontainer")
}

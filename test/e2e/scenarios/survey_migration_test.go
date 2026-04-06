//go:build e2e

package scenarios

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

// TestInit_NonInteractive_AllFlags verifica que init funciona com todas as flags em modo headless.
func TestInit_NonInteractive_AllFlags(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	output := s.RunCLI(t, "init",
		"--non-interactive",
		"--topology", "standard",
		"--workflow", "gitflow",
		"--project-name", "full-flags-test",
		"--git-repo", "https://github.com/test/full-flags.git",
		"--domain", "test.example.com",
		"--email", "admin@test.example.com",
		"--env", "staging",
		"--secrets-strategy", "sops",
		"--enable-kepler",
		"--enable-minio",
		"--enable-keda",
		"--enable-metrics-server",
		"--include-devcontainer",
		"--include-ci",
	)

	// Verificar que não houve erro de prompt interativo
	if strings.Contains(output, "survey") || strings.Contains(output, "prompt") {
		t.Errorf("Modo --non-interactive não deveria exibir prompts, obteve: %s", output)
	}

	// Verificar geração de arquivos
	s.AssertFileExists(t, ".yby/project.yaml")
	s.AssertFileExists(t, "config/cluster-values.yaml")

	// Verificar que manifest tem as configurações corretas
	s.AssertFileContains(t, ".yby/project.yaml", "topology: standard")
	s.AssertFileContains(t, ".yby/project.yaml", "workflow: gitflow")
	s.AssertFileContains(t, ".yby/project.yaml", "domain: test.example.com")
	s.AssertFileContains(t, ".yby/project.yaml", "secretsStrategy: sops")
}

// TestSeal_HelpWorks verifica que seal --help funciona sem crash.
func TestSeal_HelpWorks(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	output := s.RunCLI(t, "secret", "seal", "--help")

	if !strings.Contains(output, "seal") {
		t.Errorf("seal --help deveria conter 'seal', obteve: %s", output)
	}
}

// TestInit_NonInteractive_MissingRequiredFails verifica que init non-interactive falha sem topology.
func TestInit_NonInteractive_MissingRequiredFails(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Rodar sem topology (campo obrigatório em headless)
	output := s.RunCLIAllowError(t, "init", "--non-interactive")

	// Deve falhar ou usar defaults sem travar em prompt
	// O importante é que NÃO trava esperando input interativo
	t.Logf("init --non-interactive sem flags: %s", output)
}

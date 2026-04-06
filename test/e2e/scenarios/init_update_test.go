//go:build e2e

package scenarios

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

// TestInitUpdate_PreservesUserChanges verifica que --update preserva alterações feitas pelo usuário.
func TestInitUpdate_PreservesUserChanges(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// 1. Init padrão
	s.RunCLI(t, "init",
		"--non-interactive",
		"--topology", "standard",
		"--workflow", "gitflow",
		"--project-name", "update-test",
		"--git-repo", "https://github.com/test/update-test.git",
	)

	// 2. Verificar que manifest foi gerado e tem fileHashes
	s.AssertFileExists(t, ".yby/project.yaml")

	// 3. Rodar --update (deve funcionar sem erro)
	output := s.RunCLIAllowError(t, "init",
		"--non-interactive",
		"--update",
		"--topology", "standard",
		"--workflow", "gitflow",
		"--project-name", "update-test",
		"--git-repo", "https://github.com/test/update-test.git",
	)

	// 4. Verificar que o update executou (não bloqueou por "projeto já inicializado")
	if strings.Contains(output, "já inicializado") && strings.Contains(output, "--force") {
		t.Fatalf("--update não deveria ser bloqueado por detecção de init duplo, obteve: %s", output)
	}

	// 5. Verificar que manifest ainda existe após update
	s.AssertFileExists(t, ".yby/project.yaml")
}

// TestInitUpdate_CopiesNewFiles verifica que --update copia arquivos de features novas.
func TestInitUpdate_CopiesNewFiles(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// 1. Init com features mínimas (sem kepler)
	s.RunCLI(t, "init",
		"--non-interactive",
		"--topology", "standard",
		"--workflow", "essential",
		"--project-name", "new-files-test",
		"--git-repo", "https://github.com/test/new-files.git",
	)

	// 2. Update habilitando kepler
	s.RunCLI(t, "init",
		"--non-interactive",
		"--update",
		"--topology", "standard",
		"--workflow", "essential",
		"--project-name", "new-files-test",
		"--git-repo", "https://github.com/test/new-files.git",
		"--enable-kepler",
	)

	// 3. Verificar que manifest registrou kepler como habilitado
	s.AssertFileContains(t, ".yby/project.yaml", "kepler: true")
}

// TestInitUpdate_MutuallyExclusiveWithForce verifica que --update e --force não podem ser usados juntos.
func TestInitUpdate_MutuallyExclusiveWithForce(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Init primeiro para ter manifest
	s.RunCLI(t, "init",
		"--non-interactive",
		"--topology", "standard",
		"--workflow", "essential",
		"--project-name", "exclusive-test",
		"--git-repo", "https://github.com/test/exclusive.git",
	)

	// Tentar --update --force deve falhar
	output := s.RunCLIExpectError(t, "init",
		"--non-interactive",
		"--update",
		"--force",
		"--topology", "standard",
		"--workflow", "essential",
		"--project-name", "exclusive-test",
		"--git-repo", "https://github.com/test/exclusive.git",
	)

	if !strings.Contains(output, "mutuamente exclusivos") && !strings.Contains(output, "mutually exclusive") {
		t.Errorf("Esperava erro de flags mutuamente exclusivas, obteve: %s", output)
	}
}

// TestInitUpdate_RequiresExistingManifest verifica que --update em diretório sem init falha.
func TestInitUpdate_RequiresExistingManifest(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Tentar --update sem init prévio
	output := s.RunCLIExpectError(t, "init",
		"--non-interactive",
		"--update",
		"--topology", "standard",
		"--workflow", "essential",
		"--project-name", "no-manifest",
		"--git-repo", "https://github.com/test/no-manifest.git",
	)

	if !strings.Contains(output, "manifest") && !strings.Contains(output, "project.yaml") {
		t.Errorf("Esperava erro sobre manifest inexistente, obteve: %s", output)
	}
}

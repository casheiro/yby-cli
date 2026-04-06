//go:build e2e

package scenarios

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

// TestUpgrade_CheckVersion verifica que --check exibe versão atual e disponível.
// Como o binário E2E não consegue acessar a API do GitHub no sandbox,
// testamos que o comando reconhece a flag e exibe informações básicas.
func TestUpgrade_CheckVersion(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// O upgrade --check pode falhar por falta de rede no container,
	// mas deve reconhecer a flag e tentar executar
	output := s.RunCLIAllowError(t, "upgrade", "--check")

	// Deve conter "Versão atual" ou indicar erro de rede (ambos são válidos)
	hasVersionInfo := strings.Contains(output, "Versão atual") || strings.Contains(output, "version")
	hasNetworkError := strings.Contains(output, "falha") || strings.Contains(output, "release") || strings.Contains(output, "network")

	if !hasVersionInfo && !hasNetworkError {
		t.Errorf("upgrade --check deveria mostrar info de versão ou erro de rede, obteve: %s", output)
	}
}

// TestUpgrade_HelpWorks verifica que o help do upgrade funciona sem crash.
func TestUpgrade_HelpWorks(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	output := s.RunCLI(t, "upgrade", "--help")

	if !strings.Contains(output, "upgrade") {
		t.Errorf("upgrade --help deveria conter 'upgrade', obteve: %s", output)
	}

	if !strings.Contains(output, "--check") {
		t.Errorf("upgrade --help deveria listar flag --check, obteve: %s", output)
	}

	if !strings.Contains(output, "--version") {
		t.Errorf("upgrade --help deveria listar flag --version, obteve: %s", output)
	}
}

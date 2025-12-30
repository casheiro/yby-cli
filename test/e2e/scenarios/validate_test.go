package scenarios

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

func TestValidate_FreshInit(t *testing.T) {
	// Scenario: Validação de projeto recém-criado (Offline/Mocked)
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Setup: Instalar mocks de ferramentas (helm, kubectl, git) para evitar dependência de rede
	// Isso evita o 'apk add helm' que causava timeout/hang.
	installFakeTools(t, s)

	// Given que inicializei um projeto
	s.RunCLI(t, "init",
		"--topology", "single",
		"--workflow", "essential",
		"--git-repo", "https://github.com/test/validate",
		"--include-ci=false",
	)

	// When executo 'yby validate'
	// O comando validate tenta rodar 'helm dependency build', 'helm lint', etc.
	// Nossos fake tools vão interceptar essas chamadas e retornar sucesso (exit 0).
	output := s.RunCLI(t, "validate")

	// Then a validação deve ocorrer com sucesso
	// Validamos se o output contém mensagens chave de validação
	if !strings.Contains(output, "Validação de Charts") {
		t.Errorf("Expected validation start message. Got: %s", output)
	}

	// Como estamos usando mocks, não vamos ver erros reais de lint, mas garantimos que a CLI percorreu o fluxo
	// de invocar as ferramentas externas corretamente.
}

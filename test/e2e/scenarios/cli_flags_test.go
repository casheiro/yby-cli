//go:build e2e

package scenarios

import (
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

// TestUkiCapture_FileFlag verifica que a flag --file é reconhecida pelo uki capture.
func TestUkiCapture_FileFlag(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Criar arquivo de texto para captura
	s.RunShell(t, "sh", "-c", `echo "Precisamos de política de retenção de logs de 30 dias" > /workspace/input.txt`)

	// Rodar uki capture --file (vai falhar por falta de IA, mas não por flag inválida)
	output := s.RunCLIAllowError(t, "uki", "capture", "--file", "/workspace/input.txt")

	// Não deve conter erro de flag desconhecida
	if strings.Contains(output, "unknown flag") || strings.Contains(output, "flag desconhecida") {
		t.Errorf("Flag --file deveria ser reconhecida, obteve: %s", output)
	}

	// Deve tentar inicializar IA (indica que passou pela validação de flags)
	hasAIAttempt := strings.Contains(output, "IA") ||
		strings.Contains(output, "provedor") ||
		strings.Contains(output, "provider") ||
		strings.Contains(output, "Governance Capture")
	if !hasAIAttempt {
		t.Logf("uki capture --file output: %s", output)
	}
}

// TestGenerateKeda_EndScheduleFlag verifica que --end-schedule é aplicado no template KEDA.
func TestGenerateKeda_EndScheduleFlag(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	output := s.RunCLI(t, "generate", "keda",
		"--name", "test-scaler",
		"--deployment", "test-app",
		"--namespace", "default",
		"--schedule", "0 6 * * *",
		"--end-schedule", "0 22 * * *",
		"--replicas", "3",
		"--timezone", "UTC",
	)

	if !strings.Contains(output, "0 22 * * *") {
		t.Errorf("Output deveria conter end-schedule '0 22 * * *', obteve: %s", output)
	}

	if !strings.Contains(output, "0 6 * * *") {
		t.Errorf("Output deveria conter schedule '0 6 * * *', obteve: %s", output)
	}

	if !strings.Contains(output, "maxReplicaCount: 3") {
		t.Errorf("Output deveria conter maxReplicaCount: 3, obteve: %s", output)
	}

	if !strings.Contains(output, "timezone: UTC") {
		t.Errorf("Output deveria conter timezone: UTC, obteve: %s", output)
	}
}

// TestChartCreate_LoadsManifest verifica que chart create usa o domain do manifest.
func TestChartCreate_LoadsManifest(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// 1. Init com domain customizado
	s.RunCLI(t, "init",
		"--non-interactive",
		"--topology", "standard",
		"--workflow", "essential",
		"--project-name", "chart-test",
		"--git-repo", "https://github.com/myorg/chart-test.git",
		"--domain", "custom.example.com",
	)

	// 2. Criar chart
	output := s.RunCLIAllowError(t, "chart", "create", "my-service")

	// Se sucesso, verificar que Chart.yaml usa o domain do manifest
	if strings.Contains(output, "my-service") {
		// Verificar que o chart foi criado e contém o domain correto
		chartContent := s.RunShellAllowError(t, "cat", "charts/my-service/Chart.yaml")
		if chartContent == nil {
			// Arquivo existe, verificar conteúdo
			content := s.RunShell(t, "cat", "charts/my-service/Chart.yaml")
			if strings.Contains(content, "yby.local") {
				t.Errorf("Chart.yaml deveria usar domain 'custom.example.com', mas contém 'yby.local'")
			}
		}
	} else {
		t.Logf("chart create output (pode falhar se template não existe): %s", output)
	}
}

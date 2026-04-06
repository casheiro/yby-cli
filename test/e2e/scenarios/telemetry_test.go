//go:build e2e

package scenarios

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/test/e2e/sandbox"
)

// TestTelemetry_PersistsEvents verifica que eventos de telemetria são persistidos em JSONL.
func TestTelemetry_PersistsEvents(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Configurar HOME isolado dentro do container
	s.RunShell(t, "mkdir", "-p", "/tmp/telhome")

	// Rodar um comando simples para gerar evento de telemetria
	s.RunCLIWithEnv(t, []string{"HOME=/tmp/telhome"}, "version")

	// Verificar que o arquivo de telemetria foi criado
	output := s.RunShell(t, "cat", "/tmp/telhome/.yby/telemetry.jsonl")

	if output == "" {
		t.Fatal("Arquivo telemetry.jsonl está vazio ou não foi criado")
	}

	// Validar que cada linha é JSON válido com os campos esperados
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("Linha %d não é JSON válido: %s (erro: %v)", i+1, line, err)
			continue
		}

		if _, ok := event["name"]; !ok {
			t.Errorf("Linha %d falta campo 'name': %s", i+1, line)
		}
		if _, ok := event["duration_ms"]; !ok {
			t.Errorf("Linha %d falta campo 'duration_ms': %s", i+1, line)
		}
		if _, ok := event["success"]; !ok {
			t.Errorf("Linha %d falta campo 'success': %s", i+1, line)
		}
	}
}

// TestTelemetry_Export verifica que o comando export retorna dados de telemetria.
func TestTelemetry_Export(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Criar arquivo de telemetria de teste
	s.RunShell(t, "mkdir", "-p", "/tmp/exporthome/.yby")
	testData := `{"name":"test-cmd","duration_ms":42,"success":true,"timestamp":"2025-01-01T00:00:00Z"}`
	s.RunShell(t, "sh", "-c", "echo '"+testData+"' > /tmp/exporthome/.yby/telemetry.jsonl")

	// Rodar export com HOME apontando para o diretório de teste
	output := s.RunCLIWithEnv(t, []string{"HOME=/tmp/exporthome"}, "telemetry", "export")

	if !strings.Contains(output, "test-cmd") {
		t.Errorf("telemetry export deveria conter dados de teste, obteve: %s", output)
	}
	if !strings.Contains(output, "duration_ms") {
		t.Errorf("telemetry export deveria conter campo duration_ms, obteve: %s", output)
	}
}

// TestTelemetry_DisabledViaConfig verifica que telemetria não é gravada quando desabilitada.
func TestTelemetry_DisabledViaConfig(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Criar config com telemetria desabilitada
	s.RunShell(t, "mkdir", "-p", "/tmp/nothome/.yby")
	s.RunShell(t, "sh", "-c", `cat > /tmp/nothome/.yby/config.yaml << 'YAML'
telemetry:
  enabled: false
YAML`)

	// Rodar comando
	s.RunCLIWithEnv(t, []string{"HOME=/tmp/nothome"}, "version")

	// Verificar que telemetry.jsonl NÃO foi criado
	// test -f retorna exit 0 se existe, exit 1 se não existe
	err := s.RunShellAllowError(t, "test", "-f", "/tmp/nothome/.yby/telemetry.jsonl")
	if err == nil {
		t.Error("telemetry.jsonl NÃO deveria existir quando telemetria está desabilitada")
	}
}

// TestTelemetry_Rotation verifica que arquivos grandes de telemetria são rotacionados.
func TestTelemetry_Rotation(t *testing.T) {
	s := sandbox.NewSandbox(t)
	s.Start(t)
	defer s.Stop()

	// Criar arquivo de telemetria com > 5MB
	s.RunShell(t, "mkdir", "-p", "/tmp/rothome/.yby")
	// Gerar ~5.1MB de dados JSONL
	s.RunShell(t, "sh", "-c", `
		line='{"name":"bulk","duration_ms":1,"success":true,"timestamp":"2025-01-01T00:00:00Z"}'
		# Cada linha tem ~80 bytes. 5MB / 80 = ~65536 linhas
		for i in $(seq 1 66000); do echo "$line"; done > /tmp/rothome/.yby/telemetry.jsonl
	`)

	// Verificar que o arquivo é > 5MB
	sizeOutput := s.RunShell(t, "sh", "-c", "wc -c < /tmp/rothome/.yby/telemetry.jsonl")
	t.Logf("Tamanho do arquivo de telemetria: %s bytes", strings.TrimSpace(sizeOutput))

	// Rodar comando para gerar mais dados (triggera rotação)
	s.RunCLIWithEnv(t, []string{"HOME=/tmp/rothome"}, "version")

	// Verificar que rotação ocorreu
	s.RunShell(t, "test", "-f", "/tmp/rothome/.yby/telemetry.jsonl.1")
}

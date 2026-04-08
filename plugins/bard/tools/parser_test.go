package tools

import (
	"testing"
)

// TestParseToolCalls_SemToolCall verifica que texto sem tool calls retorna vazio.
func TestParseToolCalls_SemToolCall(t *testing.T) {
	calls, remaining := ParseToolCalls("Aqui está uma resposta normal sem ferramentas.")

	if len(calls) != 0 {
		t.Errorf("esperava 0 calls, obteve %d", len(calls))
	}
	if remaining != "Aqui está uma resposta normal sem ferramentas." {
		t.Errorf("texto restante inesperado: %q", remaining)
	}
}

// TestParseToolCalls_CodeFence verifica extração de tool call em code fence.
func TestParseToolCalls_CodeFence(t *testing.T) {
	response := "Vou verificar os pods:\n```json\n{\"tool\": \"kubectl_get\", \"params\": {\"resource\": \"pods\", \"namespace\": \"default\"}}\n```\nAguarde."

	calls, remaining := ParseToolCalls(response)

	if len(calls) != 1 {
		t.Fatalf("esperava 1 call, obteve %d", len(calls))
	}
	if calls[0].Name != "kubectl_get" {
		t.Errorf("nome esperado 'kubectl_get', obtido '%s'", calls[0].Name)
	}
	if calls[0].Params["resource"] != "pods" {
		t.Errorf("param resource esperado 'pods', obtido '%s'", calls[0].Params["resource"])
	}
	if calls[0].Params["namespace"] != "default" {
		t.Errorf("param namespace esperado 'default', obtido '%s'", calls[0].Params["namespace"])
	}
	if remaining != "Vou verificar os pods:\n\nAguarde." {
		t.Errorf("texto restante inesperado: %q", remaining)
	}
}

// TestParseToolCalls_CodeFenceComTag verifica code fence com tag ```json.
func TestParseToolCalls_CodeFenceComTag(t *testing.T) {
	response := "```json\n{\"tool\": \"kubectl_logs\", \"params\": {\"pod\": \"nginx-abc\"}}\n```"

	calls, remaining := ParseToolCalls(response)

	if len(calls) != 1 {
		t.Fatalf("esperava 1 call, obteve %d", len(calls))
	}
	if calls[0].Name != "kubectl_logs" {
		t.Errorf("nome esperado 'kubectl_logs', obtido '%s'", calls[0].Name)
	}
	if calls[0].Params["pod"] != "nginx-abc" {
		t.Errorf("param pod esperado 'nginx-abc', obtido '%s'", calls[0].Params["pod"])
	}
	if remaining != "" {
		t.Errorf("texto restante deveria estar vazio, obteve: %q", remaining)
	}
}

// TestParseToolCalls_Inline verifica extração de JSON inline (sem code fence).
func TestParseToolCalls_Inline(t *testing.T) {
	response := `Vou executar {"tool": "kubectl_get", "params": {"resource": "services"}} agora.`

	calls, remaining := ParseToolCalls(response)

	if len(calls) != 1 {
		t.Fatalf("esperava 1 call, obteve %d", len(calls))
	}
	if calls[0].Name != "kubectl_get" {
		t.Errorf("nome esperado 'kubectl_get', obtido '%s'", calls[0].Name)
	}
	if calls[0].Params["resource"] != "services" {
		t.Errorf("param resource esperado 'services', obtido '%s'", calls[0].Params["resource"])
	}
	if remaining != "Vou executar  agora." {
		t.Errorf("texto restante inesperado: %q", remaining)
	}
}

// TestParseToolCalls_MultiplasTools verifica extração de múltiplos tool calls.
func TestParseToolCalls_MultiplasTools(t *testing.T) {
	response := "Vou consultar:\n```json\n{\"tool\": \"kubectl_get\", \"params\": {\"resource\": \"pods\"}}\n```\nE também:\n```json\n{\"tool\": \"kubectl_events\", \"params\": {\"namespace\": \"kube-system\"}}\n```\nPronto."

	calls, remaining := ParseToolCalls(response)

	if len(calls) != 2 {
		t.Fatalf("esperava 2 calls, obteve %d", len(calls))
	}
	if calls[0].Name != "kubectl_get" {
		t.Errorf("primeira tool esperada 'kubectl_get', obtida '%s'", calls[0].Name)
	}
	if calls[1].Name != "kubectl_events" {
		t.Errorf("segunda tool esperada 'kubectl_events', obtida '%s'", calls[1].Name)
	}
	if remaining != "Vou consultar:\n\nE também:\n\nPronto." {
		t.Errorf("texto restante inesperado: %q", remaining)
	}
}

// TestParseToolCalls_JSONInvalido verifica que JSON inválido é ignorado.
func TestParseToolCalls_JSONInvalido(t *testing.T) {
	response := "```json\n{\"tool\": \"test\", params: invalid}\n```"

	calls, _ := ParseToolCalls(response)

	if len(calls) != 0 {
		t.Errorf("esperava 0 calls para JSON inválido, obteve %d", len(calls))
	}
}

// TestParseToolCalls_SemNomeTool verifica que JSON sem campo tool é ignorado.
func TestParseToolCalls_SemNomeTool(t *testing.T) {
	response := "```json\n{\"params\": {\"resource\": \"pods\"}}\n```"

	calls, _ := ParseToolCalls(response)

	if len(calls) != 0 {
		t.Errorf("esperava 0 calls sem nome de tool, obteve %d", len(calls))
	}
}

// TestParseToolCalls_TextoVazio verifica comportamento com texto vazio.
func TestParseToolCalls_TextoVazio(t *testing.T) {
	calls, remaining := ParseToolCalls("")

	if len(calls) != 0 {
		t.Errorf("esperava 0 calls, obteve %d", len(calls))
	}
	if remaining != "" {
		t.Errorf("texto restante deveria estar vazio, obteve: %q", remaining)
	}
}

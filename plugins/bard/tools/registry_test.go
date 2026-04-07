package tools

import (
	"context"
	"strings"
	"testing"
)

// TestRegisterAndGet verifica registro e recuperação de ferramenta.
func TestRegisterAndGet(t *testing.T) {
	Reset()
	defer Reset()

	tool := &Tool{
		Name:        "test_tool",
		Description: "Ferramenta de teste",
		Parameters: []ToolParam{
			{Name: "arg1", Description: "Primeiro argumento", Required: true},
		},
		Execute: func(ctx context.Context, params map[string]string) (string, error) {
			return "ok", nil
		},
	}

	Register(tool)

	got := Get("test_tool")
	if got == nil {
		t.Fatal("ferramenta 'test_tool' não encontrada após registro")
	}
	if got.Name != "test_tool" {
		t.Errorf("nome esperado 'test_tool', obtido '%s'", got.Name)
	}
	if got.Description != "Ferramenta de teste" {
		t.Errorf("descrição inesperada: '%s'", got.Description)
	}
}

// TestGet_Inexistente verifica que Get retorna nil para ferramenta inexistente.
func TestGet_Inexistente(t *testing.T) {
	Reset()
	defer Reset()

	got := Get("nao_existe")
	if got != nil {
		t.Error("esperava nil para ferramenta inexistente")
	}
}

// TestAll verifica que All retorna todas as ferramentas registradas.
func TestAll(t *testing.T) {
	Reset()
	defer Reset()

	Register(&Tool{Name: "tool_a", Description: "A"})
	Register(&Tool{Name: "tool_b", Description: "B"})

	all := All()
	if len(all) != 2 {
		t.Fatalf("esperava 2 ferramentas, obteve %d", len(all))
	}

	names := map[string]bool{}
	for _, tool := range all {
		names[tool.Name] = true
	}
	if !names["tool_a"] || !names["tool_b"] {
		t.Errorf("ferramentas esperadas não encontradas: %v", names)
	}
}

// TestAll_Vazio verifica que All retorna slice vazio sem registros.
func TestAll_Vazio(t *testing.T) {
	Reset()
	defer Reset()

	all := All()
	if len(all) != 0 {
		t.Errorf("esperava 0 ferramentas, obteve %d", len(all))
	}
}

// TestFormatToolsPrompt_ComFerramentas verifica a formatação do prompt com ferramentas.
func TestFormatToolsPrompt_ComFerramentas(t *testing.T) {
	Reset()
	defer Reset()

	Register(&Tool{
		Name:        "kubectl_get",
		Description: "Executa kubectl get para listar recursos",
		Parameters: []ToolParam{
			{Name: "resource", Description: "Tipo de recurso", Required: true},
			{Name: "namespace", Description: "Namespace alvo", Required: false},
		},
	})

	prompt := FormatToolsPrompt()

	if !strings.Contains(prompt, "Ferramentas Disponíveis") {
		t.Error("prompt deveria conter cabeçalho")
	}
	if !strings.Contains(prompt, "kubectl_get") {
		t.Error("prompt deveria conter nome da ferramenta")
	}
	if !strings.Contains(prompt, "obrigatório") {
		t.Error("prompt deveria indicar parâmetros obrigatórios")
	}
	if !strings.Contains(prompt, "opcional") {
		t.Error("prompt deveria indicar parâmetros opcionais")
	}
}

// TestFormatToolsPrompt_SemFerramentas verifica retorno vazio sem ferramentas.
func TestFormatToolsPrompt_SemFerramentas(t *testing.T) {
	Reset()
	defer Reset()

	prompt := FormatToolsPrompt()
	if prompt != "" {
		t.Errorf("esperava string vazia, obteve: %q", prompt)
	}
}

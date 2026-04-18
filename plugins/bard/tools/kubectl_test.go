package tools

import (
	"context"
	"testing"
)

// TestKubectlTools_Registradas verifica que as 4 tools kubectl estão registradas via init().
func TestKubectlTools_Registradas(t *testing.T) {
	expectedTools := []string{"kubectl_get", "kubectl_logs", "kubectl_events", "kubectl_describe"}

	for _, name := range expectedTools {
		tool := Get(name)
		if tool == nil {
			t.Errorf("ferramenta '%s' não encontrada no registry", name)
			continue
		}
		if tool.Description == "" {
			t.Errorf("ferramenta '%s' sem descrição", name)
		}
		if tool.Execute == nil {
			t.Errorf("ferramenta '%s' sem função Execute", name)
		}
	}
}

// TestKubectlGet_SemResource verifica que kubectl_get exige o parâmetro resource.
func TestKubectlGet_SemResource(t *testing.T) {
	tool := Get("kubectl_get")
	if tool == nil {
		t.Fatal("ferramenta kubectl_get não encontrada")
	}

	_, err := tool.Execute(context.Background(), map[string]string{})
	if err == nil {
		t.Error("esperava erro quando resource está vazio")
	}
}

// TestKubectlGet_Parametros verifica que kubectl_get tem os parâmetros corretos.
func TestKubectlGet_Parametros(t *testing.T) {
	tool := Get("kubectl_get")
	if tool == nil {
		t.Fatal("ferramenta kubectl_get não encontrada")
	}

	if len(tool.Parameters) != 3 {
		t.Fatalf("esperava 3 parâmetros, obteve %d", len(tool.Parameters))
	}

	// resource deve ser obrigatório
	if !tool.Parameters[0].Required {
		t.Error("parâmetro 'resource' deveria ser obrigatório")
	}
	// namespace e output_format opcionais
	if tool.Parameters[1].Required {
		t.Error("parâmetro 'namespace' deveria ser opcional")
	}
	if tool.Parameters[2].Required {
		t.Error("parâmetro 'output_format' deveria ser opcional")
	}
}

// TestKubectlLogs_SemPod verifica que kubectl_logs exige o parâmetro pod.
func TestKubectlLogs_SemPod(t *testing.T) {
	tool := Get("kubectl_logs")
	if tool == nil {
		t.Fatal("ferramenta kubectl_logs não encontrada")
	}

	_, err := tool.Execute(context.Background(), map[string]string{})
	if err == nil {
		t.Error("esperava erro quando pod está vazio")
	}
}

// TestKubectlLogs_Parametros verifica parâmetros da tool logs.
func TestKubectlLogs_Parametros(t *testing.T) {
	tool := Get("kubectl_logs")
	if tool == nil {
		t.Fatal("ferramenta kubectl_logs não encontrada")
	}

	if len(tool.Parameters) != 3 {
		t.Fatalf("esperava 3 parâmetros, obteve %d", len(tool.Parameters))
	}

	if !tool.Parameters[0].Required {
		t.Error("parâmetro 'pod' deveria ser obrigatório")
	}
}

// TestKubectlEvents_Parametros verifica parâmetros da tool events.
func TestKubectlEvents_Parametros(t *testing.T) {
	tool := Get("kubectl_events")
	if tool == nil {
		t.Fatal("ferramenta kubectl_events não encontrada")
	}

	if len(tool.Parameters) != 1 {
		t.Fatalf("esperava 1 parâmetro, obteve %d", len(tool.Parameters))
	}

	if tool.Parameters[0].Required {
		t.Error("parâmetro 'namespace' deveria ser opcional")
	}
}

// TestKubectlDescribe_SemResource verifica que kubectl_describe exige resource.
func TestKubectlDescribe_SemResource(t *testing.T) {
	tool := Get("kubectl_describe")
	if tool == nil {
		t.Fatal("ferramenta kubectl_describe não encontrada")
	}

	_, err := tool.Execute(context.Background(), map[string]string{"name": "nginx"})
	if err == nil {
		t.Error("esperava erro quando resource está vazio")
	}
}

// TestKubectlDescribe_SemName verifica que kubectl_describe exige name.
func TestKubectlDescribe_SemName(t *testing.T) {
	tool := Get("kubectl_describe")
	if tool == nil {
		t.Fatal("ferramenta kubectl_describe não encontrada")
	}

	_, err := tool.Execute(context.Background(), map[string]string{"resource": "pod"})
	if err == nil {
		t.Error("esperava erro quando name está vazio")
	}
}

// TestKubectlDescribe_Parametros verifica parâmetros da tool describe.
func TestKubectlDescribe_Parametros(t *testing.T) {
	tool := Get("kubectl_describe")
	if tool == nil {
		t.Fatal("ferramenta kubectl_describe não encontrada")
	}

	if len(tool.Parameters) != 3 {
		t.Fatalf("esperava 3 parâmetros, obteve %d", len(tool.Parameters))
	}

	// resource e name obrigatórios
	if !tool.Parameters[0].Required {
		t.Error("parâmetro 'resource' deveria ser obrigatório")
	}
	if !tool.Parameters[1].Required {
		t.Error("parâmetro 'name' deveria ser obrigatório")
	}
	if tool.Parameters[2].Required {
		t.Error("parâmetro 'namespace' deveria ser opcional")
	}
}

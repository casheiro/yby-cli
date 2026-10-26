package tools

import (
	"context"
	"strings"
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

// semKubectl tira o kubectl do PATH e o Sentinel do HOME: o teste nunca chega a um
// cluster de verdade, e a validação, se faltar, aparece como "não encontrado" em vez do
// erro esperado.
func semKubectl(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	DefinirKube("", "")
}

// O conteúdo dos recursos (o data dos Secrets, em qualquer grafia) não vai à IA: o
// kubectl_get só lista em wide ou name.
func TestKubectlGet_FormatoQueMostraConteudoRecusado(t *testing.T) {
	semKubectl(t)
	for _, recurso := range []string{"secrets", "secret", "sec", "secret/db", "configmaps", "pods"} {
		for _, formato := range []string{"yaml", "json", "jsonpath={.data}", "go-template={{.data}}", "custom-columns=D:.data"} {
			_, err := executeKubectlGet(context.Background(), map[string]string{"resource": recurso, "output_format": formato})
			if err == nil || !strings.Contains(err.Error(), "formato") {
				t.Errorf("get %s -o %s deveria ser recusado pelo formato, veio: %v", recurso, formato, err)
			}
		}
	}
}

// Valor da IA começando com "-" (ou com espaço) viraria flag do kubectl ou do Sentinel.
func TestFerramentas_ValorDaIAQueViraFlag(t *testing.T) {
	semKubectl(t)
	casos := []struct {
		ferramenta string
		params     map[string]string
	}{
		{"kubectl_get", map[string]string{"resource": "--raw=/api/v1/namespaces/x/secrets/y"}},
		{"kubectl_get", map[string]string{"resource": "pods", "namespace": "--as=system:admin"}},
		{"kubectl_get", map[string]string{"resource": "pods -o yaml"}},
		{"kubectl_logs", map[string]string{"pod": "--as=system:admin"}},
		{"kubectl_logs", map[string]string{"pod": "api", "tail": "--as=system:admin"}},
		{"kubectl_logs", map[string]string{"pod": "api", "namespace": "-A"}},
		{"kubectl_describe", map[string]string{"resource": "-f", "name": "x"}},
		{"kubectl_describe", map[string]string{"resource": "pod", "name": "--as=system:admin"}},
		{"kubectl_events", map[string]string{"namespace": "--as=system:admin"}},
		{"sentinel_investigate", map[string]string{"pod": "--fix"}},
		{"sentinel_investigate", map[string]string{"pod": "api", "namespace": "--fix"}},
		{"sentinel_scan", map[string]string{"namespace": "--fix"}},
	}
	for _, c := range casos {
		_, err := Get(c.ferramenta).Execute(context.Background(), c.params)
		if err == nil || !strings.Contains(err.Error(), "inválido") {
			t.Errorf("%s %v deveria ser recusado como inválido, veio: %v", c.ferramenta, c.params, err)
		}
	}
}

// Os nomes que o kubectl aceita continuam passando, e wide e name continuam valendo.
func TestConferirParams_NomesValidos(t *testing.T) {
	validos := []map[string]string{
		{"resource": "pods", "namespace": "kube-system"},
		{"resource": "deployments.apps"},
		{"resource": "pod/nginx-7c9d"},
		{"resource": "pods,svc"},
		{"resource": "Deployment", "name": "api-v2.1"},
		{"pod": "worker-1", "tail": "100"},
	}
	for _, p := range validos {
		if err := conferirParams(p, "resource", "name", "pod", "namespace", "tail"); err != nil {
			t.Errorf("%v deveria passar: %v", p, err)
		}
	}
	semKubectl(t)
	for _, formato := range []string{"", "wide", "name", "WIDE"} {
		_, err := executeKubectlGet(context.Background(), map[string]string{"resource": "secrets", "output_format": formato})
		if err == nil || strings.Contains(err.Error(), "formato") {
			t.Errorf("-o %q deveria chegar ao kubectl (ausente no teste), veio: %v", formato, err)
		}
	}
}

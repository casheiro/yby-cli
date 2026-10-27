package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestAtlasBlueprint_Registrada verifica que a tool atlas_blueprint está registrada.
func TestAtlasBlueprint_Registrada(t *testing.T) {
	tool := Get("atlas_blueprint")
	if tool == nil {
		t.Fatal("ferramenta atlas_blueprint não encontrada no registry")
	}
	if tool.Description == "" {
		t.Error("ferramenta atlas_blueprint sem descrição")
	}
	if tool.Execute == nil {
		t.Error("ferramenta atlas_blueprint sem função Execute")
	}
}

// TestAtlasBlueprint_SemParametros verifica que atlas_blueprint não exige parâmetros.
func TestAtlasBlueprint_SemParametros(t *testing.T) {
	tool := Get("atlas_blueprint")
	if tool == nil {
		t.Fatal("ferramenta atlas_blueprint não encontrada")
	}
	if len(tool.Parameters) != 0 {
		t.Errorf("esperava 0 parâmetros, obteve %d", len(tool.Parameters))
	}
}

// TestAtlasBlueprint_SemBinario verifica erro quando binário não está disponível.
func TestAtlasBlueprint_SemBinario(t *testing.T) {
	tool := Get("atlas_blueprint")
	if tool == nil {
		t.Fatal("ferramenta atlas_blueprint não encontrada")
	}

	// Isolar HOME e diretório de trabalho para que discoverPluginBinary não encontre binários reais
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_, err := tool.Execute(context.Background(), map[string]string{})
	if err == nil {
		t.Error("esperava erro quando binário do atlas não está disponível")
	}
}

// TestFormatBlueprintResults_Nil verifica formatação com dados nil.
func TestFormatBlueprintResults_Nil(t *testing.T) {
	result := formatBlueprintResults(nil)
	if result != "Nenhum blueprint disponível." {
		t.Errorf("resultado inesperado: %q", result)
	}
}

// TestFormatBlueprintResults_ComDados verifica o resumo do Data que o Atlas manda (o
// contrato inteiro, gravado pelo Atlas, está em contrato_atlas_test.go).
func TestFormatBlueprintResults_ComDados(t *testing.T) {
	data := map[string]interface{}{
		"blueprint": map[string]interface{}{
			"components": []interface{}{
				map[string]interface{}{"name": "api", "type": "app", "path": "svc/api", "language": "go"},
				map[string]interface{}{"name": "web", "type": "app", "path": "web", "language": "nodejs"},
			},
			"relations": []interface{}{
				map[string]interface{}{"from": "web", "to": "svc/api", "type": "imports"},
			},
			"roots": []string{"."},
		},
	}

	result := formatBlueprintResults(data)
	for _, trecho := range []string{"Componentes (2)", "api (app, go) em svc/api", "web → api (imports)"} {
		if !strings.Contains(result, trecho) {
			t.Errorf("o resumo deveria ter %q:\n%s", trecho, result)
		}
	}
	if strings.Contains(result, "Workloads") {
		t.Errorf("sem a infra, o resumo não deveria ter workloads:\n%s", result)
	}
}

// Um Data sem blueprint nem infra (outro formato) não vira um resumo vazio com título.
func TestFormatBlueprintResults_FormatoDesconhecido(t *testing.T) {
	result := formatBlueprintResults(map[string]interface{}{"components": []interface{}{}})
	if result != "Nenhum blueprint disponível." {
		t.Errorf("resultado inesperado: %q", result)
	}
}

// Um projeto grande cabe em limiteDoBlueprint: as listas viram "e mais N" e o resumo é
// cortado numa linha inteira, com o aviso.
func TestFormatBlueprintResults_ProjetoGrande(t *testing.T) {
	var componentes, recursos, relacoes []interface{}
	for i := 0; i < 500; i++ {
		componentes = append(componentes, map[string]interface{}{
			"name": fmt.Sprintf("servico-com-nome-comprido-%03d", i), "type": "app",
			"path": fmt.Sprintf("servicos/dominio-%03d/servico-com-nome-comprido-%03d", i, i), "language": "go",
		})
		recursos = append(recursos,
			map[string]interface{}{"kind": "Deployment", "name": fmt.Sprintf("workload-com-nome-comprido-%03d", i), "namespace": "producao", "path": fmt.Sprintf("deploy/%03d.yaml", i)},
			map[string]interface{}{"kind": "ConfigMap", "name": fmt.Sprintf("cm-%03d", i), "path": fmt.Sprintf("deploy/%03d.yaml", i)},
		)
		relacoes = append(relacoes, map[string]interface{}{
			"from": fmt.Sprintf("Service/producao/workload-com-nome-comprido-%03d", i),
			"to":   fmt.Sprintf("Deployment/producao/workload-com-nome-comprido-%03d", i), "type": "selects",
		})
	}
	data := map[string]interface{}{
		"blueprint": map[string]interface{}{"components": componentes},
		"infra":     map[string]interface{}{"resources": recursos, "relations": relacoes},
	}

	result := formatBlueprintResults(data)
	if len(result) > limiteDoBlueprint {
		t.Errorf("o resumo tem %d bytes, acima de %d", len(result), limiteDoBlueprint)
	}
	for _, trecho := range []string{"Componentes (500)", "- … e mais 460", "ConfigMap: cm-000", "e mais 480", "Relações da infraestrutura (500)", "… resumo cortado"} {
		if !strings.Contains(result, trecho) {
			t.Errorf("o resumo deveria ter %q:\n%s", trecho, result)
		}
	}
}

func TestCortarResumo(t *testing.T) {
	texto := strings.Repeat("linha de exemplo ção\n", 100)
	cortado := cortarResumo(texto, 300)
	if len(cortado) > 300 {
		t.Errorf("o corte passou do limite: %d bytes", len(cortado))
	}
	if !strings.HasSuffix(cortado, "ção\n… resumo cortado no limite de 300 bytes\n") {
		t.Errorf("o corte deveria cair numa linha inteira e avisar:\n%q", cortado)
	}
	if cortarResumo("curto\n", 300) != "curto\n" {
		t.Error("texto dentro do limite não deveria mudar")
	}
}

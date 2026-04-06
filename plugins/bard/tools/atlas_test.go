package tools

import (
	"context"
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

// TestFormatBlueprintResults_ComDados verifica formatação com dados de blueprint.
func TestFormatBlueprintResults_ComDados(t *testing.T) {
	data := map[string]interface{}{
		"components": []interface{}{
			map[string]interface{}{"name": "api", "type": "service"},
			map[string]interface{}{"name": "web", "type": "frontend"},
			map[string]interface{}{"name": "db", "type": "service"},
		},
		"relations": []interface{}{
			map[string]interface{}{"from": "web", "to": "api"},
			map[string]interface{}{"from": "api", "to": "db"},
		},
		"languages": []string{"Go", "TypeScript"},
	}

	result := formatBlueprintResults(data)
	if result == "" {
		t.Error("resultado não deveria estar vazio")
	}
	if result == "Nenhum blueprint disponível." {
		t.Error("resultado não deveria ser o fallback de nil")
	}
}

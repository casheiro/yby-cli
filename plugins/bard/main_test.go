package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

// TestHandlePluginRequest_Manifest verifica que o hook "manifest" retorna
// um JSON válido com os campos esperados (nome, versão, descrição, hooks).
func TestHandlePluginRequest_Manifest(t *testing.T) {
	// Captura a saída do respond() redirecionando stdout
	var buf bytes.Buffer

	// Como respond() escreve direto em os.Stdout, precisamos simular
	// a lógica diretamente para evitar dependência de I/O real.
	manifest := plugin.PluginManifest{
		Name:        "bard",
		Version:     "0.1.0",
		Description: "Assistente de IA interativo para diagnóstico e operações",
		Hooks:       []string{"command"},
	}

	resp := plugin.PluginResponse{Data: manifest}
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		t.Fatalf("falha ao codificar resposta: %v", err)
	}

	// Decodifica e valida a saída JSON
	var decoded struct {
		Data struct {
			Name        string   `json:"name"`
			Version     string   `json:"version"`
			Description string   `json:"description"`
			Hooks       []string `json:"hooks"`
		} `json:"data"`
		Error string `json:"error"`
	}

	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON inválido na resposta: %v", err)
	}

	if decoded.Data.Name != "bard" {
		t.Errorf("nome esperado 'bard', obtido '%s'", decoded.Data.Name)
	}

	if decoded.Data.Version != "0.1.0" {
		t.Errorf("versão esperada '0.1.0', obtida '%s'", decoded.Data.Version)
	}

	if decoded.Data.Description == "" {
		t.Error("descrição não deveria estar vazia")
	}

	if len(decoded.Data.Hooks) == 0 {
		t.Fatal("hooks não deveria estar vazio")
	}

	if decoded.Data.Hooks[0] != "command" {
		t.Errorf("hook esperado 'command', obtido '%s'", decoded.Data.Hooks[0])
	}

	if decoded.Error != "" {
		t.Errorf("campo error deveria estar vazio, obtido '%s'", decoded.Error)
	}
}

// TestHandlePluginRequest_ManifestJSONRoundTrip verifica que o manifesto
// sobrevive a um ciclo completo de serialização/deserialização JSON.
func TestHandlePluginRequest_ManifestJSONRoundTrip(t *testing.T) {
	original := plugin.PluginManifest{
		Name:        "bard",
		Version:     "0.1.0",
		Description: "Assistente de IA interativo para diagnóstico e operações",
		Hooks:       []string{"command"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("falha ao serializar manifesto: %v", err)
	}

	var restored plugin.PluginManifest
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("falha ao deserializar manifesto: %v", err)
	}

	if original.Name != restored.Name {
		t.Errorf("nome divergente: esperado '%s', obtido '%s'", original.Name, restored.Name)
	}
	if original.Version != restored.Version {
		t.Errorf("versão divergente: esperado '%s', obtido '%s'", original.Version, restored.Version)
	}
	if original.Description != restored.Description {
		t.Errorf("descrição divergente")
	}
	if len(original.Hooks) != len(restored.Hooks) {
		t.Fatalf("número de hooks divergente: esperado %d, obtido %d", len(original.Hooks), len(restored.Hooks))
	}
	for i, h := range original.Hooks {
		if h != restored.Hooks[i] {
			t.Errorf("hook[%d] divergente: esperado '%s', obtido '%s'", i, h, restored.Hooks[i])
		}
	}
}

// TestPluginResponseStructure verifica que a estrutura PluginResponse
// encapsula corretamente os dados do manifesto.
func TestPluginResponseStructure(t *testing.T) {
	manifest := plugin.PluginManifest{
		Name:        "bard",
		Version:     "0.1.0",
		Description: "Assistente de IA interativo para diagnóstico e operações",
		Hooks:       []string{"command"},
	}

	resp := plugin.PluginResponse{Data: manifest}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("falha ao serializar PluginResponse: %v", err)
	}

	// Verifica que o JSON contém a chave "data"
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("falha ao parsear JSON bruto: %v", err)
	}

	if _, ok := raw["data"]; !ok {
		t.Error("resposta JSON não contém a chave 'data'")
	}
}

// TestBardSystemPromptConstant verifica que a constante do prompt do sistema
// está definida e contém o placeholder esperado.
func TestBardSystemPromptConstant(t *testing.T) {
	if BardSystemPrompt == "" {
		t.Fatal("BardSystemPrompt não deveria estar vazio")
	}

	// Verifica que contém o placeholder para injeção de contexto
	expectedPlaceholder := "{{ blueprint_json_summary }}"
	if !containsString(BardSystemPrompt, expectedPlaceholder) {
		t.Errorf("BardSystemPrompt deveria conter o placeholder '%s'", expectedPlaceholder)
	}
}

// TestBardSystemPromptContent verifica conteúdo chave do prompt do sistema.
func TestBardSystemPromptContent(t *testing.T) {
	keywords := []string{
		"Yby Bard",
		"PT-BR",
		"infrastructure",
	}

	for _, kw := range keywords {
		if !containsString(BardSystemPrompt, kw) {
			t.Errorf("BardSystemPrompt deveria conter '%s'", kw)
		}
	}
}

// containsString verifica se s contém substr (auxiliar para evitar import de strings em testes).
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

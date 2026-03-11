package main

import (
	"encoding/json"
	"testing"
)

// TestPluginManifestJSON verifica que o manifesto do plugin viz
// serializa corretamente para JSON com todos os campos esperados.
func TestPluginManifestJSON(t *testing.T) {
	manifest := PluginManifest{
		Name:        "viz",
		Description: "Observabilidade visual no terminal (Dashboards TUI)",
		Version:     "0.1.0",
		Hooks:       []string{"command"},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("falha ao serializar manifesto: %v", err)
	}

	var decoded PluginManifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("falha ao deserializar manifesto: %v", err)
	}

	if decoded.Name != "viz" {
		t.Errorf("nome esperado 'viz', obtido '%s'", decoded.Name)
	}

	if decoded.Version != "0.1.0" {
		t.Errorf("versão esperada '0.1.0', obtida '%s'", decoded.Version)
	}

	if decoded.Description == "" {
		t.Error("descrição não deveria estar vazia")
	}

	if len(decoded.Hooks) != 1 || decoded.Hooks[0] != "command" {
		t.Errorf("hooks esperado ['command'], obtido %v", decoded.Hooks)
	}
}

// TestPluginResponseJSON verifica que a resposta encapsula corretamente
// os dados do manifesto no formato esperado pelo protocolo de plugins.
func TestPluginResponseJSON(t *testing.T) {
	manifest := PluginManifest{
		Name:        "viz",
		Description: "Observabilidade visual no terminal (Dashboards TUI)",
		Version:     "0.1.0",
		Hooks:       []string{"command"},
	}

	resp := PluginResponse{Data: manifest}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("falha ao serializar resposta: %v", err)
	}

	// Verifica estrutura do JSON de resposta
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("falha ao parsear JSON bruto: %v", err)
	}

	if _, ok := raw["data"]; !ok {
		t.Error("resposta JSON não contém a chave 'data'")
	}

	// Decodifica o conteúdo completo para verificar valores
	var fullResp struct {
		Data PluginManifest `json:"data"`
	}
	if err := json.Unmarshal(data, &fullResp); err != nil {
		t.Fatalf("falha ao decodificar resposta completa: %v", err)
	}

	if fullResp.Data.Name != "viz" {
		t.Errorf("nome no data esperado 'viz', obtido '%s'", fullResp.Data.Name)
	}
}

// TestPluginManifestJSONRoundTrip verifica que o manifesto sobrevive
// a um ciclo completo de serialização e deserialização.
func TestPluginManifestJSONRoundTrip(t *testing.T) {
	original := PluginManifest{
		Name:        "viz",
		Description: "Observabilidade visual no terminal (Dashboards TUI)",
		Version:     "0.1.0",
		Hooks:       []string{"command"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("falha ao serializar: %v", err)
	}

	var restored PluginManifest
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("falha ao deserializar: %v", err)
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

// TestPluginRequestJSON verifica que a estrutura PluginRequest
// serializa e deserializa corretamente o hook.
func TestPluginRequestJSON(t *testing.T) {
	tests := []struct {
		name string
		hook string
	}{
		{name: "hook manifest", hook: "manifest"},
		{name: "hook command", hook: "command"},
		{name: "hook vazio", hook: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := PluginRequest{Hook: tt.hook}
			data, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("falha ao serializar: %v", err)
			}

			var decoded PluginRequest
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("falha ao deserializar: %v", err)
			}

			if decoded.Hook != tt.hook {
				t.Errorf("hook esperado '%s', obtido '%s'", tt.hook, decoded.Hook)
			}
		})
	}
}

// TestManifestResponseFormat verifica que a resposta do manifesto segue
// exatamente o formato map[string]interface{}{"data": manifest} usado
// pelo subcomando cobra "manifest".
func TestManifestResponseFormat(t *testing.T) {
	manifest := PluginManifest{
		Name:        "viz",
		Description: "Observabilidade visual no terminal (Dashboards TUI)",
		Version:     "0.1.0",
		Hooks:       []string{"command"},
	}

	// Simula o formato exato usado no manifestCmd
	envelope := map[string]interface{}{
		"data": manifest,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("falha ao serializar envelope: %v", err)
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("falha ao parsear envelope: %v", err)
	}

	dataRaw, ok := parsed["data"]
	if !ok {
		t.Fatal("envelope não contém chave 'data'")
	}

	var m PluginManifest
	if err := json.Unmarshal(dataRaw, &m); err != nil {
		t.Fatalf("falha ao decodificar manifesto do envelope: %v", err)
	}

	if m.Name != "viz" {
		t.Errorf("nome esperado 'viz', obtido '%s'", m.Name)
	}
	if m.Version != "0.1.0" {
		t.Errorf("versão esperada '0.1.0', obtida '%s'", m.Version)
	}
}

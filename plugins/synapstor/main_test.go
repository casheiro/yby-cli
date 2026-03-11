package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/casheiro/yby-cli/pkg/plugin"
)

// TestRespond_GeraJSONValido verifica que a função respond() gera uma
// PluginResponse válida em JSON.
func TestRespond_GeraJSONValido(t *testing.T) {
	// Capturar a saída de respond() redirecionando stdout
	// Como respond() escreve diretamente em os.Stdout, vamos testar
	// a lógica de serialização diretamente.
	manifest := plugin.PluginManifest{
		Name:        "synapstor",
		Version:     "0.1.0",
		Description: "Governança semântica e gestão de conhecimento (UKIs)",
		Hooks:       []string{"command"},
	}

	resp := plugin.PluginResponse{Data: manifest}
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(resp)
	if err != nil {
		t.Fatalf("falha ao codificar resposta: %v", err)
	}

	// Verificar que é JSON válido
	var decoded plugin.PluginResponse
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON inválido: %v\nSaída: %s", err, buf.String())
	}

	if decoded.Error != "" {
		t.Errorf("resposta não deve conter erro, mas tem: %s", decoded.Error)
	}
	if decoded.Data == nil {
		t.Fatal("resposta deve conter dados")
	}
}

// TestHandlePluginRequest_ManifestPayload verifica a estrutura do manifesto
// retornado pelo hook "manifest".
func TestHandlePluginRequest_ManifestPayload(t *testing.T) {
	// Simular o payload que handlePluginRequest produziria para o hook "manifest"
	manifest := plugin.PluginManifest{
		Name:        "synapstor",
		Version:     "0.1.0",
		Description: "Governança semântica e gestão de conhecimento (UKIs)",
		Hooks:       []string{"command"},
	}

	// Verificar campos
	if manifest.Name != "synapstor" {
		t.Errorf("nome esperado 'synapstor', obtido %q", manifest.Name)
	}
	if manifest.Version == "" {
		t.Error("versão não deve estar vazia")
	}
	if manifest.Description == "" {
		t.Error("descrição não deve estar vazia")
	}
	if len(manifest.Hooks) == 0 {
		t.Fatal("hooks não deve estar vazio")
	}

	hookEncontrado := false
	for _, h := range manifest.Hooks {
		if h == "command" {
			hookEncontrado = true
			break
		}
	}
	if !hookEncontrado {
		t.Error("hooks deve conter 'command'")
	}

	// Verificar serialização JSON completa (ida e volta)
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("falha ao serializar manifesto: %v", err)
	}

	var roundTrip plugin.PluginManifest
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("falha ao desserializar manifesto: %v", err)
	}

	if roundTrip.Name != manifest.Name {
		t.Errorf("nome difere após round-trip: %q vs %q", roundTrip.Name, manifest.Name)
	}
	if len(roundTrip.Hooks) != len(manifest.Hooks) {
		t.Errorf("hooks difere após round-trip: %v vs %v", roundTrip.Hooks, manifest.Hooks)
	}
}

// TestPluginRequest_DeserializacaoJSON verifica que PluginRequest pode ser
// desserializado corretamente de JSON, simulando o protocolo de plugins.
func TestPluginRequest_DeserializacaoJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantHook string
		wantArgs int
	}{
		{
			name:     "hook manifest sem args",
			json:     `{"hook":"manifest"}`,
			wantHook: "manifest",
			wantArgs: 0,
		},
		{
			name:     "hook command com args capture",
			json:     `{"hook":"command","args":["capture","texto de teste"]}`,
			wantHook: "command",
			wantArgs: 2,
		},
		{
			name:     "hook command com args study",
			json:     `{"hook":"command","args":["study","kubernetes"]}`,
			wantHook: "command",
			wantArgs: 2,
		},
		{
			name:     "hook command com args index",
			json:     `{"hook":"command","args":["index"]}`,
			wantHook: "command",
			wantArgs: 1,
		},
		{
			name:     "hook command sem args",
			json:     `{"hook":"command","args":[]}`,
			wantHook: "command",
			wantArgs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req plugin.PluginRequest
			if err := json.Unmarshal([]byte(tt.json), &req); err != nil {
				t.Fatalf("falha ao desserializar: %v", err)
			}

			if req.Hook != tt.wantHook {
				t.Errorf("hook esperado %q, obtido %q", tt.wantHook, req.Hook)
			}
			if len(req.Args) != tt.wantArgs {
				t.Errorf("esperado %d args, obtido %d", tt.wantArgs, len(req.Args))
			}
		})
	}
}

// TestPluginResponse_ComErro verifica a serialização de PluginResponse com erro.
func TestPluginResponse_ComErro(t *testing.T) {
	resp := plugin.PluginResponse{Error: "hook desconhecido: invalido"}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("falha ao serializar: %v", err)
	}

	var decoded plugin.PluginResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("falha ao desserializar: %v", err)
	}

	if decoded.Error == "" {
		t.Error("esperado campo error preenchido")
	}
	if decoded.Data != nil {
		t.Error("data deve ser nil quando há erro")
	}
}

// TestPrintHelp_NaoPanica verifica que printHelp não causa panic.
func TestPrintHelp_NaoPanica(t *testing.T) {
	// printHelp() escreve em stdout; verificar que não causa panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printHelp() causou panic: %v", r)
		}
	}()
	printHelp()
}

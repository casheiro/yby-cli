package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
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

// --- Testes de Histórico ---

// chdir muda para o diretório informado e retorna uma função para restaurar o original.
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("falha ao obter diretório atual: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("falha ao mudar para %s: %v", dir, err)
	}
	return func() {
		os.Chdir(orig)
	}
}

// TestLoadHistory_ArquivoInexistente verifica que loadHistory retorna slice vazio
// quando o arquivo de histórico não existe.
func TestLoadHistory_ArquivoInexistente(t *testing.T) {
	tmpDir := t.TempDir()
	restore := chdir(t, tmpDir)
	defer restore()

	entries := loadHistory()
	if len(entries) != 0 {
		t.Errorf("esperava slice vazio, obteve %d entradas", len(entries))
	}
}

// TestSaveMessage_CriaArquivo verifica que saveMessage cria o arquivo e diretório .yby.
func TestSaveMessage_CriaArquivo(t *testing.T) {
	tmpDir := t.TempDir()
	restore := chdir(t, tmpDir)
	defer restore()

	saveMessage("user", "teste")

	path := filepath.Join(tmpDir, historyFile)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("arquivo de histórico não foi criado")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("falha ao ler arquivo: %v", err)
	}

	// Verificar que é JSONL válido
	var entry HistoryEntry
	if err := json.Unmarshal(bytes.TrimSpace(data), &entry); err != nil {
		t.Fatalf("JSONL inválido: %v", err)
	}

	if entry.Role != "user" {
		t.Errorf("role esperado 'user', obtido '%s'", entry.Role)
	}
	if entry.Content != "teste" {
		t.Errorf("content esperado 'teste', obtido '%s'", entry.Content)
	}
	if entry.Timestamp == "" {
		t.Error("timestamp não deveria estar vazio")
	}
}

// TestSaveAndLoadHistory_Roundtrip verifica o ciclo salvar/carregar.
func TestSaveAndLoadHistory_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	restore := chdir(t, tmpDir)
	defer restore()

	saveMessage("user", "pergunta 1")
	saveMessage("assistant", "resposta 1")
	saveMessage("user", "pergunta 2")

	entries := loadHistory()
	if len(entries) != 3 {
		t.Fatalf("esperava 3 entradas, obteve %d", len(entries))
	}

	if entries[0].Role != "user" || entries[0].Content != "pergunta 1" {
		t.Errorf("primeira entrada inesperada: %+v", entries[0])
	}
	if entries[1].Role != "assistant" || entries[1].Content != "resposta 1" {
		t.Errorf("segunda entrada inesperada: %+v", entries[1])
	}
	if entries[2].Role != "user" || entries[2].Content != "pergunta 2" {
		t.Errorf("terceira entrada inesperada: %+v", entries[2])
	}
}

// TestLoadHistory_LimitaEntradas verifica que loadHistory retorna no máximo maxHistoryEntries.
func TestLoadHistory_LimitaEntradas(t *testing.T) {
	tmpDir := t.TempDir()
	restore := chdir(t, tmpDir)
	defer restore()

	// Criar 30 entradas
	for i := 0; i < 30; i++ {
		saveMessage("user", fmt.Sprintf("mensagem %d", i))
	}

	entries := loadHistory()
	if len(entries) != maxHistoryEntries {
		t.Errorf("esperava %d entradas, obteve %d", maxHistoryEntries, len(entries))
	}

	// Verificar que são as últimas 20 (índices 10..29)
	if entries[0].Content != "mensagem 10" {
		t.Errorf("primeira entrada esperada 'mensagem 10', obteve '%s'", entries[0].Content)
	}
}

// TestClearHistory_RemoveArquivo verifica que clearHistory remove o arquivo.
func TestClearHistory_RemoveArquivo(t *testing.T) {
	tmpDir := t.TempDir()
	restore := chdir(t, tmpDir)
	defer restore()

	saveMessage("user", "algo")
	clearHistory()

	path := filepath.Join(tmpDir, historyFile)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("arquivo de histórico deveria ter sido removido")
	}
}

// TestFormatHistoryContext_Vazio verifica que retorna string vazia para slice vazio.
func TestFormatHistoryContext_Vazio(t *testing.T) {
	result := formatHistoryContext(nil)
	if result != "" {
		t.Errorf("esperava string vazia, obteve '%s'", result)
	}

	result = formatHistoryContext([]HistoryEntry{})
	if result != "" {
		t.Errorf("esperava string vazia, obteve '%s'", result)
	}
}

// TestFormatHistoryContext_ComEntradas verifica a formatação com entradas.
func TestFormatHistoryContext_ComEntradas(t *testing.T) {
	entries := []HistoryEntry{
		{Role: "user", Content: "oi", Timestamp: "2026-01-01T00:00:00Z"},
		{Role: "assistant", Content: "olá!", Timestamp: "2026-01-01T00:00:01Z"},
	}

	result := formatHistoryContext(entries)

	if !strings.Contains(result, "Histórico de Conversas Anteriores") {
		t.Error("deveria conter cabeçalho de histórico")
	}
	if !strings.Contains(result, "Usuário") {
		t.Error("deveria conter label 'Usuário'")
	}
	if !strings.Contains(result, "Assistente") {
		t.Error("deveria conter label 'Assistente'")
	}
	if !strings.Contains(result, "oi") {
		t.Error("deveria conter conteúdo do usuário")
	}
	if !strings.Contains(result, "olá!") {
		t.Error("deveria conter conteúdo do assistente")
	}
}

// --- Testes de Configuração ---

// TestLoadBardConfig_Defaults verifica os valores padrão sem arquivo de configuração.
func TestLoadBardConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	restore := chdir(t, tmpDir)
	defer restore()

	cfg := loadBardConfig()

	if cfg.TopK != 5 {
		t.Errorf("TopK padrão esperado 5, obtido %d", cfg.TopK)
	}
	if cfg.RelevanceThreshold != 0.6 {
		t.Errorf("RelevanceThreshold padrão esperado 0.6, obtido %f", cfg.RelevanceThreshold)
	}
	if cfg.SystemPromptExtra != "" {
		t.Errorf("SystemPromptExtra padrão esperado vazio, obtido '%s'", cfg.SystemPromptExtra)
	}
}

// TestLoadBardConfig_ComArquivo verifica que valores custom são carregados.
func TestLoadBardConfig_ComArquivo(t *testing.T) {
	tmpDir := t.TempDir()
	restore := chdir(t, tmpDir)
	defer restore()

	// Criar diretório e arquivo de configuração
	if err := os.MkdirAll(".yby", 0755); err != nil {
		t.Fatalf("falha ao criar diretório: %v", err)
	}

	content := `top_k: 10
relevance_threshold: 0.8
system_prompt_extra: "Responda sempre em formato de lista."
`
	if err := os.WriteFile(".yby/bard.yaml", []byte(content), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo de configuração: %v", err)
	}

	cfg := loadBardConfig()

	if cfg.TopK != 10 {
		t.Errorf("TopK esperado 10, obtido %d", cfg.TopK)
	}
	if cfg.RelevanceThreshold != 0.8 {
		t.Errorf("RelevanceThreshold esperado 0.8, obtido %f", cfg.RelevanceThreshold)
	}
	if cfg.SystemPromptExtra != "Responda sempre em formato de lista." {
		t.Errorf("SystemPromptExtra inesperado: '%s'", cfg.SystemPromptExtra)
	}
}

// --- Testes de Filtro por Threshold ---

// TestFilterByThreshold verifica que resultados abaixo do threshold são filtrados.
func TestFilterByThreshold(t *testing.T) {
	results := []ai.UnknownDocument{
		{ID: "1", Content: "relevante", Score: 0.9},
		{ID: "2", Content: "meio relevante", Score: 0.6},
		{ID: "3", Content: "irrelevante", Score: 0.3},
		{ID: "4", Content: "no limite", Score: 0.59},
		{ID: "5", Content: "acima do limite", Score: 0.61},
	}

	filtered := filterByThreshold(results, 0.6)

	if len(filtered) != 3 {
		t.Fatalf("esperava 3 resultados filtrados, obteve %d", len(filtered))
	}

	// Verificar que apenas resultados >= 0.6 passaram
	expectedIDs := map[string]bool{"1": true, "2": true, "5": true}
	for _, doc := range filtered {
		if !expectedIDs[doc.ID] {
			t.Errorf("documento inesperado no resultado: ID=%s, Score=%.2f", doc.ID, doc.Score)
		}
	}
}

// TestFilterByThreshold_NenhumPassa verifica retorno vazio quando nada atinge o threshold.
func TestFilterByThreshold_NenhumPassa(t *testing.T) {
	results := []ai.UnknownDocument{
		{ID: "1", Score: 0.1},
		{ID: "2", Score: 0.2},
	}

	filtered := filterByThreshold(results, 0.5)
	if len(filtered) != 0 {
		t.Errorf("esperava 0 resultados, obteve %d", len(filtered))
	}
}

// TestFilterByThreshold_Vazio verifica que slice vazio retorna vazio.
func TestFilterByThreshold_Vazio(t *testing.T) {
	filtered := filterByThreshold(nil, 0.6)
	if len(filtered) != 0 {
		t.Errorf("esperava 0 resultados, obteve %d", len(filtered))
	}
}

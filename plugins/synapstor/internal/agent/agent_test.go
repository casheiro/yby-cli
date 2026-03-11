package agent

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
)

// MockProvider implements ai.Provider for testing
type MockProvider struct {
	Response string
	Err      error
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) IsAvailable(ctx context.Context) bool {
	return true
}

func (m *MockProvider) GenerateGovernance(ctx context.Context, description string) (*ai.GovernanceBlueprint, error) {
	return nil, nil
}

func (m *MockProvider) Completion(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return m.Response, m.Err
}

func (m *MockProvider) StreamCompletion(ctx context.Context, systemPrompt, userPrompt string, out io.Writer) error {
	return nil
}

func (m *MockProvider) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	// Mock implementation: return empty vectors or random
	return make([][]float32, len(texts)), nil
}

func TestCapture(t *testing.T) {
	// Create temp dir for test
	tmpDir, err := os.MkdirTemp("", "synapstor-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock AI Response
	mockJson := `{
		"title": "Test UKI",
		"filename": "UKI-123-test.md",
		"content": "**ID:** UKI-MOCK-TEST\n# Test UKI\nContent here.",
		"summary": "Summary"
	}`

	mockProvider := &MockProvider{
		Response: mockJson,
	}

	agent := NewAgent(mockProvider, tmpDir)

	err = agent.Capture("Test input")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	// Verify file creation
	expectedPath := filepath.Join(tmpDir, ".synapstor", ".uki", "UKI-123-test.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected UKI file was not created at %s", expectedPath)
	}

	content, _ := os.ReadFile(expectedPath)
	// Check for semantic ID format
	if !strings.Contains(string(content), "**ID:** UKI-") {
		t.Errorf("Expected content to contain semantic ID format, but got: %s", string(content))
	}
	// Original content check (assuming it should still be there or modified)
	if !strings.Contains(string(content), "# Test UKI\nContent here.") {
		t.Errorf("Unexpected content: %s", string(content))
	}
}

func TestStudy_NoFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "synapstor-test-study")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mockProvider := &MockProvider{Response: "{}"}
	agent := NewAgent(mockProvider, tmpDir)

	// Should not fail, just find nothing
	err = agent.Study("nonexistent")
	if err != nil {
		t.Fatalf("Study failed: %v", err)
	}
}

// TestValidateResponse_CamposObrigatorios verifica a validação dos campos obrigatórios.
func TestValidateResponse_CamposObrigatorios(t *testing.T) {
	tests := []struct {
		name    string
		resp    SynapstorResponse
		wantErr bool
	}{
		{
			name:    "resposta válida",
			resp:    SynapstorResponse{Title: "Teste", Filename: "UKI-123-teste.md", Content: "conteúdo"},
			wantErr: false,
		},
		{
			name:    "título vazio",
			resp:    SynapstorResponse{Title: "", Filename: "UKI-123-teste.md", Content: "conteúdo"},
			wantErr: true,
		},
		{
			name:    "filename vazio",
			resp:    SynapstorResponse{Title: "Teste", Filename: "", Content: "conteúdo"},
			wantErr: true,
		},
		{
			name:    "content vazio",
			resp:    SynapstorResponse{Title: "Teste", Filename: "UKI-123-teste.md", Content: ""},
			wantErr: true,
		},
		{
			name:    "apenas espaços em branco",
			resp:    SynapstorResponse{Title: "  ", Filename: "UKI-123-teste.md", Content: "conteúdo"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateResponse(&tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateResponse_NormalizaFilename verifica que filenames sem prefixo UKI são normalizados.
func TestValidateResponse_NormalizaFilename(t *testing.T) {
	resp := SynapstorResponse{
		Title:    "Meu Documento",
		Filename: "documento.md",
		Content:  "conteúdo",
	}
	err := validateResponse(&resp)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !strings.HasPrefix(resp.Filename, "UKI-") {
		t.Errorf("filename deveria começar com 'UKI-', obtido %q", resp.Filename)
	}
	if !strings.HasSuffix(resp.Filename, ".md") {
		t.Errorf("filename deveria terminar com '.md', obtido %q", resp.Filename)
	}
}

// TestValidateResponse_AdicionaExtensaoMd verifica que a extensão .md é adicionada se ausente.
func TestValidateResponse_AdicionaExtensaoMd(t *testing.T) {
	resp := SynapstorResponse{
		Title:    "Teste",
		Filename: "UKI-123-teste",
		Content:  "conteúdo",
	}
	err := validateResponse(&resp)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if !strings.HasSuffix(resp.Filename, ".md") {
		t.Errorf("filename deveria terminar com '.md', obtido %q", resp.Filename)
	}
}

// TestSaveResponse_ComValidacaoERetry verifica que saveResponse faz retry quando a resposta é inválida.
func TestSaveResponse_ComValidacaoERetry(t *testing.T) {
	tmpDir := t.TempDir()

	// Primeira resposta inválida (título vazio), segunda resposta válida
	callCount := 0
	mockProvider := &MockProvider{
		Response: `{"title": "Corrigido", "filename": "UKI-123-corrigido.md", "content": "conteúdo corrigido", "summary": "resumo"}`,
	}

	// Simular resposta inválida na primeira chamada
	agent := NewAgent(mockProvider, tmpDir)

	// Usar diretamente saveResponse com JSON inválido (título vazio)
	invalidJson := `{"title": "", "filename": "UKI-123-teste.md", "content": "conteúdo", "summary": "resumo"}`
	err := agent.saveResponse(invalidJson, "Teste")
	if err != nil {
		t.Fatalf("saveResponse deveria ter corrigido via retry, mas falhou: %v", err)
	}

	_ = callCount // Evitar warning de variável não utilizada

	// Verificar que o arquivo corrigido foi criado
	files, _ := os.ReadDir(filepath.Join(tmpDir, ".synapstor", ".uki"))
	if len(files) == 0 {
		t.Error("esperado pelo menos um arquivo UKI criado após retry")
	}
}

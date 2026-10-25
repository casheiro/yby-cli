package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
	yerrors "github.com/casheiro/yby-cli/pkg/errors"
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

	res, err := agent.Capture(context.Background(), "Test input")
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}
	if res.Titulo != "Test UKI" || res.Arquivo != filepath.Join(".synapstor", ".uki", "UKI-123-test.md") {
		t.Errorf("resultado inesperado: %+v", res)
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

	// Sem arquivos sobre o tópico, devolve ErrSemArquivos sem chamar a IA
	res, err := agent.Study(context.Background(), "nonexistent")
	if !errors.Is(err, ErrSemArquivos) || res != nil {
		t.Fatalf("esperado ErrSemArquivos, obtido %v (%+v)", err, res)
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
	res, err := agent.saveResponse(context.Background(), invalidJson)
	if err != nil {
		t.Fatalf("saveResponse deveria ter corrigido via retry, mas falhou: %v", err)
	}
	if !res.Corrigida || res.Titulo != "Corrigido" {
		t.Errorf("esperado UKI corrigido, obtido %+v", res)
	}

	_ = callCount // Evitar warning de variável não utilizada

	// Verificar que o arquivo corrigido foi criado
	files, _ := os.ReadDir(filepath.Join(tmpDir, ".synapstor", ".uki"))
	if len(files) == 0 {
		t.Error("esperado pelo menos um arquivo UKI criado após retry")
	}
}

// TestCapture_FalhaDaIAViraYbyError verifica que a falha do provider sai como YbyError
// com o nome dele nos detalhes.
func TestCapture_FalhaDaIAViraYbyError(t *testing.T) {
	agt := NewAgent(&MockProvider{Err: errors.New("conexão recusada")}, t.TempDir())
	_, err := agt.Capture(context.Background(), "texto")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodeExec || ye.Context["provider"] != "mock" {
		t.Fatalf("esperado YbyError ERR_EXEC_FAILED com o provider, obtido %#v", err)
	}
}

// TestCapture_RespostaSemJSON verifica o erro de quando a IA não devolve um UKI.
func TestCapture_RespostaSemJSON(t *testing.T) {
	agt := NewAgent(&MockProvider{Response: "não sei"}, t.TempDir())
	_, err := agt.Capture(context.Background(), "texto")
	var ye *yerrors.YbyError
	if !errors.As(err, &ye) || ye.Code != yerrors.ErrCodePlugin || ye.GetHint() == "" {
		t.Fatalf("esperado YbyError ERR_PLUGIN com hint, obtido %#v", err)
	}
}

// TestCapture_NomeDoArquivoFicaNaPastaDosUKIs verifica que um filename ou um título com
// separadores de pasta, vindo da IA, grava um único UKI dentro de .synapstor/.uki, com o
// prefixo UKI-, e nada fora dela.
func TestCapture_NomeDoArquivoFicaNaPastaDosUKIs(t *testing.T) {
	casos := []struct{ nome, titulo, filename string }{
		{"barra", "Fuga", "UKI-1/../../../fuga.md"},
		{"contrabarra", "Fuga", `UKI-1\..\..\..\fuga.md`},
		{"título com barra", "CI/CD no GitLab", "cicd.md"},
		{"título com contrabarra", `..\..\fuga`, "fuga.md"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			raiz := t.TempDir()
			b, _ := json.Marshal(SynapstorResponse{Title: c.titulo, Filename: c.filename, Content: "# Fuga"})
			res, err := NewAgent(&MockProvider{Response: string(b)}, raiz).Capture(context.Background(), "texto")
			if err != nil {
				t.Fatal(err)
			}
			pasta := filepath.Join(".synapstor", ".uki")
			if filepath.Dir(res.Arquivo) != pasta || !strings.HasPrefix(filepath.Base(res.Arquivo), "UKI-") {
				t.Errorf("UKI fora da pasta ou sem o prefixo UKI-: %s", res.Arquivo)
			}
			entradas, err := os.ReadDir(filepath.Join(raiz, pasta))
			if err != nil || len(entradas) != 1 || entradas[0].IsDir() || entradas[0].Name() != filepath.Base(res.Arquivo) {
				t.Errorf("esperado só o UKI em %s, obtido %v (%v)", pasta, entradas, err)
			}
			for _, fora := range []string{"fuga.md", filepath.Join(".synapstor", "fuga.md")} {
				if _, err := os.Stat(filepath.Join(raiz, fora)); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("gravou fora da pasta dos UKIs: %s", fora)
				}
			}
		})
	}
}

// TestCapture_NaoSobrescreveUKI verifica que um UKI com o mesmo nome de um que já existe
// ganha um sufixo em vez de apagar o anterior.
func TestCapture_NaoSobrescreveUKI(t *testing.T) {
	raiz := t.TempDir()
	resp := `{"title": "Deploy", "filename": "UKI-1-deploy.md", "content": "# Novo", "summary": ""}`
	agt := NewAgent(&MockProvider{Response: resp}, raiz)
	primeiro, err := agt.Capture(context.Background(), "texto")
	if err != nil {
		t.Fatal(err)
	}
	segundo, err := agt.Capture(context.Background(), "texto")
	if err != nil {
		t.Fatal(err)
	}
	if primeiro.Arquivo == segundo.Arquivo || filepath.Base(segundo.Arquivo) != "UKI-1-deploy-2.md" {
		t.Errorf("esperado UKI-1-deploy-2.md no segundo, obtido %s e %s", primeiro.Arquivo, segundo.Arquivo)
	}
	for _, r := range []*Resultado{primeiro, segundo} {
		if _, err := os.Stat(filepath.Join(raiz, r.Arquivo)); err != nil {
			t.Errorf("UKI não gravado: %v", err)
		}
	}
}

func TestNomeDoUKI(t *testing.T) {
	casos := []struct{ nome, titulo, prefixo, sufixo string }{
		{"UKI-1-ok.md", "x", "UKI-1-ok.md", ""},
		{"UKI-1-sem-extensao", "x", "UKI-1-sem-extensao.md", ""},
		{"UKI-1-a/b.md", "x", "UKI-1-a-b.md", ""},
		{"documento.md", "CI/CD no GitLab", "UKI-", "-ci-cd-no-gitlab.md"},
		{"../..", "///", "UKI-", "-uki.md"},
		{"UKI-", "Título", "UKI-", ""},
	}
	for _, c := range casos {
		obtido := nomeDoUKI(c.nome, c.titulo)
		if !strings.HasPrefix(obtido, c.prefixo) || !strings.HasSuffix(obtido, c.sufixo) || strings.ContainsAny(obtido, `/\:`) {
			t.Errorf("nomeDoUKI(%q, %q) = %q", c.nome, c.titulo, obtido)
		}
	}
}

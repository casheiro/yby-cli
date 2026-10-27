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
	"time"

	"github.com/casheiro/yby-cli/pkg/ai"
	"github.com/casheiro/yby-cli/pkg/ai/prompts"
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
		"tipo": "regra",
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
	// o frontmatter da especificação vem antes do content; sem id no JSON, vale o do
	// formato antigo que a IA escreveu no content
	if !strings.HasPrefix(string(content), "---\nid: uki-mock-test\ntipo: regra\nstatus: rascunho\n") {
		t.Errorf("UKI sem o frontmatter da especificação: %s", string(content))
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
			err := validateResponse(&tt.resp, "referencia")
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
	err := validateResponse(&resp, "referencia")
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
	err := validateResponse(&resp, "referencia")
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
		Response: `{"title": "Corrigido", "tipo": "padrao", "filename": "UKI-123-corrigido.md", "content": "conteúdo corrigido", "summary": "resumo"}`,
	}

	// Simular resposta inválida na primeira chamada
	agent := NewAgent(mockProvider, tmpDir)

	// Usar diretamente saveResponse com JSON inválido (título vazio)
	invalidJson := `{"title": "", "filename": "UKI-123-teste.md", "content": "conteúdo", "summary": "resumo"}`
	res, err := agent.saveResponse(context.Background(), invalidJson, "", "synapstor.capture")
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
	// a causa na tela é em PT-BR; o "invalid character" do encoding/json fica no log
	if !errors.Is(err, ai.ErrRespostaNaoEJSON) || strings.Contains(ye.Cause.Error(), "invalid character") {
		t.Errorf("causa em PT-BR esperada, obtido %v", ye.Cause)
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
			b, _ := json.Marshal(SynapstorResponse{Title: c.titulo, Tipo: "regra", Filename: c.filename, Content: "# Fuga"})
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
	resp := `{"title": "Deploy", "tipo": "regra", "filename": "UKI-1-deploy.md", "content": "# Novo", "summary": ""}`
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

// TestCapture_GravaNoFormatoDaEspecificacao: o UKI gravado tem o frontmatter da
// especificação, com o tipo e as tags da IA, o status rascunho e as datas do dia vindas
// do código; o frontmatter que a IA escreveu no content não vira um segundo bloco.
func TestCapture_GravaNoFormatoDaEspecificacao(t *testing.T) {
	raiz := t.TempDir()
	resp := `{"title": "Saída em JSON", "id": "CLI.UX.Saída JSON", "tipo": "Decisão", "tags": ["CLI", "json"],
		"filename": "UKI-1-saida.md", "content": "---\nstatus: active\ncreated_at: 1999-01-01\n---\n# Saída em JSON\n\n## Problema\n", "summary": ""}`
	agt := NewAgent(&MockProvider{Response: resp}, raiz)
	agt.agora = func() time.Time { return time.Date(2026, 10, 26, 15, 0, 0, 0, time.Local) }
	res, err := agt.Capture(context.Background(), "texto")
	if err != nil {
		t.Fatal(err)
	}
	dados, err := os.ReadFile(filepath.Join(raiz, res.Arquivo))
	if err != nil {
		t.Fatal(err)
	}
	esperado := "---\nid: cli.ux.saida-json\ntipo: decisao\nstatus: rascunho\ncriado_em: 2026-10-26\n" +
		"atualizado_em: 2026-10-26\ntags: [cli, json]\n---\n\n# Saída em JSON\n\n## Problema\n"
	if string(dados) != esperado {
		t.Errorf("UKI fora do formato:\nesperado:\n%s\nobtido:\n%s", esperado, dados)
	}
}

// TestCapture_TipoForaDaListaPedeCorrecao: um tipo que a especificação não conhece leva à
// correção pela IA; o study sem tipo fica com referencia.
func TestCapture_TipoForaDaListaPedeCorrecao(t *testing.T) {
	resp := SynapstorResponse{Title: "A", Filename: "UKI-1-a.md", Content: "# A", Tipo: "conceito"}
	if err := validateResponse(&resp, ""); err == nil || !strings.Contains(err.Error(), "tipo") {
		t.Errorf("esperado erro de tipo, obtido %v", err)
	}
	resp = SynapstorResponse{Title: "A", Filename: "UKI-1-a.md", Content: "# A"}
	if err := validateResponse(&resp, ""); err == nil {
		t.Error("sem tipo e sem padrão, esperado erro")
	}
	resp = SynapstorResponse{Title: "A", Filename: "UKI-1-a.md", Content: "# A"}
	if err := validateResponse(&resp, "referencia"); err != nil || resp.Tipo != "referencia" {
		t.Errorf("esperado o tipo padrão, obtido %q (%v)", resp.Tipo, err)
	}
	// o tipo do formato antigo no content vale quando o JSON não traz
	resp = SynapstorResponse{Title: "A", Filename: "UKI-1-a.md", Content: "# A\n**Tipo:** Regra\n"}
	if err := validateResponse(&resp, ""); err != nil || resp.Tipo != "regra" {
		t.Errorf("esperado o tipo do content, obtido %q (%v)", resp.Tipo, err)
	}
}

// provedorQueGrava guarda os prompts de sistema de cada chamada e responde em sequência.
type provedorQueGrava struct {
	MockProvider
	respostas []string
	sistemas  []string
}

func (p *provedorQueGrava) Completion(ctx context.Context, sistema, usuario string) (string, error) {
	p.sistemas = append(p.sistemas, sistema)
	r := p.respostas[0]
	p.respostas = p.respostas[1:]
	return r, nil
}

// TestStudy_CorrecaoUsaOPromptDoStudy: a resposta inválida do study é corrigida com o prompt
// do próprio comando, não com o do capture.
func TestStudy_CorrecaoUsaOPromptDoStudy(t *testing.T) {
	raiz := t.TempDir()
	if err := os.WriteFile(filepath.Join(raiz, "bootstrap.go"), []byte("package bootstrap\n"), 0644); err != nil {
		t.Fatal(err)
	}
	prov := &provedorQueGrava{respostas: []string{
		`{"title": "", "filename": "UKI-1-x.md", "content": "c", "summary": "s"}`,
		`{"title": "Ok", "tipo": "referencia", "filename": "UKI-1-ok.md", "content": "# Ok", "summary": "s"}`,
	}}
	if _, err := NewAgent(prov, raiz).Study(context.Background(), "bootstrap"); err != nil {
		t.Fatalf("Study falhou: %v", err)
	}
	if len(prov.sistemas) != 2 {
		t.Fatalf("esperadas 2 chamadas, obtidas %d", len(prov.sistemas))
	}
	if !strings.HasPrefix(prov.sistemas[1], prompts.Get("synapstor.study")) {
		t.Error("a correção não usou o prompt synapstor.study")
	}
}

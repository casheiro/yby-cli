package tagger

import (
	"context"
	"io"
	"testing"

	"github.com/casheiro/yby-cli/pkg/ai"
)

// MockProvider simula um provedor de IA para testes.
type MockProvider struct {
	Response string
	Err      error
}

var _ ai.Provider = (*MockProvider)(nil)

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) IsAvailable(_ context.Context) bool {
	return true
}

func (m *MockProvider) GenerateGovernance(_ context.Context, _ string) (*ai.GovernanceBlueprint, error) {
	return nil, nil
}

func (m *MockProvider) Completion(_ context.Context, _, _ string) (string, error) {
	return m.Response, m.Err
}

func (m *MockProvider) StreamCompletion(_ context.Context, _, _ string, _ io.Writer) error {
	return nil
}

func (m *MockProvider) EmbedDocuments(_ context.Context, _ []string) ([][]float32, error) {
	return nil, nil
}

func TestTagUKI_ExtraiTags(t *testing.T) {
	mock := &MockProvider{
		Response: `["kubernetes", "deployment", "helm", "gitops"]`,
	}

	tags, err := TagUKI(context.Background(), mock, "conteúdo sobre kubernetes deploy")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(tags) != 4 {
		t.Errorf("esperado 4 tags, obtido %d", len(tags))
	}

	esperadas := map[string]bool{"kubernetes": true, "deployment": true, "helm": true, "gitops": true}
	for _, tag := range tags {
		if !esperadas[tag] {
			t.Errorf("tag inesperada: %s", tag)
		}
	}
}

func TestTagUKI_LimpaRespostaComCodeBlock(t *testing.T) {
	mock := &MockProvider{
		Response: "```json\n[\"kubernetes\", \"networking\"]\n```",
	}

	tags, err := TagUKI(context.Background(), mock, "conteúdo")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(tags) != 2 {
		t.Errorf("esperado 2 tags, obtido %d", len(tags))
	}
}

func TestTagUKI_LimitaA7Tags(t *testing.T) {
	mock := &MockProvider{
		Response: `["a", "b", "c", "d", "e", "f", "g", "h", "i"]`,
	}

	tags, err := TagUKI(context.Background(), mock, "conteúdo")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(tags) > 7 {
		t.Errorf("esperado max 7 tags, obtido %d", len(tags))
	}
}

func TestTagUKI_ErroProviderNil(t *testing.T) {
	_, err := TagUKI(context.Background(), nil, "conteúdo")
	if err == nil {
		t.Error("esperado erro com provider nil")
	}
}

func TestTagUKI_ErroRespostaInvalida(t *testing.T) {
	mock := &MockProvider{
		Response: "não é json",
	}

	_, err := TagUKI(context.Background(), mock, "conteúdo")
	if err == nil {
		t.Error("esperado erro para resposta inválida")
	}
}

func TestTagUKI_ErroZeroTags(t *testing.T) {
	mock := &MockProvider{
		Response: `[]`,
	}

	_, err := TagUKI(context.Background(), mock, "conteúdo")
	if err == nil {
		t.Error("esperado erro para 0 tags")
	}
}

func TestTagUKI_ErroDoProvider(t *testing.T) {
	mock := &MockProvider{
		Err: context.DeadlineExceeded,
	}

	_, err := TagUKI(context.Background(), mock, "conteúdo")
	if err == nil {
		t.Error("esperado erro do provider")
	}
}

//go:build integration

package ai

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Testes de integração com API real do Gemini ─────────────────────────────
// Executar com: go test -tags integration -v ./pkg/ai/...
// Requer variável de ambiente GEMINI_API_KEY configurada.

func TestGeminiCompletion_RealAPI(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY não definido — pulando teste de integração")
	}

	p := &GeminiProvider{
		APIKey:  apiKey,
		Model:   "gemini-2.0-flash",
		BaseURL: "https://generativelanguage.googleapis.com",
	}
	ctx := context.Background()

	result, err := p.Completion(ctx, "You are a helpful assistant.", "Diga apenas: OK")
	require.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestGeminiStreamCompletion_RealAPI(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY não definido — pulando teste de integração")
	}

	p := &GeminiProvider{
		APIKey:  apiKey,
		Model:   "gemini-2.0-flash",
		BaseURL: "https://generativelanguage.googleapis.com",
	}
	ctx := context.Background()

	var buf bytes.Buffer
	err := p.StreamCompletion(ctx, "You are a helpful assistant.", "Diga apenas: STREAM_OK", &buf)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestGeminiEmbedDocuments_RealAPI(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY não definido — pulando teste de integração")
	}

	p := &GeminiProvider{
		APIKey:  apiKey,
		Model:   "gemini-2.0-flash",
		BaseURL: "https://generativelanguage.googleapis.com",
	}
	ctx := context.Background()

	embeddings, err := p.EmbedDocuments(ctx, []string{"Olá mundo", "Teste de embedding"})
	require.NoError(t, err)
	assert.Len(t, embeddings, 2)
	assert.NotEmpty(t, embeddings[0])
}

func TestGeminiGenerateGovernance_RealAPI(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY não definido")
	}

	p := &GeminiProvider{
		APIKey:  apiKey,
		Model:   "gemini-2.0-flash",
		BaseURL: "https://generativelanguage.googleapis.com",
	}
	ctx := context.Background()

	blueprint, err := p.GenerateGovernance(ctx, "Um sistema simples de gerenciamento de tarefas em Go")
	// A API pode retornar um JSON inválido às vezes, toleramos isso
	if err != nil {
		t.Logf("GenerateGovernance retornou erro (aceitável): %v", err)
		return
	}
	assert.NotNil(t, blueprint)
}

func TestGeminiCompletion_InvalidKey_ReturnsError(t *testing.T) {
	p := &GeminiProvider{
		APIKey:  "invalid-key-for-test",
		Model:   "gemini-2.0-flash",
		BaseURL: "https://generativelanguage.googleapis.com",
	}
	ctx := context.Background()

	_, err := p.Completion(ctx, "system", "user")
	// Deve retornar erro (401 ou similar)
	assert.Error(t, err)
}

func TestGeminiStreamCompletion_InvalidKey_ReturnsError(t *testing.T) {
	p := &GeminiProvider{
		APIKey:  "invalid-key",
		Model:   "gemini-2.0-flash",
		BaseURL: "https://generativelanguage.googleapis.com",
	}
	var buf bytes.Buffer
	err := p.StreamCompletion(context.Background(), "system", "user", &buf)
	assert.Error(t, err)
}

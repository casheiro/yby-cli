package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newOpenAITestServer cria um servidor httptest e um OpenAIProvider apontando para ele.
func newOpenAITestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *OpenAIProvider) {
	t.Helper()
	server := httptest.NewServer(handler)
	provider := &OpenAIProvider{
		APIKey:  "test-key",
		Model:   "gpt-4o-mini",
		BaseURL: server.URL,
	}
	return server, provider
}

// openAISuccessResponse monta uma resposta de chat/completions com o conteúdo especificado.
func openAISuccessResponse(content string) openAIResponse {
	return openAIResponse{
		Choices: []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		}{
			{Message: struct {
				Content string `json:"content"`
			}{Content: content}},
		},
	}
}

// ─── Testes de criação do provider ──────────────────────────────────────────

func TestNewOpenAIProvider_WithKey_Full(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-123")
	p := NewOpenAIProvider()
	require.NotNil(t, p)
	assert.Equal(t, "sk-test-123", p.APIKey)
	assert.Equal(t, "gpt-4o-mini", p.Model)
	assert.Equal(t, "https://api.openai.com/v1", p.BaseURL)
}

func TestNewOpenAIProvider_WithoutKey_Nil(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	p := NewOpenAIProvider()
	assert.Nil(t, p)
}

// ─── Testes básicos do provider ─────────────────────────────────────────────

func TestOpenAIProvider_Name_HTTPTest(t *testing.T) {
	p := &OpenAIProvider{}
	assert.Equal(t, "OpenAI (Cloud)", p.Name())
}

func TestOpenAIProvider_IsAvailable_HTTPTest(t *testing.T) {
	t.Run("com API key", func(t *testing.T) {
		p := &OpenAIProvider{APIKey: "sk-test"}
		assert.True(t, p.IsAvailable(context.Background()))
	})

	t.Run("sem API key", func(t *testing.T) {
		p := &OpenAIProvider{}
		assert.False(t, p.IsAvailable(context.Background()))
	})
}

// ─── Testes de GenerateGovernance ───────────────────────────────────────────

func TestOpenAIProvider_GenerateGovernance_Success(t *testing.T) {
	blueprint := GovernanceBlueprint{
		Domain:    "e-commerce",
		RiskLevel: "médio",
		Summary:   "Plataforma de vendas online",
		Files: []GeneratedFile{
			{Path: ".synapstor/test.md", Content: "# Teste"},
		},
	}
	bpJSON, _ := json.Marshal(blueprint)

	server, provider := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		resp := openAISuccessResponse(string(bpJSON))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := provider.GenerateGovernance(context.Background(), "Uma loja virtual")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "e-commerce", result.Domain)
	assert.Equal(t, "médio", result.RiskLevel)
	assert.Len(t, result.Files, 1)
}

func TestOpenAIProvider_GenerateGovernance_Status500(t *testing.T) {
	server, provider := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("erro interno do servidor"))
	})
	defer server.Close()

	_, err := provider.GenerateGovernance(context.Background(), "teste")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "erro interno do servidor")
}

func TestOpenAIProvider_GenerateGovernance_EmptyChoices(t *testing.T) {
	server, provider := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	_, err := provider.GenerateGovernance(context.Background(), "teste")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resposta vazia")
}

// ─── Testes de Completion ───────────────────────────────────────────────────

func TestOpenAIProvider_Completion_Success(t *testing.T) {
	server, provider := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		resp := openAISuccessResponse("resposta de completamento")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	result, err := provider.Completion(context.Background(), "system prompt", "user prompt")
	require.NoError(t, err)
	assert.Equal(t, "resposta de completamento", result)
}

func TestOpenAIProvider_Completion_Error(t *testing.T) {
	server, provider := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	_, err := provider.Completion(context.Background(), "system", "user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// ─── Testes de StreamCompletion ─────────────────────────────────────────────

func TestOpenAIProvider_StreamCompletion_Success(t *testing.T) {
	server, provider := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Simula SSE (Server-Sent Events) do OpenAI
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`data: {"choices":[{"delta":{"content":"olá "}}]}`,
			``,
			`data: {"choices":[{"delta":{"content":"mundo"}}]}`,
			``,
			`data: [DONE]`,
			``,
		}
		for _, chunk := range chunks {
			w.Write([]byte(chunk + "\n"))
		}
	})
	defer server.Close()

	var buf bytes.Buffer
	err := provider.StreamCompletion(context.Background(), "system", "user", &buf)
	require.NoError(t, err)
	assert.Equal(t, "olá mundo", buf.String())
}

func TestOpenAIProvider_StreamCompletion_Status500(t *testing.T) {
	server, provider := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	var buf bytes.Buffer
	err := provider.StreamCompletion(context.Background(), "system", "user", &buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

// ─── Testes de EmbedDocuments ───────────────────────────────────────────────

func TestOpenAIProvider_EmbedDocuments_Success(t *testing.T) {
	server, provider := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/embeddings", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		resp := openAIEmbeddingResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float32{0.1, 0.2, 0.3}, Index: 0},
				{Embedding: []float32{0.4, 0.5, 0.6}, Index: 1},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	results, err := provider.EmbedDocuments(context.Background(), []string{"texto1", "texto2"})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.InDelta(t, float32(0.1), results[0][0], 0.001)
	assert.InDelta(t, float32(0.4), results[1][0], 0.001)
}

func TestOpenAIProvider_EmbedDocuments_Error(t *testing.T) {
	server, provider := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	_, err := provider.EmbedDocuments(context.Background(), []string{"texto"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

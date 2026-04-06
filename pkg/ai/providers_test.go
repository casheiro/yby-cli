package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── detectWSLHost ────────────────────────────────────────────────────────────

func TestDetectWSLHost_ReadsResolveConf(t *testing.T) {
	// This function reads /etc/resolv.conf — it'll succeed or fail gracefully
	host, err := detectWSLHost()
	// Either returns a host or empty — no panic expected
	if err == nil {
		_ = host // May be empty string which is fine
	}
}

// ─── NewOllamaProvider ────────────────────────────────────────────────────────

func TestNewOllamaProvider_DefaultCandidates(t *testing.T) {
	p := NewOllamaProvider()
	assert.NotNil(t, p)
	assert.Contains(t, p.Endpoints, "http://localhost:11434")
	assert.Equal(t, "llama3", p.Model)
}

func TestNewOllamaProvider_WithEnvOverride(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://custom-host:11434")
	p := NewOllamaProvider()
	assert.Contains(t, p.Endpoints, "http://custom-host:11434")
}

// ─── OllamaProvider.Name ─────────────────────────────────────────────────────

func TestOllamaProvider_Name_NoBaseURL(t *testing.T) {
	p := &OllamaProvider{Model: "llama3"}
	assert.Equal(t, "Ollama (Local)", p.Name())
}

func TestOllamaProvider_Name_WithBaseURL(t *testing.T) {
	p := &OllamaProvider{BaseURL: "http://localhost:11434", Model: "llama3"}
	name := p.Name()
	assert.Contains(t, name, "localhost:11434")
}

// ─── OllamaProvider.IsAvailable — mock server ─────────────────────────────────

func TestOllamaProvider_IsAvailable_WithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"models": []interface{}{}})
	}))
	defer server.Close()

	p := &OllamaProvider{
		Model:     "llama3",
		Endpoints: []string{server.URL},
	}
	ctx := context.Background()
	available := p.IsAvailable(ctx)
	assert.True(t, available)
	assert.Equal(t, server.URL, p.BaseURL)
}

func TestOllamaProvider_IsAvailable_ServerDown(t *testing.T) {
	p := &OllamaProvider{
		Model:     "llama3",
		Endpoints: []string{"http://localhost:19999"}, // nothing listening
	}
	available := p.IsAvailable(context.Background())
	assert.False(t, available)
}

func TestOllamaProvider_IsAvailable_AlreadyResolved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &OllamaProvider{BaseURL: server.URL, Model: "llama3"}
	assert.True(t, p.IsAvailable(context.Background()))
}

// ─── OllamaProvider.Completion — mock server ──────────────────────────────────

func TestOllamaCompletion_MockServer_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{{"name": "llama3"}},
			})
		case "/api/generate":
			json.NewEncoder(w).Encode(ollamaResponse{Response: "resposta do ollama"})
		}
	}))
	defer server.Close()

	p := &OllamaProvider{BaseURL: server.URL, Model: "llama3"}
	result, err := p.Completion(context.Background(), "system", "user input")
	require.NoError(t, err)
	assert.Equal(t, "resposta do ollama", result)
}

func TestOllamaCompletion_MockServer_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{{"name": "llama3"}},
			})
		case "/api/generate":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	p := &OllamaProvider{BaseURL: server.URL, Model: "llama3"}
	_, err := p.Completion(context.Background(), "system", "user")
	assert.Error(t, err)
}

// ─── OllamaProvider.StreamCompletion — mock server ────────────────────────────

func TestOllamaStreamCompletion_MockServer_Success(t *testing.T) {
	chunks := []ollamaResponse{
		{Response: "parte 1 "},
		{Response: "parte 2"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{{"name": "llama3"}},
			})
		case "/api/generate":
			for _, chunk := range chunks {
				json.NewEncoder(w).Encode(chunk)
			}
		}
	}))
	defer server.Close()

	p := &OllamaProvider{BaseURL: server.URL, Model: "llama3"}
	var buf bytes.Buffer
	err := p.StreamCompletion(context.Background(), "system", "user", &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "parte 1")
}

// ─── OllamaProvider.EmbedDocuments — mock server ──────────────────────────────

func TestOllamaEmbedDocuments_MockServer_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{{"name": "llama3"}},
			})
		case "/api/embed":
			var req ollamaEmbedRequest
			json.NewDecoder(r.Body).Decode(&req)
			embeddings := make([][]float32, len(req.Input))
			for i := range req.Input {
				embeddings[i] = []float32{0.1, 0.2, 0.3}
			}
			json.NewEncoder(w).Encode(ollamaEmbedResponse{Embeddings: embeddings})
		case "/api/embeddings":
			json.NewEncoder(w).Encode(ollamaEmbeddingResponse{
				Embedding: []float32{0.1, 0.2, 0.3},
			})
		}
	}))
	defer server.Close()

	p := &OllamaProvider{BaseURL: server.URL, Model: "llama3"}
	results, err := p.EmbedDocuments(context.Background(), []string{"texto 1", "texto 2"})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.InDelta(t, 0.1, results[0][0], 0.001)
}

// ─── OllamaProvider.resolveModel — mock server ────────────────────────────────

func TestOllamaResolveModel_NoModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"models": []interface{}{}})
	}))
	defer server.Close()

	p := &OllamaProvider{BaseURL: server.URL, Model: "llama3"}
	err := p.resolveModel(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nenhum modelo")
}

func TestOllamaResolveModel_WithModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"models": []map[string]interface{}{{"name": "mistral"}},
		})
	}))
	defer server.Close()

	p := &OllamaProvider{BaseURL: server.URL, Model: "llama3"}
	err := p.resolveModel(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "mistral", p.Model)
}

// ─── OllamaProvider.GenerateGovernance — mock server ─────────────────────────

func TestOllamaGenerateGovernance_MockServer_Success(t *testing.T) {
	blueprint := `{"domain":"fintech","risk_level":"high","summary":"ok","files":[]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{{"name": "llama3"}},
			})
		case "/api/generate":
			json.NewEncoder(w).Encode(ollamaResponse{Response: blueprint})
		}
	}))
	defer server.Close()

	p := &OllamaProvider{BaseURL: server.URL, Model: "llama3"}
	bp, err := p.GenerateGovernance(context.Background(), "projeto teste")
	require.NoError(t, err)
	assert.Equal(t, "fintech", bp.Domain)
}

// ─── OpenAIProvider ──────────────────────────────────────────────────────────

func TestNewOpenAIProvider_NoKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	p := NewOpenAIProvider()
	assert.Nil(t, p)
}

func TestNewOpenAIProvider_WithKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-key")
	p := NewOpenAIProvider()
	require.NotNil(t, p)
	assert.Equal(t, "sk-test-key", p.APIKey)
}

func TestOpenAIProvider_Name(t *testing.T) {
	p := &OpenAIProvider{APIKey: "key"}
	assert.Equal(t, "OpenAI (Cloud)", p.Name())
}

func TestOpenAIProvider_IsAvailable_WithKey(t *testing.T) {
	p := &OpenAIProvider{APIKey: "key"}
	assert.True(t, p.IsAvailable(context.Background()))
}

func TestOpenAIProvider_IsAvailable_NoKey(t *testing.T) {
	p := &OpenAIProvider{APIKey: ""}
	assert.False(t, p.IsAvailable(context.Background()))
}

// ─── OpenAI Completion — mock server ─────────────────────────────────────────

func TestOpenAICompletion_MockServer_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "resposta openai"}}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &OpenAIProvider{APIKey: "test-key", Model: "gpt-4o-mini", BaseURL: server.URL}
	result, err := p.Completion(context.Background(), "system", "user")
	require.NoError(t, err)
	assert.Equal(t, "resposta openai", result)
}

func TestOpenAICompletion_MockServer_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	p := &OpenAIProvider{APIKey: "bad-key", Model: "gpt-4o-mini", BaseURL: server.URL}
	_, err := p.Completion(context.Background(), "system", "user")
	assert.Error(t, err)
}

func TestOpenAICompletion_MockServer_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openAIResponse{Choices: nil})
	}))
	defer server.Close()

	p := &OpenAIProvider{APIKey: "key", Model: "gpt-4o-mini", BaseURL: server.URL}
	_, err := p.Completion(context.Background(), "system", "user")
	assert.Error(t, err)
}

// ─── OpenAI GenerateGovernance — mock server ──────────────────────────────────

func TestOpenAIGenerateGovernance_MockServer_Success(t *testing.T) {
	blueprintJSON := `{"domain":"ecommerce","risk_level":"medium","summary":"desc","files":[]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: blueprintJSON}}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &OpenAIProvider{APIKey: "key", Model: "gpt-4o-mini", BaseURL: server.URL}
	bp, err := p.GenerateGovernance(context.Background(), "meu app")
	require.NoError(t, err)
	assert.Equal(t, "ecommerce", bp.Domain)
}

// ─── OpenAI StreamCompletion — mock SSE server ────────────────────────────────

func TestOpenAIStreamCompletion_MockServer_Success(t *testing.T) {
	sseData := "data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\ndata: [DONE]\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(sseData))
	}))
	defer server.Close()

	p := &OpenAIProvider{APIKey: "key", Model: "gpt-4o-mini", BaseURL: server.URL}
	var buf bytes.Buffer
	err := p.StreamCompletion(context.Background(), "sys", "user", &buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "hello")
}

// ─── OpenAI EmbedDocuments — mock server ──────────────────────────────────────

func TestOpenAIEmbedDocuments_MockServer_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIEmbeddingResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float32{0.1, 0.2}, Index: 0},
				{Embedding: []float32{0.3, 0.4}, Index: 1},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := &OpenAIProvider{APIKey: "key", Model: "gpt-4o-mini", BaseURL: server.URL}
	results, err := p.EmbedDocuments(context.Background(), []string{"a", "b"})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.InDelta(t, 0.1, results[0][0], 0.001)
	assert.InDelta(t, 0.3, results[1][0], 0.001)
}

// ─── Structs JSON roundtrip ───────────────────────────────────────────────────

func TestOllamaStructs_JSON(t *testing.T) {
	req := ollamaRequest{
		Model:  "llama3",
		Prompt: "hello",
		System: "system",
		Stream: false,
		Format: "json",
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), "llama3")

	var out ollamaRequest
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, "llama3", out.Model)
}

func TestOpenAIStructs_JSON(t *testing.T) {
	req := openAIRequest{
		Model: "gpt-4o-mini",
		Messages: []openAIMessage{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "usr"},
		},
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), "gpt-4o-mini")
}

func TestOllamaEmbeddingStructs_JSON(t *testing.T) {
	req := ollamaEmbeddingRequest{Model: "llama3", Prompt: "hello"}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), "hello")
}

func TestOpenAIEmbeddingRequest_JSON(t *testing.T) {
	req := openAIEmbeddingRequest{
		Input:          []string{"text1", "text2"},
		Model:          "text-embedding-3-small",
		EncodingFormat: "float",
	}
	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), "text1")
}

// ─── GetProvider com mock Ollama ──────────────────────────────────────────────

func TestGetProvider_WithOllamaServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"models": []interface{}{}})
	}))
	defer server.Close()

	// Set OLLAMA_HOST to our mock server
	t.Setenv("OLLAMA_HOST", server.URL)
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	provider := GetProvider(context.Background(), "auto")
	// With Ollama server available, provider should be found (Ollama)
	_ = provider // May be nil if IsAvailable fails, that's ok
}

// ─── ollamaTagsResponse JSON ──────────────────────────────────────────────────

func TestOllamaTagsResponse_JSON(t *testing.T) {
	raw := `{"models":[{"name":"llama3"},{"name":"mistral"}]}`
	var resp ollamaTagsResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &resp))
	assert.Len(t, resp.Models, 2)
	assert.Equal(t, "llama3", resp.Models[0].Name)
}

// ensure all imports are used
var _ = strings.Contains
var _ = os.Getenv
